import { expect, test } from '@playwright/test'
import { isIP } from 'node:net'
import { screenshot, signInTenant } from '../support/console.mjs'

const endpoint = '/v1/tenant/access-location'
const profile = '/console/account/profile'
const fixture = '/fixture/access-location'
const notice = '所有代理产品均不支持在中国大陆网络环境下使用'

test('当前访问来源经过 HTTPS 入口，租户自带的转发头不改变来源', async ({ page }, info) => {
  const guest = await page.request.get(endpoint)
  expect(guest.status()).toBe(401)

  await signInTenant(page)
  const query = page.waitForResponse(response => new URL(response.url()).pathname === endpoint)
  await page.getByRole('button', { name: '账号菜单' }).click()
  await page.getByRole('menuitem', { name: '账户信息' }).click()
  await expect(page).toHaveURL(profile)
  const response = await query
  expect(response.status()).toBe(200)
  expect(await response.headerValue('cache-control')).toBe('no-store')
  const location = await response.json()
  const observedSource = await page.request.get(fixture + '/source')
  expect(observedSource.status()).toBe(200)
  const ip = (await observedSource.text()).replace(/^::ffff:/, '')
  expect(isIP(ip)).toBeGreaterThan(0)
  expect(location).toEqual({ ip, location: '', is_mainland_china: false })

  const forged = await page.request.get(endpoint, {
    headers: {
      Authorization: await response.request().headerValue('authorization'),
      'X-Forwarded-For': '223.5.5.5, 203.0.113.10',
    },
  })
  expect(forged.status()).toBe(200)
  const spoofedLocation = await forged.json()
  expect(spoofedLocation).toEqual(location)
  await info.attach('access-location-api', {
    body: JSON.stringify({ unauthenticated: guest.status(), ipFromCaddy: ip, location, spoofedLocation }, null, 2),
    contentType: 'application/json',
  })

  const card = page.getByRole('region', { name: '当前 IP 地址及归属地' })
  await expect(card.getByText(ip, { exact: true })).toBeVisible()
  await expect(card.getByText('未知', { exact: true })).toBeVisible()
  await expect(card.getByText(notice, { exact: true })).toBeVisible()
  await screenshot(page, info, 'access-location-unknown')
})

test('账户信息归属地加载、失败后重试、大陆警告和英文非大陆提示', async ({ page, baseURL }, info) => {
  await signInTenant(page)
  // 夹住首次请求以验证加载态，再模拟传输失败。重试及英文请求均经过本轮 Caddy 和控制面的离线库。
  const firstRequest = Promise.withResolvers()
  await page.route(endpoint, async route => {
    await firstRequest.promise
    await route.fulfill({ status: 503 })
  }, { times: 1 })
  await page.goto(profile)
  const card = page.getByRole('region', { name: '当前 IP 地址及归属地' })
  try {
    await expect(card.getByRole('status', { name: '正在查询当前 IP 地址及归属地' })).toBeVisible()
    await expect(card.getByText(notice, { exact: true })).toBeVisible()
  } finally {
    firstRequest.resolve()
  }
  await expect(card.getByText('无法获取当前 IP 地址及归属地', { exact: true })).toBeVisible()
  await expect(card.getByText(notice, { exact: true })).toBeVisible()

  let fixtureRegion = '/cn'
  await page.route(endpoint, async route => {
    const response = await route.fetch({ url: new URL(fixture + fixtureRegion, baseURL).href })
    await route.fulfill({ response })
  })
  const cn = page.waitForResponse(response => new URL(response.url()).pathname === endpoint)
  await card.getByRole('button', { name: '重试' }).click()
  expect(await (await cn).json()).toEqual({ ip: '223.5.5.5', location: '中国大陆', is_mainland_china: true })
  await expect(card.getByText('223.5.5.5', { exact: true })).toBeVisible()
  await expect(card.getByText('中国大陆', { exact: true })).toBeVisible()
  await expect(card.getByText('当前网络在中国大陆，无法使用 Proxy', { exact: true })).toBeVisible()
  await expect(card.getByRole('link', { name: 'IP 归属地数据由 DB-IP 提供' })).toHaveAttribute('href', 'https://db-ip.com')
  await expect(card.getByText(notice, { exact: true })).toBeVisible()
  await screenshot(page, info, 'access-location-mainland-zh')

  fixtureRegion = '/hk'
  const hk = page.waitForResponse(response => new URL(response.url()).pathname === endpoint)
  await page.getByRole('button', { name: /^语言/ }).click()
  await page.getByRole('menuitem', { name: 'English' }).click()
  await expect(page).toHaveURL(profile)
  expect(await (await hk).json()).toEqual({
    ip: '203.0.113.10', location: expect.stringContaining('Hong Kong'), is_mainland_china: false,
  })
  const englishCard = page.getByRole('region', { name: 'Current IP address and location' })
  await expect(englishCard.getByText('203.0.113.10', { exact: true })).toBeVisible()
  await expect(englishCard.getByText(/Hong Kong/)).toBeVisible()
  await expect(englishCard.getByText('Proxy is ready to use', { exact: true })).toBeVisible()
  await expect(englishCard.getByText('All proxy products are unavailable on networks in mainland China.', { exact: true })).toBeVisible()
  await screenshot(page, info, 'access-location-hong-kong-en')
})
