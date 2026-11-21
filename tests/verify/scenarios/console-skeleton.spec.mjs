import { test, expect } from '@playwright/test'
import { screenshot, signInOperator, signInTenant } from '../support/console.mjs'

// #68 前端工程骨架：三套布局、默认入口、深链回退、双语偏好和窄屏版式。控制台自 #69 起需要登录。

const heading = page => page.getByRole('heading', { level: 1 })

async function switchLanguage(page, name) {
  await page.getByRole('button', { name: /^(语言|Language)[:：]/ }).click()
  await page.getByRole('menuitem', { name, exact: true }).click()
}

test('默认入口、深链和未找到页', async ({ page }, info) => {
  await page.goto('/')
  await expect(page).toHaveURL(/\/zh-CN\/$/)
  await expect(heading(page)).toHaveText('稳定的住宅代理，按需购买')
  await expect(page.locator('html')).toHaveAttribute('lang', 'zh-CN')

  await page.goto('/en/')
  await expect(heading(page)).toHaveText('Reliable residential proxies, bought as you need them')

  await signInTenant(page)
  await page.goto('/console/wallet')
  const nav = page.getByRole('navigation', { name: '控制台导航' })
  await expect(nav.getByRole('menuitem')).toHaveText(['仪表板', '动态住宅代理-GB', '静态住宅代理', '代理子账号', '钱包', '我的账户'])
  await expect(nav.getByText('代理', { exact: true })).toBeVisible()
  await expect(nav.getByText('管理', { exact: true })).toBeVisible()
  await expect(heading(page)).toHaveText('钱包')
  await expect(page.getByRole('heading', { name: '钱包流水', exact: true })).toBeVisible()
  await screenshot(page, info, 'console-wallet-desktop')

  await page.goto('/console/static')
  await expect(page).toHaveURL(/\/console\/static\/ips$/)
  await expect(page.getByRole('tab', { name: 'IP 列表' })).toHaveAttribute('aria-selected', 'true')
  await page.getByRole('tab', { name: '价格' }).click()
  await expect(page).toHaveURL(/\/console\/static\/pricing$/)
  await page.goBack()
  await expect(page).toHaveURL(/\/console\/static\/ips$/)

  await page.goto('/ops')
  await expect(page).toHaveURL(/\/ops\/login\?redirect=/)
  await signInOperator(page)
  await expect(page).toHaveURL(/\/ops\/tenants$/)
  await expect(page.getByRole('navigation', { name: '运营导航' }).getByRole('menuitem')).toHaveText(['租户', '订单处理', '商品与定价'])

  await page.goto('/no-such-page/deeper')
  await expect(page.getByText('页面不存在')).toBeVisible()
})

test('控制台切换语言保持 URL 并记住偏好，官网切换到对应语言路由', async ({ page }, info) => {
  await signInTenant(page)
  await page.goto('/console/dynamic/pricing?keep=1')
  await expect(heading(page)).toHaveText('动态住宅代理-GB')
  await switchLanguage(page, 'English')
  await expect(page).toHaveURL(/\/console\/dynamic\/pricing\?keep=1$/)
  await expect(heading(page)).toHaveText('Rotating Residential (GB)')
  await expect(page.locator('html')).toHaveAttribute('lang', 'en')
  await page.reload()
  await expect(heading(page)).toHaveText('Rotating Residential (GB)')
  await expect(page.getByRole('tab', { name: 'Pricing' })).toHaveAttribute('aria-selected', 'true')
  await screenshot(page, info, 'console-dynamic-en')

  await page.goto('/zh-CN/?ref=verify')
  await expect(heading(page)).toHaveText('稳定的住宅代理，按需购买')
  await switchLanguage(page, 'English')
  await expect(page).toHaveURL(/\/en\/\?ref=verify$/)
  await expect(heading(page)).toHaveText('Reliable residential proxies, bought as you need them')
})

for (const width of [390, 1280]) {
  for (const locale of ['zh-CN', 'en']) {
    test(`${width}px ${locale} 版式无失控的横向溢出`, async ({ page }, info) => {
      await page.setViewportSize({ width, height: 844 })
      await signInTenant(page)
      await page.addInitScript(value => localStorage.setItem('proxy-console.locale', value), locale)
      await signInOperator(page, '/ops', 'operator', locale)
      await expect(page).toHaveURL(/\/ops\/tenants$/)
      for (const path of [`/${locale}/`, '/console/dynamic/setup', '/ops/products/dynamic']) {
        await page.goto(path)
        await expect(heading(page)).toBeVisible()
        const overflow = await page.evaluate(() => [document.documentElement, document.querySelector('header')]
          .map(el => el.scrollWidth - el.clientWidth))
        expect(overflow, path).toEqual([0, 0])
      }
      if (width < 768) {
        await page.goto('/console/dynamic/setup')
        await page.getByRole('button', { name: locale === 'en' ? 'Open navigation' : '打开导航' }).click()
        const nav = page.getByRole('navigation', { name: locale === 'en' ? 'Console navigation' : '控制台导航' })
        await nav.getByRole('menuitem', { name: locale === 'en' ? 'Wallet' : '钱包' }).click()
        await expect(page).toHaveURL(/\/console\/wallet$/)
        await expect(nav).toBeHidden()
      }
      await screenshot(page, info, `layout-${width}-${locale}`)
    })
  }
}
