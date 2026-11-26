import { expect, test } from '@playwright/test'
import { receivedCode } from '../support/mail.mjs'
import { json } from '../support/traffic.mjs'

const password = 'proxy-local-test'
const registrationPath = '/v1/access/register'
const invitePath = '/v1/tenant/invite'

async function bearer(request, loginPath, identity) {
  const session = await json(await request.post(loginPath, { data: { ...identity, password } }))
  return { Authorization: 'Bearer ' + session.access_token }
}

async function registration(request, email) {
  await json(await request.post('/v1/access/register-code', { data: { email } }))
  return { email, password, verification_code: await receivedCode(email) }
}

async function expectError(response, status, reason) {
  expect(response.status()).toBe(status)
  if (reason) expect(await response.json()).toMatchObject({ reason })
}

test('邮件注册接口保留有效验证码，允许修正或清除邀请，运营可暂停及恢复', async ({ request, baseURL }, info) => {
  const customer = email => bearer(request, '/v1/access/login', { email })
  const operator = username => bearer(request, '/v1/operations/access/login', { username })
  const [alpha, beta] = await Promise.all([
    customer('alpha@example.test'),
    customer('beta@example.test'),
  ])
  const [support, readOnly, billing] = await Promise.all([
    operator('operator-support'),
    operator('operator-readonly'),
    operator('operator-billing'),
  ])

  await expectError(await request.get(invitePath), 401)
  const alphaInvite = await json(await request.get(invitePath, { headers: alpha }))
  const betaInvite = await json(await request.get(invitePath, { headers: beta }))
  expect(alphaInvite).toMatchObject({ invite_code: expect.stringMatching(/^[0-9a-f]{32}$/i), promotion_status: 'active' })
  expect(betaInvite.invite_code).not.toBe(alphaInvite.invite_code)
  const link = new URL(alphaInvite.invite_url)
  expect(link.origin).toBe(new URL(baseURL).origin)
  expect(link.pathname).toBe('/zh-CN/register')
  expect(link.searchParams.get('invite')).toBe(alphaInvite.invite_code)
  expect((await request.get(link.href)).ok()).toBe(true)
  expect(await json(await request.get(invitePath, { headers: alpha }))).toEqual(alphaInvite)

  const { tenant_id: betaID } = await json(await request.get('/v1/tenant/account', { headers: beta }))
  const betaPath = '/v1/operations/tenants/' + betaID
  expect(await json(await request.get(betaPath + '/promotion', { headers: readOnly }))).toMatchObject({ promotion_status: 'active' })
  for (const headers of [readOnly, billing]) {
    await expectError(await request.post(betaPath + ':pause-promotion', { headers, data: {} }), 403)
  }
  expect(await json(await request.post(betaPath + ':pause-promotion', { headers: support, data: {} }))).toMatchObject({ promotion_status: 'paused' })
  expect(await json(await request.get(betaPath + '/promotion', { headers: readOnly }))).toMatchObject({ promotion_status: 'paused' })

  const first = await registration(request, 'referral-clear@example.test')
  const fromLink = { ...first, link_invite_code: alphaInvite.invite_code }
  const submit = data => request.post(registrationPath, { data })
  await expectError(await submit({ ...fromLink, invite_code: betaInvite.invite_code }), 400, 'PROMOTION_PAUSED')
  await expectError(await submit({ ...fromLink, invite_code: 'invalid' }), 400, 'INVALID_INVITE_CODE')
  const cleared = await json(await submit({ ...fromLink, invite_code: '' }))
  expect(cleared).toMatchObject({ email: first.email, tenant_id: expect.any(String) })
  await expectError(await submit({ ...fromLink, invite_code: '' }), 400, 'INVALID_VERIFICATION_CODE')

  expect(await json(await request.post(betaPath + ':resume-promotion', { headers: support, data: {} }))).toMatchObject({ promotion_status: 'active' })
  expect(await json(await request.get(invitePath, { headers: beta }))).toEqual(betaInvite)
  const second = await registration(request, 'referral-manual@example.test')
  const manual = { ...second, link_invite_code: 'invalid', invite_code: ' ' + betaInvite.invite_code.toUpperCase() + ' ' }
  const outcomes = await Promise.all([submit(manual), submit(manual)])
  expect(outcomes.map(response => response.status()).sort()).toEqual([200, 400])
  const created = await json(outcomes.find(response => response.ok()))
  const headers = await customer(second.email)
  expect(await json(await request.get('/v1/tenant/account', { headers }))).toMatchObject({ tenant_id: created.tenant_id, email: second.email })

  await info.attach('referral-api-evidence', {
    body: JSON.stringify({ emailSink: 'Mailpit', pausedManualRejected: true, correctedWithOriginalCode: true, concurrentRegistrationStatuses: [200, 400] }, null, 2),
    contentType: 'application/json',
  })
})
