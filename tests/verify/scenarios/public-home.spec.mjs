import { test, expect } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'
import { authenticatedApi, json, setTier } from '../support/traffic.mjs'

const paths = {
  catalog: '**/v1/public/products',
  staticOffers: '**/v1/public/static-offers',
}

async function sale(operator, kind, open) {
  await json(await operator.put(`/v1/operations/product-sales/${kind}`, {
    data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'verify: public homepage #96' } : {}) },
  }))
}

const homepage = test.extend({
  openedCatalog: async ({ playwright, baseURL }, use) => {
    const operator = await authenticatedApi(playwright, baseURL)
    try {
      await setTier(operator, '5gb', '5100')
      await sale(operator, 'dynamic', true)
      await sale(operator, 'static', true)
      await use(operator)
    } finally {
      try {
        await sale(operator, 'dynamic', false)
        await sale(operator, 'static', false)
      } finally {
        await operator.dispose()
      }
    }
  },
})

homepage('双语首页、两种视口、直接访问和刷新', async ({ page, openedCatalog }, info) => {
  const errors = []
  page.on('pageerror', error => errors.push(error.message))
  await page.emulateMedia({ reducedMotion: 'reduce' })
  for (const width of [1280, 390]) {
    await page.setViewportSize({ width, height: 844 })
    for (const locale of ['zh-CN', 'en']) {
      await page.goto(locale === 'zh-CN' ? '/' : '/en/')
      await expect(page).toHaveURL(new RegExp(`/${locale}/$`))
      await page.reload()
      await expect(page.locator('html')).toHaveAttribute('lang', locale)
      await expect(page.getByRole('heading', { level: 1 })).toContainText(locale === 'zh-CN' ? '用真实住宅 IP' : 'Real residential IPs to')
      await expect(page.getByRole('link', { name: /5 GB/ })).toBeVisible()
      const staticProduct = page.getByRole('article', { name: locale === 'zh-CN' ? '静态住宅 IP' : 'Static residential IPs' })
      await expect(staticProduct.getByRole('link')).toBeVisible()
      for (const section of ['#products', '#integration', '#manage', '#faq', '.closing', '.home-footer']) {
        await expect(page.locator(section)).toBeVisible()
      }
      await expect(page.locator('canvas')).toBeVisible()
      await expect(page.getByRole('button', { name: /地图|地球|Map|Globe/ })).toHaveCount(0)
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(width)
      await screenshot(page, info, `homepage-${locale}-${width}`)
    }
    if (process.env.VERIFY_HOME_PROTOTYPE_URL) {
      const reference = await page.context().newPage()
      try {
        await reference.setViewportSize({ width, height: 844 })
        await reference.emulateMedia({ reducedMotion: 'reduce' })
        await reference.goto(process.env.VERIFY_HOME_PROTOTYPE_URL)
        await expect(reference.getByRole('heading', { level: 1 })).toContainText('用真实住宅 IP')
        await reference.locator('canvas').evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))))
        await screenshot(reference, info, `reference-D-zh-CN-${width}`)
      } finally {
        await reference.close()
      }
    }
  }
  expect(errors).toEqual([])
})

homepage('真实商品价格、暂停开售、空报价和失败重试', async ({ page, openedCatalog, request }, info) => {
  const dynamicProduct = page.getByRole('article', { name: '动态住宅代理' })
  const staticProduct = page.getByRole('article', { name: '静态住宅 IP' })
  const staticQuotes = await json(await request.get('/v1/public/static-offers'))
  const available = staticQuotes.offers.filter(offer => offer.available > 0)
  expect(available.length).toBeGreaterThan(0)
  const minimum = Math.min(...available.map(offer => Number(offer.unit_minor))) / 100

  await page.route(paths.catalog, route => route.fulfill({ status: 503, json: {} }), { times: 1 })
  await page.goto('/zh-CN/')
  await expect(page.getByText('商品报价暂时无法加载')).toBeVisible()
  await page.getByRole('button', { name: '重试', exact: true }).click()
  await expect(dynamicProduct.getByText(/51\.00/)).toBeVisible()
  await expect(staticProduct.getByText(new RegExp(minimum.toFixed(2).replace('.', '\\.')))).toBeVisible()
  await expect(staticProduct.getByRole('link', { name: /查看静态价格/ })).toHaveAttribute('href', '/console/static/pricing')
  await screenshot(page, info, 'catalog-live')

  await sale(openedCatalog, 'static', false)
  await page.reload()
  await expect(staticProduct.getByText('暂未开放购买')).toBeVisible()
  await expect(staticProduct.getByRole('link')).toHaveCount(0)
  await expect(dynamicProduct.getByRole('link', { name: /5 GB/ })).toBeVisible()

  await sale(openedCatalog, 'dynamic', false)
  await sale(openedCatalog, 'static', true)
  await page.route(paths.staticOffers, route => route.fulfill({ json: { open: true, offers: [] } }), { times: 1 })
  await page.reload()
  await expect(dynamicProduct.getByText('暂未开放购买')).toBeVisible()
  await expect(dynamicProduct.getByRole('link')).toHaveCount(0)
  await expect(staticProduct.getByText('暂无独享 IP 报价')).toBeVisible()
  await expect(staticProduct.getByRole('link', { name: /查看静态价格/ })).toBeVisible()
  await screenshot(page, info, 'closed-dynamic-empty-static')

  await page.route(paths.staticOffers, route => route.fulfill({ status: 503, json: {} }), { times: 1 })
  await page.reload()
  await expect(staticProduct.getByText('静态报价暂时无法加载')).toBeVisible()
  await staticProduct.getByRole('button', { name: '重试' }).click()
  await expect(staticProduct.getByRole('link', { name: /查看静态价格/ })).toBeVisible()
  await staticProduct.getByRole('link', { name: /查看静态价格/ }).click()
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=/)
  expect(new URL(page.url()).searchParams.get('redirect')).toBe('/console/static/pricing')
})

homepage('英文注册和登录入口、动态套餐登录回跳', async ({ page, openedCatalog }, info) => {
  await page.goto('/en/')
  await page.getByRole('banner').getByRole('link', { name: 'Sign up', exact: true }).click()
  await expect(page).toHaveURL(/\/en\/register$/)
  await page.goto('/en/')
  await page.getByRole('banner').getByRole('link', { name: /Sign in|Log in/i }).click()
  await expect(page).toHaveURL(/\/en\/login$/)
  await page.goto('/en/')
  await page.getByRole('link', { name: /50 GB/ }).click()
  await expect(page).toHaveURL(/\/en\/login\?redirect=/)
  expect(new URL(page.url()).searchParams.get('redirect')).toBe('/console/dynamic/pricing?package=50gb')
  await field(page, 'Email address').fill('alpha@example.test')
  await field(page, 'Password').fill('proxy-local-test')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/console\/dynamic\/pricing\?package=50gb$/)
  await expect(page.getByRole('radio', { name: /50 GB/ })).toBeChecked()
  await screenshot(page, info, 'returned-to-50gb-plan')
})

test('切换轮换标题时页面不跳动，顶栏在滚动中保留', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/')
  const heading = page.getByRole('heading', { level: 1 })
  await expect(heading).toHaveAccessibleName(/run global stores/)
  const top = () => heading.evaluate(el => el.getBoundingClientRect().top)
  const initial = await top()
  await expect(heading).toHaveAccessibleName(/manage social accounts/)
  expect(await top()).toBeCloseTo(initial, 0)

  await page.getByRole('link', { name: /Explore products/ }).click()
  await expect(page).toHaveURL(/#products$/)
  const banner = await page.getByRole('banner').boundingBox()
  expect(banner.y).toBe(0)
  const products = await page.locator('#products').boundingBox()
  expect(products.y).toBeGreaterThanOrEqual(banner.height - 1)
})

test('首屏与四种语言的示例可复制，游客首页仅请求公开接口', async ({ page, context, browserName, baseURL }, info) => {
  test.skip(browserName !== 'chromium', 'Clipboard read permissions are available in Chromium.')
  await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: new URL(baseURL).origin })
  const privateRequests = []
  page.on('request', request => {
    if (new URL(request.url()).pathname.startsWith('/v1/tenant/')) privateRequests.push(request.url())
  })
  await page.goto('/zh-CN/')
  const clipboard = () => page.evaluate(() => navigator.clipboard.readText())
  await page.getByRole('button', { name: '复制 SOCKS5 示例命令' }).click()
  await expect(page.getByRole('button', { name: '已复制 SOCKS5 示例命令' })).toBeVisible()
  expect(await clipboard()).toContain('acme.user01-sid-a7k2:example-password@gateway.example.com')

  await expect(page.getByText('示例账号', { exact: true })).toBeVisible()
  await expect(page.getByRole('complementary', { name: '出口响应示例' })).toContainText(/192\.0\.2\.|198\.51\.100\.|203\.0\.113\./)
  await expect(page.getByText('控制台示例数据')).toBeVisible()
  const examples = { cURL: 'curl -x', Python: 'import requests', 'Node.js': 'SocksProxyAgent', Go: 'proxy.SOCKS5' }
  for (const [language, marker] of Object.entries(examples)) {
    await page.getByRole('button', { name: language, exact: true }).click()
    await expect(page.getByRole('button', { name: language, exact: true })).toHaveAttribute('aria-pressed', 'true')
    await page.getByRole('button', { name: '复制示例代码', exact: true }).click()
    await expect(page.getByText('已复制示例代码', { exact: true }).last()).toBeVisible()
    const copied = await clipboard()
    expect(copied).toContain(marker)
    expect(copied).toContain('gateway.example.com')
  }
  await screenshot(page, info, 'copied-go-example')
  expect(privateRequests).toEqual([])
})
