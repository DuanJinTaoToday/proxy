import { request } from './client'

export const resolutions = ['open', 'resolved', 'all'] as const
export const gapStatuses = ['pending_replay', 'possible_gap'] as const
export type Resolution = (typeof resolutions)[number]
export type GapStatus = (typeof gapStatuses)[number]
export type FindingsTab = 'gaps' | 'anomalies'

interface FindingWire {
  gateway_id: string
  boot_id: string
  tenant_id?: string
  tenant_name?: string
  detected_at: string
  resolved_at?: string
}

interface GapWire extends FindingWire {
  kind: 'boot' | 'session'
  status?: GapStatus
  session_id?: string
  from_at: string
}

interface AnomalyWire extends FindingWire {
  kind: string
  file_id: string
  file_offset?: string
}

interface FindingsPageWire {
  next_page_token?: string
  open_count?: string
}

interface FindingsFilters {
  gatewayId?: string
  bootId?: string
  tenantId?: string
  kind?: string
  resolution: Resolution
  status?: GapStatus
}

function mapFinding(row: FindingWire) {
  return {
    gatewayId: row.gateway_id,
    bootId: row.boot_id,
    tenantId: row.tenant_id,
    tenantName: row.tenant_name,
    detectedAt: new Date(row.detected_at),
    resolvedAt: row.resolved_at ? new Date(row.resolved_at) : undefined,
  }
}

function mapGap(row: GapWire) {
  return {
    ...mapFinding(row),
    type: 'gap' as const,
    kind: row.kind,
    status: row.status,
    sessionId: row.session_id,
    fromAt: new Date(row.from_at),
  }
}

function mapAnomaly(row: AnomalyWire) {
  return {
    ...mapFinding(row),
    type: 'anomaly' as const,
    kind: row.kind,
    fileId: row.file_id,
    offset: BigInt(row.file_offset ?? 0),
  }
}
export type Finding = ReturnType<typeof mapGap> | ReturnType<typeof mapAnomaly>

export async function listOpsUsageFindings(tab: FindingsTab, filters: FindingsFilters, cursor: string | undefined, signal: AbortSignal, pageSize = 20) {
  const query = {
    gateway_id: filters.gatewayId,
    boot_id: filters.bootId,
    tenant_id: filters.tenantId,
    kind: filters.kind,
    resolution: filters.resolution,
    status: tab === 'gaps' ? filters.status : undefined,
    page_size: pageSize,
    page_token: cursor,
  }
  if (tab === 'gaps') {
    const wire = await request<FindingsPageWire & { gaps?: GapWire[] }>('/v1/operations/gaps', { query, signal })
    return { rows: (wire.gaps ?? []).map(mapGap), nextBeforeId: wire.next_page_token, openCount: BigInt(wire.open_count ?? 0) }
  }
  const wire = await request<FindingsPageWire & { anomalies?: AnomalyWire[] }>('/v1/operations/anomalies', { query, signal })
  return { rows: (wire.anomalies ?? []).map(mapAnomaly), nextBeforeId: wire.next_page_token, openCount: BigInt(wire.open_count ?? 0) }
}
