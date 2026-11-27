import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'
import { field, screenshot, signInOperator, signInTenant } from '../support/console.mjs'
import { receivedCode, registerEmail } from '../support/mail.mjs'
import { authenticatedApi, json } from '../support/traffic.mjs'

const password = 'proxy-local-test'
const registerPath = '/v1/access/register'
const inviteField = page => field(page, '邀请码')
const register = page => page.getByRole('button', { name: '注册并登录', exact: true }).click()

async function promoter({ request, playwright, baseURL }, email) {
  const { tenant_id: id } = await registerEmail(request, { email }).then(json)
  const api = await authenticatedApi(playwright, baseURL, { email })
  try {
    return { id, email, ...await api.get('/v1/tenant/invite').then(json) }
  } finally {
    await api.dispose()
  }
}

function database(sql, id) {
  const container = process.env.VERIFY_DB_CONTAINER
  if (!/^[a-f0-9]{64}$/.test(container) || !/^[a-zA-Z0-9_-]+$/.test(id)) throw new Error('Invalid isolated referral fixture')
  return JSON.parse(execFileSync('docker', [
    'exec', '-i', container, 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-v', 'tenant=' + id, '-At',
  ], { input: sql + '\n', encoding: 'utf8' }))
}

const registrationAttribution = id => database(
  "SELECT json_build_object('referrer', referrer_tenant_id, 'source', referral_source) FROM tenants WHERE id=:'tenant';", id,
)

const promotionAudit = id => database(
  "SELECT COALESCE(json_agg(json_build_object('operator', o.username, 'before', a.previous_paused, 'after', a.paused) ORDER BY a.id), '[]') FROM tenant_promotion_audit a JOIN operators o ON a.operator_id=o.id WHERE a.tenant_id=:'tenant';", id,
)

async function fillRegistration(page, url, email) {
  await page.goto(url)
  await field(page, '邮箱').fill(email)
  await page.getByRole('button', { name: '发送验证码', exact: true }).click()
  const code = await receivedCode(email)
  await field(page, '验证码').fill(code)
  await field(page, '密码').fill(password)
  return code
}

function submitRegistration(page) {
  const response = page.waitForResponse(resp => new URL(resp.url()).pathname === registerPath && resp.request().method() === 'POST')
  return register(page).then(() => response)
}

test('邀请链接、手填优先和清除只绑定一次，邀请码报错沿用原验证码', async ({ request, playwright, baseURL, browser }, info) => {
  const linked = await promoter({ request, playwright, baseURL }, 'referral-ui-link@example.test')
  const manual = await promoter({ request, playwright, baseURL }, 'referral-ui-manual@example.test')
  for (const source of ['link', 'manual', 'clear']) {
    const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true, viewport: { width: 390, height: 844 } })
    try {
      const page = await context.newPage()
      const email = 'referral-ui-new-' + source + '@example.test'
      const code = await fillRegistration(page, linked.invite_url, email)
      await expect(inviteField(page)).toHaveValue(linked.invite_code)
      expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
      if (source === 'link') {
        await page.getByRole('link', { name: '注册', exact: true }).click()
        await expect(page).toHaveURL(/\/zh-CN\/register$/)
        await expect(field(page, '验证码')).toHaveValue(code)
        await expect(inviteField(page)).toHaveValue(linked.invite_code)
      }
      if (source !== 'link') {
        await inviteField(page).fill('not-a-valid-code')
        const rejected = await submitRegistration(page)
        expect(rejected.status()).toBe(400)
        expect(await rejected.json()).toMatchObject({ reason: 'INVALID_INVITE_CODE' })
        await expect(page.getByText(/邀请码.*(无效|不正确|错误)/).filter({ visible: true }).first()).toBeVisible()
        await expect(field(page, '验证码')).toHaveValue(code)
        if (source === 'manual') await inviteField(page).fill(manual.invite_code.toUpperCase())
        else await page.getByRole('button', { name: '清除邀请码', exact: true }).click()
      }
      const accepted = await submitRegistration(page)
      const data = accepted.request().postDataJSON()
      expect(data.verification_code).toBe(code)
      if (source === 'link') {
        expect(data.link_invite_code).toBe(linked.invite_code)
        expect(data).not.toHaveProperty('invite_code')
      }
      if (source === 'manual') expect(data.invite_code.toLowerCase()).toBe(manual.invite_code)
      if (source === 'clear') expect(data.invite_code === '' || !data.link_invite_code).toBe(true)
      const { tenant_id: id } = await accepted.json()
      expect(accepted.status()).toBe(200)
      await expect(page).toHaveURL(/\/console$/)
      const attribution = source === 'clear' ? { referrer: null, source: null } : {
        referrer: source === 'link' ? linked.id : manual.id, source,
      }
      expect(registrationAttribution(id)).toEqual(attribution)
      const otherInvite = source === 'manual' ? linked.invite_url : manual.invite_url
      await page.goto(otherInvite)
      await expect(page).toHaveURL(/\/console$/)
      await page.goto('/console/account/profile')
      await expect(page.getByRole('button', { name: '复制邀请码' })).toBeVisible()
      expect(registrationAttribution(id)).toEqual(attribution)
      if (source === 'manual') await screenshot(page, info, 'mobile-referral-profile-after-registration')
    } finally {
      await context.close()
    }
  }
})

test('推广员复制分享信息，运营权限与暂停恢复审计一致', async ({ page, context, browser, request, playwright, baseURL }, info) => {
  const account = await promoter({ request, playwright, baseURL }, 'referral-ui-promotion@example.test')
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await signInTenant(page, account.email)
  await page.goto('/console/account/profile')
  const copyCode = page.getByRole('button', { name: '复制邀请码', exact: true })
  const copyLink = page.getByRole('button', { name: '复制邀请链接', exact: true })
  await copyCode.click()
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(account.invite_code)
  await copyLink.click()
  await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toBe(account.invite_url)
  await screenshot(page, info, 'desktop-referral-clipboard')

  const detail = '/ops/tenants/' + account.id
  const opsPath = '/v1/operations/tenants/' + account.id
  const promotion = opsPath + '/promotion'
  for (const username of ['operator-readonly', 'operator-billing', 'operator-support']) {
    const operatorContext = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
    try {
      const ops = await operatorContext.newPage()
      const loaded = ops.waitForResponse(response => new URL(response.url()).pathname === promotion)
      await signInOperator(ops, detail, username)
      expect(await (await loaded).json()).toMatchObject({ promotion_status: 'active' })
      const pause = ops.getByRole('button', { name: /暂停推广/ }).first()
      if (username !== 'operator-support') {
        if (await pause.isVisible()) await expect(pause).toBeDisabled()
        const { access_token } = await request.post('/v1/operations/access/login', { data: { username, password } }).then(json)
        expect((await request.post(opsPath + ':pause-promotion', {
          headers: { Authorization: 'Bearer ' + access_token }, data: {},
        })).status()).toBe(403)
        continue
      }
      async function change(button, action) {
        await button.click()
        const dialog = ops.getByRole('dialog').filter({ visible: true })
        const answered = ops.waitForResponse(response => new URL(response.url()).pathname === opsPath + ':' + action + '-promotion')
        await dialog.getByRole('button', { name: /确认|暂停推广|恢复推广/ }).last().click()
        const response = await answered
        expect(response.ok()).toBe(true)
        return response.json()
      }
      expect(await change(pause, 'pause')).toMatchObject({ promotion_status: 'paused' })
      expect(promotionAudit(account.id)).toEqual([{ operator: username, before: false, after: true }])
      for (const limitedUser of ['operator-readonly', 'operator-billing']) {
        const { access_token } = await request.post('/v1/operations/access/login', {
          data: { username: limitedUser, password },
        }).then(json)
        const headers = { Authorization: 'Bearer ' + access_token }
        expect((await request.post(opsPath + ':resume-promotion', { headers, data: {} })).status()).toBe(403)
        expect(await request.get(promotion, { headers }).then(json)).toMatchObject({ promotion_status: 'paused' })
      }
      await page.reload()
      await expect(page.getByText(/推广.*暂停|已暂停/).filter({ visible: true }).first()).toBeVisible()
      await screenshot(ops, info, 'promotion-paused-by-account-operator')
      const guest = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
      try {
        const invited = await guest.newPage()
        const code = await fillRegistration(invited, account.invite_url, 'referral-ui-paused@example.test')
        const rejected = await submitRegistration(invited)
        expect(rejected.status()).toBe(400)
        expect(await rejected.json()).toMatchObject({ reason: 'PROMOTION_PAUSED' })
        await expect(invited.getByText(/推广.*暂停|邀请码.*暂停/).filter({ visible: true }).first()).toBeVisible()
        await invited.getByRole('button', { name: '清除邀请码', exact: true }).click()
        await expect(field(invited, '验证码')).toHaveValue(code)
        const accepted = await submitRegistration(invited)
        expect(accepted.ok()).toBe(true)
        const { tenant_id } = await accepted.json()
        expect(registrationAttribution(tenant_id)).toEqual({ referrer: null, source: null })
      } finally {
        await guest.close()
      }
      expect(await change(ops.getByRole('button', { name: /恢复推广/ }).first(), 'resume')).toMatchObject({ promotion_status: 'active' })
      expect(promotionAudit(account.id)).toEqual([
        { operator: username, before: false, after: true },
        { operator: username, before: true, after: false },
      ])
      await page.reload()
      await expect(copyCode).toBeEnabled()
    } finally {
      await operatorContext.close()
    }
  }
})

test('服务端推广状态未知时，分享和运营页不把它显示为可推广', async ({ page, browser, baseURL }) => {
  await signInTenant(page)
  await page.route('**/v1/tenant/invite', route => route.fulfill({
    json: { invite_code: 'fixture', invite_url: 'https://localhost/zh-CN/register', promotion_status: 'unrecognized' },
  }))
  await page.goto('/console/account/profile')
  await expect(page.getByText(/加载.*推广|无法.*推广|推广.*加载失败/).filter({ visible: true }).first()).toBeVisible()
  await expect(page.getByRole('button', { name: '复制邀请码', exact: true })).toHaveCount(0)
  const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
  try {
    const ops = await context.newPage()
    await ops.route('**/v1/operations/tenants/alpha/promotion', route => route.fulfill({ json: {} }))
    await signInOperator(ops, '/ops/tenants/alpha', 'operator-support')
    await expect(ops.getByText(/加载.*推广|无法.*推广|推广.*加载失败/).filter({ visible: true }).first()).toBeVisible()
    await expect(ops.getByRole('button', { name: '暂停推广', exact: true })).toHaveCount(0)
  } finally {
    await context.close()
  }
})
