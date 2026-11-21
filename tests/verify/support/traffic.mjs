import { randomUUID } from 'node:crypto'
import { expect } from '@playwright/test'

export const purchasePath = '/v1/tenant/orders/traffic-packages'

export async function json(response) {
  expect(response.ok(), `${response.status()} ${new URL(response.url()).pathname}`).toBe(true)
  return response.json()
}

export async function authenticatedApi(playwright, baseURL, { phone } = {}) {
  const caller = await playwright.request.newContext({ baseURL, ignoreHTTPSErrors: true })
  try {
    const login = await json(await caller.post(phone ? '/v1/access/login' : '/v1/operations/access/login', {
      data: { ...(phone ? { phone } : { username: 'operator' }), password: 'proxy-local-test' },
    }))
    return await playwright.request.newContext({
      baseURL, ignoreHTTPSErrors: true, extraHTTPHeaders: { Authorization: 'Bearer ' + login.access_token },
    })
  } finally {
    await caller.dispose()
  }
}

export const setSale = (operator, open) => operator.put('/v1/operations/product-sales/dynamic', {
  data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'verify: dynamic pricing #72' } : {}) },
}).then(json)

export async function setTier(operator, id, price, listed = true) {
  const { packages } = await json(await operator.get('/v1/operations/traffic-packages'))
  const tier = packages.find(record => record.id === id)
  return json(await operator.patch('/v1/operations/traffic-packages/' + id, {
    data: { version: tier.version, capacity_bytes: tier.capacity_bytes, price_minor: price, listed },
  }))
}

export async function recharge(operator, tenant, amount_minor) {
  const { tenant_id } = await json(await tenant.get('/v1/tenant/account'))
  const path = '/v1/operations/tenants/' + tenant_id + '/wallet'
  const request_key = randomUUID()
  const { confirmation_token } = await json(await operator.post(path + '/recharge-quotes', {
    data: { request_key, amount_minor, reason: 'verify #72 ' + process.env.VERIFY_ID },
  }))
  await json(await operator.post(path + '/recharges', { data: { request_key, confirmation_token } }))
}

export async function trafficRecords(tenant) {
  const [overview, orders, ledger] = await Promise.all([
    tenant.get('/v1/tenant/overview').then(json),
    tenant.get('/v1/tenant/orders?kind=traffic_package').then(json),
    tenant.get('/v1/tenant/wallet/ledger').then(json),
  ])
  return {
    wallet: overview.wallet_minor ?? '0',
    traffic: overview.traffic_bytes ?? '0',
    orders: orders.orders ?? [],
    debits: (ledger.entries ?? []).filter(entry => entry.kind === 'debit'),
  }
}

export async function expectSettled(tenant, orderId, balance) {
  const result = await trafficRecords(tenant)
  expect(result).toMatchObject({ wallet: balance, traffic: '5000000000' })
  expect(result.orders).toHaveLength(1)
  expect(result.orders[0]).toMatchObject({
    id: orderId, kind: 'traffic_package', status: 'succeeded', debited_minor: '1500', delivered_bytes: '5000000000',
  })
  expect(result.debits).toHaveLength(1)
  expect(result.debits[0]).toMatchObject({ order_id: orderId, before_minor: String(Number(balance) + 1500), after_minor: balance })
  return result
}
