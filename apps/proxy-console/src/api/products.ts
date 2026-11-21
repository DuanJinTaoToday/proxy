import { request } from './client'

interface TrafficPackageWire {
  id: string
  capacity_bytes: string
  price_minor?: string
  listed?: boolean
  version: string
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
}

export async function getTrafficCatalog(signal: AbortSignal) {
  const { products } = await request<{ products?: ProductWire[] }>('/v1/public/products', { signal })
  const dynamic = products?.find(product => product.kind === 'dynamic')
  const packages: TrafficPackage[] = (dynamic?.packages ?? [])
    .filter(tier => tier.listed && BigInt(tier.price_minor ?? 0) > 0n)
    .map(tier => ({
      id: tier.id,
      capacityBytes: BigInt(tier.capacity_bytes),
      priceMinor: BigInt(tier.price_minor ?? 0),
      version: String(tier.version),
    }))
  return { open: dynamic?.open === true, canPurchase: dynamic?.can_purchase === true, packages }
}
