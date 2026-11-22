import { execFileSync } from 'node:child_process'
import { readFileSync } from 'node:fs'
import { expect, test } from '@playwright/test'
import { screenshot, signInOperator } from '../support/console.mjs'
import { json } from '../support/traffic.mjs'

test.use({ locale: 'zh-CN', timezoneId: 'Asia/Shanghai' })

function applyFixture(name) {
  if (!/^[0-9a-f-]{36}$/.test(process.env.VERIFY_ID ?? '')) throw new Error('Expected an isolated verify run')
  const container = execFileSync('docker', ['ps', '-q',
    '--filter', 'label=com.docker.compose.project=proxy-verify-' + process.env.VERIFY_ID,
    '--filter', 'label=com.docker.compose.service=db',
  ], { encoding: 'utf8' }).trim()
  if (!/^[0-9a-f]{12}$/.test(container)) throw new Error('Expected one PostgreSQL container for this run')
  execFileSync('docker', ['exec', '-i', container, 'psql', '-U', 'proxy', '-d', 'proxy', '-v', 'ON_ERROR_STOP=1'],
    { input: readFileSync(new URL('../fixtures/' + name + '.sql', import.meta.url)) })
}

async function selectGapStatus(page, name) {
  await page.getByRole('combobox', { name: '按缺口状态筛选' }).press('Enter')
  await page.getByRole('option', { name, exact: true }).click()
}

test('运营只读排查用量缺口和 Recorder 记录异常', async ({ page, request }, info) => {
  applyFixture('ops-usage')
  const usage = '/ops/usage'
  await signInOperator(page, usage, 'operator-readonly')
  await expect(page.getByRole('heading', { name: '用量异常' })).toBeVisible()
  await expect(page.getByRole('tab', { name: '用量缺口2' })).toBeVisible()
  await expect(page.getByRole('tab', { name: '记录异常2' })).toBeVisible()
  const pending = page.getByRole('row').filter({ hasText: 'b-20261120-d7' })
  await expect(pending).toContainText('待回放')
  await expect(pending.getByRole('link')).toHaveCount(0)
  const gap = page.getByRole('row').filter({ hasText: 's-8c1e44f0' })
  await expect(gap).toContainText('可能缺失')
  await expect(gap).toContainText('gw-sg-01')
  await expect(gap).toContainText('b-20261119-a3')
  await expect(gap).toContainText('2026/11/19 22:58')
  await expect(gap.getByRole('link', { name: '陈小岚工作室' })).toHaveAttribute('href', '/ops/tenants/visual-recharge-tenant')
  await screenshot(page, info, '13-usage-gaps')
  await selectGapStatus(page, '可能缺失')
  await expect(pending).toHaveCount(0)
  await expect(gap).toBeVisible()
  await page.getByText('已解决', { exact: true }).click()
  await expect(page.getByText('暂无记录')).toBeVisible()
  await screenshot(page, info, '13-usage-gaps-empty')
  await selectGapStatus(page, '全部状态')
  await expect(page.getByText('解决于 2026/11/17 13:31')).toBeVisible()
  await screenshot(page, info, '13-usage-gaps-resolved')
  await page.getByText('未解决', { exact: true }).first().click()

  await page.getByRole('tab', { name: '记录异常2' }).click()
  const anomaly = page.getByRole('row').filter({ hasText: 'rec-000412' })
  await expect(anomaly).toContainText('decode_error')
  await expect(anomaly).toContainText('gw-sg-01')
  await expect(anomaly).toContainText('b-20261119-a3')
  await expect(anomaly).toContainText('@ 1,048,576')
  await expect(anomaly.getByRole('link', { name: '映像科技' })).toHaveAttribute('href', '/ops/tenants/t-1004')
  await screenshot(page, info, '14-recorder-anomalies')
  await page.getByRole('row').filter({ hasText: 'rec-000388' }).getByRole('link', { name: '陈小岚工作室' }).click()
  await expect(page.getByRole('heading', { name: '陈小岚工作室' })).toBeVisible()
  await page.goBack()
  await expect(anomaly).toBeVisible()
  await page.getByText('已解决', { exact: true }).click()
  await expect(page.getByText('解决于 2026/11/18 09:05')).toBeVisible()
  await screenshot(page, info, '14-recorder-anomalies-resolved')

  const prototypeURL = process.env.VERIFY_OPERATOR_PROTOTYPE
  if (prototypeURL) {
    const prototype = await page.context().newPage()
    await prototype.goto(new URL('#usage', prototypeURL).href)
    await expect(prototype.getByText('s-8c1e44f0')).toBeVisible()
    await screenshot(prototype, info, '13-usage-gaps-prototype')
    await prototype.getByText('已解决', { exact: true }).click()
    await expect(prototype.getByText('解决于 2026/11/17 13:31')).toBeVisible()
    await screenshot(prototype, info, '13-usage-gaps-resolved-prototype')
    await prototype.getByText('未解决', { exact: true }).first().click()
    await prototype.getByRole('tab', { name: /^记录异常/ }).click()
    await expect(prototype.getByText('rec-000412')).toBeVisible()
    await screenshot(prototype, info, '14-recorder-anomalies-prototype')
    await prototype.getByText('已解决', { exact: true }).click()
    await expect(prototype.getByText('解决于 2026/11/18 09:05')).toBeVisible()
    await screenshot(prototype, info, '14-recorder-anomalies-resolved-prototype')
    await prototype.close()
  }

  applyFixture('ops-usage-pagination')
  for (const { tab, prefix } of [{ tab: 'gaps', prefix: 'session' }, { tab: 'anomalies', prefix: 'fixture' }]) {
    await page.goto(usage + '?gateway_id=gw-pagination&boot_id=paged&tab=' + tab)
    const first = page.getByText(prefix + '-43', { exact: true })
    const onlyOnNext = page.getByText(prefix + '-1', { exact: true })
    await expect(first).toBeVisible()
    await expect(onlyOnNext).toHaveCount(0)
    await page.getByRole('button', { name: '下一页' }).click()
    await expect(onlyOnNext).toBeVisible()
    await expect(first).toHaveCount(0)
    await page.getByRole('button', { name: '上一页' }).click()
    await expect(first).toBeVisible()
    await page.getByRole('button', { name: '下一页' }).click()
    await expect(onlyOnNext).toBeVisible()
    await page.getByText('已解决', { exact: true }).click()
    await expect(page.getByText(prefix + '-42', { exact: true })).toBeVisible()
    await expect(page.getByRole('button', { name: '上一页' })).toBeDisabled()
    await page.getByRole('button', { name: '下一页' }).click()
    await expect(page.getByText(prefix + '-2', { exact: true })).toBeVisible()
    await expect(onlyOnNext).toHaveCount(0)
    await page.getByText('全部', { exact: true }).click()
    await expect(first).toBeVisible()
  }

  const observer = await json(await request.post('/v1/operations/access/login',
    { data: { username: 'verify-no-read', password: 'proxy-local-test' } }))
  for (const route of ['gaps', 'anomalies']) {
    const path = '/v1/operations/' + route + '?resolution=all'
    expect((await request.get(path)).status()).toBe(401)
    expect((await request.get(path, { headers: { Authorization: 'Bearer ' + observer.access_token } })).status()).toBe(403)
  }
})
