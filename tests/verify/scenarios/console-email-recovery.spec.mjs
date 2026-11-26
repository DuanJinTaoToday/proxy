import { expect, test } from '@playwright/test'
import { field, screenshot, signInOperator } from '../support/console.mjs'
import { receivedLink, registerEmail } from '../support/mail.mjs'

async function customerHeaders(request, email, password) {
  const response = await request.post('/v1/access/login', { data: { email, password } })
  expect(response.ok()).toBeTruthy()
  return { Authorization: 'Bearer ' + (await response.json()).access_token }
}

async function changePassword(page, link, button, password) {
  await page.goto(link)
  await expect.poll(() => new URL(page.url()).search).toBe('')
  await field(page, '新密码').fill(password)
  await field(page, '确认新密码').fill(password)
  await page.getByRole('button', { name: button, exact: true }).click()
}

test('客户从真实邮件重置密码，旧凭据、会话、链接失效', async ({ page, request, baseURL }, info) => {
  const email = 'browser-reset@example.test'
  const oldPassword = 'proxy-local-test'
  const newPassword = 'browser-new-reset'
  expect((await registerEmail(request, { email })).ok()).toBeTruthy()
  const previous = await customerHeaders(request, email, oldPassword)
  await page.goto('/zh-CN/login')
  await page.getByText('忘记密码？', { exact: true }).click()
  await expect(page.getByRole('heading', { name: '找回密码', exact: true })).toBeVisible()
  await field(page, '邮箱').fill(email.toUpperCase())
  await page.getByRole('button', { name: '发送重置链接', exact: true }).click()
  await expect(page.getByText('如果该邮箱已注册，重置密码的链接已发送。')).toBeVisible()
  const link = await receivedLink(email, 'RocProxy password reset', baseURL)
  await changePassword(page, link, '重置密码', newPassword)
  await expect(page.getByText('密码已重置', { exact: true })).toBeVisible()
  await screenshot(page, info, 'password-reset-confirmed')
  expect((await request.get('/v1/tenant/account', { headers: previous })).status()).toBe(401)
  expect((await request.post('/v1/access/login', { data: { email, password: oldPassword } })).status()).toBe(401)
  const fresh = await customerHeaders(request, email, newPassword)
  expect((await request.get('/v1/tenant/account', { headers: fresh })).ok()).toBeTruthy()
  await changePassword(page, link, '重置密码', 'browser-reset-replay')
  await expect(page.getByText('重置链接不可用', { exact: true })).toBeVisible()
})

test('运营核实后发恢复邮件，客户换绑并保留同一租户', async ({ page, browser, request, baseURL }, info) => {
  const oldEmail = 'browser-recovery-old@example.test'
  const newEmail = 'browser-recovery-new@example.test'
  const oldPassword = 'proxy-local-test'
  const newPassword = 'browser-new-recovery'
  const registered = await registerEmail(request, { email: oldEmail })
  expect(registered.ok()).toBeTruthy()
  const original = await registered.json()
  const previous = await customerHeaders(request, oldEmail, oldPassword)
  await signInOperator(page, '/ops/tenants', 'operator-support')
  await field(page, '主账号名或邮箱').fill(' ' + oldEmail.toUpperCase() + ' ')
  await page.getByRole('button', { name: '查找', exact: true }).click()
  await page.getByRole('link').filter({ hasText: original.primary_account_name }).click()
  await expect(page).toHaveURL(new URL('/ops/tenants/' + original.tenant_id, baseURL).href)
  await page.getByRole('button', { name: '发送恢复邮件', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '发送恢复邮件' })
  await expect(dialog).toContainText(oldEmail)
  const send = dialog.getByRole('button', { name: '确认发送', exact: true })
  await expect(send).toBeDisabled()
  await dialog.getByText('选择核实方式', { exact: true }).click()
  await page.getByRole('option', { name: '对公转账回单核实', exact: true }).click()
  await field(dialog, '核实记录').fill('运营已核实对公转账回单')
  await field(dialog, '接收恢复链接的邮箱').fill(newEmail)
  await send.click()
  await expect(dialog.getByText('已向 ' + newEmail + ' 发送账号恢复邮件')).toBeVisible()
  await screenshot(page, info, 'operator-recovery-sent')

  const link = await receivedLink(newEmail, 'RocProxy account recovery', baseURL)
  const customer = await browser.newPage({ ignoreHTTPSErrors: true })
  try {
    await changePassword(customer, link, '完成恢复', newPassword)
    await expect(customer.getByText('账号已恢复', { exact: true })).toBeVisible()
    await screenshot(customer, info, 'customer-recovered')
  } finally {
    await customer.close()
  }
  expect((await request.get('/v1/tenant/account', { headers: previous })).status()).toBe(401)
  expect((await request.post('/v1/access/login', { data: { email: oldEmail, password: oldPassword } })).status()).toBe(401)
  const fresh = await customerHeaders(request, newEmail, newPassword)
  const verified = await request.get('/v1/tenant/account', { headers: fresh })
  expect(await verified.json()).toMatchObject({ tenant_id: original.tenant_id, primary_account_name: original.primary_account_name, email: newEmail })
})
