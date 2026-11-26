import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'

const transferPath = '/v1/tenant/commission-transfers'
const commissionPath = '/v1/tenant/commissions'
const walletPath = '/v1/tenant/wallet'
const walletLedgerPath = walletPath + '/ledger'
const ratesPath = '/v1/operations/commission-rates'
const transferFilter = '?kind=commission_transfer'

async function registerCustomer({ request, playwright, baseURL }, email, invite) {
  const { tenant_id: id } = await json(await registerEmail(request, { email, invite }))
  const { access_token: token } = await request.post('/v1/access/login', {
    data: { email, password: 'proxy-local-test' },
  }).then(json)
  const api = await playwright.request.newContext({
    baseURL, ignoreHTTPSErrors: true, extraHTTPHeaders: { Authorization: 'Bearer ' + token },
  })
  return { id, token, api }
}

async function failure(response, status, reason) {
  const body = await response.text()
  expect(response.status(), body).toBe(status)
  const actual = JSON.parse(body)
  expect(actual.reason).toBe(reason)
  return { status, reason: actual.reason }
}

async function ledger(request, path = walletLedgerPath, options) {
  const { entries } = await request.get(path, options).then(json)
  return entries ?? []
}

function walletAmounts(wallet) {
  return {
    available: wallet.available_minor ?? '0',
    totalRecharge: wallet.total_recharge_minor,
    totalSpend: wallet.total_spend_minor,
  }
}

async function loseTransferReceipt(page, token, data) {
  await page.goto(walletPath)
  let first
  await page.route(url => url.pathname === transferPath, async route => {
    try {
      const response = await route.fetch({ maxRetries: 0, maxRedirects: 0 })
      try {
        first = { status: response.status(), data: route.request().postDataJSON(), receipt: await response.json() }
      } finally {
        await response.dispose()
      }
    } finally {
      await route.abort('failed')
    }
  }, { times: 1 })

  const outcome = await page.evaluate(({ path, token, data }) => fetch(path, {
    method: 'POST', headers: { Authorization: 'Bearer ' + token, 'Content-Type': 'application/json' }, body: JSON.stringify(data),
  }).then(response => response.status, () => 'no-receipt'), { path: transferPath, token, data })
  expect(outcome).toBe('no-receipt')
  expect(first).toMatchObject({ status: 200, data })
  return first.receipt
}

test('暂停推广仍可转入，争用和回执丢失只各记一笔，转入余额消费继续计佣', async ({ page, request, playwright, baseURL }, info) => {
  const operator = await authenticatedApi(playwright, baseURL)
  const customers = []
  const register = async (suffix, invite) => {
    const customer = await registerCustomer({ request, playwright, baseURL }, `transfer-${suffix}@example.test`, invite)
    customers.push(customer.api)
    return customer
  }
  const beforeRates = await operator.get(ratesPath).then(json)
  const setRates = (first, repeat) => operator.put(ratesPath, {
    data: { first_purchase_bps: first, repeat_purchase_bps: repeat },
  }).then(json)

  try {
    const a = await register('a')
    const { invite_code: codeA } = await a.api.get('/v1/tenant/invite').then(json)
    const b = await register('b', { link_invite_code: codeA })
    const { invite_code: codeB } = await b.api.get('/v1/tenant/invite').then(json)
    const c = await register('c', { invite_code: codeB })
    const { access_token: readerToken } = await request.post('/v1/operations/access/login', {
      data: { username: 'operator-readonly', password: 'proxy-local-test' },
    }).then(json)
    const reader = { headers: { Authorization: 'Bearer ' + readerToken } }
    const opsTenant = '/v1/operations/tenants/' + b.id
    const opsLedger = opsTenant + '/wallet/ledger'
    const readBCommission = () => b.api.get(commissionPath).then(json)
    const readBWallet = () => b.api.get(walletPath).then(json).then(walletAmounts)
    const transfer = data => b.api.post(transferPath, { data })
    const buy = (customer, tier) => customer.api.post(purchasePath, {
      data: { package_id: '5gb', package_version: tier.version, request_key: randomUUID() },
    }).then(json)

    const funded = await test.step('用已结算订单为直接推荐人产生 4000 分佣金，再暂停推广', async () => {
      await setRates(10000, 10000)
      const tier = await setTier(operator, '5gb', '4000')
      await setSale(operator, true)
      await recharge(operator, c.api, '5000')
      const order = await buy(c, tier)
      expect(order).toMatchObject({ status: 'succeeded', settled_minor: '4000' })
      expect(walletAmounts(await c.api.get(walletPath).then(json)).totalRecharge).toBe('5000')
      const bCommission = await readBCommission()
      expect(bCommission).toMatchObject({ available_minor: '4000', records: [expect.objectContaining({
        product_kind: 'traffic_package', commission_minor: '4000', net_debit_minor: '4000', settled_at: order.settled_at,
      })] })
      expect(await a.api.get(commissionPath).then(json)).toMatchObject({ available_minor: '0', records: [] })
      await operator.post(opsTenant + ':pause-promotion').then(json)
      expect(await b.api.get('/v1/tenant/invite').then(json)).toMatchObject({ promotion_status: 'paused' })
      const wallet = await readBWallet()
      expect(wallet).toEqual({ available: '0', totalRecharge: '0', totalSpend: '0' })
      return { order, bCommission, wallet }
    })

    const rejected = await test.step('拒绝越权、无效金额与超额，余额和流水保持原样', async () => {
      const amount = { request_key: randomUUID(), amount_minor: '1' }
      const errors = {
        unauthenticated: await failure(await request.post(transferPath, { data: amount }), 401, 'AUTHENTICATION_REQUIRED'),
        operator: await failure(await operator.post(transferPath, { data: amount }), 403, 'ACCESS_DENIED'),
        anotherTenant: await failure(await c.api.post(transferPath, { data: amount }), 409, 'COMMISSION_BALANCE_INSUFFICIENT'),
        operatorLedger: await failure(await c.api.get(opsLedger + transferFilter), 403, 'ACCESS_DENIED'),
        zero: await failure(await transfer({ ...amount, amount_minor: '0' }), 400, 'INVALID_WALLET_REQUEST'),
        negative: await failure(await transfer({ ...amount, amount_minor: '-1' }), 400, 'INVALID_WALLET_REQUEST'),
        overdraw: await failure(await transfer({ ...amount, amount_minor: '4001' }), 409, 'COMMISSION_BALANCE_INSUFFICIENT'),
      }
      expect(await readBWallet()).toEqual(funded.wallet)
      expect(await readBCommission()).toEqual(funded.bCommission)
      expect(await ledger(b.api)).toEqual([])
      return errors
    })

    const competition = await test.step('同一余额接收不同键的两笔 2500 分请求，只允许一笔入账', async () => {
      const inputs = Array.from({ length: 2 }, () => ({ request_key: randomUUID(), amount_minor: '2500' }))
      const results = await Promise.all(inputs.map(transfer))
      expect(results.map(response => response.status()).sort()).toEqual([200, 409])
      const successful = results.find(response => response.status() === 200)
      const rejected = results.find(response => response.status() === 409)
      const receipt = await json(successful)
      const error = await failure(rejected, 409, 'COMMISSION_BALANCE_INSUFFICIENT')
      expect(receipt).toMatchObject({ id: expect.stringMatching(/^\d+$/), kind: 'commission_transfer', after_minor: '2500' })
      expect(receipt.before_minor ?? '0').toBe('0')
      const commissions = await readBCommission()
      const wallet = await readBWallet()
      expect(commissions).toMatchObject({ available_minor: '1500', records: funded.bCommission.records })
      expect(wallet).toEqual({ available: '2500', totalRecharge: '0', totalSpend: '0' })
      expect(await ledger(b.api, walletLedgerPath + transferFilter)).toEqual([receipt])
      return { inputs, receipt, error, wallet, commissions }
    })

    const replay = await test.step('服务端处理成功后丢弃回执，以原请求键重试且不重复扣款', async () => {
      const original = { request_key: randomUUID(), amount_minor: '1500' }
      const receipt = await loseTransferReceipt(page, b.token, original)
      expect(receipt).toMatchObject({ id: expect.stringMatching(/^\d+$/), kind: 'commission_transfer',
        before_minor: '2500', after_minor: '4000' })
      expect(receipt.id).not.toBe(competition.receipt.id)
      expect(await readBCommission()).toMatchObject({ available_minor: '0', records: funded.bCommission.records })
      const wallet = await readBWallet()
      expect(wallet).toEqual({ available: '4000', totalRecharge: '0', totalSpend: '0' })
      const duplicate = await transfer(original).then(json)
      expect(duplicate).toEqual(receipt)
      const conflict = await failure(await transfer({ ...original, amount_minor: '1499' }), 409, 'WALLET_REQUEST_CONFLICT')
      expect(await readBCommission()).toMatchObject({ available_minor: '0', records: funded.bCommission.records })
      expect(await readBWallet()).toEqual(wallet)
      const customerEntries = await ledger(b.api, walletLedgerPath + transferFilter)
      expect(customerEntries).toEqual([receipt, competition.receipt])
      const operatorEntries = await ledger(request, opsLedger + transferFilter, reader)
      expect(operatorEntries).toEqual(customerEntries)
      expect((await request.get(opsTenant, reader).then(json)).available_minor).toBe(wallet.available)
      expect(await ledger(b.api, walletLedgerPath + '?kind=recharge')).toEqual([])
      expect(await ledger(c.api, walletLedgerPath + transferFilter)).toEqual([])
      return { original, receipt, duplicate, conflict, wallet, customerEntries, operatorEntries }
    })

    const purchase = await test.step('仅使用转入余额买套餐，核对订单、钱包与直接推荐人的佣金', async () => {
      const tier = await setTier(operator, '5gb', '3500')
      const order = await buy(b, tier)
      expect(order).toMatchObject({ kind: 'traffic_package', status: 'succeeded', settled_minor: '3500', debited_minor: '3500' })
      expect(await b.api.get('/v1/tenant/orders/' + order.id).then(json)).toMatchObject({
        id: order.id, status: 'succeeded', settled_minor: '3500', settled_at: order.settled_at,
      })
      const wallet = await readBWallet()
      expect(wallet).toEqual({ available: '500', totalRecharge: '0', totalSpend: '3500' })
      const entries = await ledger(b.api)
      expect(entries).toEqual([expect.objectContaining({ kind: 'debit', order_id: order.id,
        before_minor: '4000', after_minor: '500' }), ...replay.customerEntries])
      expect(await ledger(request, opsLedger + transferFilter, reader)).toEqual(replay.operatorEntries)
      const aCommission = await a.api.get(commissionPath).then(json)
      expect(aCommission).toMatchObject({ available_minor: '3500', records: [expect.objectContaining({
        product_kind: 'traffic_package', status: 'credited', classification: 'first_purchase', referral_source: 'link',
        commission_minor: '3500', net_debit_minor: '3500', settled_at: order.settled_at,
      })] })
      expect(await readBCommission()).toMatchObject({ available_minor: '0', records: funded.bCommission.records })
      return { order, wallet, entries, aCommission }
    })

    await info.attach('commission-transfer-api-evidence', {
      body: JSON.stringify({ funded, rejected, competition, replay, purchase }, null, 2), contentType: 'application/json',
    })
  } finally {
    try {
      await setSale(operator, false)
      await setRates(beforeRates.first_purchase_bps, beforeRates.repeat_purchase_bps)
    } finally {
      await Promise.all([...customers, operator].map(api => api.dispose()))
    }
  }
})
