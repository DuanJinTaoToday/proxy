import { test, expect } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'
import { receivedCode } from '../support/mail.mjs'

const heading = page => page.getByRole('heading', { level: 1 })
const emailInput = page => field(page, '邮箱')
const passwordInput = page => field(page, '密码')

async function submit(page, email, password, button) {
  await emailInput(page).fill(email)
  await passwordInput(page).fill(password)
  await page.getByRole('button', { name: button, exact: true }).click()
}

async function openAccountMenu(page) {
  await page.getByRole('button', { name: '账号菜单' }).click()
  return page.locator('.el-dropdown__popper').filter({ visible: true })
}

async function signOut(page) {
  const menu = await openAccountMenu(page)
  await menu.getByRole('menuitem', { name: '退出登录' }).click()
  await page.getByRole('dialog').getByRole('button', { name: '退出登录' }).click()
  await expect(page).toHaveURL(/\/zh-CN\/login$/)
}

test('未登录跳转登录 → 账密错误 → 登录回到原页 → 刷新保持 → 退出', async ({ page }, info) => {
  await page.goto('/console/wallet?from=verify')
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=/)
  await expect(heading(page)).toHaveText('登录')

  await submit(page, 'alpha@example.test', 'not-the-password', '登录')
  await expect(page.getByRole('alert')).toContainText('账号或密码错误。')
  await expect(page).toHaveURL(/\/zh-CN\/login/)

  await submit(page, ' ALPHA@EXAMPLE.TEST ', 'proxy-local-test', '登录')
  await expect(page).toHaveURL(/\/console\/wallet\?from=verify$/)
  await expect(heading(page)).toHaveText('钱包')

  await page.reload()
  await expect(heading(page)).toHaveText('钱包')
  const menu = await openAccountMenu(page)
  await expect(menu).toContainText('localalpha')
  await expect(menu).toContainText('alpha@example.test')
  await expect(menu.getByRole('menuitem')).toHaveText(['账户信息', '订单记录', '退出登录'])
  await screenshot(page, info, 'account-menu')
  await menu.getByRole('menuitem', { name: '订单记录' }).click()
  await expect(page).toHaveURL(/\/console\/account\/orders$/)

  await signOut(page)
  await page.goto('/console')
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=(%2F|\/)console$/)
})

test('邮件验证码注册、保留站内跳转边界、拒绝重放并再次登录', async ({ page }, info) => {
  const email = 'browser.first+alias@example.test'
  const password = 'verify-register-pw'
  await page.goto('/zh-CN/register?redirect=https%3A%2F%2Fexample.test%2F')
  await emailInput(page).fill(email)
  await page.getByRole('button', { name: '发送验证码', exact: true }).click()
  await expect(page.getByText('如果该邮箱可以注册，验证码已发送，请查收邮件。')).toBeVisible()
  const code = await receivedCode(email)
  await field(page, '验证码').fill(code)
  await submit(page, email, 'too-short', '注册并登录')
  await expect(page.getByText('密码至少需要 12 位', { exact: true })).toBeVisible()

  await submit(page, email, password, '注册并登录')
  await expect(page).toHaveURL(/\/console$/)
  await expect(heading(page)).toHaveText('仪表板')
  const menu = await openAccountMenu(page)
  await expect(menu).toContainText(email)
  await heading(page).click()
  await expect(menu).toBeHidden()
  await screenshot(page, info, 'registered-dashboard')
  await signOut(page)

  await page.goto('/zh-CN/register')
  await field(page, '验证码').fill(code)
  await submit(page, email.toUpperCase(), password, '注册并登录')
  await expect(page.getByText('验证码错误或已过期，请重新获取。')).toBeVisible()
  await page.getByRole('link', { name: '去登录' }).click()
  await expect(heading(page)).toHaveText('登录')
  await submit(page, email.toUpperCase(), password, '登录')
  await expect(heading(page)).toHaveText('仪表板')
})

test('凭据失效时清理会话并回到登录页，保留回跳地址', async ({ page }) => {
  await page.goto('/zh-CN/')
  await page.evaluate(() => localStorage.setItem('proxy-console.session', JSON.stringify({ token: 'forged-token', expiresAt: Date.now() + 3_600_000, email: 'alpha@example.test' })))
  await page.goto('/console/wallet')
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=(%2F|\/)console(%2F|\/)wallet$/)
  expect(await page.evaluate(() => localStorage.getItem('proxy-console.session'))).toBeNull()

  await submit(page, 'alpha@example.test', 'proxy-local-test', '登录')
  await expect(heading(page)).toHaveText('钱包')
})

test('已登录的窄屏顶栏只显示账号图标，不横向溢出', async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/zh-CN/login')
  await submit(page, 'alpha@example.test', 'proxy-local-test', '登录')
  await expect(heading(page)).toHaveText('仪表板')
  const overflow = await page.evaluate(() => [document.documentElement, document.querySelector('header')].map(el => el.scrollWidth - el.clientWidth))
  expect(overflow).toEqual([0, 0])
  await screenshot(page, info, 'mobile-signed-in')
})
