import { expect, test } from '@playwright/test'
import { screenshot, signInTenant } from '../support/console.mjs'
import { authenticatedApi, json } from '../support/traffic.mjs'

test('租户从代理设置获取 SOCKS5 凭据并使用双语教程', async ({ page, context, playwright, request, baseURL }, info) => {
  const phone = '13900000072'
  const account = await json(await request.post('/v1/access/register', { data: { phone, password: 'proxy-local-test' } }))
  const username = account.primary_account_name + '.test72'
  const tenant = await authenticatedApi(playwright, baseURL, { phone })
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  try {
    await json(await tenant.post('/v1/tenant/subaccounts', { data: { name: 'sleep72', password: 'Asleep72', active: false } }))
    await signInTenant(page, phone)
    await page.getByRole('menuitem', { name: '动态住宅代理-GB' }).click()
    await expect(page.getByText('还没有启用的子账号')).toBeVisible()
    await screenshot(page, info, 'no-enabled-subaccounts')
    await page.getByRole('link', { name: '去创建子账号' }).click()
    await expect(page).toHaveURL(/\/console\/sub-accounts$/)

    await page.getByRole('button', { name: '创建子账号', exact: true }).click()
    const create = page.getByRole('dialog')
    await create.getByLabel(/子账号名/).fill('test72')
    await create.getByLabel(/^\*?密码$/).fill('Example72')
    await create.getByRole('button', { name: '创建', exact: true }).click()
    await expect(create).toBeHidden()
    const second = await json(await tenant.post('/v1/tenant/subaccounts', { data: { name: 'next72', password: 'Another72', active: true } }))

    await page.route('**/v1/tenant/subaccounts/*/proxy-config', route => route.abort(), { times: 1 })
    await page.getByRole('menuitem', { name: '动态住宅代理-GB' }).click()
    const setup = page.getByRole('main')
    await setup.getByRole('button', { name: '重新加载' }).click()

    const picker = setup.getByRole('combobox', { name: '子账号' })
    await picker.click()
    await expect(page.getByRole('option')).toHaveText(['next72', 'test72'])
    const configResponse = page.waitForResponse(response => /\/v1\/tenant\/subaccounts\/[^/]+\/proxy-config$/.test(new URL(response.url()).pathname)
      && !response.url().includes(second.client_id))
    await page.getByRole('option', { name: 'test72' }).click()
    const config = await json(await configResponse)
    expect(config).toMatchObject({ username, password: 'Example72', explicit_sid: true })
    expect(config.host).toBeTruthy()
    expect(config.port).toBeGreaterThan(0)
    await expect(setup.getByText(config.username, { exact: true })).toBeVisible()
    await expect(setup.getByText(config.host, { exact: true })).toBeVisible()
    await expect(setup.getByText(String(config.port), { exact: true })).toBeVisible()
    await expect(setup).not.toContainText(config.password)
    await setup.getByRole('button', { name: '显示密码' }).click()
    await expect(setup.getByText(config.password, { exact: true })).toBeVisible()
    await expect(setup.getByText(`curl -x "socks5h://${username}:Example72@`, { exact: false })).toBeVisible()
    await setup.getByRole('button', { name: '隐藏密码' }).click()
    await screenshot(page, info, 'masked-default-credentials')

    const copy = async (field, value) => {
      await setup.getByRole('button', { name: '复制' + field, exact: true }).click()
      await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(value)
    }
    await copy('网关地址', config.host)
    await copy('端口', String(config.port))
    await copy('用户名', config.username)
    await copy('密码', config.password)
    const host = config.host.includes(':') ? '[' + config.host + ']' : config.host
    const defaultConnection = `socks5h://${config.username}:${config.password}@${host}:${config.port}`
    await copy('连接串', defaultConnection)
    await copy('curl 示例', `curl -x "${defaultConnection}" https://api.ipify.org`)

    await setup.getByRole('radio', { name: '指定会话标识' }).check()
    const sid = setup.getByRole('textbox', { name: '会话标识（sid）' })
    for (const invalid of ['Invalid-', 'a'.repeat(33)]) {
      await sid.fill(invalid)
      await expect(setup.getByText('请输入 1–32 位小写字母或数字。', { exact: true })).toBeVisible()
      await expect(setup.getByRole('button', { name: '复制连接串' })).toBeDisabled()
    }
    await screenshot(page, info, 'invalid-sid-blocked')
    await sid.fill('session72')
    await expect(setup.getByText(username + '-sid-session72', { exact: true })).toBeVisible()
    await copy('连接串', defaultConnection.replace(username, username + '-sid-session72'))
    await expect(setup.getByRole('textbox', { name: /sidtime/i })).toHaveCount(0)

    await setup.getByRole('button', { name: '显示密码' }).click()
    await picker.click()
    await page.getByRole('option', { name: 'next72' }).click()
    await expect(setup.getByText(account.primary_account_name + '.next72-sid-session72', { exact: true })).toBeVisible()
    await expect(setup).not.toContainText('Another72')
    await expect(setup).not.toContainText('Example72')

    await page.getByRole('tab', { name: '使用教程' }).click()
    await expect(page.getByRole('heading', { name: '通过 SOCKS5 接入动态住宅代理' })).toBeVisible()
    await expect(page.getByText('<主账号名>.<子账号名>', { exact: true })).toBeVisible()
    await expect(page.getByRole('heading', { name: '常见问题' })).toBeVisible()
    await screenshot(page, info, 'guide-zh')
    await page.getByRole('button', { name: /^语言[:：]/ }).click()
    await page.getByRole('menuitem', { name: 'English', exact: true }).click()
    await expect(page.getByRole('heading', { name: 'Connect to dynamic residential proxies over SOCKS5' })).toBeVisible()
    await expect(page.getByText('<primary-account>.<subaccount>-sid-<session-id>', { exact: true })).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Frequently asked questions' })).toBeVisible()
    await screenshot(page, info, 'guide-en')
    await page.getByRole('link', { name: 'Open proxy setup' }).click()
    await expect(page).toHaveURL(/\/console\/dynamic\/setup$/)
  } finally {
    await tenant.dispose()
  }
})
