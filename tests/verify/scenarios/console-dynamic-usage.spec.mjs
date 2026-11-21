import { execFileSync } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { readFile } from 'node:fs/promises'
import { expect, test } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'

const dailyUsagePath = '/v1/tenant/usage/daily'
const trafficAlertPath = '/v1/tenant/traffic-alert'
const usagePagePath = '/console/dynamic/usage'
const setupPagePath = '/console/dynamic/setup'

test.use({ reducedMotion: 'reduce' })

function sql(statement) {
  return execFileSync('docker', [
    'compose', '-p', 'proxy-verify-' + process.env.VERIFY_ID,
    '-f', 'deploy/test/compose.yaml', '-f', 'deploy/test/compose.verify.yaml',
    'exec', '-T', 'db', 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-c', statement,
  ], { cwd: new URL('../../../', import.meta.url) })
}

async function register(page, phone) {
  await page.goto('/zh-CN/register')
  await field(page, '手机号').fill(phone)
  await field(page, '密码').fill('verify-usage-password')
  await page.getByRole('button', { name: '注册并登录', exact: true }).click()
  await expect(page).toHaveURL(/\/console$/)
  await expect(page.getByText('最近 7 天暂无扣减记录')).toBeVisible()
}

const goToDashboard = page => page.getByRole('menuitem', { name: '仪表板', exact: true }).click()
const notification = page => page.getByRole('button', { name: /^通知(?:（有未读提醒）)?$/ })
const unreadNotification = page => page.getByRole('button', { name: '通知（有未读提醒）', exact: true })
const todo = page => page.getByRole('region', { name: '待处理', exact: true })

test('数据统计使用上海业务日，可查询禁用账号历史并导出相同筛选的 CSV', async ({ page }, info) => {
  // Shanghai has already started May 16 while UTC is still on May 15.
  await page.clock.setFixedTime(new Date('2025-05-16T01:15:00+08:00'))
  const phone = '13900000073'
  await register(page, phone)
  const activeId = randomUUID()
  const disabledId = randomUUID()
  const oldUpdate = '2025-05-15T15:10:12Z'
  const latestUpdate = '2025-05-15T16:05:07Z'
  const customUpdate = '2025-05-15T16:00:00Z'
  // Prepare known metered totals for this tenant only. All list and export responses come from the live API.
  sql(`
    CREATE TEMP TABLE usage_tenant AS SELECT tenant_id FROM tenant_admins WHERE phone_e164 = '+86${phone}';
    INSERT INTO gateways (id, name) VALUES ('usage-verify', 'Usage verification');
    INSERT INTO proxy_subaccounts (client_id, tenant_id, name, password_ciphertext, active)
      SELECT client_id, tenant_id, name, '', active FROM usage_tenant CROSS JOIN
        (VALUES ('${activeId}', 'liveacct', true), ('${disabledId}', 'oldacct', false)) AS account(client_id, name, active);
    INSERT INTO usage_daily (tenant_id, gateway_id, usage_date, input_bytes, output_bytes, billable_bytes, record_count, last_received_at, client_id)
      SELECT tenant_id, 'usage-verify', day, bytes, 0, bytes, 1, updated_at, client_id FROM usage_tenant CROSS JOIN
        (VALUES
          (DATE '2025-04-16', 90000000000, TIMESTAMPTZ '${oldUpdate}', '${disabledId}'),
          (DATE '2025-04-17', 3000000000, TIMESTAMPTZ '${oldUpdate}', '${disabledId}'),
          (DATE '2025-04-26', 10000000000, TIMESTAMPTZ '${oldUpdate}', '${activeId}'),
          (DATE '2025-05-08', 500000000, TIMESTAMPTZ '${oldUpdate}', '${disabledId}'),
          (DATE '2025-05-10', 750000000, TIMESTAMPTZ '${customUpdate}', '${disabledId}'),
          (DATE '2025-05-10', 1250000000, TIMESTAMPTZ '${latestUpdate}', '${activeId}'),
          (DATE '2025-05-16', 1400000000, TIMESTAMPTZ '${latestUpdate}', '${activeId}')
        ) AS usage(day, bytes, updated_at, client_id);
  `)

  await page.route('**' + dailyUsagePath + '?*', route => route.abort(), { times: 1 })
  await page.getByRole('link', { name: '查看数据统计', exact: true }).click()
  await expect(page).toHaveURL(new RegExp(usagePagePath))
  const retry = page.getByRole('button', { name: '重试', exact: true })
  await expect(retry).toBeVisible()
  await screenshot(page, info, 'daily-usage-load-error')

  const sevenDaysRequest = page.waitForRequest(request => new URL(request.url()).pathname === dailyUsagePath)
  await retry.click()
  const sevenDaysParams = new URL((await sevenDaysRequest).url()).searchParams
  expect(sevenDaysParams.get('start_date')).toBe('2025-05-10')
  expect(sevenDaysParams.get('end_date')).toBe('2025-05-16')
  const chart = page.getByRole('img', { name: '按天扣减流量面积图' })
  const dayRow = (month, day) => page.getByRole('row').filter({ hasText: new RegExp(`2025[-/]${month}[-/]${day}`) })
  await expect(chart).toBeVisible()
  await expect(page.getByText(/^3\.40?\s*GB$/)).toBeVisible()
  await expect(dayRow('05', '10')).toContainText(/2(?:\.00)?\s*GB/)
  await expect(dayRow('05', '16')).toContainText(/1\.40?\s*GB/)
  await expect(dayRow('05', '08')).toHaveCount(0)

  const thirtyDaysRequest = page.waitForRequest(request => new URL(request.url()).pathname === dailyUsagePath)
  await page.getByText('近 30 天', { exact: true }).click()
  const thirtyDaysParams = new URL((await thirtyDaysRequest).url()).searchParams
  expect(thirtyDaysParams.get('start_date')).toBe('2025-04-17')
  expect(thirtyDaysParams.get('end_date')).toBe('2025-05-16')
  await expect(page.getByText(/^16\.90?\s*GB$/)).toBeVisible()
  await expect(dayRow('04', '17')).toContainText(/3(?:\.00)?\s*GB/)
  await expect(dayRow('04', '16')).toHaveCount(0)

  await page.getByPlaceholder('开始日期', { exact: true }).fill('2025-05-08')
  const endDate = page.getByPlaceholder('结束日期', { exact: true })
  await endDate.fill('2025-05-10')
  await endDate.press('Enter')
  await expect.poll(() => Object.fromEntries(new URL(page.url()).searchParams)).toMatchObject({
    range: 'custom', startDate: '2025-05-08', endDate: '2025-05-10',
  })

  const accountFilter = page.getByRole('combobox', { name: '按子账号筛选', exact: true })
  await accountFilter.press('Enter')
  await page.getByRole('option', { name: 'oldacct（已禁用）', exact: true }).click()
  const customQuery = {
    range: 'custom', startDate: '2025-05-08', endDate: '2025-05-10', subaccountId: disabledId,
  }
  await expect.poll(() => Object.fromEntries(new URL(page.url()).searchParams)).toEqual(customQuery)
  async function expectDisabledAccountUsage() {
    await expect(chart).toBeVisible()
    await expect(page.getByText('1.25 GB', { exact: true })).toBeVisible()
    await expect(dayRow('05', '08')).toContainText(/0\.50?\s*GB/)
    await expect(dayRow('05', '10')).toContainText('0.75 GB')
    await expect(dayRow('05', '10')).toContainText('750,000,000')
    await expect(dayRow('05', '16')).toHaveCount(0)
    await expect(page.locator('footer time')).toHaveAttribute('datetime', new Date(customUpdate).toISOString())
  }
  await expectDisabledAccountUsage()

  const exportButton = page.getByRole('button', { name: '导出 CSV', exact: true })
  await page.route('**' + dailyUsagePath + '.csv?*', route => route.abort(), { times: 1 })
  await exportButton.click()
  const exportError = page.getByRole('alert').filter({ hasText: 'CSV 导出失败' })
  await expect(exportError).toBeVisible()
  const downloadPromise = page.waitForEvent('download')
  const exportRequest = page.waitForRequest(request => new URL(request.url()).pathname === dailyUsagePath + '.csv')
  await exportError.getByRole('button', { name: '重试', exact: true }).click()
  const exportParams = new URL((await exportRequest).url()).searchParams
  expect(Object.fromEntries(exportParams)).toEqual({
    start_date: customQuery.startDate, end_date: customQuery.endDate, client_id: disabledId,
  })
  const download = await downloadPromise
  expect(download.suggestedFilename()).toMatch(/\.csv$/)
  const csv = await readFile(await download.path(), 'utf8')
  expect(csv).toBe(`date,billable_gb,updated_at\n2025-05-08,0.5,${oldUpdate}\n2025-05-10,0.75,${customUpdate}\n`)
  await info.attach('daily-usage-filtered-csv', { body: csv, contentType: 'text/csv' })

  const filteredUrl = page.url()
  await page.reload()
  await expect(page).toHaveURL(filteredUrl)
  await expectDisabledAccountUsage()
  await expect(page.getByText('oldacct（已禁用）', { exact: true }).first()).toBeVisible()
  await screenshot(page, info, 'daily-usage-disabled-account')
  await goToDashboard(page)
  await expect(page).toHaveURL(/\/console$/)
  await page.goBack()
  await expect(page).toHaveURL(filteredUrl)
  await expectDisabledAccountUsage()

  const emptyQuery = new URLSearchParams({ ...customQuery, startDate: '2025-05-09', endDate: '2025-05-09' })
  await page.goto(usagePagePath + '?' + emptyQuery)
  await expect(exportButton).toBeEnabled()
  await expect(chart).toBeHidden()
  const emptyDownloadPromise = page.waitForEvent('download')
  await exportButton.click()
  const emptyCsv = await readFile(await (await emptyDownloadPromise).path(), 'utf8')
  expect(emptyCsv).toBe('date,billable_gb,updated_at\n')
  await info.attach('daily-usage-empty-csv', { body: emptyCsv, contentType: 'text/csv' })

  await page.setViewportSize({ width: 390, height: 844 })
  await page.addInitScript(() => localStorage.setItem('proxy-console.locale', 'en'))
  await page.goto(filteredUrl)
  await expect(page.getByText('oldacct (disabled)', { exact: true })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(390)
  await screenshot(page, info, 'daily-usage-en-mobile')
})

test('低余额阈值写入、修改和停用后，仪表板和通知按当前阈值展示', async ({ page }, info) => {
  const phone = '13900000074'
  await register(page, phone)
  sql(`UPDATE tenant_balances SET available_bytes = 750000000
    WHERE tenant_id = (SELECT tenant_id FROM tenant_admins WHERE phone_e164 = '+86${phone}')`)
  await page.getByRole('menuitem', { name: '动态住宅代理-GB', exact: true }).click()
  await expect(page).toHaveURL(new RegExp(setupPagePath + '$'))
  await expect(page.getByRole('heading', { name: '低余额提醒', exact: true })).toBeVisible()
  const threshold = page.getByRole('textbox', { name: '提醒阈值（GB）', exact: true })
  const save = page.getByRole('button', { name: '保存阈值', exact: true })
  const disable = page.getByRole('button', { name: '停用提醒', exact: true })
  const successfulPut = () => page.waitForResponse(response =>
    new URL(response.url()).pathname === trafficAlertPath &&
    response.request().method() === 'PUT' && response.ok())

  await threshold.fill('0.75')
  const alertPanel = page.getByRole('region', { name: '低余额提醒', exact: true })
  const failPut = route => route.request().method() === 'PUT' ? route.abort() : route.fallback()
  await page.route('**' + trafficAlertPath, failPut)
  await save.click()
  await expect(alertPanel.getByRole('alert')).toContainText('未能确认提醒设置，请重试。')
  await expect(threshold).toHaveValue('0.75')
  await page.unroute('**' + trafficAlertPath, failPut)

  const submit = Promise.withResolvers()
  await page.route('**' + trafficAlertPath, async route => {
    if (route.request().method() !== 'PUT') return route.fallback()
    await submit.promise
    await route.continue()
  })
  const setAlert = successfulPut()
  await alertPanel.getByRole('button', { name: '重试', exact: true }).click()
  try {
    await expect(save).toBeDisabled()
  } finally {
    submit.resolve()
  }
  expect(await (await setAlert).json()).toMatchObject({
    threshold_bytes: '750000000', occurrence: '1',
  })
  await expect(page.getByText('已保存低余额阈值。', { exact: true })).toBeVisible()
  await expect(unreadNotification(page)).toBeVisible()
  await screenshot(page, info, 'traffic-alert-saved')
  const failOverview = route => route.abort()
  await page.route('**/v1/tenant/overview', failOverview)
  await unreadNotification(page).click()
  await expect(page.getByRole('alert')).toContainText('暂时无法加载通知。')
  await expect(page.getByRole('link', { name: /动态流量余额偏低/ })).toBeVisible()
  await page.unroute('**/v1/tenant/overview', failOverview)
  await page.reload()
  await expect(threshold).toHaveValue('0.75')

  await goToDashboard(page)
  await expect(todo(page)).toContainText('动态流量偏低')
  await unreadNotification(page).click()
  await expect(page.getByRole('link', { name: /动态流量余额偏低/ })).toBeVisible()
  await screenshot(page, info, 'traffic-alert-dashboard-and-notification')
  await page.getByRole('button', { name: '关闭通知', exact: true }).click()

  await page.getByRole('menuitem', { name: '动态住宅代理-GB', exact: true }).click()
  await threshold.fill('0')
  const updateAlert = successfulPut()
  await save.click()
  const updated = await (await updateAlert).json()
  expect(updated.threshold_bytes).toBe('0')
  expect(updated).not.toHaveProperty('occurrence')
  await goToDashboard(page)
  await expect(todo(page)).toContainText('暂无待处理事项')
  await notification(page).click()
  await expect(page.getByText('暂无通知', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: '关闭通知', exact: true }).click()

  await page.getByRole('menuitem', { name: '动态住宅代理-GB', exact: true }).click()
  await expect(threshold).toHaveValue('0')
  await expect(disable).toBeVisible()
  await threshold.fill('1.25')
  const reactivateAlert = successfulPut()
  await save.click()
  expect(await (await reactivateAlert).json()).toMatchObject({
    threshold_bytes: '1250000000', occurrence: '2',
  })
  await expect(unreadNotification(page)).toBeVisible()
  const disableAlert = successfulPut()
  await disable.click()
  const disabled = await (await disableAlert).json()
  expect(disabled).not.toHaveProperty('threshold_bytes')
  expect(disabled).not.toHaveProperty('occurrence')
  await expect(page.getByText('已停用低余额提醒。', { exact: true })).toBeVisible()
  await expect(unreadNotification(page)).toBeHidden()
  await goToDashboard(page)
  await expect(todo(page)).toContainText('暂无待处理事项')
  await notification(page).click()
  await expect(page.getByText('暂无通知', { exact: true })).toBeVisible()
  await screenshot(page, info, 'traffic-alert-disabled')
})
