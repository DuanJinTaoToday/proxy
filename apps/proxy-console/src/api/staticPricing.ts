import { request } from './client'

export type StaticCatalogFilter =
  | { shareMode: 'exclusive' }
  | { shareMode: 'shared', purposeId?: number }

interface StaticOfferWire {
  node_id: string
  region?: string
  ip_type?: string
  continent?: string
  line?: string
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
    continent: wire.continent ?? '',
    line: wire.line ?? '',
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

export const maxStaticPurchaseLines = 25
export const maxStaticPurchaseQuantity = 2_147_483_647

export interface StaticPurchaseLine {
  offer: StaticPurchaseOffer
  quantity: number
}

export interface StaticPurchaseSelection {
  term: string
  lines: StaticPurchaseLine[]
}

interface StaticPurchaseQuoteWire {
  lines?: { offer: StaticOfferWire, quantity: number }[]
  confirmation_token: string
}

export async function quoteStaticPurchase(selection: StaticPurchaseSelection, filter: StaticCatalogFilter, signal: AbortSignal) {
  const { term, lines } = selection
  const wire = await request<StaticPurchaseQuoteWire>('/v1/tenant/static-purchase-quotes', {
    body: { term, lines: lines.map(({ offer, quantity }) => ({ node_id: offer.nodeId, quantity })), ...catalogParams(filter) },
    signal,
  })
  const quoted = new Map(wire.lines?.map(line => [String(line.offer.node_id), line]))
  const confirmedLines = lines.map(({ offer, quantity }) => {
    const line = quoted.get(offer.nodeId)
    if (!line || line.quantity !== quantity || line.offer.term !== term) throw new Error('Unexpected static purchase quote line')
    return { offer: toOffer(line.offer), quantity }
  })
  if (wire.lines?.length !== lines.length || !wire.confirmation_token) throw new Error('Unexpected static purchase quote')
  return { term, lines: confirmedLines, confirmationToken: wire.confirmation_token }
}

export type StaticPurchaseQuote = Awaited<ReturnType<typeof quoteStaticPurchase>>

export function makeStaticPurchaseRequest(quote: StaticPurchaseQuote, filter: StaticCatalogFilter) {
  return {
    term: quote.term,
    lines: quote.lines.map(({ offer, quantity }) => ({ node_id: offer.nodeId, quantity, unit_minor: offer.unitMinor.toString() })),
    confirmation_token: quote.confirmationToken,
    request_key: crypto.randomUUID(),
    ...catalogParams(filter),
  }
}

type StaticPurchaseBatchRequest = ReturnType<typeof makeStaticPurchaseRequest>
type StaticPurchaseOrderLine = StaticPurchaseBatchRequest['lines'][number]
type StaticPurchaseSingleRequest = Omit<StaticPurchaseBatchRequest, 'lines'> & StaticPurchaseOrderLine
export type StaticPurchaseRequest = StaticPurchaseBatchRequest | StaticPurchaseSingleRequest

function isPurchaseLine(value: unknown): value is StaticPurchaseOrderLine {
  if (!value || typeof value !== 'object') return false
  return 'node_id' in value && typeof value.node_id === 'string' && /^[1-9]\d*$/.test(value.node_id)
    && 'quantity' in value && typeof value.quantity === 'number' && Number.isInteger(value.quantity)
    && value.quantity > 0 && value.quantity <= maxStaticPurchaseQuantity
    && 'unit_minor' in value && typeof value.unit_minor === 'string' && /^[1-9]\d*$/.test(value.unit_minor)
}

export function isStaticPurchaseRequest(value: unknown): value is StaticPurchaseRequest {
  if (!value || typeof value !== 'object') return false
  const validEnvelope = 'term' in value && typeof value.term === 'string' && value.term.length > 0
    && 'confirmation_token' in value && typeof value.confirmation_token === 'string' && value.confirmation_token.length > 0
    && 'request_key' in value && typeof value.request_key === 'string' && value.request_key.length > 0
    && (!('share_mode' in value) || value.share_mode === 'shared')
    && (!('purpose_id' in value) || (typeof value.purpose_id === 'number' && Number.isInteger(value.purpose_id)))
  if (!validEnvelope) return false
  if (!('lines' in value)) return isPurchaseLine(value)
  return Array.isArray(value.lines) && value.lines.length > 0 && value.lines.length <= maxStaticPurchaseLines
    && value.lines.every(isPurchaseLine)
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
