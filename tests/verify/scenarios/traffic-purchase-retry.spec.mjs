import { expect, test } from '@playwright/test'
import { screenshot, signInTenant } from '../support/console.mjs'
import { authenticatedApi, expectSettled, purchasePath, recharge, setSale, setTier } from '../support/traffic.mjs'

const purchaseURL = url => url.pathname === purchasePath

test('成交回执丢失，刷新后重试原请求只扣款和到账一次', async ({ page, playwright, baseURL }, info) => {
  const phone = '13800000001'
  const operator = await authenticatedApi(playwright, baseURL)
  const tenant = await authenticatedApi(playwright, baseURL, { phone })

  try {
    await setTier(operator, '5gb', '1500')
    await setSale(operator, true)
    await recharge(operator, tenant, '4000')
    await signInTenant(page, phone)
    await page.goto('/console/dynamic/pricing?package=5gb')
    await expect(page.getByRole('radio', { name: '5 GB 流量包' })).toBeChecked()
    await page.getByRole('button', { name: '立即购买' }).click()

    let first
    await page.route(purchaseURL, async route => {
      try {
        const response = await route.fetch({ maxRetries: 0, maxRedirects: 0 })
        first = { status: response.status(), payload: route.request().postDataJSON(), order: await response.json() }
        await response.dispose()
      } finally {
        await route.abort('failed')
      }
    }, { times: 1 })

    await page.getByRole('button', { name: '确认支付' }).click()
    const retry = page.getByRole('button', { name: '重试原提交' })
    await expect(retry).toBeEnabled()
    expect(first).toMatchObject({
      status: 200,
      payload: { package_id: '5gb', package_version: expect.stringMatching(/^\d+$/), request_key: expect.stringMatching(/\S/) },
      order: { id: expect.stringMatching(/^\d+$/), status: 'succeeded', debited_minor: '1500', delivered_bytes: '5000000000' },
    })
    const id = first.order.id
    const beforeReplay = await expectSettled(tenant, id, '2500')
    await screenshot(page, info, 'lost-receipt')

    // Replay must work even when the original quote is no longer available.
    await setSale(operator, false)
    await page.reload()
    await expect(page.getByText('暂未开放购买')).toBeVisible()
    await expect(retry).toBeEnabled()
    const response = page.waitForResponse(value => purchaseURL(new URL(value.url())))
    await retry.click()
    const replay = await response
    expect(replay.request().postDataJSON()).toEqual(first.payload)
    expect(replay.status()).toBe(200)
    expect(await replay.json()).toEqual(first.order)
    await expect(page.getByRole('status').filter({ hasText: '订单 #' + id })).toContainText('到账 5 GB，实扣 ¥15.00')
    await expect(retry).toHaveCount(0)
    await screenshot(page, info, 'replayed-receipt')
    await page.reload()
    await expect(page.getByText('暂未开放购买')).toBeVisible()
    await expect(retry).toHaveCount(0)
    const afterReplay = await expectSettled(tenant, id, '2500')
    expect(afterReplay).toEqual(beforeReplay)
    await info.attach('idempotency.json', {
      body: JSON.stringify({ first, replayedPayload: replay.request().postDataJSON(), beforeReplay, afterReplay }, null, 2),
      contentType: 'application/json',
    })
  } finally {
    await operator.dispose()
    await tenant.dispose()
  }
})
