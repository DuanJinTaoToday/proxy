import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { screenshot, signInOperator } from '../support/console.mjs'

const password = 'proxy-local-test'
const extend30Days = time => new Date(Date.parse(time) + 30 * 86_400_000).toISOString()

async function json(response) {
  expect(response.ok(), `${response.url()} returned ${response.status()}`).toBe(true)
  return response.json()
}

async function authenticate(request, path, credentials) {
  const { access_token } = await json(await request.post(path, { data: { ...credentials, password } }))
  return { Authorization: `Bearer ${access_token}` }
}

function setOrderAge(id, hours) {
  expect(id).toMatch(/^\d+$/)
  const updated = execFileSync('docker', [
    'compose', '-p', 'proxy-verify-' + process.env.VERIFY_ID,
    'exec', '-T', 'db', 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-At', '-c',
    `UPDATE purchase_orders SET created_at = now() - make_interval(hours => ${hours}) WHERE id = ${id} AND kind = 'static_renewal'`,
  ], { cwd: new URL('../../../', import.meta.url), encoding: 'utf8' })
  expect(updated.trim()).toBe('UPDATE 1')
}

test('续费满 24 小时转运营，按 IP 确认与退款，迟到和重复通知不改变结果', async ({ page, request, browser }, info) => {
  const staff = await authenticate(request, '/v1/operations/access/login', { username: 'operator' })
  const readonly = await authenticate(request, '/v1/operations/access/login', { username: 'operator-readonly' })
  const phone = '13800009201'
  const { tenant_id } = await json(await request.post('/v1/access/register', { data: { phone, password } }))
  const customer = await authenticate(request, '/v1/access/login', { phone })
  const tenantPath = `/v1/operations/tenants/${tenant_id}`
  const recharge = { tenant_id, request_key: randomUUID(), amount_minor: '10000', reason: 'Issue #92 verification' }
  const { confirmation_token } = await json(await request.post(`${tenantPath}/wallet/recharge-quotes`, { headers: staff, data: recharge }))
  await json(await request.post(`${tenantPath}/wallet/recharges`, {
    headers: staff, data: { tenant_id, request_key: recharge.request_key, confirmation_token },
  }))

  const salePath = '/v1/operations/product-sales/static'
  const fixture = async (path, data) => json(await request.post(new URL(path, process.env.VERIFY_IPCROWN_URL).href, { data }))
  const ips = async () => (await json(await request.get('/v1/tenant/static-ips', { headers: customer }))).allocations
  const balance = async () => (await json(await request.get('/v1/tenant/wallet', { headers: customer }))).available_minor

  let order
  let confirmed
  let refunded
  await json(await request.put(salePath, { headers: staff, data: { open: true, quote_ready: true, acceptance_evidence: 'Controlled supplier verification' } }))
  try {
    const selection = { node_id: '111', term: 'yp1', quantity: 2 }
    const buyQuote = await json(await request.post('/v1/tenant/static-purchase-quotes', { headers: customer, data: selection }))
    const purchase = await json(await request.post('/v1/tenant/static-orders', {
      headers: customer, data: { ...selection, unit_minor: buyQuote.offer.unit_minor, confirmation_token: buyQuote.confirmation_token, request_key: randomUUID() },
    }))
    expect(purchase.status).toBe('succeeded')
    const allocated = await ips()
    expect(allocated).toHaveLength(2)
    confirmed = allocated[0]
    refunded = allocated[1]
    await fixture('/fixture/renewals/delay', { id: Number(refunded.supplier_item_id) })
    const renewal = { supplier_item_ids: [confirmed.supplier_item_id, refunded.supplier_item_id], term: 'yp1' }
    const quote = await json(await request.post('/v1/tenant/static-renewal-quotes', { headers: customer, data: renewal }))
    await fixture('/fixture/detail', { available: false })
    order = await json(await request.post('/v1/tenant/static-renewal-orders', {
      headers: customer, data: { ...renewal, confirmation_token: quote.confirmation_token, request_key: randomUUID() },
    }))
    expect(order).toMatchObject({ status: 'pending', unsettled_minor: '1800' })
  } finally {
    await fixture('/fixture/detail', { available: true })
    await json(await request.put(salePath, { headers: staff, data: { open: false } }))
  }
  expect(await balance()).toBe('6400')

  const resolution = `${tenantPath}/orders/${order.id}/renewal-resolution`
  const close = { supplier_item_id: refunded.supplier_item_id, close_unknown: true, evidence: '受控供应商晚到结果' }
  const targetInQueue = queue => queue.getByRole('row').filter({ has: queue.getByText(order.id, { exact: true }) })
  const dialog = page.getByRole('dialog', { name: '核实静态订单' })

  setOrderAge(order.id, 23)
  await signInOperator(page, '/ops/orders')
  await expect(page.getByRole('heading', { name: '确认中的静态订单' })).toBeVisible()
  await expect(targetInQueue(page)).toHaveCount(0)

  setOrderAge(order.id, 25)
  await page.reload()
  await expect(targetInQueue(page)).toContainText('待运营核实')
  await targetInQueue(page).getByRole('button', { name: '核实' }).click()
  await expect(dialog.getByText(confirmed.ip, { exact: true })).toBeVisible()
  await expect(dialog.getByText(refunded.ip, { exact: true })).toBeVisible()
  await screenshot(page, info, '12b-renewal-dialog')
  await dialog.getByRole('row').filter({ hasText: confirmed.ip }).getByRole('radio').click()
  await dialog.getByRole('textbox', { name: /核实依据/ }).fill('核对皇冠明细的顺延时间')
  await dialog.getByRole('button', { name: '保存', exact: true }).click()
  await expect(dialog).toBeHidden()
  await expect(page.getByText('已核实续费成功', { exact: true })).toBeVisible()
  await expect(targetInQueue(page)).toContainText('1 / 2')

  const early = await request.post(resolution, { headers: staff, data: close })
  expect(early.status()).toBe(409)
  expect((await early.json()).reason).toBe('WALLET_ORDER_STATE')

  const withoutPermission = await browser.newContext({ ignoreHTTPSErrors: true, baseURL: new URL(page.url()).origin })
  try {
    const reader = await withoutPermission.newPage()
    await signInOperator(reader, '/ops/orders', 'operator-readonly')
    await targetInQueue(reader).getByRole('button', { name: '核实' }).click()
    await expect(reader.getByRole('dialog').getByRole('button', { name: '保存', exact: true })).toBeDisabled()
    const denied = await request.post(resolution, { headers: readonly, data: close })
    expect(denied.status()).toBe(403)
  } finally {
    await withoutPermission.close()
  }

  setOrderAge(order.id, 73)
  await page.reload()
  await expect(targetInQueue(page)).toContainText('可关闭')
  await targetInQueue(page).getByRole('button', { name: '核实' }).click()
  await expect(dialog.getByRole('row').filter({ hasText: confirmed.ip }).getByRole('radio')).toHaveCount(0)
  await dialog.getByRole('row').filter({ hasText: refunded.ip }).getByRole('radio').click()
  await dialog.getByRole('radio', { name: '关闭该明细，退回对应金额' }).click()
  await dialog.getByRole('textbox', { name: /核实依据/ }).fill(close.evidence)
  await expect(dialog.getByText('¥9.00', { exact: true }).last()).toBeVisible()
  await screenshot(page, info, 'renewal-close-preview')
  await dialog.getByRole('button', { name: '确认关闭' }).click()
  await expect(dialog).toBeHidden()
  await expect(targetInQueue(page)).toHaveCount(0)
  await expect(page.getByText('已关闭明细并退回钱包', { exact: true })).toBeVisible()
  await screenshot(page, info, 'renewal-queue-after-refund')

  await json(await request.post(resolution, { headers: staff, data: close }))
  await fixture('/fixture/renewals/confirm', { id: Number(refunded.supplier_item_id) })
  await json(await request.post(`/v1/tenant/static-renewal-orders/${order.id}/recheck`, { headers: customer, data: {} }))
  await json(await request.post(resolution, { headers: staff, data: { supplier_item_id: refunded.supplier_item_id, evidence: '退款后收到供应商确认' } }))

  const final = await json(await request.get(`/v1/tenant/orders/${order.id}`, { headers: customer }))
  expect(final).toMatchObject({ status: 'partial', delivered_count: '1', settled_minor: '900', refunded_minor: '900' })
  expect(final.items).toEqual(expect.arrayContaining([
    expect.objectContaining({ supplier_item_id: confirmed.supplier_item_id, outcome: 'succeeded' }),
    expect.objectContaining({ supplier_item_id: refunded.supplier_item_id, outcome: 'failed' }),
  ]))
  expect(final.wallet_entries).toHaveLength(2)
  expect(final.wallet_entries.map(entry => entry.amount_minor).sort()).toEqual(['-1800', '900'])
  const currentIPs = await ips()
  expect(currentIPs.find(ip => ip.supplier_item_id === confirmed.supplier_item_id)).toMatchObject({
    ip: confirmed.ip, expires_at: extend30Days(confirmed.expires_at),
  })
  expect(currentIPs.find(ip => ip.supplier_item_id === refunded.supplier_item_id)).toMatchObject({
    ip: refunded.ip, expires_at: refunded.expires_at,
  })
  expect(await balance()).toBe('7300')
})
