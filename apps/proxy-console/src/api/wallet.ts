import { request } from './client'

export interface WalletEntryWire {
  id: string
  kind: string
  order_id?: string
  before_minor?: string
  after_minor?: string
  created_at: string
  operator_name?: string
  reason?: string
}

type WalletEntryKind = 'recharge' | 'commission_transfer' | 'debit' | 'hold' | 'release' | 'refund' | 'unknown'

function entryKind(value: string): WalletEntryKind {
  switch (value) {
    case 'recharge':
    case 'commission_transfer':
    case 'debit':
    case 'hold':
    case 'release':
    case 'refund':
      return value
    default:
      return 'unknown'
  }
}

export function mapWalletEntry(wire: WalletEntryWire) {
  const afterMinor = BigInt(wire.after_minor ?? 0)
  const beforeMinor = BigInt(wire.before_minor ?? 0)
  const createdAt = new Date(wire.created_at)
  if (!Number.isFinite(createdAt.getTime())) throw new Error('Invalid wallet entry timestamp')
  return {
    id: wire.id,
    orderId: wire.order_id && wire.order_id !== '0' ? wire.order_id : undefined,
    kind: entryKind(wire.kind),
    createdAt,
    beforeMinor,
    afterMinor,
    amountMinor: afterMinor - beforeMinor,
    operatorName: wire.operator_name,
    reason: wire.reason,
  }
}

interface WalletSummaryWire {
  available_minor?: string
  total_recharge_minor?: string
  total_spend_minor?: string
}

export async function getWalletSummary(signal: AbortSignal) {
  const wire = await request<WalletSummaryWire>('/v1/tenant/wallet', { signal })
  return {
    availableMinor: BigInt(wire.available_minor ?? 0),
    totalRechargeMinor: BigInt(wire.total_recharge_minor ?? 0),
    totalSpendMinor: BigInt(wire.total_spend_minor ?? 0),
  }
}

export interface WalletEntriesWire {
  entries?: WalletEntryWire[]
  next_before_id?: string
}

export type WalletFilterKind = 'recharge' | 'commission_transfer' | 'debit' | 'refund'
interface WalletEntriesQuery {
  kind?: WalletFilterKind
  start_date?: string
  end_date?: string
  before_id?: string
}

export async function listWalletEntries(query: WalletEntriesQuery, signal: AbortSignal) {
  const wire = await request<WalletEntriesWire>('/v1/tenant/wallet/ledger', {
    query: { ...query },
    signal,
  })
  return mapWalletEntries(wire)
}

export function mapWalletEntries(wire: WalletEntriesWire) {
  return {
    entries: (wire.entries ?? []).map(mapWalletEntry),
    nextBeforeId: wire.next_before_id === '0' ? undefined : wire.next_before_id,
  }
}
