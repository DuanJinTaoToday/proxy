import { request, requestText } from './client'

interface StaticIpWire {
  allocation_id: string
  ip: string
  port: number
  expires_at: string
  region?: string
  ip_type?: string
  pending_renewal_order_id?: string
}

interface RenewalTermWire { term: string, term_days: number }
interface RenewalQuoteLineWire {
  allocation_id: string
  ip: string
  unit_minor: string
  term_days: number
  original_expiry: string
}
interface RenewalOrderWire {
  id: string
  status: 'succeeded' | 'failed' | 'partial' | 'pending'
  items: { allocation_id: string, specification: string, outcome: RenewalOutcome }[]
}
export type RenewalOutcome = 'pending' | 'succeeded' | 'failed'

export interface RenewalSubmission {
  allocation_ids: string[]
  term: string
  request_key: string
  confirmation_token: string
}

const ipsPath = '/v1/tenant/static-ips'
const ordersPath = '/v1/tenant/static-renewal-orders'

export async function listStaticIps(signal: AbortSignal) {
  const { allocations = [] } = await request<{ allocations?: StaticIpWire[] }>(ipsPath, { signal })
  return allocations.map(wire => ({
    id: wire.allocation_id,
    ip: wire.ip,
    port: wire.port,
    expiry: new Date(wire.expires_at),
    region: wire.region ?? '',
    ipType: wire.ip_type ?? '',
    pendingOrderId: wire.pending_renewal_order_id,
  }))
}

export type StaticIp = Awaited<ReturnType<typeof listStaticIps>>[number]

export async function getStaticIpCredentials(id: string, signal: AbortSignal) {
  const wire = await request<{ ip: string, port: number, username: string, password: string }>(
    `${ipsPath}/${encodeURIComponent(id)}/proxy-config`, { signal },
  )
  return { ip: wire.ip, port: wire.port, username: wire.username, password: wire.password }
}

export function exportStaticIps(ids: string[], signal: AbortSignal) {
  return requestText(`${ipsPath}:export`, { body: { allocation_ids: ids }, signal }, 'text/plain')
}

export async function listRenewalTerms(ids: string[], signal: AbortSignal) {
  const { terms = [] } = await request<{ terms?: RenewalTermWire[] }>('/v1/tenant/static-renewal-terms', {
    body: { allocation_ids: ids }, signal,
  })
  return terms.map(wire => ({ code: wire.term, days: wire.term_days }))
}

export async function quoteStaticRenewal(ids: string[], term: string, signal: AbortSignal) {
  const { lines, confirmation_token } = await request<{ lines: RenewalQuoteLineWire[], confirmation_token: string }>(
    '/v1/tenant/static-renewal-quotes', { body: { allocation_ids: ids, term }, signal },
  )
  return {
    token: confirmation_token,
    lines: lines.map(wire => ({
      id: wire.allocation_id,
      ip: wire.ip,
      unitMinor: BigInt(wire.unit_minor),
      newExpiry: new Date(Date.parse(wire.original_expiry) + wire.term_days * 86_400_000),
    })),
  }
}

function toRenewalOrder(wire: RenewalOrderWire) {
  return {
    id: wire.id,
    status: wire.status,
    items: wire.items.map(item => ({ id: item.allocation_id, ip: item.specification, outcome: item.outcome })),
  }
}

export async function submitStaticRenewal(body: RenewalSubmission, signal: AbortSignal) {
  return toRenewalOrder(await request<RenewalOrderWire>(ordersPath, { body, signal }))
}

export async function getStaticRenewalOrder(orderId: string, signal: AbortSignal) {
  return toRenewalOrder(await request<RenewalOrderWire>(`/v1/tenant/orders/${encodeURIComponent(orderId)}`, { signal }))
}

export async function recheckStaticRenewal(orderId: string, signal: AbortSignal) {
  return toRenewalOrder(await request<RenewalOrderWire>(`${ordersPath}/${encodeURIComponent(orderId)}/recheck`, { body: {}, signal }))
}

export type RenewalOrder = ReturnType<typeof toRenewalOrder>
