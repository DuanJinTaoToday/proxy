import { expect, test } from '@playwright/test'
import { startSalesmartlyFixture } from '../fixtures/salesmartly.mjs'

let fixture

test.beforeAll(async () => {
  fixture = await startSalesmartlyFixture()
})

test.afterAll(async () => {
  await fixture?.close()
})

for (const entry of ['website', 'console']) {
  test(`${entry} embeds support without sharing parent state`, async ({ page }) => {
    const parent = fixture.origins[entry]
    await page.goto(parent)
    const support = page.frameLocator('iframe[title="访客客服"]')
    const heading = support.getByRole('heading', { name: '客服隔离来源' })
    await expect(heading).toBeVisible()

    if (entry === 'console') {
      const order = await page.evaluate(async () => {
        const token = localStorage.getItem('proxy-console.session')
        const response = await fetch('/v1/tenant/orders', { headers: { Authorization: 'Bearer ' + token } })
        return response.json()
      })
      expect(order).toEqual({ orders: [{ id: 'synthetic-order' }] })
      await expect(page.getByText('synthetic-order')).toBeVisible()
    }

    const body = await heading.evaluate(async (_, parentOrigin) => {
      function parentRead(read) {
        try {
          read()
          return 'readable'
        } catch (error) {
          return error.name
        }
      }

      return {
        origin: location.origin,
        parentDOM: parentRead(() => parent.document.body.textContent),
        parentStorage: parentRead(() => parent.localStorage.getItem('proxy-console.session')),
        ownSession: localStorage.getItem('proxy-console.session'),
        orderRequest: await fetch(parentOrigin + '/v1/tenant/orders', { credentials: 'include' })
          .then(() => 'readable', error => error.name),
      }
    }, parent)
    expect(body).toEqual({
      origin: fixture.origins.support,
      parentDOM: 'SecurityError',
      parentStorage: 'SecurityError',
      ownSession: null,
      orderRequest: 'TypeError',
    })
  })
}

test('support origin has no anonymous or CORS access to the console orders API', async ({ request }) => {
  const consoleOrigin = new URL(fixture.origins.console)
  const response = await request.get(`http://127.0.0.1:${consoleOrigin.port}/v1/tenant/orders`, {
    headers: { Host: consoleOrigin.host, Origin: fixture.origins.support },
  })
  expect(response.status()).toBe(401)
  expect(response.headers()['access-control-allow-origin']).toBeUndefined()
})

const blockedEmbeds = [
  { parent: 'website', path: '/?frame=unrelated', directive: 'frame-src', heading: '未授权嵌入来源' },
  { parent: 'console', path: '/?frame=unrelated', directive: 'frame-src', heading: '未授权嵌入来源' },
  { parent: 'unrelated', path: '/', directive: 'frame-ancestors', heading: '客服隔离来源' },
]

for (const { parent, path, directive, heading } of blockedEmbeds) {
  test(`${parent} rejects an unapproved embed through ${directive}`, async ({ page }) => {
    const violation = page.waitForEvent('console', {
      predicate: message => message.text().includes('Content Security Policy') && message.text().includes(directive),
    })
    await page.goto(fixture.origins[parent] + path)
    await violation
    await expect(page.frameLocator('iframe[title="访客客服"]').getByRole('heading', { name: heading })).toHaveCount(0)
  })
}
