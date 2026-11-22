import { request } from './client'
import { mapWalletEntry, type WalletEntryWire } from './wallet'

export const orderProducts = ['dynamic', 'static'] as const
export const orderStatuses = ['pending', 'succeeded', 'partial', 'failed'] as const
export const orderKinds = ['traffic_package', 'static_purchase', 'static_renewal'] as const
export type OrderProduct = (typeof orderProducts)[number]
export type OrderKind = (typeof orderKinds)[number]
export type OrderStatus = (typeof orderStatuses)[number]

interface OrderItemWire {
  position: number
  specification?: string
  term_days?: number
  capacity_bytes?: string
  unit_minor?: string
  quantity?: number
  delivered_count?: number
  supplier_item_id?: string
  outcome?: string
}

interface OrderWire {
  id: string
  kind: string
  status: string
  created_at: string
  locked_minor?: string
  prepaid_minor?: string
  settled_minor?: string
  unsettled_minor?: string
  refunded_minor?: string
  delivered_bytes?: string
  delivered_count?: string
  summary_item?: OrderItemWire
  item_count?: string
}

export interface OrdersWire {
  orders?: OrderWire[]
  next_before_id?: string
  pending_count?: string
}

export interface OrderDetailWire extends OrderWire {
  items?: OrderItemWire[]
  wallet_entries?: WalletEntryWire[]
}

interface OrderFilters {
  product?: OrderProduct
  status?: OrderStatus
  orderQuery?: string
  start?: string
  end?: string
  beforeId?: string
}

function mapItem(wire: OrderItemWire) {
  return {
    position: wire.position,
    specification: wire.specification,
    termDays: wire.term_days ?? 0,
    capacityBytes: BigInt(wire.capacity_bytes ?? 0),
    unitMinor: BigInt(wire.unit_minor ?? 0),
    quantity: wire.quantity ?? 0,
    deliveredCount: wire.delivered_count ?? 0,
    supplierItemId: wire.supplier_item_id,
    outcome: wire.outcome,
  }
}

function mapOrder(wire: OrderWire) {
  const kind: OrderKind | 'unknown' = orderKinds.find(kind => kind === wire.kind) ?? 'unknown'
  const status: OrderStatus | 'unknown' = orderStatuses.find(status => status === wire.status) ?? 'unknown'
  const createdAt = new Date(wire.created_at)
  if (!Number.isFinite(createdAt.getTime())) throw new Error('Invalid order timestamp')
  return {
    id: wire.id,
    kind,
    status,
    createdAt,
    lockedMinor: BigInt(wire.locked_minor ?? 0),
    prepaidMinor: BigInt(wire.prepaid_minor ?? 0),
    settledMinor: BigInt(wire.settled_minor ?? 0),
    unsettledMinor: BigInt(wire.unsettled_minor ?? 0),
    refundedMinor: BigInt(wire.refunded_minor ?? 0),
    deliveredBytes: BigInt(wire.delivered_bytes ?? 0),
    deliveredCount: BigInt(wire.delivered_count ?? 0),
    summaryItem: wire.summary_item && mapItem(wire.summary_item),
    itemCount: Number(wire.item_count ?? 0),
  }
}

export type OrderItem = ReturnType<typeof mapItem>
export type OrderSummary = ReturnType<typeof mapOrder>

export async function listOrders(filters: OrderFilters, signal: AbortSignal) {
  const wire = await request<OrdersWire>('/v1/tenant/orders', {
    query: {
      product: filters.product,
      status: filters.status,
      order_query: filters.orderQuery,
      start_date: filters.start,
      end_date: filters.end,
      before_id: filters.beforeId,
    },
    signal,
  })
  return mapOrders(wire)
}

export function mapOrders(wire: OrdersWire) {
  return {
    orders: (wire.orders ?? []).map(mapOrder),
    pendingCount: Number(wire.pending_count ?? 0),
    nextBeforeId: wire.next_before_id && wire.next_before_id !== '0' ? wire.next_before_id : undefined,
  }
}

export async function getPendingOrderCount(signal: AbortSignal) {
  const wire = await request<OrdersWire>('/v1/tenant/orders', { query: { status: 'pending' }, signal })
  return Number(wire.pending_count ?? 0)
}

export async function getOrder(orderId: string, signal: AbortSignal) {
  const wire = await request<OrderDetailWire>(`/v1/tenant/orders/${encodeURIComponent(orderId)}`, { signal })
  return mapOrderDetail(wire)
}

export function mapOrderDetail(wire: OrderDetailWire) {
  return {
    ...mapOrder(wire),
    items: (wire.items ?? []).map(mapItem),
    walletEntries: (wire.wallet_entries ?? []).map(mapWalletEntry),
  }
}
