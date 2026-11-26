import { test, expect } from '@playwright/test'
import { screenshot, signInOperator, signInTenant } from '../support/console.mjs'

// #101 应用 RocProxy 品牌：提案 A 图标、RocProxy 字标、三类入口浏览器标题与图标、素材加载及导航。

const CONCEPT_A_MARKERS = [
  'viewBox="0 0 40 40"',
  'id="roc-a-bg"',
  'stop-color="#3b82f6"',
  'stop-color="#1e3a8a"',
  'M10 11H23.6C27.8 11 30.2 13.4 30.2 16.7',
  'M10 13.8L14.4 17.8V29H10V13.8Z',
  'M19.2 23.8H24.5L30.2 29H24.4L19.2 23.8Z',
  'fill="#38bdf8"',
]

async function switchLanguage(page, name) {
  await page.getByRole('button', { name: /^(语言|Language)[:：]/ }).click()
  await page.getByRole('menuitem', { name, exact: true }).click()
}

async function verifyFavicon(page) {
  const icon = page.locator('head link[rel="icon"]')
  await expect(icon).toHaveCount(1)
  await expect(icon).toHaveAttribute('type', 'image/svg+xml')
  await expect(icon).toHaveAttribute('href', '/favicon.svg')

  const response = await page.request.get('/favicon.svg')
  expect(response.status()).toBe(200)
  expect(response.headers()['content-type'] ?? '').toContain('image/svg+xml')
  const svg = await response.text()
  for (const marker of CONCEPT_A_MARKERS) {
    expect(svg).toContain(marker)
  }
  return { href: '/favicon.svg', status: response.status(), contentType: response.headers()['content-type'] ?? '' }
}

async function verifyBrandImage(img) {
  await expect(img).toBeVisible()
  await expect(img).toHaveAttribute('src', '/favicon.svg')
  const state = await img.evaluate(el => ({
    complete: el.complete,
    naturalWidth: el.naturalWidth,
    naturalHeight: el.naturalHeight,
    width: el.getBoundingClientRect().width,
    height: el.getBoundingClientRect().height,
  }))
  expect(state.complete).toBe(true)
  expect(state.naturalWidth).toBeGreaterThan(0)
  expect(state.naturalHeight).toBeGreaterThan(0)
  expect(state.width).toBeGreaterThanOrEqual(24)
  expect(state.height).toBeGreaterThanOrEqual(24)
  return state
}

async function verifyHeaderBrand(page) {
  const brandLink = page.getByRole('banner').getByRole('link', { name: 'RocProxy', exact: true })
  await expect(brandLink).toBeVisible()
  await expect(brandLink).toHaveText('RocProxy')
  const imageState = await verifyBrandImage(brandLink.locator('img.brand-mark__icon'))
  return { brandLink, imageState }
}

test('官网、客户控制台、运营台展示提案 A 和 RocProxy 字标，标题图标一致且导航可用', async ({ page }, info) => {
  const pageErrors = []
  const failedAssets = []
  const unexpectedWrites = []
  page.on('pageerror', error => pageErrors.push(error.message))
  page.on('request', request => {
    const method = request.method()
    const pathname = new URL(request.url()).pathname
    if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(method) &&
      pathname !== '/v1/access/login' &&
      pathname !== '/v1/operations/access/login') {
      unexpectedWrites.push(`${method} ${pathname}`)
    }
  })
  page.on('response', response => {
    if (new URL(response.url()).pathname === '/favicon.svg' && response.status() >= 400) {
      failedAssets.push(`${response.status()} ${response.url()}`)
    }
  })

  const records = []

  // 1. 官网入口（中英文 + 登录页回跳首页）
  await page.emulateMedia({ reducedMotion: 'reduce' })
  await page.goto('/zh-CN/')
  await expect(page).toHaveURL(/\/zh-CN\/$/)
  await expect(page).toHaveTitle('首页 · RocProxy')
  const publicFavicon = await verifyFavicon(page)
  const publicBrand = await verifyHeaderBrand(page)
  const footerBrand = page.locator('.footer-brand')
  await expect(footerBrand).toContainText('RocProxy')
  await verifyBrandImage(footerBrand.locator('img.brand-mark__icon'))
  await expect(page.locator('.footer-bottom')).toContainText('RocProxy')
  await screenshot(page, info, 'brand-public-zh-CN')
  records.push({
    entry: 'public-zh-CN',
    url: page.url(),
    title: await page.title(),
    favicon: publicFavicon,
    logo: publicBrand.imageState,
  })

  await page.goto('/zh-CN/login')
  await expect(page).toHaveTitle('登录 · RocProxy')
  const loginBrand = await verifyHeaderBrand(page)
  await loginBrand.brandLink.click()
  await expect(page).toHaveURL(/\/zh-CN\/$/)

  await switchLanguage(page, 'English')
  await expect(page).toHaveURL(/\/en\/$/)
  await expect(page).toHaveTitle('Home · RocProxy')
  const publicEnBrand = await verifyHeaderBrand(page)
  await screenshot(page, info, 'brand-public-en')
  records.push({
    entry: 'public-en',
    url: page.url(),
    title: await page.title(),
    favicon: await verifyFavicon(page),
    logo: publicEnBrand.imageState,
  })

  await page.setViewportSize({ width: 390, height: 844 })
  const narrowHeaderOverflow = await page.evaluate(() => {
    const header = document.querySelector('header')
    return header ? header.scrollWidth - header.clientWidth : -1
  })
  expect(narrowHeaderOverflow).toBe(0)
  const signUpBox = await page.getByRole('banner').getByRole('link', { name: 'Sign up', exact: true }).boundingBox()
  expect(signUpBox.x + signUpBox.width).toBeLessThanOrEqual(390)
  await page.setViewportSize({ width: 1440, height: 1000 })

  // 2. 客户控制台入口（直接打开 + 侧栏与品牌导航 + 双语标题）
  await signInTenant(page)
  await page.goto('/console')
  await expect(page).toHaveURL(/\/console$/)
  await expect(page).toHaveTitle('仪表板 · 用户控制台 · RocProxy')
  const consoleFavicon = await verifyFavicon(page)
  const consoleBrand = await verifyHeaderBrand(page)
  await screenshot(page, info, 'brand-console-zh-CN')
  records.push({
    entry: 'console-zh-CN',
    url: page.url(),
    title: await page.title(),
    favicon: consoleFavicon,
    logo: consoleBrand.imageState,
  })

  await page.getByRole('navigation', { name: '控制台导航' }).getByRole('menuitem', { name: '钱包', exact: true }).click()
  await expect(page).toHaveURL(/\/console\/wallet$/)
  await expect(page).toHaveTitle('钱包 · 用户控制台 · RocProxy')

  await switchLanguage(page, 'English')
  await expect(page).toHaveTitle('Wallet · Console · RocProxy')
  const consoleEnBrand = await verifyHeaderBrand(page)
  await screenshot(page, info, 'brand-console-en')
  records.push({
    entry: 'console-en',
    url: page.url(),
    title: await page.title(),
    favicon: await verifyFavicon(page),
    logo: consoleEnBrand.imageState,
  })

  await switchLanguage(page, '简体中文')
  await expect(page).toHaveTitle('钱包 · 用户控制台 · RocProxy')
  await (await verifyHeaderBrand(page)).brandLink.click()
  await expect(page).toHaveURL(/\/zh-CN\/$/)

  // 3. 运营台入口（直接打开未登录态 + 登录后直接打开 + 侧栏与品牌导航 + 双语标题）
  await page.goto('/ops')
  await expect(page).toHaveURL(/\/ops\/login\?redirect=/)
  await expect(page).toHaveTitle('运营登录 · 运营控制台 · RocProxy')
  const opsLoginBrand = page.locator('.operator-login__brand')
  await expect(opsLoginBrand).toContainText('RocProxy')
  await expect(opsLoginBrand).toContainText('运营后台')
  await verifyBrandImage(opsLoginBrand.locator('img.brand-mark__icon'))
  await verifyFavicon(page)
  await screenshot(page, info, 'brand-ops-login-zh-CN')

  await signInOperator(page, '/ops/tenants')
  await expect(page).toHaveURL(new URL('/ops/tenants', page.url()).href)
  await expect(page.getByRole('navigation', { name: '运营导航' })).toBeVisible()
  await page.goto('/ops/tenants')
  await expect(page).toHaveURL(new URL('/ops/tenants', page.url()).href)
  await expect(page).toHaveTitle('租户 · 运营控制台 · RocProxy')
  const opsFavicon = await verifyFavicon(page)
  const opsBrand = await verifyHeaderBrand(page)
  await expect(page.getByRole('banner')).toContainText('运营后台')
  await screenshot(page, info, 'brand-ops-zh-CN')
  records.push({
    entry: 'ops-zh-CN',
    url: page.url(),
    title: await page.title(),
    favicon: opsFavicon,
    logo: opsBrand.imageState,
  })

  await page.getByRole('navigation', { name: '运营导航' }).getByRole('menuitem', { name: '订单处理', exact: true }).click()
  await expect(page).toHaveURL(/\/ops\/orders$/)
  await expect(page).toHaveTitle('订单处理 · 运营控制台 · RocProxy')

  await switchLanguage(page, 'English')
  await expect(page).toHaveTitle('Order Handling · Operations Console · RocProxy')
  await expect(page.getByRole('banner')).toContainText('Operations')
  const opsEnBrand = await verifyHeaderBrand(page)
  await screenshot(page, info, 'brand-ops-en')
  records.push({
    entry: 'ops-en',
    url: page.url(),
    title: await page.title(),
    favicon: await verifyFavicon(page),
    logo: opsEnBrand.imageState,
  })

  await opsEnBrand.brandLink.click()
  await expect(page).toHaveURL(new URL('/ops/tenants', page.url()).href)

  await info.attach('brand-checks', {
    body: JSON.stringify(records, null, 2),
    contentType: 'application/json',
  })

  expect(unexpectedWrites).toEqual([])
  expect(failedAssets).toEqual([])
  expect(pageErrors).toEqual([])
})
