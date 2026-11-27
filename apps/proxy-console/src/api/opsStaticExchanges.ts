import { request } from './client'
import { opsTenantPath } from './opsTenants'

interface StaticIpExchangeCoreWire {
  allocation_id?: string
  node_id?: string
  idempotency_key?: string
  status?: string
  created_at?: string
  updated_at?: string
}

interface OperatorStaticIpExchangeWire {
  exchange?: StaticIpExchangeCoreWire
  tenant_id?: string
  supplier_order_id?: string
  supplier_item_id?: string
  old_ip?: string
  new_ip?: string
  unknown_at?: string
  in_queue?: boolean
  resolution?: string
  resolution_conflict?: boolean
  corrected?: boolean
}

interface OperatorStaticIpExchangesWire {
  exchanges?: OperatorStaticIpExchangeWire[]
  next_cursor?: string
}

interface OperatorStaticIpExchangeQuotaWire {
  total?: number
  remaining?: number
  reserved?: number
  used?: number
}

interface OperatorStaticIpExchangeAuditEntryWire {
  id?: string
  kind?: string
  idempotency_key?: string
  request_key?: string
  actor_id?: string
  evidence?: string
  outcome?: string
  supplier_record_id?: string
  observed_ip?: string
  before?: OperatorStaticIpExchangeQuotaWire
  after?: OperatorStaticIpExchangeQuotaWire
  created_at?: string
}

interface OperatorStaticIpExchangeAuditPageWire {
  entries?: OperatorStaticIpExchangeAuditEntryWire[]
  next_before_id?: string
}

export interface OperatorStaticIpExchangeQuota {
  total: number
  remaining: number
  reserved: number
  used: number
}

export interface OperatorStaticIpExchange {
  allocationId: string
  nodeId: string
  idempotencyKey: string
  status: string
  createdAt?: Date
  updatedAt?: Date
  tenantId: string
  supplierOrderId: string
  supplierItemId: string
  oldIp: string
  newIp: string
  unknownAt?: Date
  inQueue: boolean
  resolution: string
  resolutionConflict: boolean
  corrected: boolean
}

export interface OperatorStaticIpExchangesPage {
  exchanges: OperatorStaticIpExchange[]
  nextCursor?: string
}

export interface OperatorStaticIpExchangeAuditEntry {
  id: string
  kind: string
  idempotencyKey: string
  requestKey: string
  actorId: string
  evidence: string
  outcome: string
  supplierRecordId: string
  observedIp: string
  before?: OperatorStaticIpExchangeQuota
  after?: OperatorStaticIpExchangeQuota
  createdAt?: Date
}

export interface OperatorStaticIpExchangeAuditPage {
  entries: OperatorStaticIpExchangeAuditEntry[]
  nextBeforeId?: string
}

export type StaticIpExchangeResolutionOutcome = 'executed' | 'not_executed'

export interface AdjustStaticIpExchangeQuotaPayload {
  delta: number
  request_key: string
  evidence: string
}

export interface ResolveStaticIpExchangePayload {
  request_key: string
  outcome: StaticIpExchangeResolutionOutcome
  evidence: string
  expected_ip?: string
  correction?: boolean
}

function parseOptionalDate(value?: string): Date | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? undefined : date
}

function mapQuota(wire?: OperatorStaticIpExchangeQuotaWire): OperatorStaticIpExchangeQuota {
  return {
    total: wire?.total ?? 0,
    remaining: wire?.remaining ?? 0,
    reserved: wire?.reserved ?? 0,
    used: wire?.used ?? 0,
  }
}

function mapExchange(wire: OperatorStaticIpExchangeWire): OperatorStaticIpExchange {
  return {
    allocationId: wire.exchange?.allocation_id ?? '',
    nodeId: wire.exchange?.node_id ?? '',
    idempotencyKey: wire.exchange?.idempotency_key ?? '',
    status: wire.exchange?.status ?? '',
    createdAt: parseOptionalDate(wire.exchange?.created_at),
    updatedAt: parseOptionalDate(wire.exchange?.updated_at),
    tenantId: wire.tenant_id ?? '',
    supplierOrderId: wire.supplier_order_id ?? '',
    supplierItemId: wire.supplier_item_id ?? '',
    oldIp: wire.old_ip ?? '',
    newIp: wire.new_ip ?? '',
    unknownAt: parseOptionalDate(wire.unknown_at),
    inQueue: Boolean(wire.in_queue),
    resolution: wire.resolution ?? '',
    resolutionConflict: Boolean(wire.resolution_conflict),
    corrected: Boolean(wire.corrected),
  }
}

function mapAuditEntry(wire: OperatorStaticIpExchangeAuditEntryWire): OperatorStaticIpExchangeAuditEntry {
  return {
    id: wire.id ?? '',
    kind: wire.kind ?? '',
    idempotencyKey: wire.idempotency_key ?? '',
    requestKey: wire.request_key ?? '',
    actorId: wire.actor_id ?? '',
    evidence: wire.evidence ?? '',
    outcome: wire.outcome ?? '',
    supplierRecordId: wire.supplier_record_id ?? '',
    observedIp: wire.observed_ip ?? '',
    before: wire.before !== undefined ? mapQuota(wire.before) : undefined,
    after: wire.after !== undefined ? mapQuota(wire.after) : undefined,
    createdAt: parseOptionalDate(wire.created_at),
  }
}

export function adjustStaticIpExchangeQuotaPath(tenantId: string) {
  return `${opsTenantPath(tenantId)}/static-ip-exchange-quota:adjust`
}

export function resolveStaticIpExchangePath(tenantId: string, idempotencyKey: string) {
  return `${opsTenantPath(tenantId)}/static-ip-exchanges/${encodeURIComponent(idempotencyKey)}/resolution`
}

export async function listOpsStaticIpExchangeQueue(
  options: { tenantId?: string, cursor?: string } = {},
  signal?: AbortSignal,
): Promise<OperatorStaticIpExchangesPage> {
  const wire = await request<OperatorStaticIpExchangesWire>('/v1/operations/static-ip-exchange-queue', {
    query: { tenant_id: options.tenantId, cursor: options.cursor },
    signal,
  })
  return {
    exchanges: (wire?.exchanges ?? []).map(mapExchange),
    nextCursor: wire?.next_cursor || undefined,
  }
}

export async function listOpsTenantStaticIpExchanges(
  tenantId: string,
  options: { cursor?: string } = {},
  signal?: AbortSignal,
): Promise<OperatorStaticIpExchangesPage> {
  const wire = await request<OperatorStaticIpExchangesWire>(`${opsTenantPath(tenantId)}/static-ip-exchanges`, {
    query: { cursor: options.cursor },
    signal,
  })
  return {
    exchanges: (wire?.exchanges ?? []).map(mapExchange),
    nextCursor: wire?.next_cursor || undefined,
  }
}

export async function getOpsStaticIpExchangeQuota(
  tenantId: string,
  signal?: AbortSignal,
): Promise<OperatorStaticIpExchangeQuota> {
  const wire = await request<OperatorStaticIpExchangeQuotaWire>(`${opsTenantPath(tenantId)}/static-ip-exchange-quota`, { signal })
  return mapQuota(wire)
}

export async function listOpsStaticIpExchangeAudit(
  tenantId: string,
  options: { beforeId?: string } = {},
  signal?: AbortSignal,
): Promise<OperatorStaticIpExchangeAuditPage> {
  const wire = await request<OperatorStaticIpExchangeAuditPageWire>(`${opsTenantPath(tenantId)}/static-ip-exchange-audit`, {
    query: { before_id: options.beforeId },
    signal,
  })
  return {
    entries: (wire?.entries ?? []).map(mapAuditEntry),
    nextBeforeId: wire?.next_before_id || undefined,
  }
}

export async function adjustOpsStaticIpExchangeQuota(
  path: string,
  payload: AdjustStaticIpExchangeQuotaPayload,
): Promise<OperatorStaticIpExchangeAuditEntry> {
  const wire = await request<OperatorStaticIpExchangeAuditEntryWire>(path, {
    method: 'POST',
    body: payload,
  })
  return mapAuditEntry(wire ?? {})
}

export async function resolveOpsStaticIpExchange(
  path: string,
  payload: ResolveStaticIpExchangePayload,
): Promise<OperatorStaticIpExchange> {
  const wire = await request<OperatorStaticIpExchangeWire>(path, {
    method: 'POST',
    body: payload,
  })
  return mapExchange(wire ?? {})
}
