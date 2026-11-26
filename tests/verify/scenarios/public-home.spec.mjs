import { test, expect } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'
import { authenticatedApi, json, setTier } from '../support/traffic.mjs'

async function sale(operator, kind, open) {
  await json(await operator.put(`/v1/operations/product-sales/${kind}`, {
    data: { open, ...(open ? { quote_ready: true, acceptance_evidence: 'verify: public homepage #113' } : {}) },
  }))
}

const homepage = test.extend({
  openedCatalog: async ({ playwright, baseURL }, use) => {
    const operator = await authenticatedApi(playwright, baseURL)
    try {
      await setTier(operator, '5gb', '5100')
      await setTier(operator, '50gb', '18000')
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

test('双语首页、两种视口、直接访问和刷新，展示钴蓝版式、固定起售价、10 国网格与客服坞', async ({ page }, info) => {
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
      await expect(page.getByRole('heading', { level: 1 })).toContainText(locale === 'zh-CN' ? '用真实住宅 IP' : 'Use real residential IPs to')
      for (const section of ['#products', '#coverage', '#solutions', '#faq', '.home-footer']) {
        await expect(page.locator(section)).toBeVisible()
      }
      await expect(page.locator('#integration')).toHaveCount(0)
      await expect(page.locator('#manage')).toHaveCount(0)
      for (const price of ['¥3.5', '¥16', '¥2']) {
        await expect(page.locator('#products').getByText(price, { exact: true })).toBeVisible()
      }
      await expect(page.locator('#coverage .country-card')).toHaveCount(10)
      await expect(page.locator('#solutions .col')).toHaveCount(4)
      await expect(page.locator('#faq details')).toHaveCount(5)
      await expect(page.locator('canvas').first()).toBeVisible()
      const supportNav = page.getByRole('navigation', { name: locale === 'zh-CN' ? '联系人工客服' : 'Contact human support' })
      await expect(supportNav).toBeVisible()
      if (width === 390) {
        await supportNav.getByRole('button', { name: locale === 'zh-CN' ? /在线客服/ : /Live chat/ }).click()
        await expect(page.getByText(locale === 'zh-CN' ? '即将开通' : 'Coming soon').last()).toBeVisible()
      }
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(width)
      await screenshot(page, info, `homepage-${locale}-${width}`)
    }
  }
  expect(errors).toEqual([])
})

homepage('英文注册和登录入口、首页跳转套餐报价与动态套餐登录回跳', async ({ page, openedCatalog }, info) => {
  await page.goto('/en/')
  await page.getByRole('banner').getByRole('link', { name: 'Sign up', exact: true }).click()
  await expect(page).toHaveURL(/\/en\/register$/)

  await page.goto('/en/')
  await page.getByRole('banner').getByRole('link', { name: /Sign in|Log in/i }).click()
  await expect(page).toHaveURL(/\/en\/login$/)

  await page.goto('/en/')
  await page.locator('#products').getByRole('link', { name: 'Dynamic residential', exact: true }).click()
  await expect(page).toHaveURL(/\/en\/pricing\?category=dynamic$/)
  await page.getByRole('link', { name: /Buy 50 GB/ }).click()
  await expect(page).toHaveURL(/\/en\/login\?redirect=/)
  expect(new URL(page.url()).searchParams.get('redirect')).toBe('/console/dynamic/pricing?package=50gb')
  await field(page, 'Email address').fill('alpha@example.test')
  await field(page, 'Password').fill('proxy-local-test')
  await page.getByRole('button', { name: 'Sign in', exact: true }).click()
  await expect(page).toHaveURL(/\/console\/dynamic\/pricing\?package=50gb$/)
  await expect(page.getByRole('radio', { name: /50 GB/ })).toBeChecked()
  await screenshot(page, info, 'returned-to-50gb-plan')
})

test('切换轮换标题时页面不跳动，顶栏在滚动中保留，游客首页不请求私有接口', async ({ page }) => {
  const privateRequests = []
  page.on('request', request => {
    if (new URL(request.url()).pathname.startsWith('/v1/tenant/')) privateRequests.push(request.url())
  })
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/en/')
  const heading = page.getByRole('heading', { level: 1 })
  await expect(heading).toHaveAccessibleName(/run cross-border stores/)
  const top = () => heading.evaluate(el => el.getBoundingClientRect().top)
  const initial = await top()
  await expect(heading).toHaveAccessibleName(/grow social media matrices/)
  expect(await top()).toBeCloseTo(initial, 0)

  await page.locator('#products').scrollIntoViewIfNeeded()
  const banner = await page.getByRole('banner').boundingBox()
  expect(banner.y).toBe(0)
  expect(privateRequests).toEqual([])
})
