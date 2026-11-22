import { expect, test } from '@playwright/test'
import { screenshot, signInOperator } from '../support/console.mjs'

test.use({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })

test('运营从同一租户详情进入订单和钱包流水', async ({ page }, info) => {
  await signInOperator(page, '/ops/tenants/alpha')
  await expect(page.getByRole('heading', { name: 'Alpha' })).toBeVisible()
  await expect(page.getByRole('tab', { name: '订单', exact: true })).toHaveAttribute('aria-selected', 'true')
  await expect(page.getByRole('combobox', { name: '按类型筛选' })).toBeVisible()
  await screenshot(page, info, '03-tenant-orders')
  await page.getByRole('tab', { name: '钱包流水', exact: true }).click()
  await expect(page).toHaveURL(/\/ops\/tenants\/alpha\/ledger$/)
  await expect(page.getByRole('columnheader', { name: '变动前 → 后' })).toBeVisible()
  await screenshot(page, info, '04-tenant-ledger')
})
