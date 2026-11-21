import type { RenewalSubmission, StaticIp } from '@/api/staticIps'

export type RenewalIntent =
  | { kind: 'renew', ips: StaticIp[] }
  | { kind: 'order', id: string }
  | { kind: 'resume', submission: RenewalSubmission }

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
    && 'supplier_item_ids' in value && Array.isArray(value.supplier_item_ids)
    && value.supplier_item_ids.length > 0 && value.supplier_item_ids.length <= 25
    && value.supplier_item_ids.every(item => typeof item === 'string' && /^[1-9]\d*$/.test(item))
    && 'term' in value && typeof value.term === 'string' && value.term.length > 0
    && 'request_key' in value && typeof value.request_key === 'string' && value.request_key.length > 0
    && 'confirmation_token' in value && typeof value.confirmation_token === 'string' && value.confirmation_token.length > 0
}
