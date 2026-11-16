import { request, requestDownload } from './control'
import type { Order } from './wallet'

export interface StaticOffer {
  node_id: string
  region: string
  ip_type?: string
  term: string
  term_days: number
  unit_minor: string
  available?: number
}

export interface StaticIP {
  supplier_item_id: string
  order_id: string
  ip: string
  port: number
  username: string
  password: string
  expires_at: string
}

export interface StaticQuote {
  offer: StaticOffer
  quantity: number
  confirmation_token: string
  expires_at: string
}

export function quoteStaticPurchase(token: string, offer: StaticOffer, quantity: number) {
  return request<StaticQuote>('/v1/tenant/static-purchase-quotes', { token, body: {
    node_id: offer.node_id, term: offer.term, quantity,
  } })
}

export function staticOffers(signal: AbortSignal) {
  return request<{ open?: boolean; offers?: StaticOffer[] }>('/v1/public/static-offers', { signal })
}

export function buyStaticIP(token: string, offer: StaticOffer, quantity: number, requestKey: string, confirmationToken: string) {
  return request<Order>('/v1/tenant/static-orders', { token, body: {
    node_id: offer.node_id, term: offer.term, unit_minor: offer.unit_minor, quantity, request_key: requestKey, confirmation_token: confirmationToken,
  } })
}

export function staticIPs(token: string, signal: AbortSignal) {
  return request<{ allocations?: StaticIP[] }>('/v1/tenant/static-ips', { token, signal })
}

export function staticIPConfig(token: string, id: string, signal: AbortSignal) {
  return request<StaticIP>('/v1/tenant/static-ips/' + encodeURIComponent(id) + '/proxy-config', { token, signal })
}

export function staticIPExport(token: string, ids: string[], signal: AbortSignal) {
  return requestDownload('/v1/tenant/static-ips:export', { token, signal, body: { supplier_item_ids: ids } }, 'text/plain')
}

export function resolveStaticPurchase(token: string, tenant: string, order: string, body: {
  supplier_order_id: string; evidence: string; close_remaining: boolean; confirmed_undeliverable: boolean
}) {
  return request<Order>('/v1/operations/tenants/' + encodeURIComponent(tenant) +
    '/orders/' + encodeURIComponent(order) + '/static-resolution', { token, body })
}


export interface PendingStaticPurchase {
  order_id: string
  tenant_id: string
  supplier_order_id?: string
  created_at: string
  delivered_count?: string
  can_close?: boolean
}

export function pendingStaticPurchases(token: string, after: string, signal: AbortSignal) {
  return request<{ purchases?: PendingStaticPurchase[]; next_after_id?: string }>(
    '/v1/operations/orders/static-pending?' + new URLSearchParams({ after_id: after || '0' }), { token, signal })
}


export function claimStaticIPExpiryNotices(token: string, signal: AbortSignal) {
  return request<{ count?: number }>('/v1/tenant/static-ip-expiry-notices', { token, signal, body: {} })
}
