import { test, expect } from '@playwright/test'
import { login, screenshot, signInOperator, signInTenant } from '../support/console.mjs'

test('运营独立登录后返回租户详情，退出保留客户会话', async ({ page }, info) => {
  await signInTenant(page)
  await page.goto('/ops/tenants/alpha')
  await expect(page).toHaveURL(/\/ops\/login\?redirect=/)
  await login(page, '/ops/tenants/alpha', '运营账号', 'operator')
  await expect(page).toHaveURL(new URL('/ops/tenants/alpha', page.url()).href)
  await expect(page.getByRole('heading', { name: 'Alpha' })).toBeVisible()
  await page.reload()
  await expect(page.getByRole('heading', { name: 'Alpha' })).toBeVisible()
  await screenshot(page, info, 'operator-tenant-detail')

  await page.getByRole('button', { name: 'operator', exact: true }).click()
  await page.getByRole('menuitem', { name: '退出登录' }).click()
  await page.getByRole('dialog').getByRole('button', { name: '退出登录', exact: true }).click()
  await expect(page).toHaveURL(/\/ops\/login$/)
  await page.goto('/ops/tenants/alpha')
  await expect(page).toHaveURL(/\/ops\/login\?redirect=/)
  await page.goto('/console/wallet')
  await expect(page.getByRole('heading', { name: '钱包', exact: true })).toBeVisible()
})

test('运营会话过期返回登录并保留原目标', async ({ page }) => {
  await page.clock.install()
  await signInOperator(page, '/ops/tenants/alpha')
  await expect(page.getByRole('heading', { name: 'Alpha' })).toBeVisible()
  await page.clock.fastForward('08:01:00')
  await expect(page).toHaveURL(/\/ops\/login\?redirect=/)
  expect(new URL(page.url()).searchParams.get('redirect')).toBe('/ops/tenants/alpha')
})

test('运营权限分别来自控制面，禁止只读和账号运营充值', async ({ page, request }, info) => {
  const loginApi = async (path, account) => {
    const response = await request.post(path, { data: { ...account, password: 'proxy-local-test' } })
    expect(response.status()).toBe(200)
    return { Authorization: 'Bearer ' + (await response.json()).access_token }
  }
  const clientHeaders = await loginApi('/v1/access/login', { phone: '13800000001' })
  expect((await request.get('/v1/operations/access/account', { headers: clientHeaders })).status()).toBe(403)
  expect((await request.get('/v1/operations/tenants/alpha', { headers: clientHeaders })).status()).toBe(403)

  for (const [username, permissions] of [
    ['operator-readonly', ['read_operations']],
    ['operator-billing', ['read_operations', 'manage_billing']],
    ['operator-support', ['read_operations', 'manage_tenant_accounts']],
  ]) {
    const headers = await loginApi('/v1/operations/access/login', { username })
    expect(await (await request.get('/v1/operations/access/account', { headers })).json()).toEqual({ username, permissions })
    expect((await request.get('/v1/tenant/account', { headers })).status()).toBe(403)
    if (!permissions.includes('manage_billing')) {
      const data = { request_key: 'denied-' + username, reason: 'verify permissions', amount_minor: '1000' }
      const quote = await request.post('/v1/operations/tenants/alpha/wallet/recharge-quotes', { headers, data })
      expect(quote.status()).toBe(403)
      const write = await request.post('/v1/operations/tenants/alpha/wallet/recharges', {
        headers, data: { request_key: data.request_key, confirmation_token: '00000000-0000-4000-8000-000000000001' },
      })
      expect(write.status()).toBe(403)
    }
  }

  await signInOperator(page, '/ops/tenants/alpha', 'operator-readonly')
  await expect(page.getByRole('button', { name: '钱包充值' })).toBeDisabled()
  await screenshot(page, info, 'readonly-tenant')
})
