import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { field, screenshot, signInOperator } from '../support/console.mjs'

const pendingPath = '/v1/operations/orders/static-pending'
const orderPath = '/v1/tenant/static-orders'

async function json(response) {
  expect(response.ok(), `${response.url()} returned ${response.status()}`).toBe(true)
  return response.json()
}

async function login(request, path, identity) {
  const { access_token } = await json(await request.post(path, {
    data: { ...identity, password: 'proxy-local-test' },
  }))
  return { Authorization: `Bearer ${access_token}` }
}

function setAge(order, seconds) {
  expect(order).toMatch(/^\d+$/)
  execFileSync('docker', [
    'compose', '-p', 'proxy-verify-' + process.env.VERIFY_ID,
    'exec', '-T', 'db', 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-c',
    `UPDATE purchase_orders SET created_at = clock_timestamp() - make_interval(secs => ${seconds}) WHERE id = ${order}`,
  ], { cwd: new URL('../../../', import.meta.url) })
}

test('运营在 24 小时后核实静态采购，72 小时后关闭剩余并只退一次', async ({ page, browser, request }, info) => {
  const phone = '13900009101'
  const { tenant_id } = await json(await request.post('/v1/access/register', {
    data: { phone, password: 'proxy-local-test' },
  }))
  const ops = await login(request, '/v1/operations/access/login', { username: 'operator' })
  const reader = await login(request, '/v1/operations/access/login', { username: 'operator-readonly' })
  const tenant = await login(request, '/v1/access/login', { phone })
  const tenantPath = '/v1/operations/tenants/' + tenant_id

  const request_key = randomUUID()
  const { confirmation_token } = await json(await request.post(tenantPath + '/wallet/recharge-quotes', {
    headers: ops, data: { request_key, amount_minor: '10000', reason: 'Issue #91 verification' },
  }))
  await json(await request.post(tenantPath + '/wallet/recharges', {
    headers: ops, data: { request_key, confirmation_token },
  }))
  const salePath = '/v1/operations/product-sales/static'
  await json(await request.put(salePath, {
    headers: ops, data: { open: true, quote_ready: true, acceptance_evidence: 'Issue #91 controlled supplier' },
  }))
  let order
  try {
    const selection = { node_id: '101', term: 'yp1', quantity: 3 }
    const quote = await json(await request.post('/v1/tenant/static-purchase-quotes', { headers: tenant, data: selection }))
    order = await json(await request.post(orderPath, { headers: tenant, data: {
      ...selection, unit_minor: quote.offer.unit_minor, confirmation_token: quote.confirmation_token, request_key: randomUUID(),
    } }))
  } finally {
    await json(await request.put(salePath, { headers: ops, data: { open: false } }))
  }
  expect(order).toMatchObject({ status: 'pending', delivered_count: '2', locked_minor: '3000' })
  const pending = async headers => (await json(await request.get(pendingPath, { headers }))).purchases ?? []
  const row = page.getByRole('row').filter({ has: page.getByText(order.id, { exact: true }) })
  const resolvePath = `${tenantPath}/orders/${order.id}/static-resolution`
  const closeBody = { close_remaining: true, evidence: '皇冠工单验证：剩余部分仍未确认' }

  setAge(order.id, 23 * 3600 + 59 * 60)
  expect((await pending(ops)).some(item => item.order_id === order.id)).toBe(false)
  await signInOperator(page, '/ops/orders')
  await expect(page.getByRole('heading', { name: '确认中的静态订单' })).toBeVisible()
  await expect(row).toHaveCount(0)

  setAge(order.id, 24 * 3600)
  expect((await pending(reader)).find(item => item.order_id === order.id)).toMatchObject({ delivered_count: '2' })
  expect((await request.post(resolvePath, { headers: reader, data: closeBody })).status()).toBe(403)
  const other = await browser.newContext({ baseURL: process.env.VERIFY_URL, ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1000 } })
  try {
    const readonly = await other.newPage()
    await signInOperator(readonly, '/ops/orders', 'operator-readonly')
    await readonly.getByRole('row').filter({ hasText: order.id }).getByRole('button', { name: '核实' }).click()
    await expect(readonly.getByRole('dialog').getByRole('button', { name: '保存' })).toBeDisabled()
  } finally {
    await other.close()
  }

  await page.reload()
  await expect(row).toContainText('待运营核实')
  await row.getByRole('button', { name: '核实' }).click()
  await field(page, '核实依据').fill('已核对皇冠查单：交付两项，剩余仍待确认')
  await page.getByRole('button', { name: '保存' }).click()
  await expect(page.getByText('已同步交付结果，订单仍在确认中')).toBeVisible()
  await expect(row).toContainText('2 / 3')

  setAge(order.id, 71 * 3600 + 59 * 60)
  expect((await request.post(resolvePath, { headers: ops, data: closeBody })).status()).toBe(409)
  await page.reload()
  await expect(row).toContainText('待运营核实')
  await row.getByRole('button', { name: '核实' }).click()
  await page.getByText('关闭剩余，退回未交付部分', { exact: true }).click()
  await expect(page.getByRole('alert').filter({ hasText: '下单未满 72 小时' })).toBeVisible()
  await field(page, '核实依据').fill(closeBody.evidence)
  await expect(page.getByRole('button', { name: '确认关闭' })).toBeDisabled()

  setAge(order.id, 72 * 3600)
  expect((await pending(ops)).find(item => item.order_id === order.id)).toMatchObject({ can_close: true })
  await page.reload()
  await expect(row).toContainText('可关闭退款')
  await expect(row).toContainText('2 / 3')
  await screenshot(page, info, '11-pending-orders')
  const detail = await json(await request.get(`${tenantPath}/orders/${order.id}`, { headers: ops }))
  const { name, primary_account_name } = await json(await request.get(tenantPath, { headers: ops }))
  const { supplier_order_id } = (await pending(ops)).find(item => item.order_id === order.id)
  await info.attach('visual-fixture', {
    body: Buffer.from(JSON.stringify({
      id: order.id, name, primary_account_name, supplier_order_id, created_at: detail.created_at, items: detail.items,
    })),
    contentType: 'application/json',
  })
  await row.getByRole('button', { name: '核实' }).click()
  await screenshot(page, info, '12-resolve-dialog')
  await page.getByText('关闭剩余，退回未交付部分', { exact: true }).click()
  await field(page, '核实依据').fill(closeBody.evidence)
  await page.getByRole('button', { name: '确认关闭' }).click()
  await expect(page.getByRole('dialog')).toBeHidden()
  await expect(page.getByText('订单已关闭，已退回 ¥10.00')).toBeVisible()
  await expect(row).toHaveCount(0)
  await screenshot(page, info, '13-closed-orders')

  const customerOrder = '/v1/tenant/orders/' + order.id
  const settled = await json(await request.get(customerOrder, { headers: tenant }))
  expect(settled).toMatchObject({ status: 'partial', settled_minor: '2000', refunded_minor: '1000', delivered_count: '2' })
  const releases = settled.wallet_entries.filter(entry => entry.kind === 'release')
  expect(releases).toHaveLength(1)
  expect(releases[0]).toMatchObject({ amount_minor: '1000', after_minor: '8000' })
  await request.post(resolvePath, { headers: ops, data: closeBody })
  expect(await json(await request.get(customerOrder, { headers: tenant }))).toEqual(settled)
  expect(await json(await request.get('/v1/tenant/wallet', { headers: tenant }))).toMatchObject({ available_minor: '8000' })
  const allocations = await json(await request.get('/v1/tenant/static-ips', { headers: tenant }))
  const delivered = allocations.allocations.filter(ip => ip.order_id === order.id)
  expect(delivered).toHaveLength(2)
  for (const ip of delivered) {
    expect(Date.parse(ip.expires_at)).toBeGreaterThan(Date.now())
    const config = await json(await request.get('/v1/tenant/static-ips/' + ip.supplier_item_id + '/proxy-config', { headers: tenant }))
    expect(config).toMatchObject({ ip: ip.ip, username: 'fixture-user', password: 'fixture-pass' })
  }
})
