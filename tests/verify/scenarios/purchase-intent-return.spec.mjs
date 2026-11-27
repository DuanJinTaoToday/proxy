import { expect, test } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'
import { receivedCode } from '../support/mail.mjs'
import { authenticatedApi, json, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'

const password = 'proxy-local-test'
const alpha = 'alpha@example.test'
const staticOrders = '/v1/tenant/static-orders'

const setStaticSale = (operator, open) => operator.put('/v1/operations/product-sales/static', {
  data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'verify #111' } : {}) },
}).then(json)

const setInventory = (request, { price = 10, stock = 6 } = {}) => request
  .post(new URL('/fixture/catalog', process.env.VERIFY_IPCROWN_URL).href, { data: { price, stock } }).then(json)

async function balancesAndOrders(tenant) {
  const [wallet, orders, ledger] = await Promise.all([
    tenant.get('/v1/tenant/wallet').then(json),
    tenant.get('/v1/tenant/orders').then(json),
    tenant.get('/v1/tenant/wallet/ledger').then(json),
  ])
  return { wallet: wallet.available_minor ?? '0', orders: orders.orders ?? [], ledger: ledger.entries ?? [] }
}

async function staticCartToSignIn(page) {
  await page.goto('/zh-CN/pricing?category=exclusive')
  await page.locator('.offer-row', { hasText: 'United States' }).first().click()
  await page.getByRole('button', { name: '增加数量', exact: true }).click()
  await expect(page.locator('.cart-list .stepper strong')).toHaveText('2')
  await page.getByRole('button', { name: '立即购买', exact: true }).click()
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=/)
  const target = new URL(new URL(page.url()).searchParams.get('redirect'), page.url())
  expect(target.pathname).toBe('/console/static/pricing')
  expect(target.searchParams.get('shareMode')).toBe('exclusive')
  expect(target.searchParams.get('term')).toBe('yp1')
  expect(target.searchParams.get('items')).toBe('101:2')
}

async function authenticateFromCart(page, email, register = false) {
  if (register) {
    await page.getByRole('link', { name: '立即注册', exact: true }).click()
    await expect(page).toHaveURL(/\/zh-CN\/register\?redirect=/)
    await field(page, '邮箱').fill(email)
    await page.getByRole('button', { name: '发送验证码', exact: true }).click()
    await field(page, '验证码').fill(await receivedCode(email))
  } else {
    await field(page, '邮箱').fill(email)
  }
  await field(page, '密码').fill(password)
  await page.getByRole('button', { name: register ? '注册并登录' : '登录', exact: true }).click()
}

function trackPurchaseWrites(page) {
  const paths = []
  page.on('request', request => {
    const path = new URL(request.url()).pathname
    if (request.method() === 'POST' && [staticOrders, purchasePath].includes(path)) paths.push(path)
  })
  return paths
}

async function showStaticConfirmation(page) {
  const dialog = page.getByRole('dialog', { name: '确认购买', exact: true })
  await expect(dialog).toBeVisible()
  return dialog
}

test('静态购物车登录回跳时复核新价，用户确认后仅写入一笔订单', async ({ page, request, playwright, baseURL }, info) => {
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email: alpha })
  try {
    await setInventory(request)
    await setStaticSale(operator, true)
    await recharge(operator, tenant, '10000')
    const before = await balancesAndOrders(tenant)
    const writes = trackPurchaseWrites(page)
    await staticCartToSignIn(page)
    await setInventory(request, { price: 13 })
    await authenticateFromCart(page, alpha)
    await expect(page).toHaveURL(/\/console\/static\/pricing(?:\?|$)/)
    const review = await showStaticConfirmation(page)
    await expect(review).toContainText('United States')
    await expect(review).toContainText('30 天')
    await expect(review.locator('.quote-line')).toContainText(/数量\s*2/)
    await expect(review.getByText('¥26.00', { exact: true })).toBeVisible()
    await expect(page.getByText(/价格.*(?:变更|变化|更新)|(?:变更|变化|更新).*价格/).filter({ visible: true }).first()).toBeVisible()
    expect(writes).toEqual([])
    expect(await balancesAndOrders(tenant)).toEqual(before)
    await screenshot(page, info, 'static-return-price-review')
    await review.getByRole('button', { name: '确认支付', exact: true }).click()
    await expect(page.getByRole('dialog', { name: '购买结果' })).toBeVisible()
    const after = await balancesAndOrders(tenant)
    expect(after.orders).toHaveLength(before.orders.length + 1)
    expect(after.orders[0]).toMatchObject({ kind: 'static_purchase', settled_minor: '2600' })
    expect(BigInt(after.wallet)).toBe(BigInt(before.wallet) - 2600n)
    expect(writes).toEqual([staticOrders])
    await info.attach('price-confirmation.json', { body: JSON.stringify({ before, after, writes }, null, 2), contentType: 'application/json' })
  } finally {
    await setInventory(request)
    await setStaticSale(operator, false)
    await tenant.dispose()
    await operator.dispose()
  }
})

test('游客邮箱验码注册后恢复静态购物车，窄屏余额为零且未扣款', async ({ page, request, playwright, baseURL }, info) => {
  const email = 'cart-return-registration@example.test'
  const operator = await authenticatedApi(playwright, baseURL)
  let tenant
  try {
    await setInventory(request)
    await setStaticSale(operator, true)
    const writes = trackPurchaseWrites(page)
    await page.setViewportSize({ width: 390, height: 844 })
    await staticCartToSignIn(page)
    await authenticateFromCart(page, email, true)
    await expect(page).toHaveURL(/\/console\/static\/pricing(?:\?|$)/)
    await expect(page.locator('.cart-line')).toContainText('United States')
    await expect(page.locator('.cart-line input[type="number"], .cart-line input[role="spinbutton"]')).toHaveValue('2')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
    tenant = await authenticatedApi(playwright, baseURL, { email })
    expect(await balancesAndOrders(tenant)).toEqual({ wallet: '0', orders: [], ledger: [] })
    expect(writes).toEqual([])
    await screenshot(page, info, 'registered-static-return-mobile')
  } finally {
    await setStaticSale(operator, false)
    await tenant?.dispose()
    await operator.dispose()
  }
})

for (const change of ['stock', 'sale']) {
  test(`静态购物车登录后遇到${change === 'stock' ? '缺货' : '停售'}仍未落单扣款`, async ({ page, request, playwright, baseURL }, info) => {
    const operator = await authenticatedApi(playwright, baseURL)
    const tenant = await authenticatedApi(playwright, baseURL, { email: alpha })
    try {
      await setInventory(request)
      await setStaticSale(operator, true)
      const before = await balancesAndOrders(tenant)
      const writes = trackPurchaseWrites(page)
      await staticCartToSignIn(page)
      if (change === 'stock') await setInventory(request, { stock: 1 })
      else await setStaticSale(operator, false)
      await authenticateFromCart(page, alpha)
      await expect(page).toHaveURL(/\/console\/static\/pricing(?:\?|$)/)
      if (change === 'stock') {
        await expect(page.getByText(/(?:库存|可购).*1.*(?:调整|移除)|(?:调整|移除).*1/).filter({ visible: true }).first()).toBeVisible()
        await expect(page.locator('.cart-line').getByRole('spinbutton')).toHaveValue('2')
      } else {
        await expect(page.getByText(/暂未开放购买|已停售|停售|暂不可购/).filter({ visible: true }).first()).toBeVisible()
      }
      await expect(page.getByRole('button', { name: '确认支付', exact: true })).toHaveCount(0)
      expect(writes).toEqual([])
      expect(await balancesAndOrders(tenant)).toEqual(before)
      await screenshot(page, info, `static-return-${change}`)
    } finally {
      await setInventory(request)
      await setStaticSale(operator, false)
      await tenant.dispose()
      await operator.dispose()
    }
  })
}

test('动态公开套餐回跳保持档位和单份语义，变价只在确认后扣款', async ({ page, playwright, baseURL }, info) => {
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email: alpha })
  try {
    await setTier(operator, '5gb', '1500')
    await setSale(operator, true)
    await recharge(operator, tenant, '10000')
    const before = await balancesAndOrders(tenant)
    const writes = trackPurchaseWrites(page)
    await page.goto('/zh-CN/pricing')
    await page.locator('a[href*="package=5gb"]').click()
    await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=/)
    await setTier(operator, '5gb', '1800')
    await authenticateFromCart(page, alpha)
    await expect(page).toHaveURL(/\/console\/dynamic\/pricing(?:\?|$)/)
    await expect(page.getByRole('radio', { name: '5 GB 流量包', exact: true })).toBeChecked()
    await expect(page.getByText(/价格.*(?:变更|变化|更新)|(?:变更|变化|更新).*价格/).filter({ visible: true }).first()).toBeVisible()
    await page.getByRole('button', { name: '立即购买', exact: true }).click()
    const dialog = page.getByRole('dialog', { name: '确认购买流量' })
    await expect(dialog.getByText('¥18.00', { exact: true })).toBeVisible()
    expect(await balancesAndOrders(tenant)).toEqual(before)
    expect(writes).toEqual([])
    await screenshot(page, info, 'dynamic-return-price-review')
    await dialog.getByRole('button', { name: '确认支付', exact: true }).click()
    await expect.poll(async () => (await balancesAndOrders(tenant)).orders.length).toBe(before.orders.length + 1)
    const after = await balancesAndOrders(tenant)
    expect(after.orders[0]).toMatchObject({ kind: 'traffic_package', settled_minor: '1800', delivered_bytes: '5000000000' })
    expect(BigInt(after.wallet)).toBe(BigInt(before.wallet) - 1800n)
    expect(writes).toEqual([purchasePath])
  } finally {
    await setSale(operator, false)
    await tenant.dispose()
    await operator.dispose()
  }
})

test('公开购物车达到接口行数上限后仍可购买已选 25 个规格', async ({ page }) => {
  await page.route('**/v1/public/static-offers*', route => route.fulfill({ json: {
    open: true,
    quoted_at: new Date().toISOString(),
    offers: Array.from({ length: 26 }, (_, index) => ({
      node_id: String(10001 + index), region: `Test region ${String(index + 1).padStart(2, '0')}`,
      continent: 'North America', line: 'Residential', isp: '独享',
      term: 'yp1', term_days: 30, unit_minor: '1000', available: 5,
    })),
  } }))
  await page.goto('/zh-CN/pricing?category=exclusive')
  const offers = page.locator('.offer-row')
  await expect(offers).toHaveCount(26)
  for (let index = 0; index < 25; index++) await offers.nth(index).click()
  const lastOffer = offers.nth(25)
  if (!(await lastOffer.isDisabled())) await lastOffer.click()
  await expect(page.locator('.cart-list .cart-item')).toHaveCount(25)
  await expect(page.getByText(/最多.*25|上限.*25|25.*上限/).filter({ visible: true }).first()).toBeVisible()
  const buy = page.getByRole('button', { name: '立即购买', exact: true })
  await expect(buy).toBeEnabled()
  await buy.click()
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=/)
  const target = new URL(new URL(page.url()).searchParams.get('redirect'), page.url())
  expect(target.searchParams.get('items').split(',')).toHaveLength(25)
})
