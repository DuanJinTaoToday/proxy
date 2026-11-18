import { expect } from '@playwright/test'

export async function login(page, route, label, account) {
  await page.goto(route)
  await page.getByLabel(label, { exact: true }).fill(account)
  await page.getByLabel('密码', { exact: true }).fill('proxy-local-test')
  await page.getByRole('button', { name: '登录', exact: true }).click()
}

export async function findTenant(operator, account) {
  await operator.getByLabel('主账号名或手机号').fill(account)
  await operator.getByRole('button', { name: '查找租户', exact: true }).click()
}

export async function prepareRecharge(operator, amount, reason) {
  await operator.getByLabel('充值金额（人民币元）').fill(amount)
  await operator.getByLabel('充值原因').fill(reason)
  await operator.getByRole('button', { name: '核对充值', exact: true }).click()
}

export async function submitRecharge(operator) {
  await operator.getByRole('button', { name: '确认充值', exact: true }).click()
  const status = operator.getByRole('region', { name: '钱包充值' }).getByRole('status')
  await expect(status).toHaveText(/充值已到账。流水 #[^，]+，到账时间/)
  return /流水 #([^，]+)，/.exec(await status.innerText())[1]
}

export async function screenshot(page, info, name) {
  const path = info.outputPath(name + '.png')
  await page.screenshot({ path, fullPage: true })
  await info.attach(name, { path, contentType: 'image/png' })
}
