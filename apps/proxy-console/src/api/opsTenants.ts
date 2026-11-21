import { request } from './client'
import { mapWalletEntry, type WalletEntryWire } from './wallet'

interface OpsTenantWire {
  tenant_id: string
  name: string
  primary_account_name: string
  phone?: string
  registered_at?: string
  available_minor?: string
  available_traffic_bytes?: string
}

function mapTenant(wire: OpsTenantWire) {
  return {
    id: wire.tenant_id,
    name: wire.name,
    account: wire.primary_account_name,
    phone: wire.phone,
    registeredAt: wire.registered_at ? new Date(wire.registered_at) : undefined,
    availableMinor: BigInt(wire.available_minor ?? 0),
    availableTrafficBytes: BigInt(wire.available_traffic_bytes ?? 0),
  }
}

export type OpsTenant = ReturnType<typeof mapTenant>
export const opsTenantPath = (id: string) => '/v1/operations/tenants/' + encodeURIComponent(id)

export async function findOpsTenant(query: string, signal: AbortSignal) {
  return mapTenant(await request<OpsTenantWire>('/v1/operations/tenants', { query: { query }, signal }))
}

export async function getOpsTenant(id: string, signal: AbortSignal) {
  return mapTenant(await request<OpsTenantWire>(opsTenantPath(id), { signal }))
}

export async function quoteWalletRecharge(tenant: string, { key, reason, amountMinor }: { key: string, reason: string, amountMinor: bigint }) {
  const quote = await request<{
    tenant: OpsTenantWire
    amount_minor: string
    expected_after_minor: string
    confirmation_token: string
    reason: string
  }>(opsTenantPath(tenant) + '/wallet/recharge-quotes', {
    body: { request_key: key, reason, amount_minor: amountMinor.toString() },
  })
  return {
    tenant: mapTenant(quote.tenant),
    amountMinor: BigInt(quote.amount_minor),
    expectedAfterMinor: BigInt(quote.expected_after_minor),
    token: quote.confirmation_token,
    reason: quote.reason,
    key,
  }
}

export async function rechargeWallet(tenant: string, { key, token }: { key: string, token: string }) {
  const wire = await request<WalletEntryWire & { operator_name: string }>(opsTenantPath(tenant) + '/wallet/recharges', {
    body: { request_key: key, confirmation_token: token },
  })
  return { ...mapWalletEntry(wire), operatorName: wire.operator_name }
}
