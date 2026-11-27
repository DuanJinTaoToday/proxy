import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { field, screenshot, signInOperator } from '../support/console.mjs'
import { clock, setup, synchronizeLookup } from '../support/static-exchange-operations.mjs'
import { json } from '../support/traffic.mjs'

const cta = (page, name) => page.getByRole('button', { name, exact: true })
const visibleDialog = page => page.getByRole('dialog').filter({ visible: true })
const rowFor = (page, key) => page.getByRole('row').filter({ hasText: key }).first()
const urlPath = message => new URL(message.url()).pathname
const exchangeTab = page => page.getByRole('tab', { name: '静态 IP 换号', exact: true }).first()
const auditPanel = page => page.locator('.exchange-card').filter({
  has: page.getByRole('heading', { name: '额度与结案审计', exact: true }),
})

async function openExchangeOps(page, username) {
  await signInOperator(page, '/ops/orders', username)
  await expect(page).toHaveURL(/\/ops\/orders/)
  await exchangeTab(page).click()
}

async function selectTenant(page, email) {
  const search = field(page, '租户搜索')
  await search.fill(email)
  await search.press('Enter')
  await expect(page.locator('.selected-tenant__meta')).toContainText(email)
}

async function openOutcome(page, who, action) {
  await cta(who, action).click()
  const dialog = visibleDialog(page)
  await expect(dialog).toBeVisible()
  return dialog
}

async function chooseOutcome(dialog, text) {
  const label = dialog.getByText(text, { exact: true }).filter({ visible: true })
  if (await label.count()) await label.last().click()
  else {
    await dialog.getByRole('combobox', { name: '结案结果' }).click()
    await dialog.getByRole('option', { name: text }).click()
  }
}

test('只读可查未入队状态，计费调额丢回执同键恢复并凭证据结案', async ({ page, browser, request, playwright, baseURL }, info) => {
  test.setTimeout(120000)
  const email = 'ops-exchange-ui-adjust@example.test'
  const f = await setup({ request, playwright, baseURL }, email)
  try {
    const ip = await f.buy('101')
    await f.fixture('exchanges', { ip: ip.ip, outcome: 'unknown-empty' })
    const record = await f.submit(ip)
    expect(record.status).toBe('unknown')
    const key = record.idempotency_key
    clock(key, 'recent')
    expect(await f.quota()).toEqual({ remaining: 9, reserved: 1, used: 0 })

    const readonlyContext = await browser.newContext({ baseURL, ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 } })
    try {
      const reader = await readonlyContext.newPage()
      await openExchangeOps(reader, 'operator-readonly')
      await selectTenant(reader, email)
      await expect(rowFor(reader, key)).toContainText('未入队')
      const adjust = cta(reader, '调整额度')
      if (await adjust.isVisible()) await expect(adjust).toBeDisabled()
      await expect(auditPanel(reader)).toContainText('该租户暂无额度或结案审计记录')
      await expect(cta(auditPanel(reader), '加载更多')).toHaveCount(0)
      await expect(cta(reader, '人工结案').filter({ visible: true })).toHaveCount(0)
      expect(await reader.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
      await screenshot(reader, info, 'mobile-readonly-tenant-exchange-history')
    } finally {
      await readonlyContext.close()
    }

    await openExchangeOps(page, 'operator-billing')
    await selectTenant(page, email)
    const adjust = await openOutcome(page, page, '调整额度')
    const delta = adjust.getByRole('spinbutton', { name: /额度变动/ }).or(field(adjust, '额度变动'))
    await delta.fill('-10')
    await field(adjust, '核查依据').fill('Crown fixture quota boundary')
    if (await cta(adjust, '确认调额').isEnabled()) {
      await cta(adjust, '确认调额').click()
      await expect(adjust.getByText(/预占|剩余|额度不足/).filter({ visible: true }).last()).toBeVisible()
    }
    expect(await f.quota()).toEqual({ remaining: 9, reserved: 1, used: 0 })
    await delta.fill('-9')
    await field(adjust, '核查依据').fill('Crown fixture quota adjustment')
    let original
    const adjustPath = f.ops + '/static-ip-exchange-quota:adjust'
    await page.route('**' + adjustPath, async route => {
      original = route.request().postDataJSON()
      const result = await route.fetch()
      expect(result.ok()).toBe(true)
      await route.abort('failed')
    }, { times: 1 })
    await cta(adjust, '确认调额').click()
    await expect(cta(adjust, '重试原请求')).toBeVisible()
    await expect.poll(f.quota).toEqual({ remaining: 0, reserved: 1, used: 0 })
    expect(original).toMatchObject({ delta: -9, request_key: expect.any(String) })

    const expired = page.waitForResponse(response => urlPath(response) === adjustPath && response.status() === 401)
    await page.route('**' + adjustPath, route => route.fulfill({ status: 401, json: { reason: 'INVALID_SESSION' } }), { times: 1 })
    await cta(adjust, '重试原请求').click()
    expect((await expired).request().postDataJSON()).toEqual(original)
    await expect(page).toHaveURL(/\/ops\/login/)
    await signInOperator(page, '/ops/orders?tab=static-exchanges', 'operator-billing')
    await expect(exchangeTab(page)).toHaveAttribute('aria-selected', 'true')
    await selectTenant(page, email)
    const replay = page.waitForResponse(response => urlPath(response) === adjustPath && response.ok())
    await cta(page, '重试原请求').filter({ visible: true }).first().click()
    await cta(visibleDialog(page), '重试原请求').click()
    expect((await replay).request().postDataJSON()).toEqual(original)
    const changes = (await f.audit()).filter(a => a.kind === 'adjustment')
    expect(changes).toHaveLength(1)
    expect(changes[0]).toMatchObject({
      actor_id: expect.any(String), request_key: original.request_key,
      before: { total: 10, reserved: 1 }, after: { total: 1, reserved: 1, remaining: 0 },
    })
    await expect(visibleDialog(page)).toContainText(changes[0].actor_id)
    await expect(visibleDialog(page)).toContainText('变更前')
    await expect(visibleDialog(page)).toContainText('变更后')
    await cta(visibleDialog(page), '关闭').click()
    const adjustmentRow = rowFor(page, original.request_key)
    await expect(adjustmentRow).toContainText('额度调整')
    await expect(adjustmentRow).toContainText(changes[0].actor_id)
    await expect(adjustmentRow).toContainText('Crown fixture quota adjustment')
    await expect(adjustmentRow).toContainText('变更前')
    await expect(adjustmentRow).toContainText('变更后')
    await expect(cta(auditPanel(page), '加载更多')).toHaveCount(0)

    clock(key, 'aged')
    expect(await f.queue()).toEqual(expect.arrayContaining([expect.objectContaining({
      in_queue: true, exchange: expect.objectContaining({ idempotency_key: key }),
    })]))
    const other = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
    try {
      const reader = await other.newPage()
      await openExchangeOps(reader, 'operator-readonly')
      await expect(rowFor(reader, key)).toContainText('已入队')
      await expect(cta(rowFor(reader, key), '人工结案')).toBeDisabled()
      const denied = await f.reader.post(f.ops + '/static-ip-exchanges/' + key + '/resolution', {
        data: { request_key: randomUUID(), outcome: 'not_executed', evidence: 'readonly cannot resolve' },
      })
      expect(denied.status()).toBe(403)
    } finally {
      await other.close()
    }
    await page.reload()
    await expect(exchangeTab(page)).toHaveAttribute('aria-selected', 'true')
    await selectTenant(page, email)
    const close = await openOutcome(page, rowFor(page, key), '人工结案')
    await expect(close.getByText(/查无记录|旧 IP 未变/).filter({ visible: true }).first()).toBeVisible()
    await chooseOutcome(close, /确认未执行/)
    await field(close, '核查依据').fill('Crown fixture ticket 126 confirmed no execution')
    const resolution = page.waitForResponse(response => urlPath(response) === f.ops + '/static-ip-exchanges/' + key + '/resolution' && response.ok())
    await cta(close, '确认结案').click()
    expect(await (await resolution).json()).toMatchObject({ resolution: 'not_executed' })
    expect(await f.quota()).toEqual({ remaining: 1, reserved: 0, used: 0 })
    const audit = (await f.audit()).find(a => a.idempotency_key === key && a.kind === 'manual')
    expect(audit).toMatchObject({
      outcome: 'not_executed', actor_id: expect.any(String), before: { reserved: 1 }, after: { reserved: 0 },
    })
    const auditRow = rowFor(page, audit.request_key)
    await expect(auditRow).toContainText('人工结案')
    await expect(auditRow).toContainText(audit.actor_id)
    await expect(auditRow).toContainText('Crown fixture ticket 126 confirmed no execution')
    await screenshot(page, info, 'billing-exchange-resolution-and-audit')
  } finally {
    await f.close()
  }
})

test('计费运营按证据更正迟到冲突，旧记录不覆盖同一分配的新配置', async ({ page, request, playwright, baseURL }, info) => {
  test.setTimeout(100000)
  const email = 'ops-exchange-ui-correct@example.test'
  const f = await setup({ request, playwright, baseURL }, email)
  try {
    // Japan has its own fixture address range, unlike the shared US/Canada prefix.
    const ip = await f.buy('201')
    const supplier = await f.fixture('exchanges', { ip: ip.ip, outcome: 'unknown-empty' })
    const old = await f.submit(ip)
    expect(old.status).toBe('unknown')
    const key = old.idempotency_key
    clock(key, 'aged')
    await f.resolution(key, { request_key: randomUUID(), outcome: 'not_executed', evidence: 'Crown fixture initial receipt' }).then(json)
    await f.fixture('exchanges', { id: supplier.id, outcome: 'ready' })
    const newer = await f.submit(ip)
    await expect.poll(() => f.lookup(newer.idempotency_key).then(r => r.status), { timeout: 20000 }).toBe('ready')
    const latest = await f.config(ip)
    const previousAddress = '192.0.2.246'
    await f.fixture('exchange-records', { id: supplier.id, ip: ip.ip, reported_ip: previousAddress })
    await synchronizeLookup(f, ip.ip, key)
    await expect.poll(() => f.queue().then(rows => rows.find(row => row.exchange.idempotency_key === key)?.resolution_conflict)).toBe(true)
    const denied = await f.reader.post(f.ops + '/static-ip-exchanges/' + key + '/resolution', {
      data: { request_key: randomUUID(), outcome: 'executed', correction: true, expected_ip: previousAddress, evidence: 'readonly cannot correct' },
    })
    expect(denied.status()).toBe(403)

    await openExchangeOps(page, 'operator-billing')
    await expect(rowFor(page, key)).toContainText('迟到')
    const correct = await openOutcome(page, rowFor(page, key), '更正结案')
    await chooseOutcome(correct, /确认已执行/)
    await field(correct, '核查依据').fill('Crown fixture revised receipt verified by billing')
    await field(correct, '(核验新 IP|新 IP)').fill(previousAddress)
    const result = page.waitForResponse(response => urlPath(response) === f.ops + '/static-ip-exchanges/' + key + '/resolution' && response.ok())
    await cta(correct, '确认结案').click()
    const response = await result
    expect(response.request().postDataJSON()).toMatchObject({ outcome: 'executed', correction: true, expected_ip: previousAddress })
    expect(await response.json()).toMatchObject({ resolution: 'executed', corrected: true })
    expect(await f.config(ip)).toMatchObject({ ip: latest.ip, port: latest.port, allocation_id: ip.allocation_id })
    expect(await f.quota()).toMatchObject({ used: 2, reserved: 0 })
    expect((await f.audit()).find(a => a.idempotency_key === key && a.kind === 'correction'))
      .toMatchObject({ actor_id: expect.any(String), before: { used: 1 }, after: { used: 2 } })
    await selectTenant(page, email)
    await expect(rowFor(page, key)).toContainText('已更正')
    await screenshot(page, info, 'late-conflict-corrected-with-audit')
  } finally {
    await f.close()
  }
})
