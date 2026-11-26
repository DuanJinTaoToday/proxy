import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, recharge } from '../support/traffic.mjs'

const sale = '/v1/operations/product-sales/static'
const queue = '/v1/operations/static-ip-exchange-queue'
const exchange = '/v1/tenant/static-ip-exchanges'
const ips = '/v1/tenant/static-ips'
const testQuota = '/v1/tenant/static-ip-exchange-quota'

async function readerSession(playwright, baseURL) {
  const caller = await playwright.request.newContext({ baseURL, ignoreHTTPSErrors: true })
  try {
    const login = await json(await caller.post('/v1/operations/access/login', {
      data: { username: 'operator-readonly', password: 'proxy-local-test' },
    }))
    return { headers: { Authorization: 'Bearer ' + login.access_token } }
  } finally {
    await caller.dispose()
  }
}

function clock(key, mode) {
  const shifts = {
    // Prioritize this fixture operation ahead of older records left by other scenarios.
    check: "next_check_at=statement_timestamp()-interval '1 day'",
    recent: "created_at=statement_timestamp()-interval '30 minutes', unknown_at=statement_timestamp()",
    aged: "unknown_at=statement_timestamp()-interval '15 minutes 1 second'",
  }
  const container = process.env.VERIFY_DB_CONTAINER
  if (!/^[a-f0-9]{64}$/.test(container) || !/^[a-f0-9-]{36}$/.test(key) || !Object.hasOwn(shifts, mode)) {
    throw new Error('Invalid isolated exchange clock fixture')
  }
  execFileSync('docker', [
    'exec', '-i', container, 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-v', 'key=' + key,
  ], { input: "UPDATE static_ip_exchanges SET " + shifts[mode] + " WHERE idempotency_key=:'key';\n" })
}

async function setup({ request, playwright, baseURL }, email) {
  await json(await registerEmail(request, { email }))
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email })
  const { tenant_id: id } = await json(await tenant.get('/v1/tenant/account'))
  const ops = '/v1/operations/tenants/' + id
  const { headers } = await readerSession(playwright, baseURL)
  const reader = await playwright.request.newContext({ baseURL, ignoreHTTPSErrors: true, extraHTTPHeaders: headers })
  await recharge(operator, tenant, '30000')
  await json(await operator.put(sale, {
    data: { open: true, quote_ready: true, acceptance_evidence: 'Issue #107 local Crown fixture' },
  }))
  const allocationList = async () => (await tenant.get(ips).then(json)).allocations ?? []
  return {
    id, ops, operator, reader, tenant, headers,
    quota: () => tenant.get(testQuota).then(json),
    queue: () => reader.get(queue + '?tenant_id=' + id).then(json).then(body => body.exchanges ?? []),
    history: () => reader.get(ops + '/static-ip-exchanges').then(json).then(body => body.exchanges ?? []),
    auditPath: ops + '/static-ip-exchange-audit',
    audit: () => reader.get(ops + '/static-ip-exchange-audit').then(json).then(body => body.entries ?? []),
    adjustment: (delta, request_key = randomUUID()) => operator.post(ops + '/static-ip-exchange-quota:adjust', {
      data: { delta, request_key, evidence: 'fixture quota request' },
    }),
    resolution: (key, data) => operator.post(ops + '/static-ip-exchanges/' + key + '/resolution', { data }),
    fixture: (path, data) => request.post(new URL('/fixture/' + path, process.env.VERIFY_IPCROWN_URL).href, { data }).then(json),
    async buy(node_id) {
      const selection = { node_id, term: 'yp1', quantity: 1 }
      const quote = await tenant.post('/v1/tenant/static-purchase-quotes', { data: selection }).then(json)
      const order = await tenant.post('/v1/tenant/static-orders', { data: {
        ...selection, unit_minor: quote.offer.unit_minor, request_key: randomUUID(), confirmation_token: quote.confirmation_token,
      } }).then(json)
      expect(order.status).toBe('succeeded')
      return (await allocationList()).find(item => item.order_id === order.id)
    },
    async submit(allocation, idempotency_key = randomUUID()) {
      return tenant.post(exchange, { data: {
        allocation_id: allocation.allocation_id, node_id: allocation.node_id, idempotency_key,
      } }).then(json)
    },
    config: allocation => tenant.get(ips + '/' + allocation.allocation_id + '/proxy-config').then(json),
    lookup: key => tenant.get(exchange + '/' + key).then(json),
    close: async () => {
      try { await json(await operator.put(sale, { data: { open: false } })) }
      finally { await Promise.all([operator.dispose(), tenant.dispose(), reader.dispose()]) }
    },
  }
}

async function synchronizeLookup(f, ip, key) {
  const { reads } = await f.fixture('exchange-records', { ip })
  clock(key, 'check')
  await expect.poll(() => f.fixture('exchange-records', { ip }).then(r => r.reads), { timeout: 20000 }).toBeGreaterThan(reads)
}

async function snapshot(page, info, f, evidence) {
  await info.attach('exchange-operations-api-evidence', {
    body: JSON.stringify(evidence, null, 2), contentType: 'application/json',
  })
  await page.setExtraHTTPHeaders(f.headers)
  const response = await page.goto(f.auditPath)
  expect(response.status()).toBe(200)
  await info.attach('readonly-operations-audit', { body: await page.screenshot(), contentType: 'image/png' })
}

test('运营调额保留预占，待核实满十五分钟入队后通过皇冠两条只读证据核销一次', async ({ page, request, playwright, baseURL }, info) => {
  test.setTimeout(100000)
  const f = await setup({ request, playwright, baseURL }, 'exchange-ops-proof@example.test')
  try {
    const allocation = await f.buy('101')
    const supplier = await f.fixture('exchanges', { ip: allocation.ip, outcome: 'unknown-tracked', history_after_write: 'hidden' })
    await f.fixture('exchange-records', { ip: allocation.ip, id: supplier.id, reported_ip: '192.0.2.242' })
    const record = await f.submit(allocation)
    expect(record.status).toBe('unknown')
    const key = record.idempotency_key
    const quotaBefore = await f.quota()
    expect(quotaBefore).toEqual({ remaining: 9, reserved: 1, used: 0 })
    const readOnlyWrite = await f.reader.post(f.ops + '/static-ip-exchange-quota:adjust', {
      data: { delta: 1, request_key: randomUUID(), evidence: 'readonly' },
    })
    expect(readOnlyWrite.status()).toBe(403)
    expect((await f.tenant.get(f.auditPath)).status()).toBe(403)
    expect((await f.adjustment(-10)).status()).toBe(409)
    const adjustmentKey = randomUUID()
    const adjustments = await Promise.all([f.adjustment(-9, adjustmentKey).then(json), f.adjustment(-9, adjustmentKey).then(json)])
    expect(adjustments[0].id).toBe(adjustments[1].id)
    const adjusted = await f.reader.get(f.ops + '/static-ip-exchange-quota').then(json)
    expect(adjusted).toMatchObject({ total: 1, remaining: 0, reserved: 1, used: 0 })

    await f.fixture('exchange-records', { ip: allocation.ip, mode: 'hidden' })
    clock(key, 'recent')
    expect(await f.queue()).toHaveLength(0)
    await synchronizeLookup(f, allocation.ip, key)
    expect(await f.quota()).toEqual({ remaining: 0, reserved: 1, used: 0 })
    await f.fixture('exchange-records', { ip: allocation.ip, mode: 'timeout' })
    await synchronizeLookup(f, allocation.ip, key)
    expect(await f.quota()).toEqual({ remaining: 0, reserved: 1, used: 0 })
    clock(key, 'aged')
    const queued = (await f.queue()).find(item => item.exchange.idempotency_key === key)
    expect(queued).toMatchObject({ in_queue: true, exchange: { status: 'unknown' } })
    expect(Date.parse(queued.unknown_at)).toBeGreaterThan(Date.parse(queued.exchange.created_at))

    await f.fixture('exchange-records', { ip: allocation.ip, mode: 'truncated' })
    await synchronizeLookup(f, allocation.ip, key)
    expect(await f.lookup(key)).toMatchObject({ status: 'unknown' })
    await f.fixture('exchange-records', { ip: allocation.ip, mode: 'visible' })
    await synchronizeLookup(f, allocation.ip, key)
    await expect.poll(() => f.lookup(key).then(op => op.status), { timeout: 18000 }).toBe('ready')
    const finalQuota = await f.quota()
    expect(finalQuota).toEqual({ remaining: 0, reserved: 0, used: 1 })
    const config = await f.config(allocation)
    expect(config.ip).not.toBe(allocation.ip)
    expect(config.ip).not.toBe('192.0.2.242')
    expect(config).toMatchObject({ allocation_id: allocation.allocation_id, order_id: allocation.order_id, expires_at: allocation.expires_at })
    expect(await f.queue()).toHaveLength(0)
    expect((await f.fixture('exchanges', { id: supplier.id })).calls).toBe(1)
    const audit = await f.audit()
    expect(audit).toHaveLength(2)
    expect(audit[0]).toMatchObject({ kind: 'automatic', outcome: 'executed', supplier_record_id: expect.any(String),
      before: { total: 1, reserved: 1, used: 0 }, after: { total: 1, reserved: 0, used: 1 } })
    expect(audit[1]).toMatchObject({ kind: 'adjustment', actor_id: expect.any(String), request_key: adjustmentKey,
      before: { total: 10, reserved: 1, used: 0 }, after: { total: 1, reserved: 1, used: 0 } })
    expect(JSON.stringify(audit)).not.toContain('fixture-pass')
    await f.adjustment(1).then(json)
    const baselineMissing = await f.buy('201')
    const secondSupplier = await f.fixture('exchanges', {
      ip: baselineMissing.ip, outcome: 'unknown-tracked', history_after_write: 'hidden',
    })
    await f.fixture('exchange-records', { ip: baselineMissing.ip, mode: 'timeout' })
    const unresolved = await f.submit(baselineMissing)
    expect(unresolved.status).toBe('unknown')
    clock(unresolved.idempotency_key, 'aged')
    await f.fixture('exchange-records', { ip: baselineMissing.ip, mode: 'visible' })
    await synchronizeLookup(f, baselineMissing.ip, unresolved.idempotency_key)
    await expect.poll(() => f.audit().then(entries => entries.find(entry =>
      entry.idempotency_key === unresolved.idempotency_key)?.kind)).toBe('observation')
    expect(await f.lookup(unresolved.idempotency_key)).toMatchObject({ status: 'unknown' })
    expect(await f.quota()).toEqual({ remaining: 0, reserved: 1, used: 1 })
    await f.fixture('exchange-records', { ip: baselineMissing.ip, mode: 'hidden' })
    const initiallyClosed = await f.resolution(unresolved.idempotency_key, {
      request_key: randomUUID(), outcome: 'not_executed', evidence: 'Crown fixture ticket 102 initial finding',
    }).then(json)
    expect(initiallyClosed).toMatchObject({ resolution: 'not_executed' })
    expect(await f.quota()).toEqual({ remaining: 1, reserved: 0, used: 1 })
    await f.fixture('exchange-records', { ip: baselineMissing.ip, mode: 'visible' })
    await synchronizeLookup(f, baselineMissing.ip, unresolved.idempotency_key)
    await expect.poll(() => f.queue().then(entries => entries.find(entry =>
      entry.exchange.idempotency_key === unresolved.idempotency_key)?.resolution_conflict)).toBe(true)
    expect(await f.quota()).toEqual({ remaining: 1, reserved: 0, used: 1 })
    const { address } = await f.fixture('exchanges', { id: secondSupplier.id })
    const verified = await f.resolution(unresolved.idempotency_key, {
      request_key: randomUUID(), outcome: 'executed', expected_ip: address, correction: true,
      evidence: 'Crown fixture ticket 102 revised finding with receipt',
    }).then(json)
    expect(verified).toMatchObject({ resolution: 'executed', corrected: true })
    const noBaselineAudit = (await f.audit()).filter(entry => entry.idempotency_key === unresolved.idempotency_key)
    expect(noBaselineAudit.map(entry => entry.kind)).toEqual(['correction', 'conflict', 'manual', 'observation'])
    expect(await f.quota()).toEqual({ remaining: 0, reserved: 0, used: 2 })
    await snapshot(page, info, f, { quotaBefore, adjusted, queued, finalQuota, audit, noBaselineAudit })
  } finally {
    await f.close()
  }
})

test('人工结案与自动核对只结一次，迟到的历史结果更正次数且保留新配置和当前预占', async ({ page, request, playwright, baseURL }, info) => {
  test.setTimeout(100000)
  const f = await setup({ request, playwright, baseURL }, 'exchange-ops-closure@example.test')
  try {
    await f.adjustment(-7).then(json)
    const first = await f.buy('111')
    const other = await f.buy('101')
    const concurrent = await f.buy('201')
    const firstFixture = await f.fixture('exchanges', { ip: first.ip, outcome: 'unknown-empty' })
    const old = await f.submit(first)
    expect(old.status).toBe('unknown')
    const beforeRelease = await f.quota()
    expect(beforeRelease).toEqual({ remaining: 2, reserved: 1, used: 0 })
    const negative = { request_key: randomUUID(), outcome: 'not_executed', evidence: 'Crown fixture ticket 100 confirmed not executed' }
    const tooSoon = await f.resolution(old.idempotency_key, negative)
    expect(tooSoon.status()).toBe(409)
    expect(await tooSoon.json()).toMatchObject({ reason: 'STATIC_EXCHANGE_NOT_IN_QUEUE' })
    expect(await f.quota()).toEqual(beforeRelease)
    clock(old.idempotency_key, 'aged')
    expect(await f.resolution(old.idempotency_key, negative).then(json)).toMatchObject({ exchange: { status: 'failed' } })
    expect(await f.resolution(old.idempotency_key, negative).then(json)).toMatchObject({ exchange: { status: 'failed' } })
    expect(await f.quota()).toEqual({ remaining: 3, reserved: 0, used: 0 })

    await f.fixture('exchanges', { id: firstFixture.id, outcome: 'ready' })
    const newer = await f.submit(first)
    await expect.poll(() => f.lookup(newer.idempotency_key).then(record => record.status), { timeout: 18000 }).toBe('ready')
    const newerConfig = await f.config(first)
    await f.fixture('exchanges', { ip: other.ip, outcome: 'unknown-empty' })
    expect((await f.submit(other)).status).toBe('unknown')

    const raceSupplier = await f.fixture('exchanges', { ip: concurrent.ip, outcome: 'unknown-tracked', history_after_write: 'hidden' })
    const racing = await f.submit(concurrent)
    clock(racing.idempotency_key, 'aged')
    await f.fixture('exchange-records', { ip: concurrent.ip, mode: 'blocked' })
    await synchronizeLookup(f, concurrent.ip, racing.idempotency_key)
    const { address: expectedIP } = await f.fixture('exchanges', { id: raceSupplier.id })
    const positive = { request_key: randomUUID(), outcome: 'executed', expected_ip: expectedIP, evidence: 'Crown fixture ticket 101 confirming receipt' }
    expect(await f.resolution(racing.idempotency_key, positive).then(json)).toMatchObject({ resolution: 'executed' })
    expect((await f.audit()).find(entry => entry.idempotency_key === racing.idempotency_key)).toMatchObject({ kind: 'manual' })
    await f.fixture('exchange-records', { ip: concurrent.ip, mode: 'visible' })
    await expect.poll(() => f.audit().then(audit => audit.filter(entry => entry.idempotency_key === racing.idempotency_key).map(entry => entry.kind)), {
      timeout: 15000,
    }).toEqual(['duplicate', 'manual'])

    const lateIP = '192.0.2.245'
    await f.fixture('exchange-records', { ip: first.ip, id: firstFixture.id, reported_ip: lateIP })
    await synchronizeLookup(f, first.ip, old.idempotency_key)
    await expect.poll(() => f.queue().then(entries => entries.find(entry => entry.exchange.idempotency_key === old.idempotency_key)?.resolution_conflict)).toBe(true)
    const beforeCorrection = await f.quota()
    expect(beforeCorrection).toEqual({ remaining: 0, reserved: 1, used: 2 })
    const correction = { request_key: randomUUID(), outcome: 'executed', expected_ip: lateIP, correction: true, evidence: 'Crown fixture ticket 100 revised with receipt' }
    expect(await f.resolution(old.idempotency_key, correction).then(json)).toMatchObject({ resolution: 'executed', corrected: true })
    expect(await f.resolution(old.idempotency_key, correction).then(json)).toMatchObject({ corrected: true })
    const afterCorrection = await f.quota()
    expect(afterCorrection).toEqual({ remaining: 0, reserved: 1, used: 3 })
    expect(await f.config(first)).toMatchObject({ ip: newerConfig.ip, port: newerConfig.port, expires_at: newerConfig.expires_at })
    expect((await f.fixture('exchanges', { id: firstFixture.id })).calls).toBe(2)
    const audit = await f.audit()
    const oldAudit = audit.filter(entry => entry.idempotency_key === old.idempotency_key)
    expect(oldAudit.map(entry => entry.kind)).toEqual(['correction', 'conflict', 'manual'])
    expect(oldAudit[0]).toMatchObject({ actor_id: expect.any(String), before: { used: 2, reserved: 1 }, after: { used: 3, reserved: 1 } })
    expect(oldAudit[1]).toMatchObject({ outcome: 'executed', observed_ip: lateIP, supplier_record_id: expect.any(String) })
    await snapshot(page, info, f, { beforeRelease, beforeCorrection, afterCorrection, oldAudit, raceAudit: audit.filter(entry => entry.idempotency_key === racing.idempotency_key) })
  } finally {
    await f.close()
  }
})
