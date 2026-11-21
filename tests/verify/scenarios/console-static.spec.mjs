
import { randomUUID } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, test } from '@playwright/test'
import { screenshot, signInTenant } from '../support/console.mjs'

const catalogPath = '/v1/public/static-offers'
const tenantOrderPath = '/v1/tenant/static-orders'
const opsTenant = '/v1/operations/tenants/alpha'
const password = 'proxy-local-test'

async function json(response) {
  expect(response.ok(), `${response.url()} returned ${response.status()}`).toBe(true)
  return response.json()
}

async function authenticated(request, path, credentials) {
  const result = await json(await request.post(path, { data: { ...credentials, password } }))
  return { Authorization: `Bearer ${result.access_token}` }
}

async function openStaticSales(request) {
  const headers = await authenticated(request, '/v1/operations/access/login', { username: 'operator' })
  const request_key = randomUUID()
  const { confirmation_token } = await json(await request.post(`${opsTenant}/wallet/recharge-quotes`, {
    headers, data: { tenant_id: 'alpha', request_key, amount_minor: '30000', reason: 'Issue #74 verification' },
  }))
  await json(await request.post(`${opsTenant}/wallet/recharges`, {
    headers, data: { tenant_id: 'alpha', request_key, confirmation_token },
  }))
  await json(await request.put('/v1/operations/product-sales/static', {
    headers, data: { open: true, quote_ready: true, acceptance_evidence: 'Issue #74 isolated supplier fixture' },
  }))
  return headers
}

async function fixture(request, path, data = {}) {
  await json(await request.post(new URL(path, process.env.VERIFY_IPCROWN_URL).href, { data }))
}

async function buyByApi(request, headers, selection) {
  const quote = await json(await request.post('/v1/tenant/static-purchase-quotes', { headers, data: selection }))
  return json(await request.post(tenantOrderPath, {
    headers,
    data: { ...selection, unit_minor: quote.offer.unit_minor, confirmation_token: quote.confirmation_token, request_key: randomUUID() },
  }))
}

const rowFor = (page, ip) => page.getByRole('row').filter({ hasText: ip })

test('静态代理教程中英文可完整切换并返回 IP 列表', async ({ page }, info) => {
  await signInTenant(page)
  await page.goto('/console/static/guide')
  for (const name of ['使用静态住宅 IP', '查看接入信息', '配置你的代理客户端', '到期前续费']) {
    await expect(page.getByRole('heading', { name })).toBeVisible()
  }
  await expect(page.getByText('IP:端口:用户名:密码', { exact: true })).toBeVisible()
  await screenshot(page, info, 'guide-zh-CN')

  await page.getByRole('button', { name: /^语言/ }).click()
  await page.getByRole('menuitem', { name: 'English', exact: true }).click()
  await expect(page).toHaveURL(/\/console\/static\/guide$/)
  for (const name of ['Use static residential IPs', 'Find your connection details', 'Configure your proxy client', 'Renew before expiry']) {
    await expect(page.getByRole('heading', { name })).toBeVisible()
  }
  await expect(page.getByText('IP:port:username:password', { exact: true })).toBeVisible()
  await screenshot(page, info, 'guide-en')
  await page.getByRole('link', { name: 'Back to IP list' }).click()
  await expect(page).toHaveURL(/\/console\/static\/ips$/)
})



async function closeStaticSales(request, headers) {
  await json(await request.put('/v1/operations/product-sales/static', { headers, data: { open: false } }))
}

test('逐次复核价格和库存，实交两条且未交付一条退款', async ({ page, request }, info) => {
  await signInTenant(page)
  await page.goto('/console/static/pricing')
  await expect(page.getByText('静态住宅代理暂未开放购买')).toBeVisible()
  const ops = await openStaticSales(request)
  const tenant = await authenticated(request, '/v1/access/login', { phone: '13800000001' })
  const orders = async () => (await json(await request.get('/v1/tenant/orders?kind=static_purchase', { headers: tenant }))).orders ?? []

  try {
    await page.reload()
    const chooseUS = page.getByRole('button', { name: '选择 United States' })
    const chooseJP = page.getByRole('button', { name: '选择 Japan' })
    await expect(chooseUS).toBeVisible()
    const region = page.getByRole('combobox', { name: '地区', exact: true })
    await region.click()
    await page.getByRole('option', { name: 'Japan' }).click()
    await expect(chooseJP).toBeVisible()
    await expect(chooseUS).toHaveCount(0)

    await page.getByText('共享静态 IP', { exact: true }).click()
    await expect(page.getByRole('button', { name: '选择 Singapore' })).toBeVisible()
    const purposeQuery = page.waitForResponse(response => {
      const url = new URL(response.url())
      return url.pathname === catalogPath && url.searchParams.get('purpose_id') === '10' && response.ok()
    })
    await page.getByRole('combobox', { name: '用途' }).click()
    await page.getByRole('option', { name: 'ChatGPT' }).click()
    const shared = await (await purposeQuery).json()
    expect(shared.offers.find(offer => offer.term === 'yp1')).toMatchObject({
      share_mode: 'shared', purpose_id: 10, unit_minor: '1100', available: 5,
    })
    await page.getByRole('combobox', { name: '租期' }).click()
    await page.getByRole('option', { name: '30 天', exact: true }).click()
    await expect(page.locator('.offer').filter({ has: page.getByRole('heading', { name: 'Singapore' }) }))
      .toContainText(/¥11\.00.*5/s)
    await screenshot(page, info, 'shared-chatgpt-price-stock')

    await page.getByText('独享静态 IP', { exact: true }).click()
    await chooseUS.click()
    const quantity = page.getByRole('spinbutton', { name: '数量' })
    await quantity.fill('3')
    await quantity.press('Tab')
    const beforeOrders = await orders()
    const beforeWallet = await json(await request.get('/v1/tenant/wallet', { headers: tenant }))
    await page.getByRole('button', { name: '立即购买' }).click()
    const dialog = page.getByRole('dialog', { name: '确认购买' })
    const pay = dialog.getByRole('button', { name: '确认支付' })
    await expect(dialog.getByText('¥30.00', { exact: true })).toBeVisible()

    await fixture(request, '/fixture/catalog', { price: 12, stock: 6 })
    await pay.click()
    await expect(dialog.getByText('价格或库存已更新，请重新确认。')).toBeVisible()
    await expect(dialog.getByText('¥36.00', { exact: true })).toBeVisible()
    expect(await orders()).toEqual(beforeOrders)

    await fixture(request, '/fixture/catalog', { price: 12, stock: 4 })
    await pay.click()
    await expect(dialog.locator('dl > div').filter({ hasText: '可购库存' }).locator('dd')).toHaveText('4')
    expect(await orders()).toEqual(beforeOrders)
    await screenshot(page, info, 'purchase-reconfirmed-price-stock')

    let originalBody, originalOrder
    await page.route(tenantOrderPath, async route => {
      originalBody = route.request().postDataJSON()
      originalOrder = await json(await route.fetch())
      await route.abort()
    }, { times: 1 })
    await pay.click()
    await expect(page.getByRole('dialog', { name: '提交结果未知' })).toBeVisible()
    expect(originalOrder).toMatchObject({ status: 'pending', delivered_count: '2' })

    await page.reload()
    const replayed = page.waitForResponse(response => response.url().endsWith(tenantOrderPath)
      && response.request().method() === 'POST' && response.ok())
    await page.getByRole('button', { name: '重试原提交' }).click()
    const replay = await replayed
    expect(replay.request().postDataJSON()).toEqual(originalBody)
    const order = await replay.json()
    expect(order.id).toBe(originalOrder.id)
    await expect(page.getByText('已交付 2 个 IP')).toBeVisible()
    await json(await request.post(`${opsTenant}/orders/${order.id}/static-resolution`, {
      headers: ops,
      data: { evidence: 'Fixture confirmed remaining IP could not be delivered', close_remaining: true, confirmed_undeliverable: true },
    }))
    await page.getByRole('button', { name: '刷新订单' }).click()
    await expect(page.getByRole('heading', { name: '部分交付' })).toBeVisible()
    await expect(page.getByText('已交付 2 个 IP')).toBeVisible()
    await expect(page.getByText('退款金额 ¥12.00')).toBeVisible()
    const afterWallet = await json(await request.get('/v1/tenant/wallet', { headers: tenant }))
    expect(BigInt(beforeWallet.available_minor) - BigInt(afterWallet.available_minor)).toBe(2400n)
    expect(await orders()).toHaveLength(beforeOrders.length + 1)
    await screenshot(page, info, 'purchase-partial-refund')
    await page.getByRole('link', { name: '查看 IP 列表' }).click()
    await expect(rowFor(page, '192.0.2.101')).toBeVisible()
    await expect(rowFor(page, '192.0.2.102')).toBeVisible()
  } finally {
    await closeStaticSales(request, ops)
  }
})


test('IP 筛选、复制与导出，批量续费逐项结算并从列表重查', async ({ page, request, context }, info) => {
  const ops = await openStaticSales(request)
  const tenant = await authenticated(request, '/v1/access/login', { phone: '13800000001' })
  const jp = ['198.51.100.201', '198.51.100.202', '198.51.100.203']

  try {
    await fixture(request, '/fixture/catalog', { price: 10, stock: 6 })
    const japan = await buyByApi(request, tenant, { node_id: '201', term: 'yp1', quantity: 3 })
    expect(japan).toMatchObject({ status: 'succeeded', delivered_count: '3' })
    const us = await buyByApi(request, tenant, { node_id: '101', term: 'yp1', quantity: 1 })
    expect(us).toMatchObject({ status: 'succeeded', delivered_count: '1' })
    const shared = await buyByApi(request, tenant, {
      node_id: '301', term: 'yp1', quantity: 1, share_mode: 'shared', purpose_id: 10,
    })
    expect(shared).toMatchObject({ status: 'succeeded', delivered_count: '1' })
    const allocations = (await json(await request.get('/v1/tenant/static-ips', { headers: tenant }))).allocations
    const usIP = allocations.find(ip => ip.order_id === us.id).ip
    const sharedIP = allocations.find(ip => ip.order_id === shared.id).ip

    await signInTenant(page)
    await page.goto('/console/static/ips')
    await expect(rowFor(page, jp[0])).toContainText('即将到期')
    await page.getByRole('combobox', { name: '按状态筛选' }).click()
    await page.getByRole('option', { name: '即将到期' }).click()
    await expect(rowFor(page, jp[0])).toBeVisible()
    await expect(rowFor(page, jp[1])).toHaveCount(0)
    await page.getByRole('combobox', { name: '按状态筛选' }).click()
    await page.getByRole('option', { name: '全部状态' }).click()
    await page.getByRole('combobox', { name: '按地区筛选' }).click()
    await page.getByRole('option', { name: 'Japan' }).click()
    await expect(rowFor(page, usIP)).toHaveCount(0)
    await expect(rowFor(page, jp[1])).toBeVisible()

    const search = page.getByRole('textbox', { name: '搜索 IP 或地区' })
    await search.fill(jp[1])
    await expect(rowFor(page, jp[1])).toBeVisible()
    await expect(rowFor(page, jp[0])).toHaveCount(0)
    await context.grantPermissions(['clipboard-read', 'clipboard-write'])
    const config = page.waitForResponse(response => response.url().endsWith('/20102/proxy-config') && response.ok())
    await rowFor(page, jp[1]).getByRole('button', { name: /^复制/ }).click()
    await config
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(`${jp[1]}:43000:fixture-user:fixture-pass`)
    await search.clear()

    for (const ip of jp) await rowFor(page, ip).getByRole('checkbox').check()
    const downloaded = page.waitForEvent('download')
    await page.getByRole('button', { name: '导出 TXT' }).click()
    const file = await (await downloaded).path()
    const txt = await readFile(file, 'utf8')
    expect(txt.trimEnd().split(/\r?\n/).sort()).toEqual(jp.map(ip => `${ip}:43000:fixture-user:fixture-pass`))
    await screenshot(page, info, 'ip-list-filters-and-selection')

    // US and Japan each have a longer term unavailable in the other region.
    await page.getByRole('combobox', { name: '按地区筛选' }).click()
    await page.getByRole('option', { name: '全部地区' }).click()
    await rowFor(page, usIP).getByRole('checkbox').check()
    await rowFor(page, sharedIP).getByRole('checkbox').check()
    for (const ip of jp) {
      const select = rowFor(page, ip).getByRole('checkbox')
      if (!(await select.isChecked())) await select.check()
    }
    await page.getByRole('button', { name: '批量续费' }).click()
    const renewDialog = page.getByRole('dialog')
    const terms = renewDialog.getByRole('radiogroup', { name: '续费租期' })
    await expect(terms.getByRole('radio')).toHaveCount(1)
    await expect(terms.getByRole('radio', { name: '30 天' })).toBeChecked()
    await renewDialog.getByRole('button', { name: '获取报价' }).click()
    await expect(renewDialog.getByText('¥66.00', { exact: true })).toBeVisible()

    const submitted = page.waitForResponse(response => response.url().endsWith('/v1/tenant/static-renewal-orders') && response.ok())
    await renewDialog.getByRole('button', { name: '确认续费' }).click()
    const order = await (await submitted).json()
    const outcomes = order => Object.fromEntries(order.items.map(item => [item.supplier_item_id, item.outcome]))
    expect(outcomes(order)).toMatchObject({ 20101: 'succeeded', 20102: 'pending', 20103: 'pending' })

    await json(await request.post(`${opsTenant}/orders/${order.id}/renewal-resolution`, {
      headers: ops,
      data: { supplier_item_id: '20102', confirmed_failure: true, evidence: 'Fixture confirmed renewal failed' },
    }))
    await renewDialog.getByRole('button', { name: '重查结果' }).click()
    await expect(rowFor(renewDialog, jp[0])).toContainText('成功')
    await expect(rowFor(renewDialog, jp[1])).toContainText('失败')
    await expect(rowFor(renewDialog, jp[2])).toContainText('确认中')
    await screenshot(page, info, 'renewal-per-ip-outcomes')
    await renewDialog.getByRole('button', { name: '完成' }).click()

    await page.reload()
    await expect(rowFor(page, jp[2])).toContainText('续费确认中')
    await fixture(request, '/fixture/renewals/confirm')
    await rowFor(page, jp[2]).getByRole('button', { name: '重查结果' }).click()
    await renewDialog.getByRole('button', { name: '重查结果' }).click()
    await expect(rowFor(renewDialog, jp[2])).toContainText('成功')
    const settled = await json(await request.get(`/v1/tenant/orders/${order.id}`, { headers: tenant }))
    expect(settled).toMatchObject({ status: 'partial', delivered_count: '4', refunded_minor: '1500' })
    await screenshot(page, info, 'renewal-rechecked-after-reload')
  } finally {
    await closeStaticSales(request, ops)
  }
})
