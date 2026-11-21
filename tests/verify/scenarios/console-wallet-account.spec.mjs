import { execFileSync } from 'node:child_process'
import { expect, test } from '@playwright/test'
import { screenshot, signInTenant } from '../support/console.mjs'

const businessDay = '2026-01-15'
const walletPath = '/console/wallet'
const ordersPath = '/console/account/orders'
const balance = page => page.getByRole('region', { name: '可用余额', exact: true })

function sql(statement) {
  return execFileSync('docker', [
    'compose', '-p', 'proxy-verify-' + process.env.VERIFY_ID,
    'exec', '-T', 'db', 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1', '-At', '-c', statement,
  ], { cwd: new URL('../../../', import.meta.url), encoding: 'utf8' }).trim()
}

async function registerTenant(request, phone) {
  const response = await request.post('/v1/access/register', {
    data: { phone, password: 'proxy-local-test' },
  })
  await expect(response).toBeOK()
  const account = await response.json()
  expect(account.tenant_id).toMatch(/^[a-f0-9]{32}$/)
  return account
}

async function recharge(request, tenantId) {
  const login = await request.post('/v1/operations/access/login', {
    data: { username: 'operator', password: 'proxy-local-test' },
  })
  await expect(login).toBeOK()
  const { access_token } = await login.json()
  const headers = { Authorization: 'Bearer ' + access_token }
  const request_key = 'verify-76-' + process.env.VERIFY_ID
  const endpoint = '/v1/operations/tenants/' + tenantId + '/wallet'
  const response = await request.post(endpoint + '/recharge-quotes', {
    headers, data: { request_key, reason: 'Issue 76 verification', amount_minor: '1000' },
  })
  await expect(response).toBeOK()
  const { confirmation_token } = await response.json()
  const receipt = await request.post(endpoint + '/recharges', { headers, data: { request_key, confirmation_token } })
  await expect(receipt).toBeOK()
  const result = await receipt.json()
  expect(result.after_minor).toBe('1000')
  return result.id
}

function seedHistory(tenantId) {
  // The first order is one second before Shanghai midnight. The other orders
  // belong to the next business day even though both use the same UTC date.
  return sql(`
    BEGIN;
    INSERT INTO wallet_entries (tenant_id, request_key, request_hash, kind, actor, reason, before_minor, after_minor, created_at)
      SELECT '${tenantId}', 'verify-76-history-' || n, '', 'recharge', 'local-operator', 'test fixture',
        900 + n*100, 1000 + n*100, '2026-01-13T16:00:00Z' FROM generate_series(1, 25) n;
    CREATE TEMP TABLE verify76_orders AS
      WITH inserted AS (
        INSERT INTO purchase_orders
          (id, tenant_id, kind, status, locked_minor, debited_minor, released_minor, refunded_minor, delivered_bytes, delivered_count, created_at)
        OVERRIDING SYSTEM VALUE VALUES
          (7601, '${tenantId}', 'traffic_package', 'succeeded', 700, 700, 0, 0, 5000000000, 0, '2026-01-14T15:59:59Z'),
          (7602, '${tenantId}', 'static_purchase', 'partial', 1200, 1200, 600, 0, 0, 1, '2026-01-14T16:00:00Z'),
          (7603, '${tenantId}', 'static_renewal', 'pending', 500, 500, 0, 0, 0, 0, '2026-01-14T16:01:00Z')
        RETURNING id, kind
      ) SELECT * FROM inserted;
    INSERT INTO order_items
      (tenant_id, order_id, position, specification, reference, capacity_bytes, term_days, unit_minor, quantity, delivered_count)
      SELECT '${tenantId}', id, 1,
        CASE kind WHEN 'static_purchase' THEN 'Singapore' WHEN 'static_renewal' THEN '192.0.2.1' ELSE '5 GB' END, '',
        CASE WHEN kind = 'traffic_package' THEN 5000000000 ELSE 0 END,
        CASE WHEN kind = 'traffic_package' THEN 0 ELSE 30 END,
        CASE kind WHEN 'static_purchase' THEN 600 WHEN 'static_renewal' THEN 500 ELSE 700 END,
        CASE WHEN kind = 'static_purchase' THEN 2 ELSE 1 END,
        CASE WHEN kind = 'static_renewal' THEN 0 ELSE 1 END
      FROM verify76_orders;
    INSERT INTO wallet_entries (tenant_id, request_key, request_hash, order_id, kind, reason, before_minor, after_minor, created_at)
      SELECT '${tenantId}', 'verify-76-' || event, '', id, action, '', before_minor, after_minor, date::timestamptz
      FROM verify76_orders JOIN (VALUES
        ('traffic_package', 'traffic', 'debit', 3500, 2800, '2026-01-14T15:59:59Z'),
        ('static_purchase', 'hold', 'hold', 2800, 1600, '2026-01-14T16:00:00Z'),
        ('static_purchase', 'release', 'release', 1600, 2200, '2026-01-14T16:00:30Z'),
        ('static_renewal', 'renewal', 'hold', 2200, 1700, '2026-01-14T16:01:00Z')
      ) AS entry(product, event, action, before_minor, after_minor, date) ON kind = product
      ORDER BY date::timestamptz;
    UPDATE wallets SET available_minor = 1700 WHERE tenant_id = '${tenantId}';
    COMMIT;
    SELECT string_agg(id::text, ',' ORDER BY id) FROM verify76_orders;
  `).split(/\r?\n/).at(-1).split(',')
}

test('钱包到账、筛选、翻页和关联订单，订单详情与账户信息可从正常入口查看', async ({ page, request }, info) => {
  const phone = '13900007601'
  const { tenant_id: tenantId, primary_account_name: accountName } = await registerTenant(request, phone)
  await signInTenant(page, phone)
  await page.getByRole('menuitem', { name: '钱包', exact: true }).click()
  await expect(balance(page)).toContainText('¥0.00')

  const rechargeId = await recharge(request, tenantId)
  await page.reload()
  await expect(balance(page)).toContainText('¥10.00')
  await expect(page.getByRole('cell', { name: rechargeId, exact: true })).toHaveCount(1)
  const [dynamicId, partialId, pendingId] = seedHistory(tenantId)

  await page.reload()
  await expect(balance(page)).toContainText('¥17.00')
  await expect(page.getByRole('region', { name: '累计充值' })).toContainText('¥35.00')
  await expect(page.getByRole('region', { name: '累计支出' })).toContainText('¥13.00')
  await page.getByRole('button', { name: '联系运营充值' }).click()
  await expect(page.getByText(/请联系您的客户经理安排转账/)).toBeVisible()
  await screenshot(page, info, 'wallet-totals-and-recharge')
  await page.getByRole('heading', { name: '钱包', exact: true }).click()

  await page.getByRole('radiogroup').getByText('充值', { exact: true }).click()
  await expect(page).toHaveURL(/kind=recharge/)
  await page.getByRole('button', { name: '下一页' }).click()
  await expect(page.getByText('第 2 页')).toBeVisible()
  await expect(page.getByRole('row')).toHaveCount(2)
  await page.getByRole('button', { name: '上一页' }).click()
  await expect(page.getByText('第 1 页')).toBeVisible()

  await page.getByRole('radiogroup').getByText('全部', { exact: true }).click()
  for (const [placeholder, query] of [['开始日期', 'start'], ['结束日期', 'end']]) {
    const date = page.getByPlaceholder(placeholder)
    await date.fill(businessDay)
    await date.press('Enter')
    await expect(page).toHaveURL(new RegExp(query + '=' + businessDay))
  }
  await expect(page.getByRole('button', { name: '订单 #' + pendingId, exact: true })).toBeVisible()
  await expect(page.getByRole('button', { name: '订单 #' + dynamicId, exact: true })).toHaveCount(0)
  await page.getByRole('radiogroup').getByText('退款', { exact: true }).click()
  await expect(page).toHaveURL(/kind=refund/)
  const linked = page.getByRole('button', { name: '订单 #' + partialId, exact: true })
  await expect(linked).toHaveCount(1)
  await screenshot(page, info, 'wallet-refund-day-filter')
  await linked.click()

  const drawer = page.getByRole('dialog', { name: '订单详情' })
  await expect(drawer).toBeVisible()
  await expect(drawer.getByText('Singapore')).toBeVisible()
  await expect(drawer.getByText('1 / 2')).toBeVisible()
  await expect(drawer.getByText('关联钱包流水')).toBeVisible()
  await expect(drawer.getByText('释放预扣')).toBeVisible()
  await expect(drawer.getByText('¥12.00', { exact: true }).first()).toBeVisible()
  await expect(drawer.getByText('¥6.00', { exact: true }).first()).toBeVisible()
  await screenshot(page, info, 'order-locked-items-refund-ledger')
  const filteredWallet = /\/console\/wallet\?start=2026-01-15&end=2026-01-15&kind=refund$/
  await expect(page).toHaveURL(filteredWallet)
  await page.keyboard.press('Escape')
  await expect(drawer).toBeHidden()
  await expect(linked).toHaveCount(1)
  await page.goBack()
  await expect(page).toHaveURL(/end=2026-01-15$/)
  await expect(page.getByRole('button', { name: '订单 #' + pendingId, exact: true })).toBeVisible()
  await page.goForward()
  await expect(page).toHaveURL(filteredWallet)
  await page.reload()
  await expect(linked).toHaveCount(1)

  await page.route('**/v1/tenant/orders', route => route.fulfill({ status: 503 }), { times: 1 })
  await page.goto(ordersPath)
  await expect(page.getByText('无法加载订单记录')).toBeVisible()
  await page.getByRole('button', { name: '重试', exact: true }).click()
  const ordersTab = page.getByRole('tab', { name: /订单记录/ })
  await expect(ordersTab.getByText('1', { exact: true })).toBeVisible()
  await page.route('**/v1/tenant/orders?status=pending', route => route.fulfill({ status: 503 }), { times: 1 })
  await page.getByRole('tab', { name: '账户信息', exact: true }).click()
  const badgeError = page.getByRole('alert').filter({ hasText: '无法加载确认中订单数量' })
  await expect(badgeError).toBeVisible()
  await badgeError.getByRole('button', { name: '重试', exact: true }).click()
  await expect(badgeError).toBeHidden()
  const account = page.getByRole('region', { name: '账户', exact: true })
  await expect(account).toContainText(accountName)
  await expect(account).toContainText('+86 139 **** 7601')
  await expect(page.getByRole('region', { name: '当前 IP 地址及归属地' })).toBeVisible()
  await expect(ordersTab.getByText('1', { exact: true })).toBeVisible()
  await screenshot(page, info, 'profile-and-pending-tab')
  await ordersTab.click()
  await page.getByRole('combobox', { name: '按产品筛选' }).press('Enter')
  await page.getByRole('option', { name: '静态住宅代理', exact: true }).click()
  await expect(page).toHaveURL(/product=static/)
  await expect(page.getByRole('cell', { name: '#' + pendingId, exact: true })).toBeVisible()
  await expect(page.getByRole('cell', { name: '#' + partialId, exact: true })).toBeVisible()
  await expect(page.getByRole('cell', { name: '#' + dynamicId, exact: true })).toHaveCount(0)

  await page.getByRole('combobox', { name: '按状态筛选' }).press('Enter')
  await page.getByRole('option', { name: '确认中', exact: true }).click()
  await expect(page).toHaveURL(/status=pending/)
  const orderSearch = page.getByRole('textbox', { name: '搜索订单号' })
  const search = page.getByRole('button', { name: '搜索', exact: true })
  const suffix = partialId.slice(-2)
  await orderSearch.fill(suffix)
  await search.click()
  await expect(page).toHaveURL(new RegExp('order=' + suffix))
  await expect(page.getByText('没有符合条件的订单')).toBeVisible()
  await page.goBack()
  await expect(orderSearch).toHaveValue('')
  await expect(page).toHaveURL(/status=pending$/)
  await page.getByRole('combobox', { name: '按状态筛选' }).press('Enter')
  await page.getByRole('option', { name: '全部状态', exact: true }).click()
  await orderSearch.fill(suffix)
  await search.click()
  await expect(page.getByRole('cell', { name: '#' + partialId, exact: true })).toBeVisible()
  await expect(page.getByRole('cell', { name: '#' + pendingId, exact: true })).toHaveCount(0)
  await page.reload()
  await expect(orderSearch).toHaveValue(suffix)
  await expect(page.getByRole('cell', { name: '#' + partialId, exact: true })).toBeVisible()

  await page.goto(ordersPath + '?start=' + businessDay + '&end=' + businessDay)
  await page.reload()
  await expect(page.getByPlaceholder('开始日期')).toHaveValue(businessDay)
  await expect(page.getByPlaceholder('结束日期')).toHaveValue(businessDay)
  await expect(page.getByRole('cell', { name: '#' + pendingId, exact: true })).toBeVisible()
  await expect(page.getByRole('cell', { name: '#' + dynamicId, exact: true })).toHaveCount(0)
  await page.getByRole('row').filter({ hasText: '#' + pendingId }).getByRole('button', { name: '详情' }).click()
  await expect(drawer.getByText('订单正在确认，金额已预扣')).toBeVisible()
  await screenshot(page, info, 'pending-order')
})

test('钱包和订单详情请求失败后可重试，租户不能读取其他租户的订单', async ({ page, browser, baseURL, request }, info) => {
  const phone = '13900007602'
  const { tenant_id: tenantId } = await registerTenant(request, phone)
  await signInTenant(page, phone)
  await page.route('**/v1/tenant/wallet/ledger', route => route.fulfill({ status: 503 }), { times: 1 })
  await page.goto(walletPath)
  await expect(page.getByText('钱包流水读取失败')).toBeVisible()
  await page.getByRole('button', { name: '重试', exact: true }).click()
  await expect(page.getByText('没有符合条件的流水')).toBeVisible()

  const ownOrder = sql(`
    INSERT INTO purchase_orders
      (tenant_id, kind, status, locked_minor, debited_minor, released_minor, refunded_minor, delivered_bytes, delivered_count)
    VALUES ('${tenantId}', 'traffic_package', 'succeeded', 100, 100, 0, 0, 1000000000, 0) RETURNING id;
  `).split(/\r?\n/)[0]
  await page.route('**/v1/tenant/orders/' + ownOrder, route => route.fulfill({ status: 503 }), { times: 1 })
  await page.goto(ordersPath + '?detail=' + ownOrder)
  const drawer = page.getByRole('dialog', { name: '订单详情' })
  await expect(drawer.getByText('无法加载订单详情')).toBeVisible()
  await screenshot(page, info, 'order-detail-retry')
  await drawer.getByRole('button', { name: '重试', exact: true }).click()
  await expect(drawer.getByText('成交明细 · 锁定价格')).toBeVisible()

  const otherTenant = await browser.newPage({ baseURL, ignoreHTTPSErrors: true })
  try {
    await signInTenant(otherTenant, '13800000001')
    const denied = otherTenant.waitForResponse(response => response.url().endsWith('/v1/tenant/orders/' + ownOrder))
    await otherTenant.goto(ordersPath + '?detail=' + ownOrder)
    expect((await denied).status()).toBe(404)
    await expect(otherTenant.getByRole('dialog', { name: '订单详情' })).toContainText('无法加载订单详情')
  } finally {
    await otherTenant.close()
  }
})
