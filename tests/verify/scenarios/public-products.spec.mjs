import { expect, test } from '@playwright/test'

test('游客通过 /v1 查询商品目录，旧接口返回 404', async ({ playwright, request, baseURL }, info) => {
  const catalogPath = '/v1/public/products'
  const operator = await playwright.request.newContext({ baseURL, ignoreHTTPSErrors: true })

  try {
    const login = await operator.post('/v1/operations/access/login', {
      data: { username: 'operator', password: 'proxy-local-test' },
    })
    expect(login.ok()).toBe(true)
    const { access_token } = await login.json()
    expect(access_token).toBeTruthy()
    const headers = { Authorization: 'Bearer ' + access_token }

    const tier = await operator.patch('/v1/operations/traffic-packages/5gb', {
      headers,
      data: { version: '1', capacity_bytes: '5000000000', price_minor: '500', listed: true },
    })
    expect(tier.ok()).toBe(true)

    const sale = await operator.put('/v1/operations/product-sales/dynamic', {
      headers,
      data: { open: true, quote_ready: true, acceptance_evidence: 'verify: public catalog' },
    })
    expect(sale.ok()).toBe(true)

    const catalog = await request.get(catalogPath)
    expect(catalog.status()).toBe(200)
    const body = await catalog.json()
    expect(body.products).toHaveLength(2)
    const dynamic = body.products.find(product => product.kind === 'dynamic')
    const staticProduct = body.products.find(product => product.kind === 'static')
    expect(dynamic).toMatchObject({ open: true, can_purchase: true })
    expect(dynamic.packages.map(tier => tier.id)).toEqual(['5gb', '50gb', '200gb'])
    expect(dynamic.packages.every(tier => tier.listed)).toBe(true)
    expect(staticProduct).toBeDefined()
    expect(staticProduct.open ?? false).toBe(false)
    expect(staticProduct.can_purchase ?? false).toBe(false)
    expect(staticProduct.packages ?? []).toEqual([])

    const retired = await request.get('/public/products', { maxRedirects: 0 })
    expect(retired.status()).toBe(404)

    await info.attach('public-catalog.json', {
      body: JSON.stringify({ path: catalogPath, status: catalog.status(), body, retiredStatus: retired.status() }, null, 2),
      contentType: 'application/json',
    })
  } finally {
    await operator.dispose()
  }
})
