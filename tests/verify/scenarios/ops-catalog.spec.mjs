import { randomUUID } from 'node:crypto'
import { expect, test } from '@playwright/test'
import { screenshot, signInOperator, signInTenant } from '../support/console.mjs'
import { authenticatedApi, json, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'
import { registerEmail } from '../support/mail.mjs'

const tiersPath = '/v1/operations/traffic-packages'
const staticSalePath = '/v1/operations/product-sales/static'
const publicProductsPath = '/v1/public/products'
const staticPurchasePath = '/v1/tenant/static-orders'
const staticQuotePath = '/v1/tenant/static-purchase-quotes'

test.use({ actionTimeout: 7000 })

async function openProducts(page, username) {
  await signInOperator(page, '/ops', username)
  await expect(page).toHaveURL(/\/ops\/tenants$/)
  await page.getByRole('menuitem', { name: '商品与定价' }).click()
  await expect(page.getByRole('heading', { name: '商品与定价' })).toBeVisible()
}

async function reason(response, status, code) {
  expect(response.status()).toBe(status)
  expect(await response.json()).toMatchObject({ reason: code })
}

async function viewportEvidence(page, info, name) {
  const path = info.outputPath(name + '.png')
  await page.screenshot({ path, animations: 'disabled' })
  await info.attach(name, { path, contentType: 'image/png' })
}

test('计费运营独立开售、调价和下架，公开报价及成交快照同步验证', async ({ page, request, playwright, baseURL }, info) => {
  const email = 'catalog-' + Date.now() + '@example.test'
  await json(await registerEmail(request, { email, password: 'proxy-local-test' }))
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { email })
  try {
    await setTier(operator, '5gb', '2000')
    await setTier(operator, '50gb', '18000')
    await setTier(operator, '200gb', '64000')
    await setSale(operator, true)
    await json(await operator.put(staticSalePath, { data: {
      open: true, quote_ready: true, acceptance_evidence: 'verify #90 / isolated IPCrown fixture',
    } }))

    await openProducts(page, 'operator-billing')
    const dynamicSwitch = page.getByRole('switch', { name: '动态住宅代理-GB开售' })
    const staticSwitch = page.getByRole('switch', { name: '静态住宅代理开售' })
    const five = page.getByRole('region', { name: '动态流量包' }).getByRole('row', { name: /^5 GB/ })
    await expect(five).toContainText('¥20.00')
    await expect(dynamicSwitch).toBeChecked()
    await expect(staticSwitch).toBeChecked()
    await viewportEvidence(page, info, '09-products')
    const dynamicHeading = page.getByRole('heading', { name: '动态住宅代理-GB', exact: true })
    const staticHeading = page.getByRole('heading', { name: '静态住宅代理', exact: true })
    const desktopViewport = page.viewportSize()
    await page.setViewportSize({ width: 390, height: 844 })
    await expect(dynamicHeading).toBeVisible()
    await expect(staticHeading).toBeVisible()
    expect((await staticHeading.boundingBox()).x).toBe((await dynamicHeading.boundingBox()).x)
    expect((await staticHeading.boundingBox()).y).toBeGreaterThan((await dynamicHeading.boundingBox()).y)
    await page.setViewportSize(desktopViewport)
    const auditResponse = await json(await operator.get('/v1/operations/product-changes'))
    await info.attach('visual-fixture-audit', { body: JSON.stringify(auditResponse, null, 2), contentType: 'application/json' })

    const { packages } = await json(await operator.get(tiersPath))
    const original = packages.find(tier => tier.id === '5gb')
    await recharge(operator, tenant, '2500')
    const place = version => tenant.post(purchasePath, { data: {
      package_id: '5gb', package_version: version, request_key: randomUUID(),
    } })
    const order = await json(await place(original.version))

    await five.getByRole('button', { name: '调价' }).click()
    const dialog = page.getByRole('dialog', { name: '调整 5 GB 流量包售价' })
    await expect(dialog).toBeVisible()
    await expect(dialog.getByText('¥4.00', { exact: true })).toBeVisible()
    await viewportEvidence(page, info, '10-price-dialog')
    await dialog.getByRole('textbox', { name: '售价（元）' }).fill('22.50')
    await dialog.getByRole('button', { name: '保存' }).click()
    await expect(five).toContainText('¥22.50')
    await page.reload()
    await expect(five).toContainText('¥22.50')
    const priceAudit = page.getByRole('row', { name: /5 GB 流量包 · 售价.*¥20.00.*¥22.50.*operator-billing/ })
    await expect(priceAudit).toBeVisible()
    await expect(priceAudit.locator('time')).toHaveAttribute('datetime', /^\d{4}-\d{2}-\d{2}T/)

    const customerContext = await page.context().browser().newContext({ baseURL, ignoreHTTPSErrors: true })
    const visitorContext = await page.context().browser().newContext({ baseURL, ignoreHTTPSErrors: true })
    try {
      const customer = await customerContext.newPage()
      await signInTenant(customer, email)
      await customer.goto('/console/dynamic/pricing')
      await expect(customer.getByRole('radio', { name: '5 GB 流量包' })).toBeVisible()
      const visitor = await visitorContext.newPage()
      await visitor.goto('/zh-CN/')
      const publicDynamic = visitor.getByRole('article', { name: '动态住宅代理' })
      const publicStatic = visitor.getByRole('article', { name: '静态住宅 IP' })
      const publicFive = publicDynamic.getByRole('link', { name: '购买 5 GB 流量包' })
      const publicFifty = publicDynamic.getByRole('link', { name: '购买 50 GB 流量包' })
      const publicStaticPricing = publicStatic.getByRole('link', { name: /查看静态价格/ })
      const publicFiveOffer = publicDynamic.getByRole('listitem').filter({ has: visitor.getByRole('link', { name: '购买 5 GB 流量包' }) })
      await expect(publicFiveOffer.getByText('¥22.50', { exact: true })).toBeVisible()
      await expect(publicStaticPricing).toBeVisible()

      await five.getByRole('button', { name: '下架', exact: true }).click()
      await expect(five.getByText('已下架', { exact: true })).toBeVisible()
      await page.reload()
      await expect(five.getByText('已下架', { exact: true })).toBeVisible()
      await expect(page.getByRole('row', { name: /5 GB 流量包 · 状态.*上架.*已下架.*operator-billing/ })).toBeVisible()
      const publicProducts = () => request.get(publicProductsPath).then(json)
      let { products } = await publicProducts()
      await visitor.reload()
      await expect(publicFifty).toBeVisible()
      await expect(publicFive).toHaveCount(0)
      expect(products.find(product => product.kind === 'dynamic').packages.map(tier => tier.id)).not.toContain('5gb')
      await customer.reload()
      await expect(customer.getByRole('radio', { name: '50 GB 流量包' })).toBeVisible()
      await expect(customer.getByRole('radio', { name: '5 GB 流量包' })).toHaveCount(0)
      await reason(await place(original.version), 409, 'PACKAGE_UNAVAILABLE')
      const { orders } = await json(await tenant.get('/v1/tenant/orders?kind=traffic_package'))
      expect(orders.find(row => row.id === order.id)).toMatchObject({ debited_minor: '2000', delivered_bytes: '5000000000' })
      await screenshot(page, info, 'price-listing-audit')
      await page.getByRole('button', { name: /^语言[:：]/ }).click()
      await page.getByRole('menuitem', { name: 'English' }).click()
      const delistedTier = page.getByRole('region', { name: 'Traffic packages' }).getByRole('row', { name: /^5 GB/ })
      await expect(delistedTier.getByRole('button', { name: 'List', exact: true })).toBeEnabled()
      await expect(delistedTier.getByText('Delisted', { exact: true })).toBeVisible()
      await page.getByRole('button', { name: /^Language[:：]/ }).click()
      await page.getByRole('menuitem', { name: '简体中文' }).click()

      await dynamicSwitch.press('Enter')
      await page.getByRole('dialog').getByRole('button', { name: '暂停销售' }).click()
      await expect(dynamicSwitch).not.toBeChecked()
      await expect(staticSwitch).toBeChecked()
      await customer.reload()
      await expect(customer.getByText('暂未开放购买', { exact: true })).toBeVisible()
      await visitor.reload()
      await expect(publicDynamic.getByText('暂未开放购买')).toBeVisible()
      await expect(publicFifty).toHaveCount(0)
      await expect(publicStaticPricing).toBeVisible()
      await reason(await place(original.version), 409, 'PRODUCT_SALE_CLOSED')

      await dynamicSwitch.press('Enter')
      const open = page.getByRole('dialog', { name: '开售动态住宅代理-GB？' })
      await expect(open.getByRole('button', { name: '开售' })).toBeDisabled()
      await open.getByRole('textbox', { name: '验收记录或链接' }).fill('verify #90 / dynamic')
      await open.getByText('已确认报价就绪', { exact: true }).click()
      await open.getByRole('button', { name: '开售' }).click()
      await expect(dynamicSwitch).toBeChecked()

      const selection = { node_id: '101', term: 'yp1', quantity: 1 }
      const staticQuote = await json(await tenant.post(staticQuotePath, { data: selection }))
      await staticSwitch.press('Enter')
      await page.getByRole('dialog').getByRole('button', { name: '暂停销售' }).click()
      await expect(staticSwitch).not.toBeChecked()
      await expect(dynamicSwitch).toBeChecked()
      await customer.goto('/console/static/pricing')
      await expect(customer.getByText('静态住宅代理暂未开放购买', { exact: true })).toBeVisible()
      await visitor.reload()
      await expect(publicStatic.getByText('暂未开放购买', { exact: true })).toBeVisible()
      await expect(publicStaticPricing).toHaveCount(0)
      await expect(publicFifty).toBeVisible()
      await reason(await tenant.post(staticQuotePath, { data: selection }), 409, 'PRODUCT_SALE_CLOSED')
      await reason(await tenant.post(staticPurchasePath, { data: {
        ...selection, unit_minor: staticQuote.offer.unit_minor, confirmation_token: staticQuote.confirmation_token, request_key: randomUUID(),
      } }), 409, 'PRODUCT_SALE_CLOSED')
      ;({ products } = await publicProducts())
      expect(products.find(product => product.kind === 'dynamic').open).toBe(true)
      expect(products.find(product => product.kind === 'static').open ?? false).toBe(false)
    } finally {
      await visitorContext.close()
      await customerContext.close()
    }
  } finally {
    await setSale(operator, false)
    await json(await operator.put(staticSalePath, { data: { open: false } }))
    const { packages } = await json(await operator.get(tiersPath))
    const current = packages.find(tier => tier.id === '5gb')
    await json(await operator.patch(tiersPath + '/5gb', { data: {
      capacity_bytes: current.capacity_bytes, version: current.version, price_minor: '2000', listed: true,
    } }))
    await operator.dispose()
    await tenant.dispose()
  }
})

test('只读运营可查看目录与记录，服务端拒绝改动', async ({ page }, info) => {
  await openProducts(page, 'operator-readonly')
  const five = page.getByRole('region', { name: '动态流量包' }).getByRole('row', { name: /^5 GB/ })
  await expect(five).toBeVisible()
  await expect(page.getByRole('heading', { name: '修改记录' })).toBeVisible()
  await expect(five.getByRole('button', { name: '调价' })).toBeDisabled()
  await expect(five.getByRole('button', { name: /^(下架|上架)$/ })).toBeDisabled()
  for (const control of await page.getByRole('switch').all()) await expect(control).toBeDisabled()
  await screenshot(page, info, 'readonly')

  const login = await json(await page.request.post('/v1/operations/access/login', {
    data: { username: 'operator-readonly', password: 'proxy-local-test' },
  }))
  const headers = { Authorization: 'Bearer ' + login.access_token }
  const { packages } = await json(await page.request.get(tiersPath, { headers }))
  const tier = packages.find(item => item.id === '5gb')
  await reason(await page.request.patch(tiersPath + '/5gb', { headers, data: {
    capacity_bytes: tier.capacity_bytes, version: tier.version, listed: true, price_minor: '999',
  } }), 403, 'ACCESS_DENIED')
  await reason(await page.request.put(staticSalePath, { headers, data: { open: false } }), 403, 'ACCESS_DENIED')
})
