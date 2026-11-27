import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { screenshot, signInOperator, signInTenant } from '../support/console.mjs'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'

const profile = '/console/account/profile'
const transferPath = '/v1/tenant/commission-transfers'
const commissionsPath = '/v1/tenant/commissions'
const walletPath = '/v1/tenant/wallet'
const transferFilter = '/wallet/ledger?kind=commission_transfer'
const ratesPath = '/v1/operations/commission-rates'
const panel = page => page.getByRole('region', { name: '佣金明细', exact: true })
const amount = page => panel(page).getByRole('textbox', { name: /转入金额/ })
const submit = page => panel(page).getByRole('button', { name: '转入钱包', exact: true })
const retry = page => panel(page).getByRole('button', { name: '重试原提交', exact: true })
const success = page => expect(panel(page).getByText(/佣金已转入钱包/)).toBeVisible()
const isTransfer = request => new URL(request.url()).pathname === transferPath && request.method() === 'POST'
const transferResponse = page => page.waitForResponse(response => isTransfer(response.request()))

async function transfer(page, value) {
  await amount(page).fill(value)
  const response = transferResponse(page)
  await submit(page).click()
  return response
}

test('推广佣金转入钱包，双窗口并发、回执丢失重试及转入后消费', async ({
  page, browser, request, playwright, baseURL,
}, info) => {
  test.setTimeout(120_000)
  const operator = await authenticatedApi(playwright, baseURL)
  const contexts = []
  const accounts = []
  const priorRates = await operator.get(ratesPath).then(json)

  async function customer(name, referrer) {
    const email = 'transfer-ui-' + name + '@example.test'
    const { invite_code } = referrer ? await referrer.api.get('/v1/tenant/invite').then(json) : {}
    const { tenant_id: id } = await registerEmail(request, {
      email, invite: invite_code ? { link_invite_code: invite_code } : undefined,
    }).then(json)
    const api = await authenticatedApi(playwright, baseURL, { email })
    accounts.push(api)
    return { email, id, api }
  }

  async function window() {
    const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1000 } })
    contexts.push(context)
    return context.newPage()
  }

  try {
    const parent = await customer('parent')
    const promoter = await customer('promoter', parent)
    const buyer = await customer('buyer', promoter)
    const tier = await setTier(operator, '5gb', '4000')
    await setSale(operator, true)
    await operator.put(ratesPath, { data: { first_purchase_bps: 10000, repeat_purchase_bps: 10000 } }).then(json)
    await recharge(operator, buyer.api, '5000')
    const qualified = await buyer.api.post(purchasePath, {
      data: { package_id: '5gb', package_version: tier.version, request_key: randomUUID() },
    }).then(json)
    expect(qualified).toMatchObject({ status: 'succeeded', settled_minor: '4000' })
    await recharge(operator, promoter.api, '100')
    const opsTenantPath = '/v1/operations/tenants/' + promoter.id
    await operator.post(opsTenantPath + ':pause-promotion').then(json)
    const competing = await window()
    await Promise.all([signInTenant(page, promoter.email), signInTenant(competing, promoter.email)])
    await Promise.all([page.goto(profile), competing.goto(profile)])

    const writes = []
    page.on('request', req => { if (isTransfer(req)) writes.push(req.postDataJSON()) })
    await test.step('暂停期间显示已有佣金，拦截超额和三位小数', async () => {
      const share = page.getByRole('region', { name: '推广分享', exact: true })
      await expect(share.getByRole('definition').filter({ hasText: /^推广已暂停$/ })).toBeVisible()
      await page.route('**' + commissionsPath, route => route.fulfill({ status: 503 }), { times: 1 })
      await page.reload()
      await expect(submit(page)).toBeDisabled()
      await expect(amount(page)).toHaveAccessibleDescription('请先加载可用佣金，再转入钱包。')
      await panel(page).getByRole('button', { name: '重试', exact: true }).click()
      await expect(panel(page)).toContainText('¥40.00')
      await amount(page).fill('40.01')
      await submit(page).click()
      await expect(panel(page)).toContainText(/转入金额不能超过可用佣金/)
      await amount(page).fill('1.001')
      await submit(page).click()
      await expect(amount(page)).toHaveAccessibleDescription(/最多两位小数/)
      expect(writes).toHaveLength(0)
      await screenshot(page, info, 'commission-invalid-precision')
    })

    const competition = await test.step('隔离窗口发出不同键的真实并发提交，只入账一笔', async () => {
      await expect(panel(competing)).toContainText('¥40.00')
      const queued = []
      let release
      const bothReady = new Promise(resolve => { release = resolve })
      const waitForBoth = async route => {
        queued.push(route.request().postDataJSON())
        if (queued.length === 2) release()
        await bothReady
        await route.continue()
      }
      await Promise.all([page.route('**' + transferPath, waitForBoth), competing.route('**' + transferPath, waitForBoth)])
      const responses = await Promise.all([transfer(page, '25.50'), transfer(competing, '25.50')])
      await Promise.all([page.unroute('**' + transferPath, waitForBoth), competing.unroute('**' + transferPath, waitForBoth)])
      expect(queued.map(body => body.amount_minor)).toEqual(['2550', '2550'])
      expect(queued[0].request_key).not.toBe(queued[1].request_key)
      expect(responses.map(response => response.status()).sort()).toEqual([200, 409])
      const winner = responses.findIndex(response => response.status() === 200)
      const pages = [page, competing]
      await success(pages[winner])
      await expect(panel(pages[1 - winner])).toContainText(/可用佣金不足|转入金额不能超过可用佣金/)
      await Promise.all(pages.map(tab => expect(panel(tab)).toContainText('¥14.50')))
      const receipt = await responses[winner].json()
      expect(receipt).toMatchObject({ kind: 'commission_transfer', before_minor: '100', after_minor: '2650' })
      await screenshot(pages[winner], info, 'commission-concurrent-winner')
      const error = await responses[1 - winner].json()
      expect(error.reason).toBe('COMMISSION_BALANCE_INSUFFICIENT')
      expect((await promoter.api.get('/v1/tenant' + transferFilter).then(json)).entries).toEqual([receipt])
      return { queued, receipt, error }
    })

    async function switchAccount(email) {
      await page.getByRole('button', { name: '账号菜单' }).click()
      await page.getByRole('menuitem', { name: '退出登录' }).click()
      await page.getByRole('dialog').getByRole('button', { name: '退出登录' }).click()
      await expect(page).toHaveURL(/\/zh-CN\/login$/)
      await signInTenant(page, email)
      await page.goto(profile)
    }

    const replay = await test.step('服务端入账后在浏览器丢弃回执，重载后按原金额和键重试', async () => {
      let original
      let receipt
      await page.route('**' + transferPath, async route => {
        const response = await route.fetch({ maxRetries: 0, maxRedirects: 0 })
        original = route.request().postDataJSON()
        try {
          receipt = await json(response)
        } finally {
          await response.dispose()
          await route.abort('failed')
        }
      }, { times: 1 })
      await amount(page).fill('14.50')
      await submit(page).click()
      await expect(panel(page)).toContainText('暂未确认转入结果')
      await page.reload()
      await expect(panel(page)).toContainText('¥0.00')
      await expect(retry(page)).toBeVisible()
      await screenshot(page, info, 'commission-pending-after-reload')
      await switchAccount(buyer.email)
      await expect(panel(page)).toContainText('¥0.00')
      await expect(retry(page)).toHaveCount(0)
      await page.route('**' + commissionsPath, route => route.fulfill({ status: 503 }), { times: 1 })
      await switchAccount(promoter.email)
      await expect(panel(page).getByRole('button', { name: '重试', exact: true })).toBeVisible()
      await expect(retry(page)).toBeEnabled()
      const response = transferResponse(page)
      await retry(page).click()
      const repeated = await response
      expect(repeated.request().postDataJSON()).toEqual(original)
      expect(original.amount_minor).toBe('1450')
      expect(await json(repeated)).toEqual(receipt)
      expect(receipt).toMatchObject({ kind: 'commission_transfer', before_minor: '2650', after_minor: '4100' })
      await success(page)
      await expect(retry(page)).toHaveCount(0)
      return { original, receipt }
    })

    const ledger = await test.step('客户与只读运营按类型核对入账前后余额，累计充值仍为人工充值', async () => {
      const ops = await window()
      const opsLedger = '/ops/tenants/' + promoter.id + '/ledger'
      await signInOperator(ops, opsLedger, 'operator-readonly')
      await page.goto('/console/wallet')
      const expectedEntries = [replay.receipt, competition.receipt]
      for (const [view, label] of [[page, 'customer'], [ops, 'operator']]) {
        await view.getByRole('radiogroup', { name: '按类型筛选' }).getByText('佣金转入', { exact: true }).click()
        await expect(view).toHaveURL(/kind=commission_transfer/)
        const rows = view.getByRole('row').filter({ has: view.getByRole('cell', { name: '佣金转入', exact: true }) })
        await expect(rows).toHaveCount(2)
        const newest = rows.filter({ has: view.getByText(replay.receipt.id, { exact: true }) })
        for (const display of ['+¥14.50', '¥26.50', '¥41.00']) await expect(newest).toContainText(display)
        await screenshot(view, info, 'commission-' + label + '-ledger')
      }
      await expect(page.getByRole('region', { name: '累计充值' })).toContainText('¥1.00')
      expect(await operator.get(opsTenantPath + transferFilter).then(json)).toMatchObject({ entries: expectedEntries })

      await page.setViewportSize({ width: 390, height: 844 })
      await page.getByRole('button', { name: /^语言[:：]/ }).click()
      await page.getByRole('menuitem', { name: 'English', exact: true }).click()
      const mobileLedger = page.getByRole('region', { name: 'Wallet activity', exact: true })
      const kinds = mobileLedger.getByRole('radiogroup')
      const mobileFilters = [
        ...['All', 'Top-up', 'Commission transfer', 'Charge', 'Refund'].map(name => kinds.getByText(name, { exact: true })),
        mobileLedger.getByPlaceholder('Start date', { exact: true }),
        mobileLedger.getByPlaceholder('End date', { exact: true }),
      ]
      const toolbar = await mobileLedger.locator('.wallet-ledger__toolbar').boundingBox()
      expect(toolbar).not.toBeNull()
      for (const filter of mobileFilters) {
        await expect(filter).toBeVisible()
        const box = await filter.boundingBox()
        expect(box.x).toBeGreaterThanOrEqual(toolbar.x)
        expect(box.x + box.width).toBeLessThanOrEqual(toolbar.x + toolbar.width)
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(390)
      await screenshot(page, info, 'commission-english-mobile-wallet')
      await page.getByRole('button', { name: /^Language:/ }).click()
      await page.getByRole('menuitem', { name: '简体中文', exact: true }).click()
      await page.setViewportSize({ width: 1440, height: 1000 })
      return expectedEntries
    })

    const purchase = await test.step('浏览器使用转入余额购买，订单、钱包与直接推荐佣金一致', async () => {
      await setTier(operator, '5gb', '3500')
      await page.getByRole('menuitem', { name: '动态住宅代理-GB' }).click()
      await page.getByRole('tab', { name: '价格', exact: true }).click()
      await page.getByRole('radio', { name: '5 GB 流量包', exact: true }).click()
      await page.getByRole('button', { name: '立即购买' }).click()
      const dialog = page.getByRole('dialog', { name: '确认购买流量' })
      const result = page.waitForResponse(response => new URL(response.url()).pathname === purchasePath && response.request().method() === 'POST')
      await dialog.getByRole('button', { name: '确认支付' }).click()
      const order = await json(await result)
      expect(order).toMatchObject({ kind: 'traffic_package', status: 'succeeded', settled_minor: '3500', debited_minor: '3500' })
      await expect(page.getByRole('status').filter({ hasText: '订单 #' + order.id })).toContainText('实扣 ¥35.00')
      expect(await promoter.api.get('/v1/tenant/orders/' + order.id).then(json)).toMatchObject({
        id: order.id, status: 'succeeded', settled_minor: '3500', settled_at: order.settled_at,
      })
      const wallet = await promoter.api.get(walletPath).then(json)
      expect(wallet).toMatchObject({ available_minor: '600', total_recharge_minor: '100', total_spend_minor: '3500' })
      const { entries } = await promoter.api.get(walletPath + '/ledger').then(json)
      expect(entries.slice(0, 3)).toEqual([expect.objectContaining({ kind: 'debit', order_id: order.id,
        before_minor: '4100', after_minor: '600' }), ...ledger])
      const commissions = await parent.api.get(commissionsPath).then(json)
      expect(commissions).toMatchObject({ available_minor: '3500', records: [expect.objectContaining({
        product_kind: 'traffic_package', status: 'credited', classification: 'first_purchase',
        commission_minor: '3500', net_debit_minor: '3500', settled_at: order.settled_at,
      })] })
      expect(await promoter.api.get(commissionsPath).then(json)).toMatchObject({ available_minor: '0' })
      await screenshot(page, info, 'commission-funded-purchase')
      return { order, wallet, entries, commissions }
    })

    await info.attach('commission-transfer-browser-evidence', {
      body: JSON.stringify({ competition, replay, ledger, purchase }, null, 2), contentType: 'application/json',
    })
  } finally {
    try {
      await setSale(operator, false)
      await operator.put(ratesPath, { data: {
        first_purchase_bps: priorRates.first_purchase_bps, repeat_purchase_bps: priorRates.repeat_purchase_bps,
      } }).then(json)
    } finally {
      await Promise.all([...contexts.map(context => context.close()), ...accounts.map(api => api.dispose()), operator.dispose()])
    }
  }
})
