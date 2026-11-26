import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, recharge } from '../support/traffic.mjs'

const salePath = '/v1/operations/product-sales/static'
const exchangePath = '/v1/tenant/static-ip-exchanges'
const quotaPath = '/v1/tenant/static-ip-exchange-quota'
const ipsPath = '/v1/tenant/static-ips'
const renewalPath = '/v1/tenant/static-renewal-orders'

function fixture(request, data) {
  return request.post(new URL('/fixture/exchanges', process.env.VERIFY_IPCROWN_URL).href, { data }).then(json)
}

async function expectError(response, status, reason) {
  expect(response.status()).toBe(status)
  if (reason) expect(await response.json()).toMatchObject({ reason })
}

async function buy(tenant, selection) {
  const quote = await json(await tenant.post('/v1/tenant/static-purchase-quotes', { data: selection }))
  return json(await tenant.post('/v1/tenant/static-orders', {
    data: { ...selection, unit_minor: quote.offer.unit_minor, request_key: randomUUID(), confirmation_token: quote.confirmation_token },
  }))
}

function setStaticSales(operator, open) {
  return operator.put(salePath, {
    data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'Issue #106 local Crown fixture' } : {}) },
  }).then(json)
}

test('在部分交付订单中换号、验证 SOCKS5 生效并按原明细续费，未知结果只预占一次', async ({ request, playwright, baseURL }, info) => {
  const email = 'exchange-api@example.test'
  await json(await registerEmail(request, { email }))
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email })
  const otherTenant = await authenticatedApi(playwright, baseURL, { email: 'beta@example.test' })
  const quota = () => tenant.get(quotaPath).then(json)
  const submit = data => tenant.post(exchangePath, { data })
  const operation = key => tenant.get(exchangePath + '/' + key).then(json)
  const list = () => tenant.get(ipsPath).then(json).then(result => result.allocations)
  const walletState = () => Promise.all([
    tenant.get('/v1/tenant/wallet').then(json),
    tenant.get('/v1/tenant/wallet/ledger').then(json),
  ])
  const renewalSpec = id => ({ allocation_ids: [id], term: 'yp1' })

  try {
    await recharge(operator, tenant, '30000')
    await setStaticSales(operator, true)
    const order = await buy(tenant, { node_id: '101', term: 'yp1', quantity: 3 })
    expect(order).toMatchObject({ status: 'pending', delivered_count: '2' })
    const delivered = (await list()).filter(item => item.order_id === order.id)
    expect(delivered).toHaveLength(2)
    const [first, second] = delivered
    expect(first).toMatchObject({ share_mode: 'exclusive', node_id: '101' })
    expect(await quota()).toMatchObject({ remaining: 10, reserved: 0, used: 0 })
    const balances = await walletState()

    const initial = { allocation_id: first.allocation_id, node_id: first.node_id, idempotency_key: randomUUID() }
    const firstMock = await fixture(request, { ip: first.ip, outcome: 'ready' })
    await expectError(await submit({ ...initial, node_id: '201' }), 409, 'STATIC_EXCHANGE_REGION_MISMATCH')
    const accepted = await json(await submit(initial))
    expect(accepted).toMatchObject({ allocation_id: first.allocation_id, idempotency_key: initial.idempotency_key, status: 'activating' })
    await expect.poll(() => operation(initial.idempotency_key).then(record => record.status), { timeout: 20000 }).toBe('ready')
    const changed = (await list()).find(item => item.allocation_id === first.allocation_id)
    expect(changed).toMatchObject({ order_id: order.id, node_id: first.node_id, exchange_status: 'ready' })
    expect(changed.ip).not.toBe(first.ip)
    expect(changed.expires_at).toBe(first.expires_at)
    const config = await json(await tenant.get(ipsPath + '/' + first.allocation_id + '/proxy-config'))
    expect(config).toMatchObject({ allocation_id: first.allocation_id, ip: changed.ip, port: 1081 })
    const exported = await tenant.post(ipsPath + ':export', { data: { allocation_ids: [first.allocation_id] } })
    expect(exported.ok()).toBe(true)
    expect((await exported.text()).trim()).toBe(changed.ip + ':1081:fixture-user:fixture-pass')
    expect(await json(await submit(initial))).toMatchObject({ idempotency_key: initial.idempotency_key, status: 'ready' })
    await expectError(await submit({ ...initial, allocation_id: second.allocation_id }), 409, 'WALLET_REQUEST_CONFLICT')
    expect(await fixture(request, { id: firstMock.id })).toMatchObject({ calls: 1 })
    expect(await quota()).toMatchObject({ remaining: 9, reserved: 0, used: 1 })

    const secondMock = await fixture(request, { ip: second.ip, outcome: 'failed' })
    const denied = { allocation_id: second.allocation_id, node_id: second.node_id, idempotency_key: randomUUID() }
    expect(await json(await submit(denied))).toMatchObject({ status: 'failed' })
    expect(await quota()).toMatchObject({ remaining: 9, reserved: 0, used: 1 })

    const secondRenewal = await json(await tenant.post('/v1/tenant/static-renewal-quotes', { data: renewalSpec(second.allocation_id) }))
    await fixture(request, { id: secondMock.id, outcome: 'unknown' })
    const unknown = { ...denied, idempotency_key: randomUUID() }
    expect(await json(await submit(unknown))).toMatchObject({ status: 'unknown' })
    expect(await operation(unknown.idempotency_key)).toMatchObject({ status: 'unknown', allocation_id: second.allocation_id })
    expect(await json(await submit(unknown))).toMatchObject({ status: 'unknown', idempotency_key: unknown.idempotency_key })
    await expectError(await otherTenant.get(exchangePath + '/' + unknown.idempotency_key), 404)
    await expectError(await tenant.post(renewalPath, {
      data: { ...renewalSpec(second.allocation_id), request_key: randomUUID(), confirmation_token: secondRenewal.confirmation_token },
    }), 409, 'STATIC_EXCHANGE_PENDING')
    expect(await fixture(request, { id: secondMock.id })).toMatchObject({ calls: 2, renewals: 0 })
    expect(await quota()).toMatchObject({ remaining: 8, reserved: 1, used: 1 })
    expect(await walletState()).toEqual(balances)

    const firstRenewal = await json(await tenant.post('/v1/tenant/static-renewal-quotes', { data: renewalSpec(first.allocation_id) }))
    const renewed = await json(await tenant.post(renewalPath, {
      data: { ...renewalSpec(first.allocation_id), request_key: randomUUID(), confirmation_token: firstRenewal.confirmation_token },
    }))
    expect(renewed).toMatchObject({ status: 'succeeded' })
    expect(await fixture(request, { id: firstMock.id })).toMatchObject({ calls: 1, renewals: 1 })
    expect((await list()).find(item => item.allocation_id === first.allocation_id)).toMatchObject({ ip: changed.ip, port: 1081 })

    await info.attach('static-ip-exchange-api-evidence', {
      body: JSON.stringify({ confirmedProbe: 'ready', quotaAfterUnknown: await quota(), originalDetailRenewals: 1 }, null, 2),
      contentType: 'application/json',
    })
  } finally {
    try {
      await setStaticSales(operator, false)
    } finally {
      await Promise.all([operator.dispose(), tenant.dispose(), otherTenant.dispose()])
    }
  }
})

test('换号成功但 SOCKS5 拨测耗尽预算，保留新配置和已用额度并进入客服状态', async ({ request, playwright, baseURL }) => {
  test.setTimeout(100000)
  const email = 'exchange-probe-failed@example.test'
  await json(await registerEmail(request, { email }))
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email })
  try {
    await recharge(operator, tenant, '10000')
    await setStaticSales(operator, true)
    const order = await buy(tenant, { node_id: '111', term: 'yp1', quantity: 1 })
    expect(order.status).toBe('succeeded')
    const { allocations } = await json(await tenant.get(ipsPath))
    const [ip] = allocations
    expect(ip.order_id).toBe(order.id)
    const mock = await fixture(request, { ip: ip.ip, outcome: 'unreachable' })
    const data = { allocation_id: ip.allocation_id, node_id: ip.node_id, idempotency_key: randomUUID() }
    const lookup = () => tenant.get(exchangePath + '/' + data.idempotency_key).then(json)
    expect(await json(await tenant.post(exchangePath, { data }))).toMatchObject({ status: 'activating' })
    await expect.poll(() => lookup().then(record => record.status), { timeout: 75000 }).toBe('support_required')
    expect(await json(await tenant.get(quotaPath))).toMatchObject({ remaining: 9, reserved: 0, used: 1 })
    const config = await json(await tenant.get(ipsPath + '/' + ip.allocation_id + '/proxy-config'))
    expect(config).toMatchObject({ allocation_id: ip.allocation_id, port: 65000 })
    expect(config.ip).not.toBe(ip.ip)
    expect(await json(await tenant.post(exchangePath, { data }))).toMatchObject({ status: 'support_required' })
    await fixture(request, { id: mock.id, outcome: 'failed' })
    expect(await json(await tenant.post(exchangePath, {
      data: { ...data, idempotency_key: randomUUID() },
    }))).toMatchObject({ status: 'failed' })
    expect(await fixture(request, { id: mock.id })).toMatchObject({ calls: 2 })
    expect(await json(await tenant.get(quotaPath))).toMatchObject({ remaining: 9, reserved: 0, used: 1 })
  } finally {
    try {
      await setStaticSales(operator, false)
    } finally {
      await Promise.all([operator.dispose(), tenant.dispose()])
    }
  }
})
