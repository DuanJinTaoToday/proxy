import { request } from './client'
import type { OpsPendingOrder } from './opsOrders'
import { opsTenantPath } from './opsTenants'
import { mapOrderDetail, type OrderDetailWire } from './orders'

export async function resolveOpsStaticRenewal(
  { tenant, order }: OpsPendingOrder,
  input: { supplierItemId: string, evidence: string, action: 'sync' | 'close_unknown' | 'confirm_failure' },
) {
  const path = opsTenantPath(tenant.id) + '/orders/' + encodeURIComponent(order.id) + '/renewal-resolution'
  return mapOrderDetail(await request<OrderDetailWire>(path, {
    body: {
      supplier_item_id: input.supplierItemId,
      evidence: input.evidence,
      close_unknown: input.action === 'close_unknown',
      confirmed_failure: input.action === 'confirm_failure',
    },
  }))
}
