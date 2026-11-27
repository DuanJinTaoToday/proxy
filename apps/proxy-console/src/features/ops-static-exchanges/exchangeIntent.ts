import { ApiError } from '@/api/client'
import type {
  AdjustStaticIpExchangeQuotaPayload,
  ResolveStaticIpExchangePayload,
} from '@/api/opsStaticExchanges'

export type ExchangeIntentAction = 'adjust_quota' | 'resolve_exchange' | 'correct_exchange'

export interface IntentErrorSnapshot {
  status: number
  reason: string
  message: string
}

interface PendingExchangeIntentBase {
  operator: string
  tenantId: string
  action: ExchangeIntentAction
  path: string
  requestKey: string
  createdAt: string
  lastError?: IntentErrorSnapshot
}

export interface PendingAdjustQuotaIntent extends PendingExchangeIntentBase {
  action: 'adjust_quota'
  payload: AdjustStaticIpExchangeQuotaPayload
}

export interface PendingResolveExchangeIntent extends PendingExchangeIntentBase {
  action: 'resolve_exchange' | 'correct_exchange'
  idempotencyKey: string
  allocationId?: string
  payload: ResolveStaticIpExchangePayload
}

export type PendingExchangeIntent = PendingAdjustQuotaIntent | PendingResolveExchangeIntent

const storagePrefix = 'proxy-console.ops-static-exchange-intents.v1:'

function storageKey(operator: string) {
  return `${storagePrefix}${operator}`
}

export function exchangeIntentSlot(
  action: ExchangeIntentAction,
  tenantId: string,
  idempotencyKey?: string,
) {
  return action === 'adjust_quota'
    ? `${action}:${tenantId}`
    : `${action}:${tenantId}:${idempotencyKey ?? ''}`
}

export function generateRequestKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  const bytes = new Uint8Array(16)
  if (typeof crypto !== 'undefined' && typeof crypto.getRandomValues === 'function') {
    crypto.getRandomValues(bytes)
  } else {
    for (let i = 0; i < bytes.length; i += 1) bytes[i] = Math.floor(Math.random() * 256)
  }
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('')
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`
}

export function isDefinitiveClientRejection(error: unknown): boolean {
  return error instanceof ApiError && error.status >= 400 && error.status < 500
}

export function toIntentErrorSnapshot(error: unknown): IntentErrorSnapshot {
  if (error instanceof ApiError) {
    return { status: error.status, reason: error.reason, message: error.message }
  }
  return {
    status: 0,
    reason: 'NETWORK',
    message: error instanceof Error ? error.message : 'unknown error',
  }
}

function readAllIntents(operator: string): Record<string, PendingExchangeIntent> {
  if (!operator || typeof window === 'undefined') return {}
  try {
    const raw = window.sessionStorage.getItem(storageKey(operator))
    if (!raw) return {}
    const parsed = JSON.parse(raw) as unknown
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    const result: Record<string, PendingExchangeIntent> = {}
    for (const [slot, item] of Object.entries(parsed as Record<string, unknown>)) {
      if (!item || typeof item !== 'object') continue
      const candidate = item as Partial<PendingExchangeIntent>
      if (
        candidate.operator === operator
        && typeof candidate.tenantId === 'string'
        && typeof candidate.path === 'string'
        && typeof candidate.requestKey === 'string'
        && typeof candidate.action === 'string'
        && candidate.payload
        && typeof candidate.payload === 'object'
      ) {
        result[slot] = candidate as PendingExchangeIntent
      }
    }
    return result
  } catch {
    return {}
  }
}

function writeAllIntents(operator: string, intents: Record<string, PendingExchangeIntent>) {
  if (!operator || typeof window === 'undefined') return
  try {
    if (Object.keys(intents).length === 0) {
      window.sessionStorage.removeItem(storageKey(operator))
    } else {
      window.sessionStorage.setItem(storageKey(operator), JSON.stringify(intents))
    }
  } catch {
    // sessionStorage unavailable or quota exceeded; in-memory state still tracks the current attempt.
  }
}

export function loadPendingIntent(
  operator: string,
  action: ExchangeIntentAction,
  tenantId: string,
  idempotencyKey?: string,
): PendingExchangeIntent | undefined {
  if (!operator || !tenantId) return undefined
  const slot = exchangeIntentSlot(action, tenantId, idempotencyKey)
  const intent = readAllIntents(operator)[slot]
  if (!intent || intent.operator !== operator || intent.tenantId !== tenantId) return undefined
  return intent
}

export function listTenantPendingIntents(operator: string, tenantId: string): PendingExchangeIntent[] {
  if (!operator || !tenantId) return []
  return Object.values(readAllIntents(operator)).filter(
    item => item.operator === operator && item.tenantId === tenantId,
  )
}

export function savePendingIntent(intent: PendingExchangeIntent) {
  if (!intent.operator || !intent.tenantId) return
  const all = readAllIntents(intent.operator)
  const idempotencyKey = intent.action === 'adjust_quota' ? undefined : intent.idempotencyKey
  const slot = exchangeIntentSlot(intent.action, intent.tenantId, idempotencyKey)
  all[slot] = intent
  writeAllIntents(intent.operator, all)
}

export function clearPendingIntent(
  operator: string,
  action: ExchangeIntentAction,
  tenantId: string,
  idempotencyKey?: string,
) {
  if (!operator || !tenantId) return
  const all = readAllIntents(operator)
  const slot = exchangeIntentSlot(action, tenantId, idempotencyKey)
  if (!(slot in all)) return
  delete all[slot]
  writeAllIntents(operator, all)
}
