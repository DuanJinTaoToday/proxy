import { registerEmail } from '../support/mail.mjs'

import { randomUUID } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, test } from '@playwright/test'
import { screenshot, signInTenant } from '../support/console.mjs'

const catalogPath = '/v1/public/static-offers'
const tenantOrderPath = '/v1/tenant/static-orders'
const password = 'proxy-local-test'

test.use({ actionTimeout: 7000 })

async function json(response) {
  expect(response.ok(), `${response.url()} returned ${response.status()}`).toBe(true)
  return response.json()
}

async function authenticated(request, path, credentials) {
  const result = await json(await request.post(path, { data: { ...credentials, password } }))
  return { Authorization: `Bearer ${result.access_token}` }
}

async function registerStaticTenant(request, email) {
  const { tenant_id } = await json(await registerEmail(request, { email, password }))
  const headers = await authenticated(request, '/v1/access/login', { email })
  return { id: tenant_id, headers, opsPath: '/v1/operations/tenants/' + tenant_id }
}

async function openStaticSales(request, headers, tenant) {
  const request_key = randomUUID()
  const { confirmation_token } = await json(await request.post(`${tenant.opsPath}/wallet/recharge-quotes`, {
    headers, data: { tenant_id: tenant.id, request_key, amount_minor: '30000', reason: 'Issue #74 verification' },
  }))
  await json(await request.post(`${tenant.opsPath}/wallet/recharges`, {
    headers, data: { tenant_id: tenant.id, request_key, confirmation_token },
  }))
  await json(await request.put('/v1/operations/product-sales/static', {
    headers, data: { open: true, quote_ready: true, acceptance_evidence: 'Issue #74 isolated supplier fixture' },
  }))
}

const staticSalesTest = test.extend({
  openSales: async ({ request }, use) => {
    let headers
    try {
      await use(async tenant => {
        headers = await authenticated(request, '/v1/operations/access/login', { username: 'operator' })
        await openStaticSales(request, headers, tenant)
        return headers
      })
    } finally {
      if (headers) {
        await json(await request.put('/v1/operations/product-sales/static', { headers, data: { open: false } }))
      }
    }
  },
})

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

async function selectOption(page, combobox, option) {
  await page.getByRole('combobox', { name: combobox, exact: true }).press('Enter', { timeout: 5000 })
  await page.getByRole('option', { name: option, exact: true }).click({ timeout: 5000 })
}

async function chooseRadio(page, group, option) {
  const choices = page.getByRole('radiogroup', { name: group, exact: true })
  await choices.getByText(option, { exact: true }).click()
  await expect(choices.getByRole('radio', { name: option, exact: true })).toBeChecked()
}

async function selectIp(page, ip) {
  const checkbox = rowFor(page, ip).getByRole('checkbox')
  if (!(await checkbox.isChecked())) await checkbox.press('Space', { timeout: 5000 })
  await expect(checkbox).toBeChecked()
}

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

staticSalesTest('跨地区购物车一次下单，逐次复核价格和库存，部分交付退款', async ({ page, request, openSales }, info) => {
  const email = 'tenant-13900007401@example.test'
  const account = await registerStaticTenant(request, email)
  const tenant = account.headers
  await signInTenant(page, email)
  await page.goto('/console/static/pricing')
  await expect(page.getByText('静态住宅代理暂未开放购买')).toBeVisible()
  const ops = await openSales(account)
  const orders = async () => (await json(await request.get('/v1/tenant/orders?kind=static_purchase', { headers: tenant }))).orders ?? []

  await page.reload()
  const chooseUS = page.getByRole('button', { name: '选择 United States' })
  const chooseJP = page.getByRole('button', { name: '选择 Japan' })
  const chooseCanada = page.getByRole('button', { name: '选择 Canada' })
  await expect(chooseUS).toBeVisible()
  await expect(page.getByRole('heading', { name: 'North America', exact: true })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Asia', exact: true })).toBeVisible()
  await chooseRadio(page, '大洲', 'Asia')
  await expect(chooseJP).toBeVisible()
  await expect(chooseUS).toHaveCount(0)
  await chooseRadio(page, '大洲', '全部大洲')
  await chooseRadio(page, '代理线路', 'Residential')
  await expect(chooseUS).toBeVisible()
  await expect(chooseJP).toHaveCount(0)
  await chooseRadio(page, '代理线路', '全部线路')
  await selectOption(page, '地区', 'Japan')
  await expect(chooseJP).toBeVisible()
  await expect(chooseUS).toHaveCount(0)

  await page.getByText('共享静态 IP', { exact: true }).click()
  await expect(page.getByRole('button', { name: '选择 Singapore' })).toBeVisible()
  const purposeQuery = page.waitForResponse(response => {
    const url = new URL(response.url())
    return url.pathname === catalogPath && url.searchParams.get('purpose_id') === '10' && response.ok()
  })
  await selectOption(page, '用途', 'ChatGPT')
  const shared = await (await purposeQuery).json()
  expect(shared.offers.find(offer => offer.term === 'yp1')).toMatchObject({
    share_mode: 'shared', purpose_id: 10, unit_minor: '1100', available: 5,
  })
  await chooseRadio(page, '租期', '30 天')
  await expect(page.getByRole('button', { name: '选择 Singapore' })).toContainText(/可购 5.*¥11\.00/s)
  await screenshot(page, info, 'shared-chatgpt-price-stock')

  await page.getByText('独享静态 IP', { exact: true }).click()
  await chooseUS.click()
  await chooseCanada.click()
  const quantity = page.getByRole('spinbutton', { name: 'United States 数量', exact: true })
  await quantity.fill('3')
  await quantity.press('Tab')
  await expect(page.getByRole('spinbutton', { name: 'Canada 数量', exact: true })).toHaveValue('1')
  const beforeOrders = await orders()
  const beforeWallet = await json(await request.get('/v1/tenant/wallet', { headers: tenant }))
  await page.getByRole('button', { name: '立即购买' }).click()
  const dialog = page.getByRole('dialog', { name: '确认购买' })
  const pay = dialog.getByRole('button', { name: '确认支付' })
  const usQuote = dialog.locator('.quote-line').filter({ has: page.getByRole('heading', { name: 'United States', exact: true }) })
  await expect(dialog.getByText('¥39.00', { exact: true })).toBeVisible()

  await fixture(request, '/fixture/catalog', { price: 12, stock: 6 })
  await pay.click()
  await expect(dialog.getByText('价格或库存已更新，请重新确认。')).toBeVisible()
  await expect(dialog.getByText('¥45.00', { exact: true })).toBeVisible()
  expect(await orders()).toEqual(beforeOrders)

  await fixture(request, '/fixture/catalog', { price: 12, stock: 4 })
  await pay.click()
  await expect(usQuote.locator('dl > div').filter({ hasText: '可购库存' }).locator('dd')).toHaveText('4')
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
  expect(originalBody.lines).toEqual(expect.arrayContaining([
    { node_id: '101', quantity: 3, unit_minor: '1200' },
    { node_id: '111', quantity: 1, unit_minor: '900' },
  ]))
  expect(originalBody.lines).toHaveLength(2)
  expect(originalOrder).toMatchObject({ status: 'pending', delivered_count: '3', locked_minor: '4500' })

  await page.reload()
  const replayed = page.waitForResponse(response => response.url().endsWith(tenantOrderPath)
    && response.request().method() === 'POST' && response.ok())
  await page.getByRole('button', { name: '重试原提交' }).click()
  const replay = await replayed
  expect(replay.request().postDataJSON()).toEqual(originalBody)
  const order = await replay.json()
  expect(order.id).toBe(originalOrder.id)
  await expect(page.getByText('已交付 3 个 IP')).toBeVisible()
  await page.getByRole('dialog', { name: '购买结果' }).getByRole('button', { name: '完成' }).click()
  await page.reload()
  const pendingOrder = page.getByRole('button', { name: `查看待确认订单 ${order.id}`, exact: true })
  await expect(pendingOrder).toBeVisible()
  await json(await request.post(`${account.opsPath}/orders/${order.id}/static-resolution`, {
    headers: ops,
    data: { evidence: 'Fixture confirmed remaining IP could not be delivered', close_remaining: true, confirmed_undeliverable: true },
  }))
  await pendingOrder.click()
  await expect(page.getByRole('heading', { name: '部分交付' })).toBeVisible()
  await expect(page.getByText('已交付 3 个 IP')).toBeVisible()
  await expect(page.getByText('退款金额 ¥12.00')).toBeVisible()
  await expect(pendingOrder).toHaveCount(0)
  const afterWallet = await json(await request.get('/v1/tenant/wallet', { headers: tenant }))
  expect(BigInt(beforeWallet.available_minor) - BigInt(afterWallet.available_minor)).toBe(3300n)
  expect(await orders()).toHaveLength(beforeOrders.length + 1)
  await screenshot(page, info, 'purchase-partial-refund')
  await page.getByRole('link', { name: '查看 IP 列表' }).click()
  await expect(rowFor(page, '192.0.2.101')).toBeVisible()
  await expect(rowFor(page, '192.0.2.102')).toBeVisible()
  await expect(rowFor(page, '192.0.2.111')).toBeVisible()
})

staticSalesTest('IP 筛选、复制与导出，批量续费逐项结算并从列表重查', async ({ page, request, context, openSales }, info) => {
  const email = 'tenant-13900007402@example.test'
  const account = await registerStaticTenant(request, email)
  const tenant = account.headers
  const ops = await openSales(account)
  const jp = ['198.51.100.201', '198.51.100.202', '198.51.100.203']

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

  await signInTenant(page, email)
  await page.goto('/console/static/ips')
  await expect(rowFor(page, jp[0])).toContainText('即将到期')
  const expiringFilter = page.getByRole('button', { name: '即将到期 IP 数' })
  await expiringFilter.click()
  await expect(expiringFilter).toHaveAttribute('aria-pressed', 'true')
  await expect(rowFor(page, jp[0])).toBeVisible()
  await expect(rowFor(page, jp[1])).toHaveCount(0)
  await selectOption(page, '按状态筛选', '全部状态')
  await expect(expiringFilter).toHaveAttribute('aria-pressed', 'false')
  const availableFilter = page.getByRole('button', { name: '可用 IP 数', exact: true })
  await availableFilter.click()
  await expect(availableFilter).toHaveAttribute('aria-pressed', 'true')
  await expect(rowFor(page, jp[0])).toBeVisible()
  await expect(rowFor(page, jp[1])).toBeVisible()
  await availableFilter.click()
  await selectOption(page, '按地区筛选', 'Japan')
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

  for (const ip of jp) await selectIp(page, ip)
  const downloaded = page.waitForEvent('download')
  await page.getByRole('button', { name: '导出 TXT' }).click()
  const file = await (await downloaded).path()
  const txt = await readFile(file, 'utf8')
  expect(txt.trimEnd().split(/\r?\n/).sort()).toEqual(jp.map(ip => `${ip}:43000:fixture-user:fixture-pass`))
  await screenshot(page, info, 'ip-list-filters-and-selection')

  // US and Japan each have a longer term unavailable in the other region.
  await selectOption(page, '按地区筛选', '全部地区')
  for (const ip of [usIP, sharedIP, ...jp]) await selectIp(page, ip)
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

  await json(await request.post(`${account.opsPath}/orders/${order.id}/renewal-resolution`, {
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
  await rowFor(page, jp[2]).getByRole('button', { name: '重查结果' }).click()
  const recheck = renewDialog.getByRole('button', { name: '重查结果' })
  await fixture(request, '/fixture/detail', { available: false })
  try {
    await recheck.click()
    await expect(renewDialog.getByText('暂时无法查询供应商，请稍后重试。')).toBeVisible()
    await expect(rowFor(renewDialog, jp[2])).toContainText('确认中')
  } finally {
    await fixture(request, '/fixture/detail', { available: true })
  }
  await fixture(request, '/fixture/renewals/confirm')
  await recheck.click()
  await expect(rowFor(renewDialog, jp[2])).toContainText('成功')
  const settled = await json(await request.get(`/v1/tenant/orders/${order.id}`, { headers: tenant }))
  expect(settled).toMatchObject({ status: 'partial', delivered_count: '4', refunded_minor: '1500' })
  await screenshot(page, info, 'renewal-rechecked-after-reload')
  await renewDialog.getByRole('button', { name: '完成' }).click()

  const renewalPath = '/v1/tenant/static-renewal-orders'
  let singleBody, singleOrder
  await rowFor(page, jp[1]).getByRole('button', { name: '续费', exact: true }).click()
  await renewDialog.getByRole('button', { name: '获取报价' }).click()
  await expect(renewDialog.getByText('¥15.00', { exact: true }).first()).toBeVisible()
  await page.route(renewalPath, async route => {
    singleBody = route.request().postDataJSON()
    singleOrder = await json(await route.fetch())
    await route.abort()
  }, { times: 1 })
  await renewDialog.getByRole('button', { name: '确认续费' }).click()
  await expect(renewDialog.getByText('暂未取得续费结果。请重试原提交以获取结果。')).toBeVisible()
  expect(singleOrder).toMatchObject({ status: 'failed', refunded_minor: '1500' })
  expect(singleBody.supplier_item_ids).toEqual(['20102'])

  await page.reload()
  await rowFor(page, jp[0]).getByRole('button', { name: '续费', exact: true }).click()
  await renewDialog.getByRole('button', { name: '获取报价' }).click()
  await expect(rowFor(renewDialog, jp[0])).toContainText('¥15.00')
  await expect(rowFor(renewDialog, jp[1])).toHaveCount(0)
  await renewDialog.getByRole('button', { name: '取消' }).click()
  await expect(renewDialog).toBeHidden()

  await page.getByRole('button', { name: '重试原提交', exact: true }).click()
  const retry = page.waitForResponse(response => response.url().endsWith(renewalPath) && response.ok())
  await renewDialog.getByRole('button', { name: '重试原提交' }).click()
  const replayed = await retry
  expect(replayed.request().postDataJSON()).toEqual(singleBody)
  expect(await replayed.json()).toMatchObject({ id: singleOrder.id, status: 'failed' })
  await expect(rowFor(renewDialog, jp[1])).toContainText('失败')
  await screenshot(page, info, 'single-renewal-original-request-replayed')
})
