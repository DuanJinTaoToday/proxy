import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { field, screenshot, signInOperator, signInTenant } from '../support/console.mjs'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'

const profile = '/console/account/profile'
const customerCommissions = '/v1/tenant/commissions'
const commissionRates = '/v1/operations/commission-rates'
const tenantPage = id => '/ops/tenants/' + id + '/commissions'
const tenantPath = id => '/v1/operations/tenants/' + id
const ratePayload = (first, repeat) => ({ first_purchase_bps: first, repeat_purchase_bps: repeat })
const calls = (page, path, method = 'GET') => page.waitForResponse(response =>
  new URL(response.url()).pathname === path && response.request().method() === method)
const detail = page => page.getByRole('region', { name: '佣金明细', exact: true })
const customerRow = (page, key) => detail(page).getByRole('row').filter({ hasText: key })
const ratesSection = page => page.getByRole('region', { name: '全站佣金费率', exact: true })
const saveButton = page => ratesSection(page).getByRole('button', { name: '保存费率', exact: true })
const money = (major, minor) => new RegExp('[¥￥]' + major + '\\.' + minor)

async function operatorHeaders(request, username) {
  const { access_token } = await request.post('/v1/operations/access/login', {
    data: { username, password: 'proxy-local-test' },
  }).then(json)
  return { Authorization: 'Bearer ' + access_token }
}

async function buyOrRenewStatic(tenant, orderKind, selection) {
  const quote = await tenant.post('/v1/tenant/static-' + orderKind + '-quotes', { data: selection }).then(json)
  return tenant.post('/v1/tenant/static-' + (orderKind === 'purchase' ? '' : 'renewal-') + 'orders', {
    data: {
      ...selection,
      ...(orderKind === 'purchase' ? { unit_minor: quote.offer.unit_minor } : {}),
      confirmation_token: quote.confirmation_token,
      request_key: randomUUID(),
    },
  }).then(json)
}

async function search(page, key, count) {
  const panel = detail(page)
  await field(panel, '匿名客户标识').fill(key)
  const queried = calls(page, customerCommissions)
  await panel.getByRole('button', { name: '查询', exact: true }).click()
  const response = await queried
  expect(response.ok()).toBe(true)
  const query = new URL(response.url()).searchParams
  expect(query.get('customer_key')?.toLowerCase()).toBe(key.trim().toLowerCase())
  expect(['0', null]).toContain(query.get('before_id'))
  await expect(panel.getByRole('row').filter({ has: page.getByRole('cell') })).toHaveCount(count)
  return response.json()
}

async function expectCellsInView(panel, row, labels) {
  await expect(row).toBeVisible()
  const area = await panel.boundingBox()
  for (const label of labels) {
    const cell = row.getByRole('cell', { name: label, exact: true })
    await expect(cell).toBeVisible()
    const box = await cell.boundingBox()
    expect(box.x, label).toBeGreaterThanOrEqual(area.x)
    expect(box.x + box.width, label).toBeLessThanOrEqual(area.x + area.width)
  }
}

test('推广员从业务订单查三类佣金，计费运营改费率后历史快照仍可核对', async ({
  page, browser, baseURL, request, playwright,
}, info) => {
  test.setTimeout(120_000)
  const promoterEmail = 'commission-ui-promoter@example.test'
  const buyerEmail = 'commission-ui-buyer@example.test'
  const { tenant_id: promoterId } = await registerEmail(request, { email: promoterEmail }).then(json)
  const promoter = await authenticatedApi(playwright, baseURL, { email: promoterEmail })
  const { invite_code } = await promoter.get('/v1/tenant/invite').then(json)
  const { tenant_id: buyerId } = await registerEmail(request, { email: buyerEmail, invite: { invite_code } }).then(json)
  const buyer = await authenticatedApi(playwright, baseURL, { email: buyerEmail })
  const operator = await authenticatedApi(playwright, baseURL)
  const billing = await operatorHeaders(request, 'operator-billing')
  const support = await operatorHeaders(request, 'operator-support')
  const previousRates = await request.get(commissionRates, { headers: billing }).then(json)
  const staticSale = open => operator.put('/v1/operations/product-sales/static', {
    data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'Issue #127 local Crown UI' } : {}) },
  }).then(json)
  const testContexts = []
  async function newPage() {
    const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
    testContexts.push(context)
    return context.newPage()
  }
  async function saveRates(first, repeat, payload) {
    await field(ops, '首购费率').fill(first)
    await field(ops, '复购费率').fill(repeat)
    const saved = calls(ops, commissionRates, 'PUT')
    await saveButton(ops).click()
    const response = await saved
    expect(response.request().postDataJSON()).toEqual(payload)
    expect(response.ok()).toBe(true)
    const result = await response.json()
    expect(result).toMatchObject({ ...payload, operator_id: 'local-operator-billing' })
    return result
  }

  const ops = await newPage()
  try {
    await signInOperator(ops, '/ops/tenants/' + promoterId, 'operator-billing')
    await ops.getByRole('tab', { name: '推广佣金', exact: true }).click()
    await expect(ops).toHaveURL(baseURL + tenantPage(promoterId))
    await expect(ratesSection(ops)).toBeVisible()
    const rateWrites = []
    ops.on('request', req => {
      if (new URL(req.url()).pathname === commissionRates && req.method() === 'PUT') rateWrites.push(req.postDataJSON())
    })
    const firstField = field(ops, '首购费率')
    const repeatField = field(ops, '复购费率')
    for (const value of ['100.01', '-0.01', '12.345', '1e1', '']) {
      await firstField.fill(value)
      await repeatField.fill('12.34')
      if (await saveButton(ops).isEnabled()) await saveButton(ops).click()
      await expect(firstField).toHaveAttribute('aria-invalid', 'true')
      expect(rateWrites).toHaveLength(0)
    }
    const boundary = await saveRates('0', '100', ratePayload(0, 10000))
    const rates = await saveRates('0.01', '12.34', ratePayload(1, 1234))
    expect(rates.version).not.toBe(boundary.version)
    expect(rateWrites).toEqual([ratePayload(0, 10000), ratePayload(1, 1234)])

    const tier = await setTier(operator, '5gb', '1500')
    await setSale(operator, true)
    await staticSale(true)
    await recharge(operator, buyer, '9000')
    const dynamic = await buyer.post(purchasePath, {
      data: { package_id: tier.id, package_version: tier.version, request_key: randomUUID() },
    }).then(json)
    expect(dynamic).toMatchObject({ status: 'succeeded', settled_minor: '1500' })
    const fixed = await buyOrRenewStatic(buyer, 'purchase', { node_id: '101', term: 'yp1', quantity: 1 })
    expect(fixed).toMatchObject({ status: 'succeeded', settled_minor: '1000' })
    const { allocations } = await buyer.get('/v1/tenant/static-ips').then(json)
    const nextRates = await saveRates('100.00', '50.01', ratePayload(10000, 5001))
    expect(nextRates.version).not.toBe(rates.version)
    await request.post(tenantPath(promoterId) + ':pause-promotion', { headers: support }).then(json)
    const renewal = await buyOrRenewStatic(buyer, 'renewal', {
      allocation_ids: [allocations.find(row => row.order_id === fixed.id).allocation_id], term: 'yp1',
    })
    expect(renewal).toMatchObject({ status: 'succeeded', settled_minor: '1000' })

    await signInTenant(page, promoterEmail)
    await page.goto(profile)
    await expect(page.getByRole('button', { name: '复制邀请码', exact: true })).toBeVisible()
    const pageData = await promoter.get(customerCommissions).then(json)
    expect(pageData).toMatchObject({ available_minor: '123', records: [
      expect.objectContaining({ product_kind: 'static_renewal', status: 'promotion_paused', rate_bps: 0,
        rate_version: nextRates.version, settled_at: renewal.settled_at, commission_minor: '0' }),
      expect.objectContaining({ product_kind: 'static_purchase', status: 'credited', rate_bps: 1234,
        rate_version: rates.version, settled_at: fixed.settled_at, commission_minor: '123' }),
      expect.objectContaining({ product_kind: 'traffic_package', status: 'zero_commission', rate_bps: 1,
        rate_version: rates.version, settled_at: dynamic.settled_at, commission_minor: '0' }),
    ] })
    const key = pageData.records[0].customer_key
    const rows = customerRow(page, key)
    await expect(rows).toHaveCount(3)
    const dynamicRow = rows.filter({ hasText: /动态/ })
    await expect(dynamicRow).toContainText(/手填|手动/)
    await expect(dynamicRow).toContainText('首购')
    await expect(dynamicRow).toContainText('零佣金')
    await expect(dynamicRow).toContainText(money(15, '00'))
    await expect(dynamicRow).toContainText(money(0, '00'))
    const fixedRow = rows.filter({ hasText: /静态.*(购买|采购)/ })
    await expect(fixedRow).toContainText('复购')
    await expect(fixedRow).toContainText('已入账')
    await expect(fixedRow).toContainText(money(10, '00'))
    await expect(fixedRow).toContainText(money(1, '23'))
    await expect(fixedRow).toContainText('12.34%')
    await expect(fixedRow).toContainText(new Intl.DateTimeFormat('zh-CN', {
      timeZone: 'Asia/Shanghai', hourCycle: 'h23', hour: '2-digit', minute: '2-digit',
    }).format(new Date(fixed.settled_at)))
    const renewalRow = rows.filter({ hasText: /续费/ })
    await expect(renewalRow).toContainText('复购')
    await expect(renewalRow).toContainText('推广暂停')
    await expect(renewalRow).toContainText(money(0, '00'))
    await expect(detail(page)).not.toContainText(buyerEmail)
    await expect(detail(page)).not.toContainText(buyerId)
    expect(JSON.stringify(pageData)).not.toContain('"order_id"')
    await screenshot(page, info, 'desktop-promoter-three-products')

    await page.setViewportSize({ width: 390, height: 844 })
    const emptyKey = randomUUID()
    expect(await search(page, emptyKey, 0)).toMatchObject({ records: [], available_minor: '123' })
    await expect(detail(page)).toContainText(money(1, '23'))
    expect((await search(page, ' ' + key.toUpperCase() + ' ', 3)).records).toHaveLength(3)
    const cleared = calls(page, customerCommissions)
    await detail(page).getByRole('button', { name: '清空', exact: true }).click()
    expect(new URL((await cleared).url()).searchParams.has('customer_key')).toBe(false)
    await expect(rows).toHaveCount(3)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
    await expectCellsInView(detail(page), fixedRow, ['静态住宅购买', '¥1.23', '已入账'])
    await screenshot(page, info, 'mobile-promoter-commission')

    const outsider = await newPage()
    await signInTenant(outsider, buyerEmail)
    await outsider.goto(profile)
    await expect(detail(outsider)).toContainText(money(0, '00'))
    expect(await search(outsider, key, 0)).toMatchObject({ records: [], available_minor: '0' })
    expect((await buyer.get(tenantPath(promoterId) + '/commissions')).status()).toBe(403)

    for (const username of ['operator-readonly', 'operator-support']) {
      const reader = await newPage()
      await signInOperator(reader, tenantPage(promoterId), username)
      await expect(reader).toHaveURL(baseURL + tenantPage(promoterId))
      await expect(customerRow(reader, key)).toHaveCount(3)
      await expect(customerRow(reader, key).filter({ hasText: /静态.*(购买|采购)/ })).toContainText('12.34%')
      await expect(ratesSection(reader)).toContainText(/50\.01/)
      const edit = saveButton(reader)
      if (await edit.count()) await expect(edit).toBeDisabled()
      const headers = await operatorHeaders(request, username)
      expect((await request.put(commissionRates, { headers, data: ratePayload(99, 99) })).status()).toBe(403)
      if (username === 'operator-readonly') {
        await screenshot(reader, info, 'read-only-operator')
        await reader.setViewportSize({ width: 390, height: 844 })
        expect(await reader.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
        await expectCellsInView(detail(reader), customerRow(reader, key).filter({ hasText: /静态.*购买/ }),
          ['静态住宅购买', '¥1.23', '已入账'])
        await screenshot(reader, info, 'mobile-read-only-operator')
      }
      await reader.goto(tenantPage(buyerId))
      await expect(detail(reader)).toContainText(money(0, '00'))
      await expect(customerRow(reader, key)).toHaveCount(0)
    }
    await ops.reload()
    await expect(customerRow(ops, key)).toHaveCount(3)
    await expect(field(ops, '复购费率')).toHaveValue('50.01')
    await expect(customerRow(ops, key).filter({ hasText: /静态.*(购买|采购)/ })).toContainText('12.34%')
    await expectCellsInView(detail(ops), customerRow(ops, key).filter({ hasText: /静态.*购买/ }),
      ['静态住宅购买', '¥1.23', '已入账', key, '手填', '复购', '¥10.00'])
    await screenshot(ops, info, 'billing-operator-current-and-historical-rates')
    await info.attach('commission-settlement-evidence', {
      body: JSON.stringify({ boundary, rates, nextRates, pageData }, null, 2),
      contentType: 'application/json',
    })
  } finally {
    try {
      await Promise.all([
        setSale(operator, false), staticSale(false),
        request.put(commissionRates, { headers: billing, data: ratePayload(
          previousRates.first_purchase_bps, previousRates.repeat_purchase_bps,
        ) }).then(json),
      ])
    } finally {
      await Promise.all(testContexts.map(context => context.close()))
      await Promise.all([promoter.dispose(), buyer.dispose(), operator.dispose()])
    }
  }
})

test('佣金历史用服务端游标翻页，失败可重试，切换语言保留账目', async ({
  page, request, playwright, baseURL,
}, info) => {
  test.setTimeout(120_000)
  const promoterEmail = 'commission-page-promoter@example.test'
  const buyerEmail = 'commission-page-buyer@example.test'
  await registerEmail(request, { email: promoterEmail }).then(json)
  const promoter = await authenticatedApi(playwright, baseURL, { email: promoterEmail })
  const { invite_code } = await promoter.get('/v1/tenant/invite').then(json)
  await registerEmail(request, { email: buyerEmail, invite: { link_invite_code: invite_code } }).then(json)
  const buyer = await authenticatedApi(playwright, baseURL, { email: buyerEmail })
  const operator = await authenticatedApi(playwright, baseURL)
  const billing = await operatorHeaders(request, 'operator-billing')
  const previousRates = await request.get(commissionRates, { headers: billing }).then(json)

  try {
    await request.put(commissionRates, { headers: billing, data: ratePayload(100, 100) }).then(json)
    const tier = await setTier(operator, '5gb', '100')
    await setSale(operator, true)
    await recharge(operator, buyer, '3000')
    for (let i = 0; i < 26; i++) {
      await buyer.post(purchasePath, {
        data: { package_id: tier.id, package_version: tier.version, request_key: randomUUID() },
      }).then(json)
    }
    const { available_minor, next_before_id, records } = await promoter.get(customerCommissions).then(json)
    expect(available_minor).toBe('26')
    expect(records).toHaveLength(25)
    expect(next_before_id).toBe(records.at(-1).id)
    const key = records[0].customer_key
    await signInTenant(page, promoterEmail)
    await page.goto(profile)
    const panel = detail(page)
    await expect(customerRow(page, key)).toHaveCount(25)
    const filter = field(panel, '匿名客户标识')
    await filter.fill(key)
    const refreshed = calls(page, customerCommissions)
    await panel.getByRole('button', { name: '查询', exact: true }).click()
    expect(new URL((await refreshed).url()).searchParams.get('customer_key')).toBe(key)
    await expect(customerRow(page, key)).toHaveCount(25)

    let failed = false
    await page.route('**/v1/tenant/commissions?**', async route => {
      const query = new URL(route.request().url()).searchParams
      if (query.get('before_id') === next_before_id && !failed) {
        failed = true
        await route.fulfill({ status: 503, json: { reason: 'TEMPORARILY_UNAVAILABLE' } })
      } else {
        await route.continue()
      }
    })
    const more = panel.getByRole('button', { name: '加载更旧明细', exact: true })
    const lost = calls(page, customerCommissions)
    await more.click()
    expect((await lost).status()).toBe(503)
    await expect(customerRow(page, key)).toHaveCount(25)
    const retry = panel.getByRole('button', { name: /重试/ })
    await expect(retry).toBeVisible()
    const recovered = calls(page, customerCommissions)
    await retry.click()
    const response = await recovered
    const query = new URL(response.url()).searchParams
    expect(query.get('before_id')).toBe(next_before_id)
    expect(query.get('customer_key')).toBe(key)
    expect(await response.json()).toMatchObject({
      available_minor: '26', next_before_id: '0',
      records: [expect.objectContaining({ referral_source: 'link', classification: 'first_purchase', customer_key: key })],
    })
    await expect(customerRow(page, key).filter({ hasText: '首购' })).toContainText('链接')
    await expect(more).toBeHidden()
    await page.getByRole('button', { name: /^语言[:：]/ }).click()
    await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await expect(page).toHaveURL(baseURL + profile)
    await expect(page.getByRole('textbox', { name: /anonymous customer/i })).toHaveValue(key)
    await expect(page.getByRole('row').filter({ hasText: key }).filter({ hasText: /first purchase/i })).toBeVisible()
    await screenshot(page, info, 'english-filtered-commission-history')
  } finally {
    try {
      await Promise.all([
        setSale(operator, false),
        request.put(commissionRates, { headers: billing, data: ratePayload(
          previousRates.first_purchase_bps, previousRates.repeat_purchase_bps,
        ) }).then(json),
      ])
    } finally {
      await Promise.all([promoter.dispose(), buyer.dispose(), operator.dispose()])
    }
  }
})

test('账目和费率的空回执显示加载失败，重试后恢复运营权限内操作', async ({ page, baseURL }) => {
  const records = tenantPath('alpha') + '/commissions'
  for (const path of [commissionRates, records]) {
    await page.route('**' + path, route => route.fulfill({ json: {} }), { times: 1 })
  }
  await signInOperator(page, tenantPage('alpha'), 'operator-billing')
  await expect(page).toHaveURL(baseURL + tenantPage('alpha'))
  const rates = ratesSection(page)
  const ledger = detail(page)
  await expect(rates.getByText('无法加载全站佣金费率', { exact: true })).toBeVisible()
  await expect(ledger.getByText('无法加载佣金明细', { exact: true })).toBeVisible()
  await expect(saveButton(page)).toHaveCount(0)
  await expect(ledger).not.toContainText(/可用佣金|暂无佣金记录/)

  const loadedRates = calls(page, commissionRates)
  await rates.getByRole('button', { name: '重试', exact: true }).click()
  expect((await loadedRates).ok()).toBe(true)
  await expect(saveButton(page)).toBeEnabled()

  const accounts = calls(page, records)
  await ledger.getByRole('button', { name: '重试', exact: true }).click()
  const { available_minor } = await (await accounts).json()
  expect(available_minor).toBe('0')
  await expect(ledger).toContainText(money(0, '00'))
})
