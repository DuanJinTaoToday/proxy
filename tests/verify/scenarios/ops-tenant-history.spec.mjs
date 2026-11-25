import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { screenshot, signInOperator } from '../support/console.mjs'
import { authenticatedApi, json, purchasePath, setSale, setTier } from '../support/traffic.mjs'
import { registerEmail } from '../support/mail.mjs'

const rechargeReason = '对公转账到账，回单尾号 3381'

test.use({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })

async function select(page, name, option) {
  await page.getByRole('combobox', { name, exact: true }).press('Enter')
  await page.getByRole('option', { name: option, exact: true }).click()
}

test('运营按租户筛选和分页，核对订单交付、退款和钱包流水', async ({ page, request, playwright, baseURL }, info) => {
  const prototypeURL = process.env.VERIFY_OPERATOR_PROTOTYPE
  const email = prototypeURL ? 'chenxl-studio@example.test' : 'tenant-13900008801@example.test'
  const { tenant_id: tenantId } = prototypeURL ? { tenant_id: 'visual-recharge-tenant' }
    : await json(await registerEmail(request, { email, password: 'proxy-local-test' }))
  const tenantPage = '/ops/tenants/' + tenantId
  const opsPath = '/v1/operations/tenants/' + tenantId
  const ops = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email })
  async function buyTraffic(tier) {
    return json(await tenant.post(purchasePath, {
      data: { package_id: tier.id, package_version: tier.version, request_key: randomUUID() },
    }))
  }

  try {
    await setTier(ops, '5gb', '1500')
    const fifty = await setTier(ops, '50gb', '18000')
    await setSale(ops, true)
    await json(await ops.put('/v1/operations/product-sales/static', {
      data: { open: true, quote_ready: true, acceptance_evidence: 'Issue #88 isolated supplier fixture' },
    }))
    const request_key = randomUUID()
    const { confirmation_token } = await json(await ops.post(opsPath + '/wallet/recharge-quotes', {
      data: { request_key, amount_minor: '50000', reason: rechargeReason },
    }))
    const recharge = await json(await ops.post(opsPath + '/wallet/recharges', {
      data: { request_key, confirmation_token },
    }))
    const traffic = await buyTraffic(fifty)
    const selection = { node_id: '101', term: 'yp1', quantity: 3 }
    await json(await request.post(new URL('/fixture/catalog', process.env.VERIFY_IPCROWN_URL).href, {
      data: { price: 10, stock: 6 },
    }))
    const quote = await json(await tenant.post('/v1/tenant/static-purchase-quotes', { data: selection }))
    const order = await json(await tenant.post('/v1/tenant/static-orders', {
      data: { ...selection, unit_minor: quote.offer.unit_minor, confirmation_token: quote.confirmation_token, request_key: randomUUID() },
    }))
    const resolved = await json(await ops.post(opsPath + '/orders/' + order.id + '/static-resolution', {
      data: { evidence: 'Fixture confirmed remainder undeliverable', close_remaining: true, confirmed_undeliverable: true },
    }))
    expect(resolved).toMatchObject({ status: 'partial', delivered_count: '2', locked_minor: '3000', refunded_minor: '1000', settled_minor: '2000' })
    const pendingQuote = await json(await tenant.post('/v1/tenant/static-purchase-quotes', { data: selection }))
    const pending = await json(await tenant.post('/v1/tenant/static-orders', {
      data: { ...selection, unit_minor: pendingQuote.offer.unit_minor, confirmation_token: pendingQuote.confirmation_token, request_key: randomUUID() },
    }))
    expect(pending).toMatchObject({ status: 'pending' })
    const ledger = await json(await ops.get(opsPath + '/wallet/ledger'))
    const refund = ledger.entries.find(entry => entry.order_id === order.id && entry.kind === 'release')
    const beforeMinor = prototypeURL ? '51900' : '29000'
    const afterMinor = prototypeURL ? '52900' : '30000'
    expect(refund).toMatchObject({ before_minor: beforeMinor, after_minor: afterMinor })

    await signInOperator(page, tenantPage)
    const tenantDetails = await json(await ops.get(opsPath))
    await expect(page.getByRole('heading', { name: tenantDetails.name, exact: true })).toBeVisible()
    await expect(page.getByRole('tab', { name: /^订单/ })).toHaveAttribute('aria-selected', 'true')
    await expect(page.getByLabel('1 笔订单确认中')).toBeVisible()
    const pendingRow = page.getByRole('row').filter({ has: page.getByRole('button', { name: pending.id, exact: true }) })
    await expect(pendingRow.getByText(/已等待 [\d,]+ 小时/)).toBeVisible()
    await expect(page.getByText('共 3 条', { exact: true })).toBeVisible()
    await expect(page.getByText('50 GB 流量套餐', { exact: true })).toBeVisible()
    await expect(page.getByText('已退 ¥10.00', { exact: true })).toBeVisible()
    await screenshot(page, info, '03-tenant-orders')
    await page.getByRole('button', { name: traffic.id, exact: true }).click()
    const drawer = page.getByRole('dialog', { name: '订单详情' })
    await expect(drawer.getByText('50 GB', { exact: true })).toBeVisible()
    await expect(drawer.getByText('¥180.00', { exact: true }).first()).toBeVisible()
    await screenshot(page, info, '03-tenant-orders-detail')
    await page.keyboard.press('Escape')
    await expect(drawer).toBeHidden()

    await select(page, '按类型筛选', '静态 IP 购买')
    await select(page, '按状态筛选', '部分交付')
    await expect(page.getByRole('button', { name: order.id, exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: traffic.id, exact: true })).toHaveCount(0)
    await select(page, '按状态筛选', '失败')
    await expect(page.getByText('没有符合条件的订单')).toBeVisible()
    await screenshot(page, info, '03-tenant-orders-empty')
    await page.getByRole('button', { name: '清除筛选' }).click()
    const search = page.getByRole('textbox', { name: '搜索订单号' })
    await search.fill('9999999999999999999')
    await search.press('Enter')
    await expect(page.getByText('没有符合条件的订单')).toBeVisible()
    await expect(page).toHaveURL(/orderQuery=9999999999999999999/)
    await search.fill(traffic.id)
    await search.press('Enter')
    await expect(page.getByRole('button', { name: traffic.id, exact: true })).toBeVisible()
    await search.fill('')
    await search.press('Enter')

    await page.getByRole('tab', { name: '钱包流水', exact: true }).click()
    await expect(page).toHaveURL(new RegExp(tenantPage + '/ledger$'))
    const refundRow = page.getByRole('row').filter({ has: page.getByText(refund.id, { exact: true }) }).filter({ hasText: '释放预扣' })
    await expect(refundRow).toContainText('+¥10.00')
    await expect(refundRow).toContainText(prototypeURL ? '¥519.00 → ¥529.00' : '¥290.00 → ¥300.00')
    const rechargeRow = page.getByRole('row').filter({ has: page.getByText(recharge.id, { exact: true }) }).filter({ hasText: rechargeReason })
    await expect(rechargeRow).toContainText(rechargeReason)
    await expect(rechargeRow).toContainText('operator')
    await expect(rechargeRow).toContainText('+¥500.00')
    await expect(page.getByRole('row').filter({ hasText: '¥180.00' }).getByText(traffic.id, { exact: true })).toBeVisible()
    await screenshot(page, info, '04-tenant-ledger')
    await refundRow.getByRole('button', { name: order.id, exact: true }).click()
    await expect(drawer.getByText('#' + order.id, { exact: true })).toBeVisible()
    await expect(drawer.getByText('2 个 IP', { exact: true })).toBeVisible()
    const total = label => drawer.locator('dl > div').filter({ has: page.getByText(label, { exact: true }) }).locator('dd')
    await expect(total('锁定总价')).toHaveText('¥30.00')
    await expect(total('预扣金额')).toHaveText('¥30.00')
    await expect(total('实付')).toHaveText('¥20.00')
    await expect(total('已退款')).toHaveText('¥10.00')
    await expect(drawer.getByText('+¥10.00', { exact: true })).toBeVisible()
    await expect(drawer.getByText('流水 #' + refund.id, { exact: false })).toBeVisible()
    await screenshot(page, info, '04-tenant-ledger-linked-order')
    await page.keyboard.press('Escape')
    await page.getByText('充值', { exact: true }).first().click()
    await expect(rechargeRow).toBeVisible()
    await expect(refundRow).toHaveCount(0)
    await page.goto(tenantPage + '/ledger?start=2000-01-01&end=2000-01-02')
    await expect(page.getByText('没有符合条件的流水')).toBeVisible()
    await page.getByRole('button', { name: '清除筛选', exact: true }).click()
    await expect(page).toHaveURL(new RegExp(tenantPage + '/ledger$'))
    await expect(rechargeRow).toBeVisible()

    const { tenant_id: emptyId } = await json(await registerEmail(request, { email: 'tenant-13900008802@example.test', password: 'proxy-local-test' }))
    const emptyTenant = await json(await ops.get('/v1/operations/tenants/' + emptyId))
    await page.goto('/ops/tenants/' + emptyId + '?order=' + order.id)
    await expect(page.getByRole('heading', { name: emptyTenant.name, exact: true })).toBeVisible()
    await expect(drawer.getByText('未找到该租户或钱包记录。')).toBeVisible()
    await expect(drawer.getByText('#' + order.id, { exact: true })).toHaveCount(0)
    await expect(drawer.getByText('¥30.00', { exact: true })).toHaveCount(0)
    await screenshot(page, info, '03-tenant-orders-mismatched-detail')
    await page.keyboard.press('Escape')
    await page.getByRole('tab', { name: '钱包流水', exact: true }).click()
    await expect(page.getByText('没有符合条件的流水')).toBeVisible()
    await screenshot(page, info, '04-tenant-ledger-empty')

    // Add real purchases after the visual capture so the screenshots remain comparable.
    const five = await setTier(ops, '5gb', '100')
    for (let i = 0; i < 25; i++) await buyTraffic(five)
    await page.goto(tenantPage + '?kind=traffic_package&status=succeeded')
    await expect(page.getByRole('button', { name: '下一页' })).toBeEnabled()
    await expect(page.getByRole('button', { name: traffic.id, exact: true })).toHaveCount(0)
    await page.getByRole('button', { name: '下一页' }).click()
    await expect(page.getByText('第 2 页')).toBeVisible()
    await expect(page.getByRole('button', { name: traffic.id, exact: true })).toBeVisible()
    await page.getByRole('button', { name: '上一页' }).click()
    await expect(page.getByText('第 1 页')).toBeVisible()

    await page.getByRole('tab', { name: '钱包流水', exact: true }).click()
    await expect(page.getByRole('button', { name: '下一页' })).toBeEnabled()
    await page.getByRole('button', { name: '下一页' }).click()
    await expect(page.getByText('第 2 页')).toBeVisible()
    await expect(page.getByText(rechargeReason)).toBeVisible()
    await page.getByRole('button', { name: '上一页' }).click()
    await expect(page.getByText('第 1 页')).toBeVisible()
    await page.getByRole('button', { name: /^语言/ }).click()
    await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    await expect(page).toHaveURL(new RegExp(tenantPage + '/ledger$'))
    await expect(page.getByRole('tab', { name: 'Wallet activity', exact: true })).toBeVisible()
    await expect(page.getByRole('columnheader', { name: 'Before → after' })).toBeVisible()
    await screenshot(page, info, '04-tenant-ledger-en')

    if (prototypeURL) {
      await page.goto(prototypeURL + '#tenants/t-1001')
      await expect(page.getByRole('heading', { name: '陈小岚工作室' })).toBeVisible()
      await screenshot(page, info, '03-tenant-orders-prototype')
      await page.getByRole('row').filter({ hasText: '50 GB' }).first().click()
      await expect(page.getByRole('dialog')).toBeVisible()
      await screenshot(page, info, '03-tenant-orders-detail-prototype')
      await page.keyboard.press('Escape')
      await select(page, '按类型筛选', '静态 IP 续费')
      await select(page, '按状态筛选', '已完成')
      await expect(page.getByText('没有符合条件的订单')).toBeVisible()
      await screenshot(page, info, '03-tenant-orders-empty-prototype')
      await page.getByRole('tab', { name: '钱包流水', exact: true }).click()
      await expect(page.getByRole('columnheader', { name: '变动前 → 后' })).toBeVisible()
      await screenshot(page, info, '04-tenant-ledger-prototype')
      await page.getByRole('row').filter({ hasText: '退款' }).getByRole('button').first().click()
      await expect(page.getByRole('dialog')).toBeVisible()
      await screenshot(page, info, '04-tenant-ledger-linked-order-prototype')
      await page.keyboard.press('Escape')
      await page.goto(prototypeURL + '#tenants/t-1005')
      await page.getByRole('tab', { name: '钱包流水', exact: true }).click()
      await expect(page.getByText('没有符合条件的流水')).toBeVisible()
      await screenshot(page, info, '04-tenant-ledger-empty-prototype')
    }
  } finally {
    await setSale(ops, false)
    await json(await ops.put('/v1/operations/product-sales/static', { data: { open: false } }))
    await ops.dispose()
    await tenant.dispose()
  }
})
