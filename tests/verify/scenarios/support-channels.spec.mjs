import { test, expect } from '@playwright/test'
import { supportFixture, supportFixtureMode } from '../fixtures/support.mjs'
import { screenshot, signInOperator, signInTenant } from '../support/console.mjs'

const supportName = /Contact human support|联系人工客服/
const configured = supportFixtureMode === 'configured'

async function openSupport(page, entry) {
  if (entry === 'website') {
    await page.goto('/en/')
    return {
      channels: page.getByRole('navigation', { name: supportName }).filter({ has: page.getByRole('button', { name: /Live chat/ }) }),
      notice: page.getByRole('note', { name: /Before you contact support/ }),
    }
  }

  await signInTenant(page)
  await page.getByRole('button', { name: /Account menu|账号菜单/ }).click()
  await page.getByRole('menuitem', { name: supportName }).click()
  const dialog = page.getByRole('dialog', { name: supportName })
  await expect(dialog).toBeVisible()
  return {
    channels: dialog.getByRole('navigation', { name: supportName }),
    notice: dialog.getByRole('note'),
    dialog,
  }
}

for (const entry of ['website', 'console']) {
  test(`#112 ${entry} support (${supportFixtureMode})`, async ({ page, context, baseURL }, info) => {
    await page.setViewportSize({ width: 390, height: 844 })
    await context.grantPermissions(['clipboard-read', 'clipboard-write'], { origin: baseURL })
    const outbound = []
    await context.route(url => url.origin !== new URL(baseURL).origin, async route => {
      const request = route.request()
      outbound.push({ url: request.url(), headers: await request.allHeaders() })
      await route.fulfill({ contentType: 'text/html', body: '<title>Support test destination</title>' })
    })

    const { channels, notice, dialog } = await openSupport(page, entry)
    if (entry === 'website' && !configured) {
      await expect(notice).toHaveCount(0)
    } else {
      await expect(notice).toHaveCount(1)
      await expect(notice.getByText(/verify account ownership|核实账号归属/i)).toBeVisible()
      await expect(notice.getByText(/cannot confirm ownership.*instructions|无法核实归属.*指引/i)).toBeVisible()
      await expect(notice.getByText(/password|密码/i)).toBeVisible()
    }
    await expect(page.locator('iframe')).toHaveCount(0)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)

    if (entry === 'website') {
      const chat = channels.getByRole('button', { name: /Live chat/ })
      await chat.click()
      await expect(page.getByRole('alert').filter({ hasText: 'Coming soon' })).toBeVisible()
    }

    expect(outbound).toEqual([])
    if (!configured) {
      await expect(page.getByRole('link', { name: /Telegram|QQ/ })).toHaveCount(0)
      await expect(page.getByText(supportFixture.phone, { exact: true })).toHaveCount(0)
      if (dialog) await expect(dialog.getByRole('alert')).toBeVisible()
      await screenshot(page, info, `${entry}-${supportFixtureMode}`)
      return
    }

    for (const [name, url] of [['Telegram', supportFixture.telegram], ['QQ', supportFixture.qq]]) {
      const link = channels.getByRole('link', { name: new RegExp(name) })
      await expect(link).toHaveAttribute('href', url)
      const [popup] = await Promise.all([page.waitForEvent('popup'), link.click()])
      await expect(popup).toHaveURL(url)
      await expect(popup).toHaveTitle('Support test destination')
      expect(await popup.evaluate(() => window.opener)).toBeNull()
      await popup.close()
    }
    expect(outbound.map(request => request.url)).toEqual([supportFixture.telegram, supportFixture.qq])
    for (const { headers } of outbound) {
      expect(headers.referer).toBeUndefined()
      expect(headers.authorization).toBeUndefined()
      expect(headers.cookie).toBeUndefined()
    }

    const phone = channels.getByRole('button', { name: /Phone|电话/ })
    await phone.click()
    await expect(page.getByRole('alert').filter({ hasText: /Phone copied|电话.*复制|已复制.*电话/ })).toBeVisible()
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(supportFixture.clipboard)
    await screenshot(page, info, `${entry}-configured`)

    await page.evaluate(() => {
      Object.defineProperty(navigator.clipboard, 'writeText', {
        value: () => Promise.reject(new DOMException('Test permission refusal', 'NotAllowedError')),
      })
    })
    await phone.click()
    await expect(page.getByRole('alert').filter({ hasText: /copy failed|could not copy|复制失败/i })).toBeVisible()

    if (dialog) {
      await page.keyboard.press('Escape')
      await expect(dialog).toBeHidden()
    }
  })
}

test('#112 operations console has no customer support entry', async ({ page }) => {
  await signInOperator(page)
  await expect(page).toHaveURL(/\/ops\/tenants$/)
  await expect(page.getByRole('button', { name: supportName })).toHaveCount(0)
  await expect(page.getByRole('navigation', { name: supportName })).toHaveCount(0)
})
