import { request } from './control'

const base = '/v1/tenant/subaccounts'

export interface Subaccount {
  client_id: string
  name: string
  remark?: string
  active?: boolean
  used_bytes?: string
  limit_bytes?: string
}

export interface ProxyConfig {
  host: string
  port: number
  username: string
  password: string
  explicit_sid?: boolean
  explicit_sidtime?: boolean
}

export function listSubaccounts(token: string, signal: AbortSignal) {
  return request<{ accounts?: Subaccount[] }>(base, { token, signal })
}

export function createSubaccount(token: string, body: {
  name: string; password: string; remark: string; active: boolean; limit_bytes?: string
}) {
  return request<Subaccount>(base, { token, body })
}

function accountPath(id: string) {
  return base + '/' + encodeURIComponent(id)
}

export function updateSubaccount(token: string, id: string, body: {
  password?: string; remark?: string; active?: boolean; limit_bytes?: string; update_mask: string
}) {
  return request<Subaccount>(accountPath(id), { token, method: 'PATCH', body })
}

export function revealSubaccountPassword(token: string, id: string, signal: AbortSignal) {
  return request<{ password: string }>(accountPath(id) + '/password', { token, signal })
}

export function getSubaccountConfig(token: string, id: string, signal: AbortSignal) {
  return request<ProxyConfig>(accountPath(id) + '/proxy-config', { token, signal })
}
