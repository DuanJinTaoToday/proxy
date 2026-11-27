import { randomUUID } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, test } from '@playwright/test'
import { screenshot, signInTenant } from '../support/console.mjs'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, recharge } from '../support/traffic.mjs'

const exchanges = '/v1/tenant/static-ip-exchanges'
const quotaPath = '/v1/tenant/static-ip-exchange-quota'
const ips = '/v1/tenant/static-ips'
const sale = '/v1/operations/product-sales/static'
const rowFor = (page, ip) => page.getByRole('row').filter({ hasText: ip }).first()
const cta = (page, name) => page.getByRole('button', { name, exact: true })
const exchangeAction = row => row.getByRole('button', { name: '换号', exact: true })
const urlPath = req => new URL(req.url()).pathname

function fixture(request, data) {
  return request.post(new URL('/fixture/exchanges', process.env.VERIFY_IPCROWN_URL).href, { data }).then(json)
}

async function prepare({ request, playwright, baseURL }, email) {
  const { tenant_id: tenantId } = await registerEmail(request, { email }).then(json)
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email })
  await recharge(operator, tenant, '30000')
  await operator.put(sale, { data: { open: true, quote_ready: true, acceptance_evidence: 'issue #125 local Crown fixture' } }).then(json)
  const list = () => tenant.get(ips).then(json).then(body => body.allocations)
  return {
    quota: () => tenant.get(quotaPath).then(json),
    list,
    wallet: () => Promise.all([
      tenant.get('/v1/tenant/wallet').then(json),
      tenant.get('/v1/tenant/wallet/ledger').then(json),
    ]),
    record: key => tenant.get(exchanges + '/' + key).then(json),
    config: id => tenant.get(ips + '/' + id + '/proxy-config').then(json),
    adjust: delta => operator.post('/v1/operations/tenants/' + tenantId + '/static-ip-exchange-quota:adjust', {
      data: { delta, request_key: randomUUID(), evidence: 'issue #125 local quota boundary' },
    }).then(json),
    async buy(selection) {
      const quote = await tenant.post('/v1/tenant/static-purchase-quotes', { data: selection }).then(json)
      const order = await tenant.post('/v1/tenant/static-orders', { data: {
        ...selection, request_key: randomUUID(), unit_minor: quote.offer.unit_minor, confirmation_token: quote.confirmation_token,
      } }).then(json)
      return (await list()).filter(row => row.order_id === order.id)
    },
    async renew(allocationId) {
      const selection = { allocation_ids: [allocationId], term: 'yp1' }
      const quote = await tenant.post('/v1/tenant/static-renewal-quotes', { data: selection }).then(json)
      return tenant.post('/v1/tenant/static-renewal-orders', {
        data: { ...selection, request_key: randomUUID(), confirmation_token: quote.confirmation_token },
      }).then(json)
    },
    async close() {
      try {
        await operator.put(sale, { data: { open: false } }).then(json)
      } finally {
        await Promise.all([operator.dispose(), tenant.dispose()])
      }
    },
  }
}

async function openExchange(page, allocation) {
  const row = rowFor(page, allocation.ip)
  await exchangeAction(row).click()
  const dialog = page.getByRole('dialog').filter({ visible: true })
  await expect(cta(dialog, '确认换号')).toBeVisible()
  await expect(dialog.getByText(/同地区/).filter({ visible: true }).first()).toBeVisible()
  await expect(dialog.getByText(/免费/).filter({ visible: true }).first()).toBeVisible()
  return dialog
}

function submission(page) {
  return page.waitForRequest(req => req.method() === 'POST' && urlPath(req) === exchanges)
}

test('独享换号后复制和导出新连接参数，从原分配继续续费', async ({ page, context, request, playwright, baseURL }, info) => {
  const email = 'exchange-ui-success@example.test'
  const setup = await prepare({ request, playwright, baseURL }, email)
  try {
    const [allocation] = await setup.buy({ node_id: '101', term: 'yp1', quantity: 1 })
    const [shared] = await setup.buy({ node_id: '301', term: 'yp1', quantity: 1, share_mode: 'shared', purpose_id: 10 })
    const before = await setup.wallet()
    const upstream = await fixture(request, { ip: allocation.ip, outcome: 'ready' })
    await signInTenant(page, email)
    await page.goto('/console/static/ips')
    await expect(rowFor(page, shared.ip)).toBeVisible()
    const unavailable = exchangeAction(rowFor(page, shared.ip))
    if (await unavailable.isVisible()) await expect(unavailable).toBeDisabled()
    expect(await setup.quota()).toEqual({ remaining: 10, reserved: 0, used: 0 })

    await page.setViewportSize({ width: 390, height: 844 })
    const dialog = await openExchange(page, allocation)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
    expect(await setup.quota()).toEqual({ remaining: 10, reserved: 0, used: 0 })
    await screenshot(page, info, 'mobile-same-region-free-exchange-confirmation')
    const sent = submission(page)
    await cta(dialog, '确认换号').click()
    const payload = (await sent).postDataJSON()
    expect(payload).toMatchObject({ allocation_id: allocation.allocation_id, node_id: allocation.node_id, idempotency_key: expect.any(String) })
    await expect.poll(() => setup.record(payload.idempotency_key).then(record => record.status), { timeout: 20000 }).toBe('ready')
    const latest = await setup.config(allocation.allocation_id)
    expect(latest).toMatchObject({ allocation_id: allocation.allocation_id, port: 1081 })
    expect(latest.ip).not.toBe(allocation.ip)
    await expect(dialog).toContainText(latest.ip, { timeout: 15000 })
    await expect(rowFor(page, latest.ip)).toBeVisible()
    expect(await setup.wallet()).toEqual(before)
    expect(await setup.quota()).toEqual({ remaining: 9, reserved: 0, used: 1 })

    await page.setViewportSize({ width: 1440, height: 1000 })
    await page.reload()
    const changed = rowFor(page, latest.ip)
    await expect(changed).toBeVisible()
    await expect(rowFor(page, allocation.ip)).toHaveCount(0)
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    await changed.getByRole('button', { name: /^复制/ }).click()
    const expected = [latest.ip, latest.port, latest.username, latest.password].join(':')
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(expected)
    await changed.getByRole('checkbox').press('Space')
    const file = page.waitForEvent('download')
    const exportRequest = page.waitForRequest(req => urlPath(req) === ips + ':export')
    await cta(page, '导出 TXT').click()
    expect((await exportRequest).postDataJSON()).toEqual({ allocation_ids: [allocation.allocation_id] })
    expect((await readFile(await (await file).path(), 'utf8')).trim()).toBe(expected)
    await screenshot(page, info, 'desktop-replaced-ip-after-reload')

    await cta(changed, '续费').click()
    const renewal = page.getByRole('dialog').filter({ visible: true })
    const quoteRequest = page.waitForRequest(req => urlPath(req) === '/v1/tenant/static-renewal-quotes')
    await cta(renewal, '获取报价').click()
    expect((await quoteRequest).postDataJSON()).toMatchObject({ allocation_ids: [allocation.allocation_id] })
    const renewalResponse = page.waitForResponse(resp => urlPath(resp) === '/v1/tenant/static-renewal-orders' && resp.ok())
    await cta(renewal, '确认续费').click()
    const order = await (await renewalResponse).json()
    expect(order).toMatchObject({ status: 'succeeded' })
    expect(await fixture(request, { id: upstream.id })).toMatchObject({ calls: 1, renewals: 1 })
    expect(await setup.config(allocation.allocation_id)).toMatchObject({ ip: latest.ip, port: latest.port })
    await info.attach('exchange-success-evidence', { body: JSON.stringify({ order: order.id, allocation: allocation.allocation_id, quota: await setup.quota(), originalDetailRenewals: 1 }), contentType: 'application/json' })

    await request.post(new URL('/fixture/renewals/delay', process.env.VERIFY_IPCROWN_URL).href, {
      data: { ip: latest.ip },
    }).then(json)
    expect(await setup.renew(allocation.allocation_id)).toMatchObject({ status: 'pending' })
    await page.reload()
    await expect(exchangeAction(changed)).toBeDisabled()
    expect(await fixture(request, { id: upstream.id })).toMatchObject({ calls: 1, renewals: 2 })
  } finally {
    await setup.close()
  }
})

test('换号丢回执时复用请求键，待核实与续费互斥，额度用完仍能查原请求', async ({ page, request, playwright, baseURL }, info) => {
  const email = 'exchange-ui-retry@example.test'
  const setup = await prepare({ request, playwright, baseURL }, email)
  try {
    const items = await setup.buy({ node_id: '101', term: 'yp1', quantity: 2 })
    expect(items).toHaveLength(2)
    const [first, second] = items
    const mocked = await fixture(request, { ip: first.ip, outcome: 'unknown-empty' })
    await setup.adjust(-9)
    const before = await setup.wallet()
    await signInTenant(page, email)
    await page.goto('/console/static/ips')
    const dialog = await openExchange(page, first)
    let lostPayload
    await page.route('**/v1/tenant/static-ip-exchanges', async route => {
      lostPayload = route.request().postDataJSON()
      const response = await route.fetch()
      expect(response.ok()).toBe(true)
      await route.abort('failed')
    }, { times: 1 })
    await cta(dialog, '确认换号').click()
    await expect(cta(page, '重试原请求').filter({ visible: true }).first()).toBeVisible()
    expect(lostPayload).toMatchObject({ allocation_id: first.allocation_id, idempotency_key: expect.any(String) })
    const retry = cta(page, '重试原请求').filter({ visible: true }).first()
    const failedRefresh = page.waitForEvent('requestfailed', req => req.method() === 'GET' && urlPath(req) === ips)
    await page.route('**' + ips, route => route.abort('failed'), { times: 1 })
    const secondRequest = submission(page)
    await retry.click()
    expect((await secondRequest).postDataJSON()).toEqual(lostPayload)
    await expect.poll(() => setup.record(lostPayload.idempotency_key).then(record => record.status)).toBe('unknown')
    await expect(dialog).toContainText('待核实')
    await failedRefresh
    await cta(dialog, '完成').click()
    await expect(cta(page, '查询原结果').filter({ visible: true }).first()).toBeVisible()
    await page.reload()
    await expect(rowFor(page, first.ip)).toContainText('待核实')
    await expect(cta(rowFor(page, first.ip), '续费')).toBeDisabled()
    const originalLookup = page.waitForRequest(req => req.method() === 'GET' && urlPath(req) === exchanges + '/' + lostPayload.idempotency_key)
    await rowFor(page, first.ip).getByRole('button', { name: /查询原结果|查看换号|换号结果/ }).first().click()
    await originalLookup
    await expect(page.getByRole('dialog').filter({ visible: true })).toContainText('待核实')
    const next = exchangeAction(rowFor(page, second.ip))
    if (await next.isVisible()) await expect(next).toBeDisabled()
    expect(await setup.quota()).toEqual({ remaining: 0, reserved: 1, used: 0 })
    expect(await setup.wallet()).toEqual(before)
    expect(await fixture(request, { id: mocked.id })).toMatchObject({ calls: 1, renewals: 0 })
    await screenshot(page, info, 'unknown-exchange-zero-quota-renewal-disabled')
  } finally {
    await setup.close()
  }
})

test('换号请求超时且刷新后查无记录，仍可沿用原请求键提交', async ({ page, request, playwright, baseURL }, info) => {
  const email = 'exchange-ui-reload@example.test'
  const setup = await prepare({ request, playwright, baseURL }, email)
  const held = Promise.withResolvers()
  try {
    const [allocation] = await setup.buy({ node_id: '111', term: 'yp1', quantity: 1 })
    const upstream = await fixture(request, { ip: allocation.ip, outcome: 'ready' })
    await page.clock.install()
    await signInTenant(page, email)
    await page.goto('/console/static/ips')
    const dialog = await openExchange(page, allocation)
    await page.route('**/v1/tenant/static-ip-exchanges', async route => {
      await held.promise
      await route.abort('failed').catch(() => {})
    }, { times: 1 })
    const firstRequest = submission(page)
    await cta(dialog, '确认换号').click()
    const original = (await firstRequest).postDataJSON()
    await page.clock.fastForward(21000)
    await expect(cta(dialog, '重试原请求')).toBeVisible()
    held.resolve()
    expect(await setup.quota()).toEqual({ remaining: 10, reserved: 0, used: 0 })
    await page.reload()
    const lookup = cta(page, '查询原结果').filter({ visible: true }).first()
    await lookup.click()
    const resumed = page.getByRole('dialog').filter({ visible: true })
    await expect(resumed).toBeVisible()
    const missing = page.waitForResponse(resp => urlPath(resp) === exchanges + '/' + original.idempotency_key && resp.status() === 404)
    await cta(resumed, '查询原结果').click()
    await missing
    await expect(resumed).toContainText(/未找到|暂未查到|未查到|没有找到|未在服务端查到/)
    await screenshot(page, info, 'persisted-exchange-key-after-lookup-not-found')
    const sent = page.waitForResponse(resp => resp.ok() && resp.request().method() === 'POST' && urlPath(resp) === exchanges)
    await cta(resumed, '重试原请求').click()
    expect((await sent).request().postDataJSON()).toEqual(original)
    await expect.poll(() => setup.record(original.idempotency_key).then(r => r.status), { timeout: 20000 }).toBe('ready')
    expect(await fixture(request, { id: upstream.id })).toMatchObject({ calls: 1 })
    expect(await setup.quota()).toEqual({ remaining: 9, reserved: 0, used: 1 })
  } finally {
    held.resolve()
    await setup.close()
  }
})

test('查询丢回执的旧换号确认后，最新列表已是另一笔记录仍可续费', async ({ page, request, playwright, baseURL }, info) => {
  const email = 'exchange-ui-newer-key@example.test'
  const setup = await prepare({ request, playwright, baseURL }, email)
  try {
    const [allocation] = await setup.buy({ node_id: '101', term: 'yp1', quantity: 1 })
    await fixture(request, { ip: allocation.ip, outcome: 'ready' })
    await signInTenant(page, email)
    await page.goto('/console/static/ips')
    const dialog = await openExchange(page, allocation)
    await page.route('**' + exchanges, async route => {
      expect((await route.fetch()).ok()).toBe(true)
      await route.abort('failed')
    }, { times: 1 })
    const sent = submission(page)
    await cta(dialog, '确认换号').click()
    const original = (await sent).postDataJSON()
    await expect(cta(dialog, '重试原请求')).toBeVisible()
    await expect.poll(() => setup.record(original.idempotency_key).then(record => record.status), { timeout: 20000 }).toBe('ready')
    const current = await setup.config(allocation.allocation_id)

    // The real request above remains queryable. The list doubles for another tab's later exchange.
    const newerKey = randomUUID()
    await page.route('**' + ips, async route => {
      const response = await route.fetch()
      const body = await response.json()
      const row = body.allocations.find(ip => ip.allocation_id === allocation.allocation_id)
      row.exchange_idempotency_key = newerKey
      await route.fulfill({ response, json: body })
    })
    await page.reload()
    await cta(page, '查询原结果').filter({ visible: true }).first().click()
    const previousResult = page.getByRole('dialog').filter({ visible: true })
    await expect(previousResult).toBeVisible()
    const confirmed = page.waitForResponse(resp => resp.ok() && urlPath(resp) === exchanges + '/' + original.idempotency_key)
    await cta(previousResult, '查询原结果').click()
    expect(await (await confirmed).json()).toMatchObject({ status: 'ready', idempotency_key: original.idempotency_key })
    await expect(previousResult).toContainText(/列表.*(不同|另一笔)|当前接入参数.*列表/)
    await screenshot(page, info, 'old-exchange-result-newer-list-record')
    await cta(previousResult, '完成').click()
    const row = rowFor(page, current.ip)
    await expect(cta(row, '续费')).toBeEnabled()
    await expect(exchangeAction(row)).toBeEnabled()
    expect(await setup.quota()).toEqual({ remaining: 9, reserved: 0, used: 1 })
  } finally {
    await setup.close()
  }
})
