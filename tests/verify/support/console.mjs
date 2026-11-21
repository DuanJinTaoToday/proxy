import { expect } from '@playwright/test'

// 表单标签可能带必填星号，如「*手机号」。
export const field = (page, label) => page.getByRole('textbox', { name: new RegExp(`^\\*?${label}`) })

export async function login(page, route, label, account, locale = 'zh-CN') {
  await page.goto(route)
  await field(page, label).fill(account)
  await field(page, locale === 'en' ? 'Password' : '密码').fill('proxy-local-test')
  await page.getByRole('button', { name: locale === 'en' ? 'Sign in' : '登录', exact: true }).click()
}

// 用种子租户 alpha 从登录页登录，等到进入控制台。
export async function signInTenant(page, phone = '13800000001') {
  await login(page, '/zh-CN/login', '手机号', phone)
  await expect(page).toHaveURL(/\/console$/)
}

export async function signInOperator(page, route = '/ops', account = 'operator', locale = 'zh-CN') {
  await login(page, route, locale === 'en' ? 'Operator username' : '运营账号', account, locale)
}

export async function screenshot(page, info, name) {
  const path = info.outputPath(name + '.png')
  await page.screenshot({ path, fullPage: true })
  await info.attach(name, { path, contentType: 'image/png' })
}
