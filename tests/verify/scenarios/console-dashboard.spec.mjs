import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'
import { field, screenshot } from '../support/console.mjs'

const dateInShanghai = new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai', dateStyle: 'short' })
const notificationButton = page => page.getByRole('button', { name: /^通知(?:（有未读提醒）)?$/ })
const unreadButton = page => page.getByRole('button', { name: '通知（有未读提醒）', exact: true })
const todo = page => page.getByRole('region', { name: '待处理', exact: true })

function sql(statement) {
  return execFileSync('docker', [
    'compose', '-p', 'proxy-verify-' + process.env.VERIFY_ID,
    'exec', '-T', 'db', 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-At', '-c', statement,
  ], { cwd: new URL('../../../', import.meta.url), encoding: 'utf8' }).trim()
}

async function register(page, phone) {
  await page.goto('/zh-CN/register')
  await field(page, '手机号').fill(phone)
  await field(page, '密码').fill('verify-dashboard-pw')
  await page.getByRole('button', { name: '注册并登录', exact: true }).click()
  await expect(page).toHaveURL(/\/console$/)
}

test('仪表板展示余额、上海业务日用量和待处理事项，通知可跳转并在刷新后保持已读', async ({ page }, info) => {
  const today = dateInShanghai.format(new Date())
  const shanghaiEarlyMorning = new Date(today + 'T01:15:00+08:00')
  const firstDay = new Date(Date.parse(today + 'T00:00:00Z') - 6 * 86_400_000).toISOString().slice(0, 10)
  await page.clock.setFixedTime(shanghaiEarlyMorning)
  await register(page, '13900000071')

  await expect(page.getByRole('region', { name: '钱包余额' })).toContainText(/¥0\.00/)
  await expect(page.getByRole('region', { name: '动态代理可用流量' })).toContainText('0 GB')
  await expect(page.getByRole('region', { name: '静态代理可用 IP' })).toContainText('暂无可用静态 IP')
  await expect(page.getByText('最近 7 天暂无扣减记录')).toBeVisible()
  await expect(todo(page).getByText('暂无待处理事项')).toBeVisible()
  await expect(page.getByRole('button', { name: '创建子账号', exact: true })).toHaveCount(0)
  await notificationButton(page).click()
  await expect(page.getByText('暂无通知', { exact: true })).toBeVisible()
  await page.getByRole('heading', { name: '仪表板' }).click()
  await screenshot(page, info, 'dashboard-zero-empty')

  // Seed only this test's newly registered tenant. All app reads and notification claims use the live control API.
  const expiry = sql(`
    CREATE TEMP TABLE dashboard_tenant AS SELECT tenant_id FROM tenant_admins WHERE phone_e164 = '+8613900000071';
    UPDATE wallets SET available_minor = 12345 WHERE tenant_id IN (SELECT tenant_id FROM dashboard_tenant);
    UPDATE tenant_balances SET available_bytes = 750000000, low_balance_threshold_bytes = 1000000000, low_balance_cycle = 1
      WHERE tenant_id IN (SELECT tenant_id FROM dashboard_tenant);
    INSERT INTO tenant_notifications (tenant_id, kind, subject_key, occurrence, created_at)
      SELECT tenant_id, 'low_traffic_balance', '', 1, now() FROM dashboard_tenant;
    INSERT INTO gateways (id, name) VALUES ('dashboard-verify', 'Dashboard verification');
    INSERT INTO proxy_subaccounts (client_id, tenant_id, name, password_ciphertext, active)
      SELECT 'dashboard-client', tenant_id, 'test71', '', true FROM dashboard_tenant;
    INSERT INTO usage_daily (tenant_id, gateway_id, usage_date, input_bytes, output_bytes, billable_bytes, record_count, last_received_at, client_id)
      SELECT tenant_id, 'dashboard-verify', day, bytes, 0, bytes, 1, now(), 'dashboard-client'
      FROM dashboard_tenant CROSS JOIN (VALUES (DATE '${today}', 1250000000),
        (DATE '${firstDay}', 2000000000), (DATE '${firstDay}' - 1, 100000000000)) AS usage(day, bytes);
    WITH orders AS (
      INSERT INTO purchase_orders (tenant_id, kind, status, locked_minor, debited_minor, released_minor, refunded_minor, delivered_bytes, delivered_count)
      SELECT tenant_id, 'static_purchase', 'pending', 100, 100, 0, 0, 0, 2 FROM dashboard_tenant
      RETURNING tenant_id, id
    )
    INSERT INTO static_purchases (tenant_id, order_id) SELECT tenant_id, id FROM orders;
    INSERT INTO static_ip_allocations (supplier_item_id, tenant_id, order_id, ip, port, username, password_ciphertext, expires_at)
      SELECT item, tenant_id, order_id, '192.0.2.1', 1080, '', '', now() + offset_from_now
      FROM static_purchases JOIN dashboard_tenant USING (tenant_id)
      CROSS JOIN (VALUES ('7101', interval '3 days'), ('7102', interval '20 days'), ('7103', interval '-2 days')) AS ip(item, offset_from_now);
    SELECT expires_at FROM static_ip_allocations WHERE supplier_item_id = '7101';
  `).split(/\r?\n/).at(-1)

  const usageRequest = page.waitForRequest(request => new URL(request.url()).pathname === '/v1/tenant/usage/daily')
  const claim = page.waitForResponse(response => new URL(response.url()).pathname === '/v1/tenant/static-ip-expiry-notices')
  await page.reload()
  const params = new URL((await usageRequest).url()).searchParams
  expect(params.get('start_date')).toBe(firstDay)
  expect(params.get('end_date')).toBe(today)
  expect(await (await claim).json()).toEqual({ count: 1 })

  const wallet = page.getByRole('region', { name: '钱包余额' })
  await expect(wallet).toContainText(/¥123\.45/)
  await expect(wallet.getByRole('link', { name: '查看钱包流水' })).toHaveAttribute('href', '/console/wallet')
  const traffic = page.getByRole('region', { name: '动态代理可用流量' })
  await expect(traffic).toContainText('0.75 GB')
  await expect(traffic.getByRole('link')).toHaveAttribute('href', '/console/dynamic/pricing')
  const staticIp = page.getByRole('region', { name: '静态代理可用 IP' })
  await expect(staticIp).toContainText('2 个')
  await expect(staticIp.locator('time')).toHaveAttribute('datetime', new Date(expiry).toISOString())
  await expect(staticIp.getByRole('link', { name: '查看 IP 列表' })).toHaveAttribute('href', '/console/static/ips')
  await expect(todo(page)).toContainText('动态流量偏低')
  await expect(todo(page)).toContainText('1 个静态 IP 即将到期')
  await expect(todo(page).getByRole('link', { name: '去续费' })).toHaveAttribute('href', '/console/static/ips')
  await expect(todo(page)).toContainText('有订单确认中')
  await expect(todo(page).getByRole('link', { name: '查看确认中的订单' }))
    .toHaveAttribute('href', '/console/account/orders?status=pending')

  await expect(page.getByRole('img', { name: '近 7 天每日扣减流量面积图' })).toBeVisible()
  await page.locator(`[data-date="${today}"]`).hover()
  await expect(page.getByRole('status')).toContainText('1.25 GB')
  await expect(page.getByText('合计扣减 3.25 GB', { exact: false })).toBeVisible()
  await screenshot(page, info, 'dashboard-area-chart')
  await page.getByRole('button', { name: '查看数据表' }).click()
  const rows = page.getByRole('row')
  await expect(rows).toHaveCount(8)
  await expect(rows.filter({ hasText: firstDay })).toContainText('2 GB')
  await expect(rows.filter({ hasText: today })).toContainText('1.25 GB')
  await screenshot(page, info, 'dashboard-daily-table')
  await page.getByRole('button', { name: '查看图表' }).click()

  await unreadButton(page).click()
  await expect(page.getByRole('link', { name: /动态流量余额偏低/ })).toBeVisible()
  await screenshot(page, info, 'dashboard-unread-notifications')
  await page.getByRole('link', { name: /动态流量余额偏低/ }).click()
  await expect(page).toHaveURL(/\/console\/dynamic\/pricing$/)
  await unreadButton(page).click()
  await page.getByRole('link', { name: /1 个静态 IP 即将到期/ }).click()
  await expect(page).toHaveURL(/\/console\/static\/ips$/)
  await expect(unreadButton(page)).toHaveCount(0)
  const repeatClaim = page.waitForResponse(response => new URL(response.url()).pathname === '/v1/tenant/static-ip-expiry-notices')
  await page.reload()
  expect(await (await repeatClaim).json()).toEqual({})
  await notificationButton(page).click()
  await expect(page.getByRole('link', { name: /1 个静态 IP 即将到期/ })).toBeVisible()
  await expect(unreadButton(page)).toHaveCount(0)

  sql(`UPDATE tenant_balances SET available_bytes = -450000000
    WHERE tenant_id = (SELECT tenant_id FROM tenant_admins WHERE phone_e164 = '+8613900000071')`)
  await page.goto('/console')
  await expect(traffic).toContainText('欠量 0.45 GB')
  await expect(todo(page)).toContainText('动态流量已欠量')
  await page.setViewportSize({ width: 360, height: 880 })
  await page.getByRole('button', { name: /^语言[:：]/ }).click()
  await page.getByRole('menuitem', { name: 'English', exact: true }).click()
  await expect(page.getByRole('region', { name: 'Available rotating proxy traffic' })).toContainText('0.45 GB owed')
  await expect(page.getByText('Orders awaiting confirmation', { exact: true })).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(360)
  await screenshot(page, info, 'dashboard-en-mobile')
})

test('仪表板用量加载失败和通知加载失败均可重试', async ({ page }, info) => {
  await page.route('**/v1/tenant/usage/daily?*', route => route.abort(), { times: 1 })
  await register(page, '13900000072')
  await expect(page.getByRole('alert')).toContainText('暂时无法加载仪表板。')
  await page.getByRole('button', { name: '重试', exact: true }).click()
  await expect(page.getByText('最近 7 天暂无扣减记录')).toBeVisible()

  await page.route('**/v1/tenant/traffic-alert', route => route.abort(), { times: 1 })
  await notificationButton(page).click()
  await expect(page.getByRole('alert')).toContainText('暂时无法加载通知。')
  await screenshot(page, info, 'dashboard-notification-retry')
  await page.getByRole('button', { name: '重试', exact: true }).click()
  await expect(page.getByText('暂无通知', { exact: true })).toBeVisible()
})
