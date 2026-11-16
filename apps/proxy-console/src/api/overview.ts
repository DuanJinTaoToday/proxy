import { request } from './control'
import type { ProductKind } from './product'

export interface Overview {
  wallet_minor: string
  traffic_bytes: string
  active_static_ip_count: number
  notices: { kind: 'low_traffic_balance' | 'static_ip_expiry'; count: number }[]
  products: { kind: ProductKind; open: boolean; can_purchase: boolean }[]
  can_create_subaccount: boolean
}

export function getOverview(token: string, signal: AbortSignal) {
  return request<Overview>('/v1/tenant/overview', { token, signal })
}
