import { computed, ref } from 'vue'
import { demoNow } from './demo'
import { balanceOf, books, liveTenantId, trafficBalanceGb, type Order } from './state'

// ── 运营账号与权限 ─────────────────────────────────────────
// 原型用顶栏切换角色。正式版由 CheckPermission 判定，没有计费管理权限时隐藏结果、禁用操作并说明原因。
export type OpsRole = 'billing' | 'support'
export const opsRoles: Record<OpsRole, { name: string, title: string }> = {
  billing: { name: '李敏', title: '计费管理' },
  support: { name: '赵晨', title: '客服运营' },
}
export const role = ref<OpsRole>(sessionStorage.getItem('ops-role') === 'support' ? 'support' : 'billing')
export const operator = computed(() => opsRoles[role.value])
export const canBill = computed(() => role.value === 'billing')
export const billingOnly = '需要计费管理权限'
export function setRole(next: OpsRole) {
  role.value = next
  sessionStorage.setItem('ops-role', next)
}

// ── 租户 ─────────────────────────────────────────────
export interface Tenant { id: string, name: string, account: string, phone: string, registeredAt: number, usedGb: number }
const at = (iso: string) => Date.parse(`${iso}+08:00`)

export const tenants: Tenant[] = [
  { id: liveTenantId, name: '陈小岚工作室', account: 'chenxl-studio', phone: '+86 138 0013 6021', registeredAt: at('2026-10-31T21:14:00'), usedGb: 0 },
  { id: 't-1002', name: '诺瓦跨境', account: 'nova-trade', phone: '+86 139 1021 4410', registeredAt: at('2026-11-08T14:02:00'), usedGb: 37.6 },
  { id: 't-1003', name: '蓝峰数据', account: 'bluepeak', phone: '+86 136 2208 7713', registeredAt: at('2026-11-11T19:40:00'), usedGb: 0 },
  { id: 't-1004', name: '映像科技', account: 'ying-data', phone: '+86 158 6650 3092', registeredAt: at('2026-10-29T10:25:00'), usedGb: 16.8 },
  { id: 't-1005', name: '林一', account: 'lin-test', phone: '+86 177 0031 5528', registeredAt: at('2026-11-19T23:51:00'), usedGb: 0 },
]
export const tenantById = (id: string) => tenants.find(t => t.id === id)!

// 与 FindTenants 一致：精确匹配主账号名，或手机号（可省略 +86 和空格）。
export function findTenant(query: string) {
  const q = query.trim().toLowerCase()
  const digits = q.replace(/[\s-]/g, '').replace(/^\+?86/, '')
  return tenants.find(t => t.account === q || (/^1\d{10}$/.test(digits) && t.phone.replace(/\D/g, '').slice(2) === digits))
}

export const tenantBalance = (id: string) => balanceOf(books.value[id])
export function tenantTrafficGb(id: string) {
  if (id === liveTenantId) return trafficBalanceGb.value
  const credited = books.value[id].orders.reduce((s, o) => s + (o.creditedGb ?? 0), 0)
  return credited - tenantById(id).usedGb
}

// ── 全部订单与流水（运营视图） ───────────────────────────────
export const allOrders = computed(() => tenants.flatMap(t => books.value[t.id].orders.map(order => ({ ...order, tenantId: t.id }))).sort((a, b) => b.createdAt - a.createdAt))
export const allEntries = computed(() => tenants.flatMap(t => books.value[t.id].ledger.map(entry => ({ ...entry, tenantId: t.id }))).sort((a, b) => b.at - a.at))

// ── 待处理的静态订单 ─────────────────────────────────────────
// 下单满 24 小时无结果转运营；满 72 小时可关闭剩余并退款。72 小时前只有明确失败证据才能关闭。
export type PendingStage = 'waiting' | 'review' | 'closable'
export const pendingStageLabels: Record<PendingStage, string> = { waiting: '供应商确认中', review: '待运营核实', closable: '可关闭退款' }
export const hoursSince = (time: number) => Math.floor((demoNow - time) / 3_600_000)
export const stageOf = (order: Order): PendingStage => hoursSince(order.createdAt) >= 72 ? 'closable' : hoursSince(order.createdAt) >= 24 ? 'review' : 'waiting'
export const pendingOrders = computed(() => allOrders.value.filter(o => o.status === 'processing').sort((a, b) => a.createdAt - b.createdAt))
export const needsAction = computed(() => pendingOrders.value.filter(o => stageOf(o) !== 'waiting'))

// ── 用量异常（只读） ─────────────────────────────────────────
// 字段对应 ListRecordAnomalies / ListUsageGaps。kind 的取值由 Recorder 定义，原型使用示例值。
export interface RecordAnomaly { id: string, gateway: string, boot: string, tenantId: string, kind: string, file: string, offset: number, detectedAt: number, resolvedAt?: number }
export interface UsageGap { id: string, gateway: string, boot: string, tenantId: string, kind: 'boot' | 'session', status: 'pending_replay' | 'possible_gap', session?: string, fromAt: number, detectedAt: number, exitedAt?: number, resolvedAt?: number }

export const anomalies: RecordAnomaly[] = [
  { id: 'an-7f21', gateway: 'gw-sg-01', boot: 'b-20261119-a3', tenantId: 't-1004', kind: 'decode_error', file: 'rec-000412', offset: 1_048_576, detectedAt: at('2026-11-20T03:12:00') },
  { id: 'an-7e90', gateway: 'gw-sg-02', boot: 'b-20261118-c1', tenantId: liveTenantId, kind: 'checksum_mismatch', file: 'rec-000388', offset: 524_288, detectedAt: at('2026-11-19T17:46:00') },
  { id: 'an-7c04', gateway: 'gw-sg-01', boot: 'b-20261117-f2', tenantId: 't-1002', kind: 'truncated_record', file: 'rec-000351', offset: 2_097_152, detectedAt: at('2026-11-18T08:20:00'), resolvedAt: at('2026-11-18T09:05:00') },
]

export const usageGaps: UsageGap[] = [
  { id: '', gateway: 'gw-sg-02', boot: 'b-20261120-d7', tenantId: 't-1004', kind: 'boot', status: 'pending_replay', fromAt: at('2026-11-20T06:40:00'), detectedAt: at('2026-11-20T06:42:00'), exitedAt: at('2026-11-20T06:39:00') },
  { id: 'gap-31a8', gateway: 'gw-sg-01', boot: 'b-20261119-a3', tenantId: liveTenantId, kind: 'session', status: 'possible_gap', session: 's-8c1e44f0', fromAt: at('2026-11-19T22:58:00'), detectedAt: at('2026-11-20T01:10:00') },
  { id: 'gap-2f77', gateway: 'gw-sg-01', boot: 'b-20261117-f2', tenantId: 't-1002', kind: 'session', status: 'pending_replay', session: 's-71d0a9b2', fromAt: at('2026-11-17T13:05:00'), detectedAt: at('2026-11-17T13:07:00'), resolvedAt: at('2026-11-17T13:31:00') },
]
export const openUsageIssues = anomalies.filter(a => !a.resolvedAt).length + usageGaps.filter(g => !g.resolvedAt).length
