import { expect } from '@playwright/test'

const codePattern = /code is (\d{6})\./
const linkPattern = /https:\/\/[^\s]+/
const registrationSubject = 'RocProxy verification code'

async function readMailpit(path) {
  const response = await fetch(new URL(path, process.env.VERIFY_MAILPIT_URL))
  expect(response.ok, 'Mailpit API ' + path).toBeTruthy()
  return response.json()
}

export async function messagesTo(email, subject) {
  const params = new URLSearchParams({ query: 'to:' + email, limit: '100' })
  const { messages } = await readMailpit('/api/v1/search?' + params)
  return (messages ?? []).filter(message => message.Subject === subject && message.To.some(recipient => recipient.Address === email))
}

export async function receivedEmail(email, subject, excluding = []) {
  let message
  await expect.poll(async () => {
    message = (await messagesTo(email, subject)).find(entry => !excluding.includes(entry.ID))
    return Boolean(message)
  }, { message: 'SMTP delivery of ' + subject + ' to ' + email, timeout: 10_000 }).toBe(true)
  return readMailpit('/api/v1/message/' + message.ID)
}

export async function receivedCode(email, excluding) {
  const { Text } = await receivedEmail(email, registrationSubject, excluding)
  const code = codePattern.exec(Text)?.[1]
  expect(Boolean(code), 'received a six-digit code').toBe(true)
  return code
}

export async function receivedLink(email, subject, baseURL) {
  const { Text } = await receivedEmail(email, subject)
  const link = linkPattern.exec(Text)?.[0]
  expect(Boolean(link), 'received a recovery link').toBe(true)
  expect(new URL(link).origin).toBe(new URL(baseURL).origin)
  return link
}

export async function registerEmail(request, { email, password = 'proxy-local-test', invite }) {
  const previous = (await messagesTo(email, registrationSubject)).map(message => message.ID)
  const response = await request.post('/v1/access/register-code', { data: { email } })
  expect(response.status()).toBe(200)
  const verification_code = await receivedCode(email, previous)
  return request.post('/v1/access/register', { data: { email, password, verification_code, ...invite } })
}
