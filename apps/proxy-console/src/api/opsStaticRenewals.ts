import { request } from './client'
import type { OpsPendingOrder } from './opsOrders'
import { opsTenantPath } from './opsTenants'
import { mapOrderDetail, type OrderDetailWire } from './orders'

export async function resolveOpsStaticRenewal(
  { tenant, order }: OpsPendingOrder,
  input: { allocationId: string, evidence: string, action: 'sync' | 'close_unknown' | 'confirm_failure' },
) {
  const path = opsTenantPath(tenant.id) + '/orders/' + encodeURIComponent(order.id) + '/renewal-resolution'
  return mapOrderDetail(await request<OrderDetailWire>(path, {
    body: {
      allocation_id: input.allocationId,
      evidence: input.evidence,
      close_unknown: input.action === 'close_unknown',
      confirmed_failure: input.action === 'confirm_failure',
    },
  }))
}
