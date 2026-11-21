import { test, expect } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'

// #69 手机号注册、登录与控制台会话。种子租户 alpha：13800000001 / proxy-local-test，主账号 localalpha。

const heading = page => page.getByRole('heading', { level: 1 })
const phoneInput = page => field(page, '手机号')
const passwordInput = page => field(page, '密码')

async function submit(page, phone, password, button) {
  await phoneInput(page).fill(phone)
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

  await page.getByRole('button', { name: '忘记密码？' }).click()
  await expect(page.getByText('请联系运营，运营核实账户归属后协助重置')).toBeVisible()
  await heading(page).click()

  await submit(page, '13800000001', 'not-the-password', '登录')
  await expect(page.getByRole('alert')).toContainText('账号或密码错误。')
  await expect(page).toHaveURL(/\/zh-CN\/login/)

  await submit(page, '+86 138 0000 0001', 'proxy-local-test', '登录')
  await expect(page).toHaveURL(/\/console\/wallet\?from=verify$/)
  await expect(heading(page)).toHaveText('钱包')

  await page.reload()
  await expect(heading(page)).toHaveText('钱包')
  const menu = await openAccountMenu(page)
  await expect(menu).toContainText('localalpha')
  await expect(menu).toContainText('+86 138 **** 0001')
  await expect(menu.getByRole('menuitem')).toHaveText(['账户信息', '订单记录', '退出登录'])
  await screenshot(page, info, 'account-menu')
  await menu.getByRole('menuitem', { name: '订单记录' }).click()
  await expect(page).toHaveURL(/\/console\/account\/orders$/)

  await signOut(page)
  await page.goto('/console')
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=(%2F|\/)console$/)
})

test('注册：密码过短 → 新号码注册直接登录 → 同号码重复注册被拒 → 再次登录', async ({ page }, info) => {
  const number = '139' + String(Date.now()).slice(-8)
  await page.goto('/zh-CN/register?redirect=https%3A%2F%2Fevil.example%2F')
  await submit(page, `+86 ${number}`, 'too-short', '注册并登录')
  await expect(page.getByText('密码至少需要 12 位', { exact: true })).toBeVisible()

  await passwordInput(page).fill('verify-register-pw')
  await page.getByRole('button', { name: '注册并登录', exact: true }).click()
  await expect(page).toHaveURL(/\/console$/)
  await expect(heading(page)).toHaveText('仪表板')
  const menu = await openAccountMenu(page)
  await expect(menu).toContainText(`+86 ${number.slice(0, 3)} **** ${number.slice(7)}`)
  await heading(page).click()
  await expect(menu).toBeHidden()
  await screenshot(page, info, 'registered-dashboard')
  await signOut(page)

  await page.goto('/zh-CN/register')
  await submit(page, number, 'verify-register-pw', '注册并登录')
  await expect(page.getByText('该手机号已注册，请直接登录。', { exact: true })).toBeVisible()

  await page.getByRole('link', { name: '去登录' }).click()
  await expect(heading(page)).toHaveText('登录')
  await submit(page, number, 'verify-register-pw', '登录')
  await expect(heading(page)).toHaveText('仪表板')
})

test('凭据失效时清理会话并回到登录页，保留回跳地址', async ({ page }) => {
  await page.goto('/zh-CN/')
  await page.evaluate(() => localStorage.setItem('proxy-console.session', JSON.stringify({ token: 'forged-token', expiresAt: Date.now() + 3_600_000, phone: '13800000001' })))
  await page.goto('/console/wallet')
  await expect(page).toHaveURL(/\/zh-CN\/login\?redirect=(%2F|\/)console(%2F|\/)wallet$/)
  expect(await page.evaluate(() => localStorage.getItem('proxy-console.session'))).toBeNull()

  await submit(page, '13800000001', 'proxy-local-test', '登录')
  await expect(heading(page)).toHaveText('钱包')
})

test('已登录的窄屏顶栏只显示账号图标，不横向溢出', async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/zh-CN/login')
  await submit(page, '13800000001', 'proxy-local-test', '登录')
  await expect(heading(page)).toHaveText('仪表板')
  const overflow = await page.evaluate(() => [document.documentElement, document.querySelector('header')].map(el => el.scrollWidth - el.clientWidth))
  expect(overflow).toEqual([0, 0])
  await screenshot(page, info, 'mobile-signed-in')
})
