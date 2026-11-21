import { computed, ref } from 'vue'
import { dayMs, demoIps, demoNow, endpoint, productLabels, type StaticIp, type Term } from './demo'
import { persisted } from './persist'

export type OrderKind = 'traffic' | 'static-purchase' | 'static-renewal'
export type OrderStatus = 'completed' | 'partial' | 'refunded' | 'processing'
export type LedgerKind = 'recharge' | 'debit' | 'refund'

export interface OrderLine { label: string, quantity: number, unitCents: number, delivered: number, address?: string }

export interface Resolution { operator: string, at: number, evidence: string, supplierRef?: string, closed: boolean }

export interface Order {
  id: string
  kind: OrderKind
  createdAt: number
  term?: string
  credited?: string
  creditedGb?: number
  lines: OrderLine[]
  status: OrderStatus
  resolutions?: Resolution[]
}

export interface LedgerEntry {
  id: string
  at: number
  kind: LedgerKind
  amountCents: number
  balanceAfterCents: number
  note: string
  orderId?: string
  // 运营视图字段，客户控制台不展示。
  operator?: string
  reason?: string
}

export interface Book { orders: Order[], ledger: LedgerEntry[] }

export const orderKindLabels: Record<OrderKind, string> = { traffic: '动态流量包', 'static-purchase': '静态 IP 购买', 'static-renewal': '静态 IP 续费' }
export const orderStatusLabels: Record<OrderStatus, string> = { completed: '已完成', partial: '部分交付', refunded: '已退款', processing: '确认中' }
export const orderStatusTags: Record<OrderStatus, 'success' | 'warning' | 'info' | 'primary'> = { completed: 'success', partial: 'warning', refunded: 'info', processing: 'primary' }
export const ledgerKindLabels: Record<LedgerKind, string> = { recharge: '充值', debit: '扣款', refund: '退款' }

export function orderTotalCents(order: Order) { return order.lines.reduce((sum, l) => sum + l.quantity * l.unitCents, 0) }
export function orderRefundCents(order: Order) {
  if (order.status === 'processing') return 0
  return order.lines.reduce((sum, l) => sum + (l.quantity - l.delivered) * l.unitCents, 0)
}
const settledStatus = (lines: OrderLine[]): OrderStatus => lines.every(l => l.delivered === l.quantity) ? 'completed' : lines.every(l => l.delivered === 0) ? 'refunded' : 'partial'

const meta = persisted('meta', () => ({ clock: demoNow, seq: {} as Record<string, number> }))

function nextId(prefix: string, at: number) {
  const day = new Date(at + 8 * 3600_000).toISOString().slice(0, 10).replaceAll('-', '')
  meta.value.seq[prefix] = (meta.value.seq[prefix] ?? 0) + 1
  return `${prefix}${day}${String(meta.value.seq[prefix]).padStart(4, '0')}`
}

export function tick() {
  meta.value.clock += 60_000
  return meta.value.clock
}

export const balanceOf = (book: Book) => book.ledger.at(-1)?.balanceAfterCents ?? 0

export function post(book: Book, kind: LedgerKind, amountCents: number, note: string, at: number, extra: Pick<LedgerEntry, 'orderId' | 'operator' | 'reason'> = {}) {
  const signed = kind === 'debit' ? -amountCents : amountCents
  const entry: LedgerEntry = { id: nextId('WL', at), at, kind, amountCents: signed, balanceAfterCents: balanceOf(book) + signed, note, ...extra }
  book.ledger.push(entry)
  return entry
}

function placeOrder(book: Book, kind: OrderKind, lines: OrderLine[], at: number, extra: Pick<Order, 'term' | 'creditedGb'> & { processing?: boolean } = {}) {
  const status: OrderStatus = extra.processing ? 'processing' : settledStatus(lines)
  const credited = extra.creditedGb === undefined ? undefined : `${extra.creditedGb} GB`
  const order: Order = { id: nextId('SO', at), kind, createdAt: at, term: extra.term, credited, creditedGb: extra.creditedGb, lines, status }
  book.orders.push(order)
  post(book, 'debit', orderTotalCents(order), orderKindLabels[kind], at, { orderId: order.id })
  const refund = orderRefundCents(order)
  if (refund) post(book, 'refund', refund, '部分 IP 未能交付，退回未交付部分', at + 25 * 60_000, { orderId: order.id })
  return book.orders.at(-1)!
}

// 运营核实确认中的静态订单：更新已确认的交付数量；关闭时把仍未交付的部分退回钱包。
export function resolveOrder(tenantId: string, orderId: string, delivered: number[], resolution: Omit<Resolution, 'at'>) {
  const book = books.value[tenantId]
  const order = book.orders.find(o => o.id === orderId)!
  const at = tick()
  order.lines.forEach((line, i) => { line.delivered = delivered[i] })
  order.resolutions = [...order.resolutions ?? [], { ...resolution, at }]
  if (!resolution.closed && order.lines.some(l => l.delivered < l.quantity)) return order
  order.status = settledStatus(order.lines)
  const refund = orderRefundCents(order)
  if (refund) post(book, 'refund', refund, order.kind === 'static-renewal' ? '续费未成功，退回预扣' : '部分 IP 未能交付，退回未交付部分', at, { orderId, operator: resolution.operator })
  if (tenantId === liveTenantId && order.kind === 'static-renewal') {
    const days = Number(order.term?.match(/(\d+) 天/)?.[1] ?? 30)
    const renewed = new Map(order.lines.map(l => [l.address, l.delivered > 0]))
    ips.value = ips.value.map(ip => renewed.has(ip.address) ? { ...ip, renewing: false, expiry: ip.expiry + (renewed.get(ip.address) ? days * dayMs : 0) } : ip)
  }
  return order
}

export function rechargeWallet(tenantId: string, amountCents: number, reason: string, operator: string) {
  return post(books.value[tenantId], 'recharge', amountCents, '运营充值', tick(), { operator, reason })
}

export function recordStaticPurchase(lines: OrderLine[], term: Term) {
  return placeOrder(liveBook(), 'static-purchase', lines, tick(), { term: `${term.name} · ${term.days} 天` })
}

export function recordStaticRenewal(targets: readonly StaticIp[], term: Term, unitCents: (ip: StaticIp) => number) {
  const lines = targets.map(ip => ({ label: `${endpoint(ip)} · ${productLabels[ip.product]} · ${ip.region}`, quantity: 1, unitCents: unitCents(ip), delivered: 1, address: ip.address }))
  const addresses = new Set(targets.map(ip => ip.address))
  ips.value = ips.value.map(ip => addresses.has(ip.address) ? { ...ip, expiry: ip.expiry + term.days * dayMs } : ip)
  return placeOrder(liveBook(), 'static-renewal', lines, tick(), { term: `${term.name} · ${term.days} 天` })
}

export const ips = persisted<StaticIp[]>('ips', () => demoIps.map(ip => ({ ...ip })))

export function addIps(delivered: StaticIp[]) {
  ips.value = [...ips.value, ...delivered]
}

export const primaryAccount = 'chenxl-studio'
export const subAccountQuota = 10
export const subAccountNamePattern = /^[a-z][a-z0-9]{3,7}$/
export const subAccountPasswordPattern = /^(?=.*[A-Za-z])(?=.*\d)[A-Za-z\d]{6,12}$/

export interface SubAccount {
  name: string
  password: string
  remark: string
  active: boolean
  usedGb: number
  limitGb?: number
  createdAt: number
}

export const subAccounts = persisted<SubAccount[]>('sub-accounts', () => [
  { name: 'shop01', password: 'Tk82mfa4', remark: '亚马逊店铺运营', active: true, usedGb: 18.42, limitGb: 30, createdAt: Date.parse('2026-11-02T15:10:00+08:00') },
  { name: 'ads02', password: 'q7Wn3eLp', remark: '广告投放验证', active: true, usedGb: 20, limitGb: 20, createdAt: Date.parse('2026-11-03T10:22:00+08:00') },
  { name: 'crawl', password: 'Zx9c4Vb2', remark: '价格监控爬虫', active: true, usedGb: 41.07, createdAt: Date.parse('2026-11-06T09:41:00+08:00') },
  { name: 'test9', password: 'abc123de', remark: '', active: false, usedGb: 0.36, createdAt: Date.parse('2026-11-10T17:05:00+08:00') },
])

export function socksUsername(name: string) { return `${primaryAccount}.${name}` }

// 原型示例数据，URL 加 ?ip=cn 可查看中国大陆网络下的状态。正式版由控制面按本次请求的来源 IP 查询归属地。
export const currentIp = new URLSearchParams(location.search).get('ip') === 'cn'
  ? { address: '220.181.38.148', location: '中国 · 北京', mainland: true }
  : { address: '203.0.113.45', location: '新加坡', mainland: false }


export function upsertSubAccount(account: SubAccount) {
  const exists = subAccounts.value.some(a => a.name === account.name)
  subAccounts.value = exists
    ? subAccounts.value.map(a => a.name === account.name ? account : a)
    : [...subAccounts.value, { ...account, createdAt: tick() }]
}

const at = (iso: string) => Date.parse(`${iso}+08:00`)

// ── 动态流量 ─────────────────────────────────────────────
// 1 GB = 10^9 字节，业务日按北京时间。流量长期有效，用完为止。运营可调价和上下架，客户只看到上架的档位。
export interface TrafficPackage { id: string, gb: number, cents: number, listed: boolean }
export const trafficPackages = persisted<TrafficPackage[]>('traffic-packages', () => [
  { id: 'gb5', gb: 5, cents: 2000, listed: true },
  { id: 'gb50', gb: 50, cents: 18_000, listed: true },
  { id: 'gb200', gb: 200, cents: 64_000, listed: true },
])
// 折扣以 5 GB 档的每 GB 单价为基准，该档下架时仍按它计算。
export const basePerGbCents = computed(() => trafficPackages.value[0].cents / trafficPackages.value[0].gb)

// 两类商品各自的开售开关，由具备计费管理权限的运营开启。
export type SaleProduct = 'dynamic' | 'static'
export const productOnSale = persisted<Record<SaleProduct, boolean>>('product-sale', () => ({ dynamic: true, static: true }))

export interface ProductAudit { at: number, operator: string, target: string, before: string, after: string }
export const productAudit = persisted<ProductAudit[]>('product-audit', () => [
  { at: at('2026-11-01T09:30:00'), operator: '李敏', target: '静态住宅代理 · 开售', before: '暂未开放', after: '开售' },
  { at: at('2026-10-28T16:05:00'), operator: '李敏', target: '动态住宅代理-GB · 开售', before: '暂未开放', after: '开售' },
  { at: at('2026-10-28T15:52:00'), operator: '李敏', target: '200 GB 流量包 · 售价', before: '¥680.00', after: '¥640.00' },
])

export const gatewayHost = 'gw.proxy-demo.com'
export const gatewayPort = 1080

const trafficLines = (pkg: Pick<TrafficPackage, 'gb' | 'cents'>) => [{ label: `动态住宅代理-GB · ${pkg.gb} GB 流量包`, quantity: 1, unitCents: pkg.cents, delivered: 1 }]

export function recordTrafficPurchase(pkg: TrafficPackage) {
  return placeOrder(liveBook(), 'traffic', trafficLines(pkg), tick(), { term: '长期有效', creditedGb: pkg.gb })
}

export const trafficCreditedGb = computed(() => orders.value.reduce((s, o) => s + (o.creditedGb ?? 0), 0))
export const trafficUsedGb = computed(() => subAccounts.value.reduce((s, a) => s + a.usedGb, 0))
export const trafficBalanceGb = computed(() => trafficCreditedGb.value - trafficUsedGb.value)
// 低余额阈值由管理员在「动态住宅代理-GB / 代理设置」中设置，余额降到阈值或以下时提醒。
export const trafficThresholdGb = ref(25)
export const usageUpdatedAt = Date.parse('2026-11-20T08:30:00+08:00')

// 按子账号的每日扣减。确定性伪随机：每个子账号近 30 天的合计取其累计用量的一部分，刷新页面数据不变。
export const usageDays = Array.from({ length: 30 }, (_, i) => new Date(at('2026-11-19T12:00:00') - (29 - i) * dayMs + 8 * 3600_000).toISOString().slice(0, 10))
const shares: Record<string, { share: number, from: number }> = { shop01: { share: .8, from: 12 }, ads02: { share: .75, from: 13 }, crawl: { share: .9, from: 16 }, test9: { share: 1, from: 20 } }
export const usageRows: { day: string, account: string, gb: number }[] = Object.entries(shares).flatMap(([account, { share, from }], k) => {
  const used = subAccounts.value.find(a => a.name === account)!.usedGb * share
  const weights = usageDays.map((_, i) => i < from ? 0 : 0.4 + Math.abs(Math.sin((i + 1) * (k + 2.3) * 1.7)) + (account === 'test9' && i > 23 ? -1 : 0))
    .map(w => Math.max(0, w))
  const sum = weights.reduce((a, b) => a + b, 0)
  return usageDays.map((day, i) => ({ day, account, gb: Math.round(used * weights[i] / sum * 100) / 100 })).filter(r => r.gb > 0)
})

export function dailyTotals(days: readonly string[], account?: string) {
  return days.map(day => ({ day, gb: Math.round(usageRows.filter(r => r.day === day && (!account || r.account === account)).reduce((s, r) => s + r.gb, 0) * 100) / 100 }))
}
export const dailyUsage = dailyTotals(usageDays.slice(-7))

// ── 通知 ─────────────────────────────────────────────
// 低余额与到期提醒各触发一次，已读状态只在本次浏览中保留。
export const accountProfile = { name: '陈小岚', phone: '+86 138 **** 6021' }
export interface Notice { id: string, title: string, detail: string, at: number, to: string }
export const readNotices = ref(new Set<string>())

export const notices = computed<Notice[]>(() => {
  const list: Notice[] = []
  if (trafficBalanceGb.value <= trafficThresholdGb.value) {
    list.push({ id: `traffic-${trafficThresholdGb.value}`, title: '动态流量余额较低', detail: `剩余 ${trafficBalanceGb.value.toFixed(2)} GB，低于 ${trafficThresholdGb.value} GB 提醒阈值。`, at: at('2026-11-20T07:42:00'), to: '#dynamic/pricing' })
  }
  const expiring = ips.value.filter(ip => ip.expiry > demoNow && ip.expiry - demoNow <= 5 * dayMs && !ip.renewing)
  if (expiring.length) {
    list.push({ id: `expiring-${expiring.map(ip => ip.address).join(',')}`, title: '静态 IP 即将到期', detail: `${expiring.length} 个静态 IP 将在 5 天内到期。`, at: Math.min(...expiring.map(ip => ip.expiry)) - 5 * dayMs, to: '#static' })
  }
  return list.sort((a, b) => b.at - a.at)
})
export const unreadCount = computed(() => notices.value.filter(n => !readNotices.value.has(n.id)).length)

export function markRead(ids: string[]) {
  readNotices.value = new Set([...readNotices.value, ...ids])
}

export function relativeDay(time: number) {
  const day = (t: number) => Math.floor((t + 8 * 3600_000) / dayMs)
  const diff = day(demoNow) - day(time)
  return diff <= 0 ? '今天' : diff === 1 ? '昨天' : `${diff} 天前`
}

// ── 租户账本 ─────────────────────────────────────────────
// 客户控制台以 chenxl-studio 登录；其他租户只在运营后台出现。
export const liveTenantId = 't-1001'

function seedBooks(): Record<string, Book> {
  const live: Book = { orders: [], ledger: [] }
  post(live, 'recharge', 50_000, '运营充值', at('2026-11-01T10:12:00'), { operator: '李敏', reason: '对公转账到账，回单尾号 3381' })
  placeOrder(live, 'traffic', trafficLines({ gb: 50, cents: 18_000 }), at('2026-11-02T14:30:00'), { term: '长期有效', creditedGb: 50 })
  placeOrder(live, 'static-purchase', [
    { label: '独享 · 韩国 · 首尔 · 世宗电信', quantity: 1, unitCents: 2300, delivered: 1 },
    { label: '独享 · 日本 · 东京 · DODO K.K.,JP', quantity: 1, unitCents: 2200, delivered: 1 },
    { label: '独享 · 美国 · 洛杉矶 · 双NTT', quantity: 2, unitCents: 800, delivered: 1 },
  ], at('2026-11-05T09:20:00'), { term: '月卡 · 30 天' })
  placeOrder(live, 'static-purchase', [
    { label: '共享 · 德国 · 电信', quantity: 1, unitCents: 2500, delivered: 1 },
    { label: '共享 · 英国 · 电信', quantity: 1, unitCents: 2500, delivered: 1 },
  ], at('2026-11-12T16:05:00'), { term: '月卡 · 30 天' })
  post(live, 'recharge', 20_000, '运营充值', at('2026-11-15T11:40:00'), { operator: '李敏', reason: '微信转账到账' })
  placeOrder(live, 'traffic', trafficLines({ gb: 50, cents: 18_000 }), at('2026-11-18T20:16:00'), { term: '长期有效', creditedGb: 50 })
  placeOrder(live, 'static-renewal', [{ label: '203.0.113.77:1080 · 独享 · 美国 · 洛杉矶', quantity: 1, unitCents: 800, delivered: 0, address: '203.0.113.77' }],
    at('2026-11-19T22:10:00'), { term: '月卡 · 30 天', processing: true })

  const nova: Book = { orders: [], ledger: [] }
  post(nova, 'recharge', 100_000, '运营充值', at('2026-11-08T15:20:00'), { operator: '李敏', reason: '对公转账到账，回单尾号 0917' })
  placeOrder(nova, 'traffic', trafficLines({ gb: 50, cents: 18_000 }), at('2026-11-09T10:02:00'), { term: '长期有效', creditedGb: 50 })
  placeOrder(nova, 'static-purchase', [
    { label: '独享 · 美国 · 洛杉矶 · 双NTT', quantity: 3, unitCents: 800, delivered: 1 },
    { label: '独享 · 日本 · 东京 · DODO K.K.,JP', quantity: 2, unitCents: 2200, delivered: 0 },
  ], at('2026-11-18T20:34:00'), { term: '月卡 · 30 天', processing: true })

  const bluepeak: Book = { orders: [], ledger: [] }
  post(bluepeak, 'recharge', 30_000, '运营充值', at('2026-11-12T09:48:00'), { operator: '李敏', reason: '支付宝转账到账' })
  placeOrder(bluepeak, 'static-purchase', [{ label: '共享 · 德国 · 电信', quantity: 4, unitCents: 2500, delivered: 2 }],
    at('2026-11-16T14:55:00'), { term: '月卡 · 30 天', processing: true })

  const ying: Book = { orders: [], ledger: [] }
  post(ying, 'recharge', 200_000, '运营充值', at('2026-10-30T17:30:00'), { operator: '李敏', reason: '对公转账到账，回单尾号 5520' })
  placeOrder(ying, 'traffic', trafficLines({ gb: 200, cents: 64_000 }), at('2026-10-31T09:12:00'), { term: '长期有效', creditedGb: 200 })
  placeOrder(ying, 'static-purchase', [{ label: '独享 · 韩国 · 首尔 · 世宗电信', quantity: 2, unitCents: 2300, delivered: 2 }],
    at('2026-10-31T09:40:00'), { term: '月卡 · 30 天' })
  placeOrder(ying, 'static-renewal', [
    { label: '198.51.100.21:1080 · 独享 · 韩国 · 首尔', quantity: 1, unitCents: 2300, delivered: 0 },
    { label: '198.51.100.22:1080 · 独享 · 韩国 · 首尔', quantity: 1, unitCents: 2300, delivered: 0 },
  ], at('2026-11-17T08:06:00'), { term: '月卡 · 30 天', processing: true })

  return { [liveTenantId]: live, 't-1002': nova, 't-1003': bluepeak, 't-1004': ying, 't-1005': { orders: [], ledger: [] } }
}

export const books = persisted('books', seedBooks)
const liveBook = () => books.value[liveTenantId]
export const orders = computed(() => liveBook().orders)
export const ledger = computed(() => liveBook().ledger)
export const balanceCents = computed(() => balanceOf(liveBook()))
export const orderById = (id: string) => orders.value.find(order => order.id === id)
