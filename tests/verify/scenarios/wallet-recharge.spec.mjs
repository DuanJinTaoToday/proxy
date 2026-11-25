import { test, expect } from '@playwright/test'
import { writeFile } from 'node:fs/promises'
import { screenshot, signInOperator, signInTenant } from '../support/console.mjs'

const searchInput = page => page.getByRole('textbox', { name: '主账号名或邮箱' })
const receiptField = (page, dialog, label) => dialog.locator('dl > div').filter({ has: page.getByText(label, { exact: true }) }).locator('dd')
const rechargePath = '/v1/operations/tenants/visual-recharge-tenant/wallet/recharges'

async function search(page, value) {
  await searchInput(page).fill(value)
  await page.getByRole('button', { name: '查找', exact: true }).click()
}

test('运营充值 → 原提交重试 → 租户看到唯一一笔到账', async ({ browser, page: tenant, baseURL }, info) => {
  const context = await browser.newContext({ baseURL, ignoreHTTPSErrors: true, viewport: { width: 1440, height: 1000 }, timezoneId: 'Asia/Shanghai', locale: 'zh-CN' })
  const operator = await context.newPage()
  const reason = '对公转账到账，回单尾号 3381'
  const balance = tenant.getByRole('region', { name: '可用余额', exact: true }).locator('.wallet-overview__available')
  const before = '¥229.00'
  const after = '¥239.00'
  let receipt

  try {
    await test.step('租户登录并读取初始余额', async () => {
      await signInTenant(tenant, 'chenxl-studio@example.test')
      await tenant.getByRole('menuitem', { name: '钱包', exact: true }).click()
      await expect(balance).toHaveText(before)
      await screenshot(tenant, info, 'customer-before')
    })
    await test.step('运营通过邮箱与主账号名查找同一租户并核对信息', async () => {
      await signInOperator(operator, '/ops', 'operator-billing')
      await expect(operator).toHaveURL(new URL('/ops/tenants', baseURL).href)
      await expect(operator.getByRole('heading', { name: '查找租户' })).toBeVisible()
      await screenshot(operator, info, '01-tenant-search')
      await search(operator, 'unknown-account')
      await expect(operator.getByText(/没有找到租户/)).toBeVisible()
      await screenshot(operator, info, '01b-tenant-empty')
      await search(operator, ' CHENXL-STUDIO@EXAMPLE.TEST ')
      const match = operator.getByRole('link', { name: /陈小岚工作室.*chenxl-studio/ })
      const href = await match.getAttribute('href')
      expect(href).toBe('/ops/tenants/visual-recharge-tenant')
      await search(operator, 'chenxl-studio')
      await expect(match).toHaveAttribute('href', href)
      await screenshot(operator, info, '02-tenant-match')
      await match.click()
      await expect(operator).toHaveURL(new URL(href, baseURL).href)
      await expect(operator.getByRole('heading', { name: '陈小岚工作室' })).toBeVisible()
      await expect(operator.getByText(before, { exact: true })).toBeVisible()
      await screenshot(operator, info, '03-tenant-detail')

      await operator.getByRole('button', { name: '钱包充值', exact: true }).click()
      const dialog = operator.getByRole('dialog')
      await dialog.getByRole('spinbutton', { name: /充值金额/ }).fill('10.00')
      await dialog.getByRole('textbox', { name: /充值原因/ }).fill(reason)
      await screenshot(operator, info, '05-recharge-form')
      await dialog.getByRole('button', { name: '下一步' }).click()
      for (const [label, value] of [
        ['租户名', '陈小岚工作室'], ['主账号名', 'chenxl-studio'], ['充值原因', reason],
        ['充值金额', '¥10.00'], ['当前余额', before], ['到账后余额', after],
      ]) await expect(receiptField(operator, dialog, label)).toHaveText(value)
      await screenshot(operator, info, '06-recharge-confirm')
    })
    await test.step('首笔已经入账但回执丢失，重试保持原请求', async () => {
      const dialog = operator.getByRole('dialog')
      let firstRequest, firstReceipt, replayRequest, replayReceipt
      await operator.route('**' + rechargePath, async route => {
        const body = route.request().postDataJSON()
        const response = await route.fetch()
        expect(response.status()).toBe(200)
        if (!firstRequest) {
          firstRequest = body
          firstReceipt = await response.json()
          await route.abort('failed')
        } else {
          replayRequest = body
          replayReceipt = await response.json()
          await route.fulfill({ response })
        }
      })
      await dialog.getByRole('button', { name: /^确认充值/ }).click()
      await expect(dialog.getByRole('alert')).toHaveText(/暂未收到充值结果/)
      await dialog.getByRole('button', { name: '重试原提交' }).click()
      await expect(dialog.getByText('充值已到账', { exact: true })).toBeVisible()
      expect(replayRequest).toEqual(firstRequest)
      expect(replayReceipt).toEqual(firstReceipt)
      expect(firstReceipt).toMatchObject({ operator_name: 'operator-billing', reason })
      receipt = firstReceipt.id
      await expect(receiptField(operator, dialog, '流水号')).toHaveText(receipt)
      await expect(receiptField(operator, dialog, '操作人')).toHaveText('operator-billing')
      await expect(receiptField(operator, dialog, '到账时间')).toHaveText(/\d{4}/)
      await expect(receiptField(operator, dialog, '变动')).toHaveText(before + ' → ' + after)
      await screenshot(operator, info, '07-recharge-result')
      await dialog.getByRole('button', { name: '完成' }).click()
      await expect(operator.getByText(after, { exact: true })).toBeVisible()
    })
    for (let refresh = 1; refresh <= 2; refresh++) {
      await test.step('租户刷新并核对余额和唯一流水 ' + refresh, async () => {
        await tenant.reload()
        await expect(balance).toHaveText(after)
        const row = tenant.getByRole('row').filter({ has: tenant.getByRole('cell', { name: receipt, exact: true }) })
        await expect(row).toHaveCount(1)
        await expect(row.getByRole('cell', { name: '运营充值到账', exact: true })).toBeVisible()
        await expect(row.getByRole('cell', { name: '+¥10.00', exact: true })).toBeVisible()
        await expect(row.getByRole('cell', { name: after, exact: true })).toBeVisible()
        await expect(row).not.toContainText(reason)
        await expect(row).not.toContainText('operator-billing')
      })
    }
    await screenshot(tenant, info, 'customer-after')
    const path = info.outputPath('wallet-result.json')
    await writeFile(path, JSON.stringify({ tenant: '陈小岚工作室', account: 'chenxl-studio', before, amount: '¥10.00', after, receipt, reason, verifiedRefreshes: 2 }, null, 2) + '\n')
    await info.attach('wallet-result', { path, contentType: 'application/json' })
  } catch (error) {
    if (!operator.isClosed()) await screenshot(operator, info, 'operator-at-failure')
    throw error
  } finally {
    await context.close()
  }
})
