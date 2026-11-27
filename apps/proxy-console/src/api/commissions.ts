import { request } from './client'

export type ReferralSource = 'manual' | 'link'
export type CommissionProductKind = 'traffic_package' | 'static_purchase' | 'static_renewal'
export type CommissionClassification = 'first_purchase' | 'repeat_purchase'
export type CommissionRecordStatus = 'credited' | 'zero_commission' | 'promotion_paused'

export interface CommissionRecord {
  id: string
  customerKey: string
  referralSource: ReferralSource
  productKind: CommissionProductKind
  settlementSequence: string
  classification: CommissionClassification
  settledAt: Date
  netDebitMinor: bigint
  commissionMinor: bigint
  rateBps: number
  rateVersion: string
  ruleVersion: number
  status: CommissionRecordStatus
}

export interface CommissionListResult {
  availableMinor: bigint
  records: CommissionRecord[]
  nextBeforeId?: string
}

export interface CommissionListQuery {
  customerKey?: string
  beforeId?: string
}

export interface CommissionRates {
  version: string
  firstPurchaseBps: number
  repeatPurchaseBps: number
  operatorId: string
  updatedAt: Date
}

export interface UpdateCommissionRatesInput {
  firstPurchaseBps: number
  repeatPurchaseBps: number
}

const REFERRAL_SOURCES: readonly ReferralSource[] = ['manual', 'link']
const PRODUCT_KINDS: readonly CommissionProductKind[] = ['traffic_package', 'static_purchase', 'static_renewal']
const CLASSIFICATIONS: readonly CommissionClassification[] = ['first_purchase', 'repeat_purchase']
const RECORD_STATUSES: readonly CommissionRecordStatus[] = ['credited', 'zero_commission', 'promotion_paused']
const UUID_PATTERN = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

export function isValidCustomerKey(value: unknown): value is string {
  return typeof value === 'string' && UUID_PATTERN.test(value)
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function decodeEnum<T extends string>(value: unknown, allowed: readonly T[], field: string): T {
  const matched = allowed.find(item => item === value)
  if (matched !== undefined) return matched
  throw new Error(`Invalid commission ${field}`)
}

function decodeNonNegativeBigInt(value: unknown, field: string): bigint {
  if (typeof value === 'string' && /^\d+$/.test(value)) return BigInt(value)
  if (typeof value === 'number' && Number.isSafeInteger(value) && value >= 0) return BigInt(value)
  throw new Error(`Invalid commission ${field}`)
}

function decodePositiveBigInt(value: unknown, field: string): bigint {
  const parsed = decodeNonNegativeBigInt(value, field)
  if (parsed <= 0n) throw new Error(`Invalid commission ${field}`)
  return parsed
}

function decodePositiveIdString(value: unknown, field: string): string {
  if (typeof value === 'string' && /^[1-9]\d*$/.test(value)) return value
  if (typeof value === 'number' && Number.isSafeInteger(value) && value > 0) return String(value)
  throw new Error(`Invalid commission ${field}`)
}

function decodeNextBeforeId(value: unknown): string | undefined {
  if (value === '0' || value === 0) return undefined
  return decodePositiveIdString(value, 'next_before_id')
}

function decodeCustomerKey(value: unknown): string {
  if (isValidCustomerKey(value)) return value
  throw new Error('Invalid commission customer_key')
}

function decodeBps(value: unknown, field: string): number {
  if (typeof value === 'number' && Number.isInteger(value) && value >= 0 && value <= 10_000) return value
  throw new Error(`Invalid commission ${field}`)
}

function decodePositiveInt(value: unknown, field: string): number {
  if (typeof value === 'number' && Number.isInteger(value) && value > 0) return value
  throw new Error(`Invalid commission ${field}`)
}

function decodeDate(value: unknown, field: string): Date {
  if (typeof value !== 'string' || value.trim() === '') throw new Error(`Invalid commission ${field}`)
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) throw new Error(`Invalid commission ${field}`)
  return parsed
}

function decodeRecord(raw: unknown): CommissionRecord {
  if (!isRecord(raw)) throw new Error('Invalid commission record')
  return {
    id: decodePositiveIdString(raw.id, 'id'),
    customerKey: decodeCustomerKey(raw.customer_key),
    referralSource: decodeEnum(raw.referral_source, REFERRAL_SOURCES, 'referral_source'),
    productKind: decodeEnum(raw.product_kind, PRODUCT_KINDS, 'product_kind'),
    settlementSequence: decodePositiveIdString(raw.settlement_sequence, 'settlement_sequence'),
    classification: decodeEnum(raw.classification, CLASSIFICATIONS, 'classification'),
    settledAt: decodeDate(raw.settled_at, 'settled_at'),
    netDebitMinor: decodePositiveBigInt(raw.net_debit_minor, 'net_debit_minor'),
    commissionMinor: decodeNonNegativeBigInt(raw.commission_minor, 'commission_minor'),
    rateBps: decodeBps(raw.rate_bps, 'rate_bps'),
    rateVersion: decodePositiveIdString(raw.rate_version, 'rate_version'),
    ruleVersion: decodePositiveInt(raw.rule_version, 'rule_version'),
    status: decodeEnum(raw.status, RECORD_STATUSES, 'status'),
  }
}

function decodeCommissionList(raw: unknown): CommissionListResult {
  if (!isRecord(raw)) throw new Error('Invalid commission list response')
  if (!Array.isArray(raw.records)) throw new Error('Invalid commission records list')
  return {
    availableMinor: decodeNonNegativeBigInt(raw.available_minor, 'available_minor'),
    records: raw.records.map(decodeRecord),
    nextBeforeId: decodeNextBeforeId(raw.next_before_id),
  }
}

function decodeCommissionRates(raw: unknown): CommissionRates {
  if (!isRecord(raw)) throw new Error('Invalid commission rates response')
  if (typeof raw.operator_id !== 'string') throw new Error('Invalid commission operator_id')
  return {
    version: decodePositiveIdString(raw.version, 'version'),
    firstPurchaseBps: decodeBps(raw.first_purchase_bps, 'first_purchase_bps'),
    repeatPurchaseBps: decodeBps(raw.repeat_purchase_bps, 'repeat_purchase_bps'),
    operatorId: raw.operator_id,
    updatedAt: decodeDate(raw.updated_at, 'updated_at'),
  }
}

function toListQuery(query?: CommissionListQuery) {
  return {
    customer_key: query?.customerKey,
    before_id: query?.beforeId,
  }
}

export async function listTenantCommissions(
  query?: CommissionListQuery,
  signal?: AbortSignal,
): Promise<CommissionListResult> {
  const data = await request<unknown>('/v1/tenant/commissions', {
    query: toListQuery(query),
    signal,
  })
  return decodeCommissionList(data)
}

export async function listOpsTenantCommissions(
  tenantId: string,
  query?: CommissionListQuery,
  signal?: AbortSignal,
): Promise<CommissionListResult> {
  const data = await request<unknown>(`/v1/operations/tenants/${encodeURIComponent(tenantId)}/commissions`, {
    query: toListQuery(query),
    signal,
  })
  return decodeCommissionList(data)
}

export async function getOpsCommissionRates(signal?: AbortSignal): Promise<CommissionRates> {
  const data = await request<unknown>('/v1/operations/commission-rates', { signal })
  return decodeCommissionRates(data)
}

export async function updateOpsCommissionRates(
  input: UpdateCommissionRatesInput,
  signal?: AbortSignal,
): Promise<CommissionRates> {
  const data = await request<unknown>('/v1/operations/commission-rates', {
    method: 'PUT',
    body: {
      first_purchase_bps: input.firstPurchaseBps,
      repeat_purchase_bps: input.repeatPurchaseBps,
    },
    signal,
  })
  return decodeCommissionRates(data)
}
