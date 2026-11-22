import { request } from './client'

interface TrafficPackageWire {
  id: string
  capacity_bytes: string
  price_minor?: string
  listed?: boolean
  version: string
  discount_percent?: number
}

interface ProductWire {
  kind: string
  open?: boolean
  can_purchase?: boolean
  packages?: TrafficPackageWire[]
}

export interface TrafficPackage {
  id: string
  capacityBytes: bigint
  priceMinor: bigint
  version: string
  discountPercent: number
}

export type ProductKind = 'dynamic' | 'static'
export type OpsTrafficPackage = Omit<TrafficPackage, 'discountPercent'> & { listed: boolean }

interface ChangeValueWire {
  open?: boolean
  listed?: boolean
  capacity_bytes?: string
  price_minor?: string
}

interface ProductChangeWire {
  id: string
  target_kind: 'sale' | 'package'
  target_id: string
  operator_username: string
  created_at: string
  before: ChangeValueWire
  after: ChangeValueWire
}

function readChangeValue(value: ChangeValueWire) {
  return {
    open: value.open === true,
    listed: value.listed === true,
    capacityBytes: BigInt(value.capacity_bytes ?? 0),
    priceMinor: BigInt(value.price_minor ?? 0),
  }
}

async function listProducts(signal: AbortSignal) {
  const { products } = await request<{ products?: ProductWire[] }>('/v1/public/products', { signal, anonymous: true })
  return products ?? []
}

function readPackage(tier: TrafficPackageWire): OpsTrafficPackage {
  return {
    id: tier.id,
    capacityBytes: BigInt(tier.capacity_bytes),
    priceMinor: BigInt(tier.price_minor ?? 0),
    version: String(tier.version),
    listed: tier.listed === true,
  }
}

export async function getPublicCatalog(signal: AbortSignal) {
  const products = await listProducts(signal)
  const dynamic = products.find(product => product.kind === 'dynamic')
  const staticProduct = products.find(product => product.kind === 'static')
  const packages: TrafficPackage[] = (dynamic?.packages ?? [])
    .filter(tier => tier.listed && BigInt(tier.price_minor ?? 0) > 0n)
    .map(tier => ({
      ...readPackage(tier),
      discountPercent: tier.discount_percent ?? 0,
    }))
  return {
    dynamic: { open: dynamic?.open === true, canPurchase: dynamic?.can_purchase === true, packages },
    staticOpen: staticProduct?.open === true && staticProduct.can_purchase === true,
  }
}

export async function getTrafficCatalog(signal: AbortSignal) {
  return (await getPublicCatalog(signal)).dynamic
}

export async function getOpsCatalog(signal: AbortSignal) {
  const [products, { packages }, { changes }] = await Promise.all([
    listProducts(signal),
    request<{ packages?: TrafficPackageWire[] }>('/v1/operations/traffic-packages', { signal }),
    request<{ changes?: ProductChangeWire[] }>('/v1/operations/product-changes', { signal }),
  ])
  return {
    sales: {
      dynamic: products.find(product => product.kind === 'dynamic')?.open === true,
      static: products.find(product => product.kind === 'static')?.open === true,
    },
    packages: (packages ?? []).map(readPackage),
    changes: (changes ?? []).map(row => ({
      id: row.id, kind: row.target_kind, target: row.target_id, operator: row.operator_username, at: row.created_at,
      before: readChangeValue(row.before), after: readChangeValue(row.after),
    })),
  }
}

export function updateTrafficPackage(tier: OpsTrafficPackage, patch: { priceMinor: bigint, listed: boolean }, signal: AbortSignal) {
  return request<TrafficPackageWire>('/v1/operations/traffic-packages/' + encodeURIComponent(tier.id), {
    method: 'PATCH', signal,
    body: { capacity_bytes: tier.capacityBytes.toString(), version: tier.version, price_minor: patch.priceMinor.toString(), listed: patch.listed },
  })
}

type SalesChange = { open: false } | { open: true, evidence: string }

export function setProductSale(kind: ProductKind, change: SalesChange, signal: AbortSignal) {
  return request<ProductWire>('/v1/operations/product-sales/' + kind, {
    method: 'PUT', signal,
    body: change.open ? { open: true, quote_ready: true, acceptance_evidence: change.evidence } : { open: false },
  })
}
