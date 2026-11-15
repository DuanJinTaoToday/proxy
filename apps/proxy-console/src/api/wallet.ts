import { request } from './control'

export interface WalletTenant {
  tenant_id: string
  name: string
  primary_account_name: string
  available_minor?: string
}

export interface Entry {
  id: string
  order_id?: string
  kind: string
  before_minor?: string
  after_minor?: string
  created_at: string
  operator_id?: string
  operator_name?: string
  reason?: string
}

export interface Order {
  id: string
  kind: string
  status: string
  locked_minor: string
  debited_minor: string
  released_minor?: string
  refunded_minor?: string
  prepaid_minor?: string
  settled_minor?: string
  unsettled_minor?: string
  delivered_bytes?: string
  delivered_count?: string
  created_at: string
  items?: {
    position: number
    specification?: string
    capacity_bytes?: string
    term_days?: number
    unit_minor: string
    quantity: number
    delivered_count?: number
  }[]
}

export interface Recharge {
  request_key: string
  confirmation_token: string
}

export interface RechargeQuote {
  tenant: WalletTenant
  confirmation_token: string
  amount_minor: string
  expected_after_minor: string
  reason: string
  expires_at: string
}

const operationsWallet = '/v1/operations/tenants'
const manageBilling = 4

export function findWalletTenant(token: string, query: string, signal: AbortSignal) {
  return request<WalletTenant>(operationsWallet + '?' + new URLSearchParams({ query }), { token, signal })
}

export function getWallet(token: string, signal: AbortSignal, tenantID = '') {
  return request<WalletTenant>(tenantID ? operatorPath(tenantID) : '/v1/tenant/wallet', { token, signal })
}

export function canManageBilling(token: string, signal: AbortSignal) {
  return request('/v1/access/check', { token, signal, body: { permission: manageBilling } })
}

export function quoteRecharge(token: string, tenantID: string, body: { request_key: string; amount_minor: string; reason: string }, signal: AbortSignal) {
  return request<RechargeQuote>(operatorPath(tenantID) + '/wallet/recharge-quotes', { token, body, signal })
}

export function rechargeWallet(token: string, tenantID: string, body: Recharge) {
  return request<Entry>(operatorPath(tenantID) + '/wallet/recharges', { token, body })
}

export function listEntries(token: string, tenantID: string, before: string, signal: AbortSignal) {
  const path = tenantID ? operatorPath(tenantID) + '/wallet/ledger' : '/v1/tenant/wallet/ledger'
  return request<{ entries?: Entry[]; next_before_id?: string }>(
    path + '?' + new URLSearchParams({ before_id: before }), { token, signal })
}

export function listOrders(token: string, tenantID: string, filter: { before_id: string; kind: string; status: string }, signal: AbortSignal) {
  return request<{ orders?: Order[]; next_before_id?: string }>(
    ordersPath(tenantID) + '?' + new URLSearchParams(filter), { token, signal })
}

export function getOrder(token: string, tenantID: string, id: string, signal: AbortSignal) {
  return request<Order>(ordersPath(tenantID) + '/' + encodeURIComponent(id), { token, signal })
}

function operatorPath(tenantID: string) {
  return operationsWallet + '/' + encodeURIComponent(tenantID)
}

function ordersPath(tenantID: string) {
  return tenantID ? operatorPath(tenantID) + '/orders' : '/v1/tenant/orders'
}
