import type { RenewalSubmission, StaticIp } from '@/api/staticIps'

export type RenewalIntent =
  | { kind: 'renew', ips: StaticIp[] }
  | { kind: 'order', id: string }
  | { kind: 'resume', submission: RenewalSubmission }

export function readRenewalSubmission(key: string): RenewalSubmission | undefined {
  try {
    const value: unknown = JSON.parse(sessionStorage.getItem(key) ?? 'null')
    if (typeof value !== 'object' || value === null) return
    if (!('supplier_item_ids' in value) || !isStringList(value.supplier_item_ids)) return
    if (!('term' in value) || typeof value.term !== 'string') return
    if (!('request_key' in value) || typeof value.request_key !== 'string') return
    if (!('confirmation_token' in value) || typeof value.confirmation_token !== 'string') return
    return {
      supplier_item_ids: value.supplier_item_ids,
      term: value.term,
      request_key: value.request_key,
      confirmation_token: value.confirmation_token,
    }
  } catch {
    return undefined
  }
}

function isStringList(value: unknown): value is string[] {
  return Array.isArray(value) && value.length > 0 && value.every(item => typeof item === 'string')
}
