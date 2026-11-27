import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { expect } from '@playwright/test'
import { registerEmail } from './mail.mjs'
import { authenticatedApi, json, recharge } from './traffic.mjs'

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

export function clock(key, mode) {
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

export async function setup({ request, playwright, baseURL }, email) {
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

export async function synchronizeLookup(f, ip, key) {
  const { reads } = await f.fixture('exchange-records', { ip })
  clock(key, 'check')
  await expect.poll(() => f.fixture('exchange-records', { ip }).then(r => r.reads), { timeout: 20000 }).toBeGreaterThan(reads)
}
