import { test, expect } from '@playwright/test'
import { screenshot, submitCredentials } from '../support/console.mjs'

test('手机号自助开户 → +86 去重并登录同一租户', async ({ page }, info) => {
  const phone = '13800000031'
  const main = page.getByRole('main')
  const accountName = page.getByRole('complementary').locator('strong[title]')
  let registeredAccount

  async function expectZeroBalances() {
    await expect(page.getByRole('heading', { name: '账户概览', exact: true })).toBeVisible()
    await expect(main.getByText('¥0.00', { exact: true })).toBeVisible()
    await expect(main.getByText(/^0(?:\.0+)? (?:B|GB)$/)).toBeVisible()
  }

  await test.step('用带 +86 的测试手机号注册，查看初始余额', async () => {
    await page.goto('/console')
    await page.getByRole('button', { name: '注册账号', exact: true }).click()
    await submitCredentials(page, { account: '+86' + phone, button: '注册' })
    await expectZeroBalances()
    await expect(accountName).toHaveText(/\S+/)
    registeredAccount = await accountName.innerText()
    await screenshot(page, info, '01-new-account')
  })

  await test.step('退出，再用同一号码的本地写法注册，应拒绝重复开户', async () => {
    await page.getByRole('button', { name: '退出登录', exact: true }).click()
    await page.getByRole('button', { name: '注册账号', exact: true }).click()
    await submitCredentials(page, { account: phone, password: 'other-local-test', button: '注册' })
    await expect(page.getByRole('alert')).toHaveText('该手机号已注册，请登录。')
    await screenshot(page, info, '02-duplicate-phone')
  })

  await test.step('用不带 +86 的手机号和原密码登录，仍进入同一租户', async () => {
    await page.getByRole('button', { name: '已有账号，去登录', exact: true }).click()
    await submitCredentials(page, { account: phone })
    await expectZeroBalances()
    await expect(accountName).toHaveText(registeredAccount)
    await screenshot(page, info, '03-existing-account')
  })
})
