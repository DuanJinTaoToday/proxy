import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'

const commissionPath = '/v1/tenant/commissions'
const ratesPath = '/v1/operations/commission-rates'
const staticSales = '/v1/operations/product-sales/static'
const invitePath = '/v1/tenant/invite'

async function operatorHeaders(request, username) {
  const session = await json(await request.post('/v1/operations/access/login', {
    data: { username, password: 'proxy-local-test' },
  }))
  return { Authorization: 'Bearer ' + session.access_token }
}

async function expectError(response, status, reason) {
  expect(response.status()).toBe(status)
  if (reason) expect(await response.json()).toMatchObject({ reason })
}

async function buyStatic(tenant, node_id, quantity) {
  const selection = { node_id, term: 'yp1', quantity }
  const quote = await json(await tenant.post('/v1/tenant/static-purchase-quotes', { data: selection }))
  return json(await tenant.post('/v1/tenant/static-orders', { data: {
    ...selection, unit_minor: quote.offer.unit_minor, confirmation_token: quote.confirmation_token, request_key: randomUUID(),
  } }))
}

async function renewStatic(tenant, allocation_id) {
  const selection = { allocation_ids: [allocation_id], term: 'yp1' }
  const quote = await json(await tenant.post('/v1/tenant/static-renewal-quotes', { data: selection }))
  return json(await tenant.post('/v1/tenant/static-renewal-orders', { data: {
    ...selection, confirmation_token: quote.confirmation_token, request_key: randomUUID(),
  } }))
}

test('三级邀请只给直推人计佣，按三类订单最终净扣和结算顺序核对', async ({ request, playwright, baseURL }, info) => {
  const emailB = 'commission-b@example.test'
  const emailC = 'commission-c@example.test'
  const A = await authenticatedApi(playwright, baseURL, { email: 'alpha@example.test' })
  const operator = await authenticatedApi(playwright, baseURL)
  const reader = await operatorHeaders(request, 'operator-readonly')
  const billing = await operatorHeaders(request, 'operator-billing')
  const support = await operatorHeaders(request, 'operator-support')
  const { invite_code: codeA } = await A.get(invitePath).then(json)
  const { tenant_id: idB } = await registerEmail(request, { email: emailB, invite: { link_invite_code: codeA } }).then(json)
  const B = await authenticatedApi(playwright, baseURL, { email: emailB })
  const { invite_code: codeB } = await B.get(invitePath).then(json)
  const { tenant_id: idC } = await registerEmail(request, { email: emailC, invite: { invite_code: codeB } }).then(json)
  const C = await authenticatedApi(playwright, baseURL, { email: emailC })
  const readB = () => B.get(commissionPath).then(json)
  const readA = () => A.get(commissionPath).then(json)
  const setRates = data => request.put(ratesPath, { headers: billing, data }).then(json)
  const opsTenant = id => '/v1/operations/tenants/' + id
  const changePromotion = action => request.post(opsTenant(idB) + ':' + action + '-promotion', { headers: support }).then(json)
  const sale = (open, path = staticSales) => operator.put(path, { data: {
    open, ...(open ? { quote_ready: true, acceptance_evidence: 'Issue #108 local Crown' } : {}),
  } }).then(json)

  try {
    const previousRates = await request.get(ratesPath, { headers: reader }).then(json)
    await expectError(await request.put(ratesPath, { headers: reader, data: {
      first_purchase_bps: 10000, repeat_purchase_bps: 0,
    } }), 403, 'ACCESS_DENIED')
    await expectError(await request.put(ratesPath, { headers: support, data: {
      first_purchase_bps: 0, repeat_purchase_bps: 0,
    } }), 403, 'ACCESS_DENIED')
    for (const data of [
      { first_purchase_bps: -1, repeat_purchase_bps: 10 },
      { first_purchase_bps: 0, repeat_purchase_bps: 10001 },
      { first_purchase_bps: 0, repeat_purchase_bps: 1.25 },
      { repeat_purchase_bps: 10 },
    ]) {
      await expectError(await request.put(ratesPath, { headers: billing, data }), 400, 'INVALID_COMMISSION_RATES')
    }
    expect(await setRates({ first_purchase_bps: 10000, repeat_purchase_bps: 0 })).toMatchObject({
      first_purchase_bps: 10000, repeat_purchase_bps: 0,
    })
    const rates = await setRates({ first_purchase_bps: 1, repeat_purchase_bps: 1234 })
    expect(rates).toMatchObject({ first_purchase_bps: 1, repeat_purchase_bps: 1234, operator_id: 'local-operator-billing' })
    expect(rates.version).not.toBe(previousRates.version)
    expect(await setRates({ first_purchase_bps: 1, repeat_purchase_bps: 1234 })).toEqual(rates)
    expect(await request.get(ratesPath, { headers: reader }).then(json)).toEqual(rates)
    await expectError(await C.get(ratesPath), 403, 'ACCESS_DENIED')

    const tier = await setTier(operator, '5gb', '1500')
    const traffic = (request_key, tenant = C) => tenant.post(purchasePath, {
      data: { package_id: '5gb', package_version: tier.version, request_key },
    }).then(json)
    await setSale(operator, true)
    await sale(true)
    await recharge(operator, B, '3000')
    await recharge(operator, C, '12000')
    const fromB = await traffic(randomUUID(), B)
    expect((await readA()).records).toHaveLength(1)

    const rejectNext = new URL('/fixture/purchases/reject-next', process.env.VERIFY_IPCROWN_URL)
    await request.post(rejectNext.href, { data: { node_id: 111 } }).then(json)
    const rejected = await buyStatic(C, '111', 1)
    expect(rejected).toMatchObject({ status: 'failed', debited_minor: '900', released_minor: '900', settled_at: expect.any(String) })
    expect(rejected.settled_minor ?? '0').toBe('0')
    expect(await readB()).toMatchObject({ available_minor: '0', records: [] })

    const pending = await buyStatic(C, '101', 3)
    expect(pending).toMatchObject({ status: 'pending', delivered_count: '2', locked_minor: '3000' })
    expect(pending).not.toHaveProperty('settled_at')
    expect((await readB()).records).toEqual([])
    const firstKey = randomUUID()
    const [first, replay] = await Promise.all([traffic(firstKey), traffic(firstKey)])
    expect(first).toEqual(replay)
    expect(Number(first.id)).toBeGreaterThan(Number(pending.id))
    let { records } = await readB()
    expect(records).toEqual([expect.objectContaining({
      classification: 'first_purchase', settlement_sequence: '1', referral_source: 'manual', product_kind: 'traffic_package',
      net_debit_minor: '1500', commission_minor: '0', status: 'zero_commission', rate_bps: 1,
      rate_version: rates.version, rule_version: 1, settled_at: first.settled_at,
    })])
    const anonymous = records[0].customer_key
    expect(anonymous).toMatch(/^[0-9a-f-]{36}$/)
    expect(anonymous).not.toBe(idC)

    const resolvePath = opsTenant(idC) + '/orders/' + pending.id + '/static-resolution'
    const closed = await operator.post(resolvePath, { data: {
      close_remaining: true, confirmed_undeliverable: true, evidence: 'Crown mock confirmed remainder unfulfilled',
    } }).then(json)
    expect(closed).toMatchObject({ status: 'partial', settled_minor: '2000', released_minor: '1000' })
    expect((await readB()).records[0]).toMatchObject({
      classification: 'repeat_purchase', settlement_sequence: '2', product_kind: 'static_purchase', settled_at: closed.settled_at,
      net_debit_minor: '2000', rate_bps: 1234, commission_minor: '247', rate_version: rates.version, customer_key: anonymous,
    })

    const nextRates = await setRates({ first_purchase_bps: 10000, repeat_purchase_bps: 5000 })
    await changePromotion('pause')
    const { allocations } = await C.get('/v1/tenant/static-ips').then(json)
    const renewed = await renewStatic(C, allocations.find(item => item.order_id === pending.id).allocation_id)
    expect(renewed).toMatchObject({ status: 'succeeded', settled_minor: '1000' })
    expect((await readB()).records[0]).toMatchObject({
      classification: 'repeat_purchase', settlement_sequence: '3', product_kind: 'static_renewal', status: 'promotion_paused',
      net_debit_minor: '1000', settled_at: renewed.settled_at, rate_bps: 0, commission_minor: '0', rate_version: nextRates.version,
    })
    await changePromotion('resume')
    await Promise.all([traffic(randomUUID()), traffic(randomUUID())])

    const page = await readB()
    records = page.records
    expect(page).toMatchObject({ available_minor: '1747', next_before_id: '0' })
    expect(records.map(row => row.settlement_sequence)).toEqual(['5', '4', '3', '2', '1'])
    expect(records.slice(0, 2)).toEqual(Array(2).fill(expect.objectContaining({
      product_kind: 'traffic_package', classification: 'repeat_purchase', rate_bps: 5000, commission_minor: '750',
      customer_key: anonymous, status: 'credited', rate_version: nextRates.version,
    })))
    const a = await readA()
    expect(a.records).toEqual([expect.objectContaining({
      referral_source: 'link', product_kind: 'traffic_package', settled_at: fromB.settled_at,
    })])
    expect(a.records[0].customer_key).not.toBe(anonymous)
    expect(await A.get(commissionPath + '?customer_key=' + anonymous).then(json)).toMatchObject({ records: [] })
    expect(await C.get(commissionPath).then(json)).toMatchObject({ available_minor: '0', records: [] })
    await expectError(await C.get(opsTenant(idB) + '/commissions'), 403, 'ACCESS_DENIED')
    expect(await request.get(opsTenant(idB) + '/commissions?customer_key=' + anonymous, { headers: reader }).then(json)).toEqual(page)
    expect(JSON.stringify(records)).not.toContain(idC)
    expect(JSON.stringify(records)).not.toContain(emailC)
    expect(records.every(record => !Object.hasOwn(record, 'order_id'))).toBe(true)

    await info.attach('commission-api-evidence', {
      body: JSON.stringify({ mockMail: 'Mailpit', mockSupplier: 'IPCrown fixture', rejected, first, closed,
        commissionPage: page, directOnly: a.records.length === 1 }, null, 2),
      contentType: 'application/json',
    })
  } finally {
    await Promise.all([setSale(operator, false), sale(false)])
    await Promise.all([A.dispose(), B.dispose(), C.dispose(), operator.dispose()])
  }
})
