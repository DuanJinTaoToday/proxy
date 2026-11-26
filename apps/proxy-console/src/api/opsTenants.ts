import { request } from './client'
import { mapOrderDetail, mapOrders, type OrderDetailWire, type OrderKind, type OrderStatus, type OrdersWire } from './orders'
import { mapWalletEntries, mapWalletEntry, type WalletEntriesWire, type WalletEntryWire, type WalletFilterKind } from './wallet'

interface OpsTenantWire {
  tenant_id: string
  name: string
  primary_account_name: string
  email?: string
  registered_at?: string
  available_minor?: string
  available_traffic_bytes?: string
}

function mapTenant(wire: OpsTenantWire) {
  return {
    id: wire.tenant_id,
    name: wire.name,
    account: wire.primary_account_name,
    email: wire.email,
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

export async function resetOpsTenantPassword({ tenantId, password }: { tenantId: string, password: string }) {
  await request<unknown>(opsTenantPath(tenantId) + ':reset-password', { body: { password } })
}

// 向指定邮箱发送账号恢复链接，客户通过链接设置新密码后改用该邮箱登录。
export async function sendOpsTenantRecoveryEmail({ tenantId, email }: { tenantId: string, email: string }) {
  await request<unknown>(opsTenantPath(tenantId) + ':recover-email', { body: { email } })
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

export async function listOpsTenantOrders(
  tenantId: string,
  filters: { kind?: OrderKind, status?: OrderStatus, orderQuery?: string, beforeId?: string },
  signal: AbortSignal,
) {
  const wire = await request<OrdersWire>(opsTenantPath(tenantId) + '/orders', {
    query: { kind: filters.kind, status: filters.status, order_query: filters.orderQuery, before_id: filters.beforeId },
    signal,
  })
  return mapOrders(wire)
}

export async function getOpsTenantOrder(tenantId: string, orderId: string, signal: AbortSignal) {
  const wire = await request<OrderDetailWire>(opsTenantPath(tenantId) + '/orders/' + encodeURIComponent(orderId), { signal })
  return mapOrderDetail(wire)
}

export async function listOpsTenantWalletEntries(
  tenantId: string,
  filters: { beforeId?: string, kind?: WalletFilterKind, start?: string, end?: string },
  signal: AbortSignal,
) {
  const wire = await request<WalletEntriesWire>(opsTenantPath(tenantId) + '/wallet/ledger', {
    query: { before_id: filters.beforeId, kind: filters.kind, start_date: filters.start, end_date: filters.end },
    signal,
  })
  return mapWalletEntries(wire)
}
