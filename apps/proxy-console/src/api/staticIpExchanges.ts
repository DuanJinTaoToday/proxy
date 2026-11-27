import { request } from './client'

const exchangeStatuses = ['pending', 'unknown', 'failed', 'activating', 'ready', 'support_required'] as const
export type StaticIpExchangeStatus = (typeof exchangeStatuses)[number]

export function isStaticIpExchangeStatus(value: unknown): value is StaticIpExchangeStatus {
  return typeof value === 'string' && (exchangeStatuses as readonly string[]).includes(value)
}

interface StaticIpExchangeQuotaWire {
  remaining?: number | string
  reserved?: number | string
  used?: number | string
}

export interface StaticIpExchangeQuota {
  remaining: number
  reserved: number
  used: number
}

export interface StaticIpExchangeSubmission {
  allocation_id: string
  node_id: string
  idempotency_key: string
}

interface StaticIpExchangeWire {
  allocation_id: string
  node_id: string
  idempotency_key: string
  status: StaticIpExchangeStatus
  created_at?: string
  updated_at?: string
}

export interface StaticIpExchangeRecord {
  allocationId: string
  nodeId: string
  idempotencyKey: string
  status: StaticIpExchangeStatus
  createdAt?: Date
  updatedAt?: Date
}

const quotaPath = '/v1/tenant/static-ip-exchange-quota'
const exchangesPath = '/v1/tenant/static-ip-exchanges'

function toOptionalDate(value: string | undefined): Date | undefined {
  if (!value) return undefined
  const date = new Date(value)
  return Number.isFinite(date.getTime()) ? date : undefined
}

function toExchangeRecord(wire: StaticIpExchangeWire): StaticIpExchangeRecord {
  return {
    allocationId: wire.allocation_id,
    nodeId: wire.node_id,
    idempotencyKey: wire.idempotency_key,
    status: wire.status,
    createdAt: toOptionalDate(wire.created_at),
    updatedAt: toOptionalDate(wire.updated_at),
  }
}

export async function getStaticIpExchangeQuota(signal: AbortSignal): Promise<StaticIpExchangeQuota> {
  const wire = await request<StaticIpExchangeQuotaWire>(quotaPath, { signal })
  return {
    remaining: Number(wire.remaining ?? 0),
    reserved: Number(wire.reserved ?? 0),
    used: Number(wire.used ?? 0),
  }
}

export async function submitStaticIpExchange(
  body: StaticIpExchangeSubmission,
  signal: AbortSignal,
): Promise<StaticIpExchangeRecord> {
  return toExchangeRecord(await request<StaticIpExchangeWire>(exchangesPath, { body, signal }))
}

export async function getStaticIpExchange(
  idempotencyKey: string,
  signal: AbortSignal,
): Promise<StaticIpExchangeRecord> {
  return toExchangeRecord(
    await request<StaticIpExchangeWire>(`${exchangesPath}/${encodeURIComponent(idempotencyKey)}`, { signal }),
  )
}
