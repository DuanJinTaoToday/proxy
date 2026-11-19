import { test, expect } from '@playwright/test'
import { findTenant, login, prepareRecharge, screenshot, submitRecharge } from '../support/console.mjs'
import { expectTrafficPurchaseRecords, openTrafficPackages, openTrafficSale, openWalletOrders, trafficSaleSettings } from '../support/traffic.mjs'

test('流量套餐开售 → 余额不足 → 充值后扣款并到账', async ({ browser, page: tenant, baseURL }, info) => {
  const operatorContext = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
  const operator = await operatorContext.newPage()
  const nav = tenant.getByRole('navigation', { name: '租户导航' })
  const packages = nav.getByRole('link', { name: '流量套餐', exact: true })
  const submit = tenant.getByRole('button', { name: '确认并扣款', exact: true })
  const package5GB = tenant.getByRole('radio', { name: '5 GB ¥15.00', exact: true })
  let orderId

  try {
    await test.step('运营定价并单独开启动态套餐购买', async () => {
      await login(operator, '/operations', '运营账号', 'operator')
      const sale = await trafficSaleSettings(operator)
      const close = sale.getByRole('button', { name: '关闭购买', exact: true })
      if (await close.isVisible()) {
        await close.click()
        await expect(sale.getByRole('heading', { name: /^动态.* · 未开售$/ })).toBeVisible()
      }

      await login(tenant, '/console', '手机号', '13800000002')
      await expect(tenant.getByRole('heading', { name: '快捷操作', exact: true })).toBeVisible()
      await nav.getByRole('button', { name: '动态住宅代理-GB', exact: true }).click()
      await expect(packages).toHaveCount(0)

      await openTrafficSale(operator, sale)
      await expect(operator.getByRole('heading', { name: /^静态.* · 未开售$/ })).toBeVisible()
      await screenshot(operator, info, '01-dynamic-on-sale')
    })

    await test.step('租户购买时提示差额，刷新后仍无订单、余额或流量变化', async () => {
      await tenant.reload()
      await openTrafficPackages(nav)
      await package5GB.check()
      await submit.click()
      await expect(tenant.getByRole('alert')).toHaveText('钱包余额不足，还差 ¥15.00。请联系运营充值后重新确认。')
      await screenshot(tenant, info, '02-insufficient-wallet')
      await tenant.reload()
      await openWalletOrders(tenant, nav)
      await expect(tenant.getByText('暂无符合条件的订单。', { exact: true })).toBeVisible()
      await nav.getByRole('link', { name: '仪表板', exact: true }).click()
      await expect(tenant.getByRole('main').getByText('¥0.00', { exact: true })).toBeVisible()
      await expect(tenant.getByRole('main').getByText(/^0(?:\.0+)? (?:B|GB)$/)).toBeVisible()
    })

    await test.step('运营给同一租户充值 ¥20.00，租户重新确认购买', async () => {
      await findTenant(operator, '13800000002')
      await expect(operator.getByRole('region', { name: 'Beta', exact: true }).getByText('localbeta', { exact: true })).toBeVisible()
      await prepareRecharge(operator, '20.00', 'verify-traffic-' + process.env.VERIFY_ID)
      await submitRecharge(operator)

      await tenant.reload()
      await openTrafficPackages(nav)
      await expect(tenant.getByText(/钱包余额 ¥20\.00 · 可用流量/)).toBeVisible()
      await package5GB.check()
      await submit.click()
      const receipt = tenant.getByRole('status').filter({ hasText: /订单 #/ })
      const completedOrder = /订单 #(\d+) 成交，到账 5 GB，实扣 ¥15\.00。/
      await expect(receipt).toHaveText(completedOrder)
      orderId = completedOrder.exec(await receipt.innerText())[1]
      await screenshot(tenant, info, '03-purchase-receipt')
    })

    await test.step('刷新后核对余额、唯一订单和关联扣款流水', async () => {
      await nav.getByRole('link', { name: '仪表板', exact: true }).click()
      await tenant.reload()
      await expect(tenant.getByRole('main').getByText('¥5.00', { exact: true })).toBeVisible()
      await expect(tenant.getByRole('main').getByText('5 GB', { exact: true })).toBeVisible()
      await screenshot(tenant, info, '04-balances-after-refresh')

      await expectTrafficPurchaseRecords(tenant, nav, orderId, '¥5.00')
      await screenshot(tenant, info, '05-linked-wallet-debit')
    })
  } catch (error) {
    if (!operator.isClosed()) await screenshot(operator, info, 'operator-at-failure')
    throw error
  } finally {
    await operatorContext.close()
  }
})
