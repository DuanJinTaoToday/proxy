import type { RenewalSubmission, StaticIp } from '@/api/staticIps'

export type RenewalIntent =
  | { kind: 'renew', ips: StaticIp[] }
  | { kind: 'order', id: string }
  | { kind: 'resume', submission: RenewalSubmission }

// Persisted requests from before platform allocation IDs carry supplier IDs and must not be re-sent.
const allocationIdPattern = /^sa_[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

export function readRenewalSubmissions(key: string): RenewalSubmission[] {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(key) ?? 'null')
    const candidates = Array.isArray(value) ? value : [value]
    return candidates.filter(isRenewalSubmission)
  } catch {
    return []
  }
}

function isRenewalSubmission(value: unknown): value is RenewalSubmission {
  return typeof value === 'object' && value !== null
    && 'allocation_ids' in value && Array.isArray(value.allocation_ids)
    && value.allocation_ids.length > 0 && value.allocation_ids.length <= 25
    && value.allocation_ids.every(item => typeof item === 'string' && allocationIdPattern.test(item))
    && 'term' in value && typeof value.term === 'string' && value.term.length > 0
    && 'request_key' in value && typeof value.request_key === 'string' && value.request_key.length > 0
    && 'confirmation_token' in value && typeof value.confirmation_token === 'string' && value.confirmation_token.length > 0
}
