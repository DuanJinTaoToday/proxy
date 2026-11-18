import { test, expect } from '@playwright/test'
import { writeFile } from 'node:fs/promises'

async function login(page, route, label, account) {
  await page.goto(route)
  await page.getByLabel(label, { exact: true }).fill(account)
  await page.getByLabel('密码', { exact: true }).fill('proxy-local-test')
  await page.getByRole('button', { name: '登录', exact: true }).click()
}

test('运营充值 → 租户看到到账', async ({ browser, page: tenant, baseURL }, info) => {
  const operatorContext = await browser.newContext({ baseURL, ignoreHTTPSErrors: true })
  const operator = await operatorContext.newPage()
  const reason = 'verify-' + process.env.VERIFY_ID
  const balance = tenant.locator('.summary-card').filter({
    has: tenant.getByRole('heading', { name: '可用余额', exact: true }),
  }).locator('.summary-number')
  let before, after, receipt

  async function screenshot(page, name) {
    const path = info.outputPath(name + '.png')
    await page.screenshot({ path, fullPage: true })
    await info.attach(name, { path, contentType: 'image/png' })
  }
  try {
    await test.step('租户登录并读取初始余额', async () => {
      await login(tenant, '/console', '手机号', '13800000001')
      await tenant.getByRole('link', { name: '钱包流水', exact: true }).click()
      await expect(balance).toHaveText(/^¥[\d,]+\.\d{2}$/)
      before = await balance.innerText()
      const [yuan, fraction] = before.slice(1).replaceAll(',', '').split('.')
      after = '¥' + (BigInt(yuan) + 10n).toLocaleString('zh-CN') + '.' + fraction
      await screenshot(tenant, '01-tenant-before')
    })
    await test.step('运营核对租户、金额、原因和预计余额', async () => {
      await login(operator, '/operations', '运营账号', 'operator')
      await operator.getByLabel('主账号名或手机号').fill('13800000001')
      await operator.getByRole('button', { name: '查找租户' }).click()
      const summary = operator.getByRole('region', { name: 'Alpha', exact: true })
      await expect(summary.getByText(before, { exact: true })).toBeVisible()
      await operator.getByLabel('充值金额（人民币元）').fill('10.00')
      await operator.getByLabel('充值原因').fill(reason)
      await operator.getByRole('button', { name: '核对充值' }).click()
      const form = operator.locator('form').filter({ has: operator.getByRole('heading', { name: '确认充值信息' }) })
      const field = label => form.locator('dl > div').filter({ has: operator.getByText(label, { exact: true }) }).locator('dd')
      for (const [label, expected] of [
        ['租户名', 'Alpha'], ['主账号名', 'localalpha'], ['充值金额', '¥10.00'],
        ['预计到账后余额', after], ['充值原因', reason],
      ]) await expect(field(label)).toHaveText(expected)
      await screenshot(operator, '02-operator-confirmation')
    })
    await test.step('提交充值并读取运营回执', async () => {
      await operator.getByRole('button', { name: '确认充值', exact: true }).click()
      const status = operator.getByRole('region', { name: '钱包充值' }).getByRole('status')
      await expect(status).toHaveText(/充值已到账。流水 #\S+，到账时间/)
      receipt = /流水 #([^，]+)，/.exec(await status.innerText())[1]
      const row = operator.getByRole('row').filter({ has: operator.getByText(reason, { exact: true }) })
      await expect(row).toHaveCount(1)
      await expect(row.getByRole('rowheader')).toHaveText('#' + receipt)
      await screenshot(operator, '03-operator-receipt')
    })
    for (const step of ['租户刷新并核对到账', '再次刷新确认余额和流水稳定']) {
      await test.step(step, async () => {
        await tenant.reload()
        await expect(balance).toHaveText(after)
        const row = tenant.getByRole('row').filter({
          has: tenant.getByRole('rowheader', { name: '#' + receipt, exact: true }),
        })
        await expect(row).toHaveCount(1)
        const cells = row.getByRole('cell')
        await expect(cells.nth(1)).toHaveText('运营充值')
        await expect(cells.nth(2)).toHaveText('¥10.00')
        await expect(cells.nth(3)).toHaveText(after)
      })
    }
    await screenshot(tenant, '04-tenant-after-refresh')
    const path = info.outputPath('wallet-result.json')
    await writeFile(path, JSON.stringify({
      tenant: 'Alpha', account: 'localalpha', before, amount: '¥10.00',
      after: await balance.innerText(), receipt, reason, verifiedRefreshes: 2,
    }, null, 2) + '\n')
    await info.attach('wallet-result', { path, contentType: 'application/json' })
  } catch (error) {
    if (!operator.isClosed()) await screenshot(operator, 'operator-at-failure')
    throw error
  } finally {
    await operatorContext.close()
  }
})
