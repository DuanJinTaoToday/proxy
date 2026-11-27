import type { StaticIp } from '@/api/staticIps'
import type { StaticIpExchangeSubmission } from '@/api/staticIpExchanges'

export type ExchangeIntent =
  | { kind: 'exchange', ip: StaticIp }
  | { kind: 'inspect', ip: StaticIp, idempotencyKey: string }
  | { kind: 'resume', submission: StaticIpExchangeSubmission, ip?: StaticIp }

export function exchangeStorageKey(accountKey: string) {
  return `proxy-console.static-exchange.${accountKey}`
}

export function readExchangeSubmissions(key: string): StaticIpExchangeSubmission[] {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(key) ?? 'null')
    const candidates = Array.isArray(value) ? value : [value]
    const seen = new Set<string>()
    return candidates.filter((item): item is StaticIpExchangeSubmission => {
      if (!isExchangeSubmission(item) || seen.has(item.idempotency_key)) return false
      seen.add(item.idempotency_key)
      return true
    })
  } catch {
    return []
  }
}

export function writeExchangeSubmissions(key: string, submissions: StaticIpExchangeSubmission[]) {
  try {
    if (submissions.length) sessionStorage.setItem(key, JSON.stringify(submissions))
    else sessionStorage.removeItem(key)
  } catch {
    // Vue state still supports lookup and retry if the browser disables session storage.
  }
}

function isExchangeSubmission(value: unknown): value is StaticIpExchangeSubmission {
  return typeof value === 'object' && value !== null
    && 'allocation_id' in value && typeof value.allocation_id === 'string' && value.allocation_id.length > 0
    && 'node_id' in value && typeof value.node_id === 'string' && value.node_id.length > 0
    && 'idempotency_key' in value && typeof value.idempotency_key === 'string' && value.idempotency_key.length > 0
}
