import { request } from './client'

interface OverviewWire {
  wallet_minor?: string
  traffic_bytes?: string
  active_static_ip_count?: number
  notices?: { kind: string, count: number }[]
  products?: { kind: string, can_purchase?: boolean }[]
}

export async function getOverview(signal: AbortSignal) {
  const wire = await request<OverviewWire>('/v1/tenant/overview', { signal })
  return {
    walletMinor: BigInt(wire.wallet_minor ?? 0),
    trafficBytes: BigInt(wire.traffic_bytes ?? 0),
    activeStaticIpCount: wire.active_static_ip_count ?? 0,
    expiringIpCount: wire.notices?.find(notice => notice.kind === 'static_ip_expiry')?.count ?? 0,
    canBuyTraffic: wire.products?.some(product => product.kind === 'dynamic' && product.can_purchase) ?? false,
    canBuyStatic: wire.products?.some(product => product.kind === 'static' && product.can_purchase) ?? false,
  }
}

export async function listActiveStaticExpiries(activeCount: number, signal: AbortSignal) {
  if (activeCount === 0) return []
  const wire = await request<{ allocations?: { allocation_id: string, expires_at: string, pending_renewal_order_id?: string }[] }>('/v1/tenant/static-ips', { signal })
  const expiries = (wire.allocations ?? []).map(allocation => ({
    key: JSON.stringify([allocation.allocation_id, allocation.expires_at]),
    expiresAt: new Date(allocation.expires_at),
    id: allocation.allocation_id,
    pendingRenewal: Boolean(allocation.pending_renewal_order_id),
  })).filter(allocation => Number.isFinite(allocation.expiresAt.getTime()))
  // The overview counts active IPs using server time. The active allocations have the latest expiries.
  return expiries.sort((a, b) => a.expiresAt.getTime() - b.expiresAt.getTime()).slice(-activeCount)
}

export async function hasPendingOrders(signal: AbortSignal) {
  const wire = await request<{ orders?: { id: string }[] }>('/v1/tenant/orders', { query: { status: 'pending' }, signal })
  return Boolean(wire.orders?.length)
}

export async function claimStaticExpiryNotices() {
  const wire = await request<{ count?: number }>('/v1/tenant/static-ip-expiry-notices', { body: {} })
  return wire.count ?? 0
}
