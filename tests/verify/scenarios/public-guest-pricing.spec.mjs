import { expect, test } from '@playwright/test'
import { screenshot } from '../support/console.mjs'
import { authenticatedApi, json, setTier } from '../support/traffic.mjs'

const staticOffers = '/v1/public/static-offers'
const products = '/v1/public/products'
const sharedPurposes = [0, 8, 9, 10]
const sharedOffers = purpose => staticOffers + '?share_mode=shared' + (purpose ? '&purpose_id=' + purpose : '')

const setSale = (operator, kind, open) => operator.put('/v1/operations/product-sales/' + kind, {
  data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'verify #110' } : {}) },
}).then(json)

test('游客从首页查询动态和静态报价，区分停售、无库存和刷新失败', async ({ page, request, playwright, baseURL }, info) => {
  const operator = await authenticatedApi(playwright, baseURL)
  const supplier = await playwright.request.newContext({ baseURL: process.env.VERIFY_IPCROWN_URL })
  const setStock = stock => supplier.post('/fixture/catalog', { data: { price: 10, stock } }).then(json)
  const catalogControl = (category, purpose, unavailable) => supplier
    .post('/fixture/catalog-control', { data: { category, purpose, unavailable } }).then(json)
  const publicProducts = async () => {
    const body = await request.get(products).then(json)
    return Object.fromEntries(body.products.map(product => [product.kind, product]))
  }

  try {
    await setTier(operator, '5gb', '2000')
    await setTier(operator, '50gb', '18000')
    await setTier(operator, '200gb', '64000')
    await setSale(operator, 'dynamic', true)
    await setSale(operator, 'static', false)
    const dynamicOnly = await publicProducts()
    expect(dynamicOnly.dynamic.packages.map(tier => [tier.id, tier.price_minor])).toEqual([
      ['5gb', '2000'], ['50gb', '18000'], ['200gb', '64000'],
    ])
    expect(dynamicOnly.static.open ?? false).toBe(false)
    expect((await request.get(staticOffers).then(json)).open ?? false).toBe(false)

    await page.goto('/')
    await page.locator('a[href="/zh-CN/pricing"]').first().click()
    await expect(page).toHaveURL(/\/zh-CN\/pricing\/?$/)
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible()
    for (const label of ['5 GB', '50 GB', '200 GB']) {
      await expect(page.getByText(label, { exact: true })).toBeVisible()
    }
    await screenshot(page, info, 'guest-pricing-dynamic-only')
    await page.goto('/zh-CN/pricing?category=exclusive')
    await expect(page.locator('[data-state="closed"]').first()).toBeVisible()

    await setSale(operator, 'dynamic', false)
    await setSale(operator, 'static', true)
    const staticOnly = await publicProducts()
    expect(staticOnly.dynamic.open ?? false).toBe(false)
    expect(staticOnly.dynamic.packages ?? []).toEqual([])
    expect(staticOnly.static.open).toBe(true)
    const previous = await request.get(staticOffers).then(json)
    const sharedBefore = await request.get(sharedOffers(10)).then(json)
    expect(sharedBefore.offers.find(row => row.term === 'yp1').unit_minor).toBe('1100')
    await setStock(0)
    await catalogControl(28, 10, true)
    expect((await request.get(sharedOffers(10)).then(json)).quoted_at).toBe(sharedBefore.quoted_at)

    // Wait for the observed public snapshot, including any previous scenario's data, to expire.
    await expect.poll(async () => {
      const offer = (await request.get(staticOffers).then(json)).offers.find(row => row.node_id === '101')
      return offer.available ?? 0
    }, { timeout: Math.max(3000, Date.parse(previous.quoted_at) + 24000 - Date.now()), intervals: [250, 1000, 2000] }).toBe(0)

    const calls = await catalogControl(200001, 0)
    const [exclusive, ...repeated] = await Promise.all(Array.from({ length: 12 }, (_, i) =>
      request.get(staticOffers + (i % 2 ? '?share_mode=exclusive' : '')).then(json)))
    expect(exclusive.offers.map(row => row.region)).toEqual(expect.arrayContaining(['United States', 'Canada', 'Japan']))
    expect(exclusive.offers.find(row => row.node_id === '101').available ?? 0).toBe(0)
    expect(repeated.every(result => result.quoted_at === exclusive.quoted_at)).toBe(true)
    expect(await catalogControl(200001, 0)).toEqual(calls)

    const shared = await Promise.all(sharedPurposes.slice(0, -1).map(purpose => request.get(sharedOffers(purpose)).then(json)))
    expect(shared.map(quote => quote.offers.find(row => row.term === 'yp1').unit_minor)).toEqual(['800', '900', '1000'])
    await expect.poll(async () => (await request.get(sharedOffers(10))).status(),
      { timeout: 25000, intervals: [250, 1000, 2000] }).toBe(503)
    const failed = await request.get(sharedOffers(10))
    expect(failed.status()).toBe(503)
    expect((await failed.json()).reason).toBe('STATIC_SUPPLIER_UNAVAILABLE')

    await page.reload()
    await expect(page.getByText('United States', { exact: true }).first()).toBeVisible()
    await expect(page.getByText(/售罄|无库存/).first()).toBeVisible()
    await screenshot(page, info, 'guest-pricing-static-only')

    await page.goto('/zh-CN/pricing?category=shared&purpose=10')
    const failure = page.locator('[data-state="failed"]')
    await expect(failure).toBeVisible()
    await screenshot(page, info, 'guest-pricing-refresh-failed')
    await catalogControl(28, 10, false)
    await failure.getByRole('button', { name: '重试' }).click()
    await expect(page.getByText('Singapore', { exact: true }).first()).toBeVisible()
    await screenshot(page, info, 'guest-pricing-shared-recovered')

    await page.setViewportSize({ width: 390, height: 844 })
    await page.goto('/en/pricing?category=exclusive')
    await expect(page.locator('html')).toHaveAttribute('lang', 'en')
    await expect(page.getByText('United States', { exact: true }).first()).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
    await screenshot(page, info, 'guest-pricing-en-mobile')

    await info.attach('guest-catalog.json', {
      body: JSON.stringify({ dynamicOnly, staticOnly, exclusive, shared, sharedBefore, failedStatus: failed.status(), cachedCalls: calls.calls }, null, 2),
      contentType: 'application/json',
    })
  } finally {
    await setStock(6)
    await catalogControl(28, 10, false)
    await setSale(operator, 'dynamic', false)
    await setSale(operator, 'static', false)
    await supplier.dispose()
    await operator.dispose()
  }
})