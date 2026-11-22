import { request } from './client'
import { getOpsTenant, getOpsTenantOrder, opsTenantPath } from './opsTenants'
import { mapOrderDetail, type OrderDetailWire } from './orders'

interface PendingOrderWire {
  order_id: string
  tenant_id: string
  supplier_order_id?: string
  can_close?: boolean
  pending_renewal_items?: string[]
}

export interface OpsPendingOrder {
  tenant: Awaited<ReturnType<typeof getOpsTenant>>
  order: Awaited<ReturnType<typeof getOpsTenantOrder>>
  supplierOrderId: string
  canClose: boolean
  pendingRenewalItems: string[]
}

function pendingOrders(afterId: string | undefined, signal: AbortSignal) {
  return request<{ purchases?: PendingOrderWire[], next_after_id?: string }>(
    '/v1/operations/orders/static-pending', { query: { after_id: afterId }, signal },
  )
}

function pendingFields(item: PendingOrderWire) {
  return {
    supplierOrderId: item.supplier_order_id ?? '',
    canClose: item.can_close ?? false,
    pendingRenewalItems: item.pending_renewal_items ?? [],
  }
}

export async function refreshOpsPendingOrder(row: OpsPendingOrder, signal: AbortSignal): Promise<OpsPendingOrder | undefined> {
  const { tenant, order: { id } } = row
  // The queue sorts by ascending numeric ID. Start immediately before this order.
  const page = await pendingOrders((BigInt(id) - 1n).toString(), signal)
  const item = page.purchases?.[0]
  if (item?.order_id !== id || item.tenant_id !== tenant.id) return
  const order = await getOpsTenantOrder(tenant.id, id, signal)
  if (order.status !== 'pending') return
  return { tenant, order, ...pendingFields(item) }
}

export async function listOpsPendingOrders(afterId: string | undefined, signal: AbortSignal) {
  const wire = await pendingOrders(afterId, signal)
  const tenants = new Map<string, ReturnType<typeof getOpsTenant>>()
  function tenantFor(id: string) {
    const cached = tenants.get(id)
    if (cached) return cached
    const pending = getOpsTenant(id, signal)
    tenants.set(id, pending)
    return pending
  }
  const records = await Promise.all((wire.purchases ?? []).map(async (item): Promise<OpsPendingOrder> => {
    const [tenant, order] = await Promise.all([
      tenantFor(item.tenant_id),
      getOpsTenantOrder(item.tenant_id, item.order_id, signal),
    ])
    return { tenant, order, ...pendingFields(item) }
  }))
  return { records, nextAfterId: wire.next_after_id && wire.next_after_id !== '0' ? wire.next_after_id : undefined }
}

export async function resolveOpsStaticPurchase(
  { tenant, order }: OpsPendingOrder,
  input: { supplierOrderId: string, evidence: string, closeRemaining: boolean, confirmedUndeliverable: boolean },
) {
  const path = opsTenantPath(tenant.id) + '/orders/' + encodeURIComponent(order.id)
  return mapOrderDetail(await request<OrderDetailWire>(path + '/static-resolution', {
    body: {
      supplier_order_id: input.supplierOrderId,
      evidence: input.evidence,
      close_remaining: input.closeRemaining,
      confirmed_undeliverable: input.confirmedUndeliverable,
    },
  }))
}
