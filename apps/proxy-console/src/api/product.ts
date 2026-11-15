import { request } from './control'

export type ProductKind = 'dynamic' | 'static'

export interface TrafficPackage {
  id: string
  capacity_bytes: string
  price_minor?: string
  listed?: boolean
  version: string
}

export interface Product {
  kind: ProductKind
  description: string
  open?: boolean
  can_purchase?: boolean
  packages?: TrafficPackage[]
}

export const productNames: Record<ProductKind, string> = {
  dynamic: '动态住宅 GB',
  static: '静态住宅 IP',
}

export async function listProducts(signal?: AbortSignal) {
  return (await request<{ products: Product[] }>('/public/products', { signal })).products
}

const packagesPath = '/v1/operations/traffic-packages'

export async function listTrafficPackages(token: string, signal?: AbortSignal) {
  return (await request<{ packages: TrafficPackage[] }>(packagesPath, { token, signal })).packages
}

export function updateTrafficPackage(token: string, pkg: TrafficPackage) {
  const { id, capacity_bytes, price_minor = '0', version, listed = false } = pkg
  return request<TrafficPackage>(packagesPath + '/' + encodeURIComponent(id), {
    token, method: 'PATCH', body: { capacity_bytes, price_minor: price_minor || '0', version, listed },
  })
}

export function setProductSale(token: string, kind: ProductKind, open: boolean, quoteReady: boolean, evidence: string) {
  return request<Product>('/v1/operations/product-sales/' + kind, {
    token, method: 'PUT', body: { open, quote_ready: quoteReady, acceptance_evidence: evidence },
  })
}
