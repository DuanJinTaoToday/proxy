import { expect } from '@playwright/test'

export async function trafficSaleSettings(operator) {
  await operator.getByText('商品与套餐', { exact: true }).click()
  const sale = operator.locator('form').filter({
    has: operator.getByRole('heading', { name: /^动态.* · (已开售|未开售)$/ }),
  })
  await expect(sale).toBeVisible()
  return sale
}

export async function openTrafficSale(operator, sale) {
  const tariff = operator.locator('form').filter({
    has: operator.getByRole('heading', { name: '5gb', exact: true }),
  })
  const price = tariff.getByLabel('总价（分，0 表示未定价）')
  if (await price.inputValue() !== '1500') {
    await price.fill('1500')
    await tariff.getByRole('button', { name: '保存套餐', exact: true }).click()
  }

  const open = sale.getByRole('button', { name: '确认开售', exact: true })
  if (await open.isVisible()) {
    await sale.getByLabel('验收依据').fill('本地 Playwright 验收 ' + process.env.VERIFY_ID)
    await sale.getByRole('checkbox', { name: '已确认报价就绪' }).check()
    await open.click()
  }
  await expect(sale.getByRole('heading', { name: /^动态.* · 已开售$/ })).toBeVisible()
}

export async function expectTrafficPurchaseRecords(tenant, nav, orderId, remaining) {
  await nav.getByRole('link', { name: '订单记录', exact: true }).click()
  const orders = tenant.getByRole('region', { name: '订单记录', exact: true })
  const orderIds = orders.getByRole('rowheader')
  await expect(orderIds).toHaveText(['#' + orderId])
  const order = orders.getByRole('row').filter({
    has: tenant.getByRole('rowheader', { name: '#' + orderId, exact: true }),
  })
  await expect(order.getByRole('cell', { name: '¥15.00', exact: true })).toBeVisible()

  await nav.getByRole('link', { name: '钱包流水', exact: true }).click()
  const debit = tenant.getByRole('region', { name: '钱包流水', exact: true }).getByRole('row').filter({
    has: tenant.getByRole('cell', { name: '扣款', exact: true }),
  })
  await expect(debit).toHaveCount(1)
  await expect(debit.getByRole('button', { name: '#' + orderId, exact: true })).toBeVisible()
  await expect(debit.getByRole('cell', { name: '−¥15.00', exact: true })).toBeVisible()
  await expect(debit.getByRole('cell', { name: remaining, exact: true })).toBeVisible()
}
