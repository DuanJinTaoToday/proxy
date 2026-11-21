import { request } from './client'

export type StaticCatalogFilter =
  | { shareMode: 'exclusive' }
  | { shareMode: 'shared', purposeId?: number }

interface StaticOfferWire {
  node_id: string
  region?: string
  ip_type?: string
  term: string
  term_days: number
  unit_minor: string
  available?: number
}

function toOffer(wire: StaticOfferWire) {
  return {
    nodeId: String(wire.node_id),
    region: wire.region ?? '',
    isp: wire.ip_type ?? '',
    term: wire.term,
    termDays: wire.term_days,
    unitMinor: BigInt(wire.unit_minor),
    available: wire.available ?? 0,
  }
}

export type StaticPurchaseOffer = ReturnType<typeof toOffer>

function catalogParams(filter: StaticCatalogFilter) {
  return filter.shareMode === 'shared'
    ? { share_mode: 'shared', purpose_id: filter.purposeId }
    : {}
}

export async function listStaticOffers(filter: StaticCatalogFilter, signal: AbortSignal) {
  const wire = await request<{ open?: boolean, offers?: StaticOfferWire[] }>('/v1/public/static-offers', {
    query: catalogParams(filter),
    signal,
  })
  return { open: wire.open ?? false, offers: (wire.offers ?? []).map(toOffer) }
}

export async function getStaticWalletBalance(signal: AbortSignal) {
  const wire = await request<{ available_minor?: string }>('/v1/tenant/wallet', { signal })
  return BigInt(wire.available_minor ?? 0)
}

interface StaticPurchaseQuoteWire {
  offer: StaticOfferWire
  quantity: number
  confirmation_token: string
}

export async function quoteStaticPurchase({
  filter, offer, quantity, signal,
}: {
  filter: StaticCatalogFilter
  offer: StaticPurchaseOffer
  quantity: number
  signal: AbortSignal
}) {
  const wire = await request<StaticPurchaseQuoteWire>('/v1/tenant/static-purchase-quotes', {
    body: { node_id: offer.nodeId, term: offer.term, quantity, ...catalogParams(filter) },
    signal,
  })
  return { offer: toOffer(wire.offer), quantity: wire.quantity, confirmationToken: wire.confirmation_token }
}

export type StaticPurchaseQuote = Awaited<ReturnType<typeof quoteStaticPurchase>>

export function makeStaticPurchaseRequest(quote: StaticPurchaseQuote, filter: StaticCatalogFilter) {
  return {
    node_id: quote.offer.nodeId,
    term: quote.offer.term,
    quantity: quote.quantity,
    unit_minor: quote.offer.unitMinor.toString(),
    confirmation_token: quote.confirmationToken,
    request_key: crypto.randomUUID(),
    ...catalogParams(filter),
  }
}

type StaticPurchaseRequest = ReturnType<typeof makeStaticPurchaseRequest>

export function isStaticPurchaseRequest(value: unknown): value is StaticPurchaseRequest {
  if (!value || typeof value !== 'object') return false
  return 'node_id' in value && typeof value.node_id === 'string' && /^[1-9]\d*$/.test(value.node_id)
    && 'term' in value && typeof value.term === 'string' && value.term.length > 0
    && 'quantity' in value && typeof value.quantity === 'number' && Number.isInteger(value.quantity) && value.quantity > 0
    && 'unit_minor' in value && typeof value.unit_minor === 'string' && /^[1-9]\d*$/.test(value.unit_minor)
    && 'confirmation_token' in value && typeof value.confirmation_token === 'string' && value.confirmation_token.length > 0
    && 'request_key' in value && typeof value.request_key === 'string' && value.request_key.length > 0
    && (!('share_mode' in value) || value.share_mode === 'shared')
    && (!('purpose_id' in value) || (typeof value.purpose_id === 'number' && Number.isInteger(value.purpose_id)))
}

interface StaticPurchaseOrderWire {
  id: string
  status: string
  delivered_count?: string
  refunded_minor?: string
}

function toOrder(wire: StaticPurchaseOrderWire) {
  const { status } = wire
  if (status !== 'pending' && status !== 'succeeded' && status !== 'partial' && status !== 'failed') {
    throw new Error('Unexpected static purchase status')
  }
  return {
    id: String(wire.id),
    status,
    deliveredCount: BigInt(wire.delivered_count ?? 0),
    refundedMinor: BigInt(wire.refunded_minor ?? 0),
  }
}

export type StaticPurchaseOrder = ReturnType<typeof toOrder>

export async function purchaseStaticIP(body: StaticPurchaseRequest, signal: AbortSignal) {
  return toOrder(await request<StaticPurchaseOrderWire>('/v1/tenant/static-orders', { body, signal }))
}

export async function getStaticPurchaseOrder(id: string, signal: AbortSignal) {
  return toOrder(await request<StaticPurchaseOrderWire>(`/v1/tenant/orders/${encodeURIComponent(id)}`, { signal }))
}
