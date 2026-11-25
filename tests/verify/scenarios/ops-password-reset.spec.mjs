import { test, expect } from '@playwright/test'
import { field, screenshot, signInOperator } from '../support/console.mjs'
import { registerEmail } from '../support/mail.mjs'

test.use({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai', screenshot: 'off', trace: 'off', video: 'off' })

test('账号运营核实目标后重置租户密码，原有会话与旧密码失效', async ({ page, browser, request }, info) => {
  const email = 'fallback-' + Date.now() + '@example.test'
  const oldPassword = 'proxy-local-test'
  const registered = await registerEmail(request, { email, password: oldPassword })
  expect(registered.status()).toBe(200)
  const headersFor = async (loginPath, data) => {
    const result = await request.post(loginPath, { data: { ...data, password: oldPassword } })
    expect(result.status()).toBe(200)
    return { Authorization: 'Bearer ' + (await result.json()).access_token }
  }
  const customerHeaders = await headersFor('/v1/access/login', { email })
  const accountResponse = await request.get('/v1/tenant/account', { headers: customerHeaders })
  expect(accountResponse.status()).toBe(200)
  const account = await accountResponse.json()
  const detail = '/ops/tenants/' + account.tenant_id
  const api = '/v1/operations/tenants/' + account.tenant_id
  const reset = (headers, password) => request.post(api + ':reset-password', { headers, data: { password } })
  const operatorPath = '/v1/operations/access/login'
  const resetButton = page.getByRole('button', { name: '重置登录密码', exact: true })
  const supportHeaders = await headersFor(operatorPath, { username: 'operator-support' })
  const tenant = await (await request.get(api, { headers: supportHeaders })).json()

  for (const username of ['operator-readonly', 'operator-billing']) {
    const result = await reset(await headersFor(operatorPath, { username }), oldPassword)
    expect(result.status()).toBe(403)
    expect((await result.json()).reason).toBe('ACCESS_DENIED')
  }
  const customerAttempt = await reset(customerHeaders, oldPassword)
  expect(customerAttempt.status()).toBe(403)
  expect((await customerAttempt.json()).reason).toBe('ACCESS_DENIED')
  const invalid = await reset(supportHeaders, '12345678901')
  expect(invalid.status()).toBe(400)
  expect((await invalid.json()).reason).toBe('INVALID_PASSWORD')

  await signInOperator(page, detail, 'operator-readonly')
  await expect(resetButton).toBeDisabled()
  const billingPage = await browser.newPage({ ignoreHTTPSErrors: true })
  await signInOperator(billingPage, new URL(detail, page.url()).href, 'operator-billing')
  await expect(billingPage.getByRole('button', { name: '重置登录密码', exact: true })).toBeDisabled()
  await billingPage.close()

  const supportPage = await browser.newPage({ ignoreHTTPSErrors: true, locale: 'zh-CN', timezoneId: 'Asia/Shanghai', viewport: { width: 1440, height: 1000 } })
  await signInOperator(supportPage, new URL('/ops/tenants/visual-recharge-tenant', page.url()).href, 'operator-support')
  await supportPage.getByRole('button', { name: '重置登录密码', exact: true }).click()
  const dialog = supportPage.getByRole('dialog', { name: '重置登录密码' })
  await expect(dialog).toContainText('陈小岚工作室')
  await expect(dialog).toContainText('chenxl-studio')
  await screenshot(supportPage, info, '08-reset-form')

  await supportPage.goto(new URL(detail, page.url()).href)
  await supportPage.getByRole('button', { name: '重置登录密码', exact: true }).click()
  await expect(dialog).toContainText(tenant.name)
  await expect(dialog).toContainText(tenant.primary_account_name)
  const confirm = dialog.getByRole('button', { name: '确认重置' })
  await expect(confirm).toBeDisabled()
  await dialog.getByText('选择核实方式', { exact: true }).click()
  await supportPage.getByRole('option', { name: '回拨客户预留电话核实' }).click()
  await expect(confirm).toBeDisabled()
  await field(dialog, '核实记录').fill('回拨已完成，本人确认开户信息')

  // Give the browser a real not-found response from the control plane on the first attempt.
  await supportPage.route('**' + api + ':reset-password', async route => {
    const missing = await route.fetch({ url: new URL('/v1/operations/tenants/missing-reset-tenant:reset-password', page.url()).href })
    await route.fulfill({ response: missing })
  }, { times: 1 })
  await confirm.click()
  await expect(dialog.getByText('找不到租户，请返回租户查找。')).toBeVisible()
  await expect(confirm).toBeEnabled()

  await confirm.click()
  await expect(dialog.getByText('临时密码只显示这一次', { exact: false })).toBeVisible()
  await expect(dialog).toContainText('operator-support')
  const password = dialog.locator('.temp-password .mono')
  const newPassword = await password.textContent()
  expect(newPassword.length).toBeGreaterThanOrEqual(12)
  const evidence = info.outputPath('08-reset-result.png')
  await supportPage.screenshot({ path: evidence, fullPage: true, animations: 'disabled', mask: [password] })
  await info.attach('08-reset-result (password masked)', { path: evidence, contentType: 'image/png' })
  await dialog.getByRole('button', { name: '完成' }).click()
  await supportPage.getByRole('button', { name: '重置登录密码', exact: true }).click()
  await expect(password).toHaveCount(0)
  await expect(confirm).toBeDisabled()

  const stale = await request.get('/v1/tenant/account', { headers: customerHeaders })
  expect(stale.status()).toBe(401)
  const old = await request.post('/v1/access/login', { data: { email, password: oldPassword } })
  expect(old.status()).toBe(401)
  const customerPage = await browser.newPage({ ignoreHTTPSErrors: true })
  await customerPage.goto(new URL('/zh-CN/login', page.url()).href)
  await field(customerPage, '邮箱').fill(email)
  await field(customerPage, '密码').fill(newPassword)
  await customerPage.getByRole('button', { name: '登录', exact: true }).click()
  await expect(customerPage).toHaveURL(/\/console$/)
  await customerPage.close()
  await supportPage.close()
})
