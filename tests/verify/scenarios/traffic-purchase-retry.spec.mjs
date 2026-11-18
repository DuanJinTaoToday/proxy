import { test, expect } from '@playwright/test'
import { writeFile } from 'node:fs/promises'
import { findTenant, login, prepareRecharge, screenshot, submitRecharge } from '../support/console.mjs'
import { expectTrafficPurchaseRecords, openTrafficSale, trafficSaleSettings } from '../support/traffic.mjs'

const purchasePath = '/v1/tenant/orders/traffic-packages'
const purchaseURL = url => url.pathname === purchasePath

test('购买结果未知 → 重试原提交 → 只扣款和到账一次', async ({ browser, page: tenant, baseURL }, info) => {
  const operatorContext = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
  const nav = tenant.getByRole('navigation', { name: '租户导航' })
  const wallet = tenant.getByText(/^钱包余额 .* · 可用流量 /)
  const receipt = tenant.getByRole('status').filter({ hasText: /订单 #/ })
  const retryButton = tenant.getByRole('button', { name: '重试原提交', exact: true })

  try {
    const { funded, remaining } = await test.step('准备 5 GB 套餐及足够支付两次的测试余额', async () => {
      const operator = await operatorContext.newPage()
      await login(operator, '/operations', '运营账号', 'operator')
      await openTrafficSale(operator, await trafficSaleSettings(operator))
      await findTenant(operator, '13800000001')
      await expect(operator.getByRole('region', { name: 'Alpha', exact: true }).getByText('localalpha', { exact: true })).toBeVisible()
      await prepareRecharge(operator, '40.00', 'verify-retry-' + process.env.VERIFY_ID)
      await submitRecharge(operator)

      await login(tenant, '/console', '手机号', '13800000001')
      await nav.getByRole('link', { name: '流量套餐', exact: true }).click()
      const startingWallet = /^钱包余额 ¥([\d,]+)\.(\d{2}) · 可用流量 0(?:\.0+)? (?:B|GB)$/
      await expect(wallet).toHaveText(startingWallet)
      const [, yuan, cents] = startingWallet.exec(await wallet.innerText())
      const wholeYuan = BigInt(yuan.replaceAll(',', ''))
      expect(wholeYuan >= 30n, '钱包必须足够扣两次 ¥15.00').toBe(true)
      await tenant.getByRole('radio', { name: '5 GB ¥15.00', exact: true }).check()
      return {
        funded: '¥' + yuan + '.' + cents,
        remaining: '¥' + (wholeYuan - 15n).toLocaleString('zh-CN') + '.' + cents,
      }
    })

    const first = await test.step('让真实服务成交，再丢弃发给浏览器的回执', async () => {
      let serverReceipt
      await tenant.route(purchaseURL, async route => {
        try {
          const request = route.request()
          const response = await route.fetch({ maxRetries: 0, maxRedirects: 0 })
          serverReceipt = {
            method: request.method(), payload: request.postDataJSON(),
            status: response.status(), order: await response.json(),
          }
          await response.dispose()
        } finally {
          await route.abort('failed')
        }
      }, { times: 1 })

      await tenant.getByRole('button', { name: '确认并扣款', exact: true }).click()
      await expect(tenant.getByRole('alert')).toHaveText('暂时无法确认成交结果，请用原请求重试。')
      await expect(receipt).toHaveCount(0)
      await expect(retryButton).toBeEnabled()
      expect(serverReceipt).toMatchObject({
        method: 'POST',
        status: 200,
        payload: { package_id: '5gb', package_version: expect.stringMatching(/^\d+$/), request_key: expect.stringMatching(/\S/) },
        order: {
          id: expect.stringMatching(/^[1-9]\d*$/), kind: 'traffic_package', status: 'succeeded',
          debited_minor: '1500', delivered_bytes: '5000000000',
        },
      })
      await screenshot(tenant, info, '01-browser-missed-receipt')
      return serverReceipt
    })

    const orderId = first.order.id
    const settledWallet = '钱包余额 ' + remaining + ' · 可用流量 5 GB'
    const retryStatus = await test.step('刷新后确认已扣款到账，再让浏览器重试原提交', async () => {
      await tenant.reload()
      await expect(wallet).toHaveText(settledWallet)
      await expect(retryButton).toBeEnabled()
      await screenshot(tenant, info, '02-committed-before-retry')
      const [response] = await Promise.all([
        tenant.waitForResponse(response => response.request().method() === 'POST' && purchaseURL(new URL(response.url()))),
        retryButton.click(),
      ])
      expect(response.request().postDataJSON()).toEqual(first.payload)
      expect(response.status()).toBe(200)
      expect(await response.json()).toMatchObject({
        id: orderId, status: 'succeeded', debited_minor: '1500', delivered_bytes: '5000000000',
      })
      await expect(receipt).toHaveText('订单 #' + orderId + ' 成交，到账 5 GB，实扣 ¥15.00。')
      await expect(retryButton).toHaveCount(0)
      await screenshot(tenant, info, '03-retry-receipt')
      return response.status()
    })

    await test.step('重新读取余额，核对唯一订单及唯一扣款流水', async () => {
      await tenant.reload()
      await expect(wallet).toHaveText(settledWallet)
      await expectTrafficPurchaseRecords(tenant, nav, orderId, remaining)
      await screenshot(tenant, info, '04-single-wallet-debit')
    })

    const path = info.outputPath('purchase-retry.json')
    await writeFile(path, JSON.stringify({
      tenant: 'Alpha', orderId,
      purchase: first.payload,
      firstResponse: { status: first.status, deliveredToBrowser: false },
      retryResponse: { status: retryStatus, reusedOriginalPayload: true, sameOrderId: true },
      before: { wallet: funded, trafficBytes: '0' },
      afterLostReceipt: { wallet: remaining, trafficBytes: '5000000000' },
      afterRetry: { wallet: remaining, trafficBytes: '5000000000', orders: 1, debits: 1 },
    }, null, 2) + '\n')
    await info.attach('purchase-retry', { path, contentType: 'application/json' })
  } finally {
    await operatorContext.close()
  }
})
