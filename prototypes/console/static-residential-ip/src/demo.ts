import { supplierNodes, type ProjectKey, type SupplierNode, type SupplierPrice } from './catalog'

export type Product = 'exclusive' | 'shared'
export const productLabels: Record<Product, string> = { exclusive: '独享', shared: '共享' }

const termDays: Record<string, number> = { rp: 1, zp: 7, yp1: 30, yp3: 90, yp6: 180, np: 360 }
const termOrder = Object.keys(termDays)

export interface Term { code: string, name: string, days: number }

export function toTerm(price: SupplierPrice): Term {
  return { code: price.code, name: price.label.replace(/（.*）/, ''), days: termDays[price.code] }
}

export function termsOf(prices: readonly SupplierPrice[]): Term[] {
  return prices.filter(p => p.code in termDays).map(toTerm).sort((a, b) => termOrder.indexOf(a.code) - termOrder.indexOf(b.code))
}

export function priceCents(prices: readonly SupplierPrice[], code: string) {
  const price = prices.find(p => p.code === code)
  return price ? Math.round(price.yuan * 100) : undefined
}

export interface StaticIp {
  address: string
  port: number
  username: string
  password: string
  product: Product
  region: string
  isp: string
  prices: SupplierPrice[]
  expiry: number
  renewing?: boolean
}

export type IpStatus = 'available' | 'expiring' | 'renewing' | 'expired'

export const demoNow = Date.parse('2026-11-20T09:00:00+08:00')
export const dayMs = 86_400_000
export const expiringWindowDays = 5

export function regionName(node: SupplierNode) { return node.province ? `${node.country} · ${node.province}` : node.country }
export function nodeById(id: number) { return supplierNodes.find(node => node.nodeId === id)! }

function ownedIp(nodeId: number, product: Product, address: string, username: string, password: string, expiry: string, renewing = false): StaticIp {
  const node = nodeById(nodeId)
  return { address, port: 1080, username, password, product, region: regionName(node), isp: node.isp, prices: node[product].prices, expiry: Date.parse(expiry), renewing }
}

export const demoIps: StaticIp[] = [
  ownedIp(13, 'exclusive', '203.0.113.18', 'px8f21', 'Kd82mQ4x', '2026-11-23T18:00:00+08:00'),
  ownedIp(11, 'exclusive', '198.51.100.42', 'px3c07', 'Wv5nT9pe', '2026-12-18T09:30:00+08:00'),
  ownedIp(7, 'shared', '192.0.2.61', 'px6a90', 'Hr3zL7bc', '2026-12-20T12:00:00+08:00'),
  ownedIp(4, 'exclusive', '203.0.113.77', 'px1d54', 'Qa6yF2ks', '2026-11-24T10:00:00+08:00', true),
  ownedIp(6, 'shared', '198.51.100.9', 'px9e33', 'Mt4gB8wu', '2026-11-12T20:00:00+08:00'),
]

const dateTime = new Intl.DateTimeFormat('zh-CN', {
  timeZone: 'Asia/Shanghai',
  year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
})

const yuan = new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' })

export function formatExpiry(value: number) { return dateTime.format(value) }
export function formatMoney(cents: number) { return yuan.format(cents / 100) }
export function endpoint(ip: StaticIp) { return `${ip.address}:${ip.port}` }
export function credentialLine(ip: StaticIp) { return `${endpoint(ip)}:${ip.username}:${ip.password}` }

export function statusOf(ip: StaticIp): IpStatus {
  if (ip.expiry <= demoNow) return 'expired'
  if (ip.renewing) return 'renewing'
  if (ip.expiry - demoNow <= expiringWindowDays * dayMs) return 'expiring'
  return 'available'
}

export function canRenew(ip: StaticIp) {
  const status = statusOf(ip)
  return status === 'available' || status === 'expiring'
}

export function remainingLabel(ip: StaticIp) {
  const diff = ip.expiry - demoNow
  if (diff <= 0) return `已过期 ${Math.ceil(-diff / dayMs)} 天`
  return `剩余 ${Math.ceil(diff / dayMs)} 天`
}

export function stockOf(node: SupplierNode, product: Product, project: ProjectKey) {
  return product === 'exclusive' ? node.exclusive.stock : node.shared.stock[project]
}

let deliverySeq = 0

export function deliverIps(node: SupplierNode, product: Product, quantity: number, days: number): StaticIp[] {
  return Array.from({ length: quantity }, () => {
    const n = deliverySeq++
    return {
      address: `198.18.${Math.floor(n / 250)}.${(n % 250) + 1}`,
      port: 1080,
      username: `px${(4096 + n * 7919).toString(16).slice(-4)}`,
      password: `Dm${(n * 48271 % 99991).toString(36)}x${n}`,
      product,
      region: regionName(node),
      isp: node.isp,
      prices: node[product].prices,
      expiry: demoNow + days * dayMs,
    }
  })
}
