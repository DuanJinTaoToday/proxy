import { request } from './client'

interface SubaccountWire {
  client_id: string
  name: string
  remark?: string
  active?: boolean
  used_bytes?: string
  limit_bytes?: string
  created_at?: string
}

export interface Subaccount {
  clientId: string
  name: string
  remark: string
  active: boolean
  usedBytes: bigint
  limitBytes: bigint | undefined
  createdAt: string | undefined
}

interface CreateSubaccount {
  name: string
  password: string
  remark: string
  active: boolean
  limitBytes: bigint | undefined
}

export type SubaccountChanges =
  | { kind: 'details', remark?: string, limit?: { bytes: bigint | undefined } }
  | { kind: 'password', password: string }
  | { kind: 'status', active: boolean }

const path = '/v1/tenant/subaccounts'
const accountPath = (clientId: string) => `${path}/${encodeURIComponent(clientId)}`

function toSubaccount(wire: SubaccountWire): Subaccount {
  return {
    clientId: wire.client_id,
    name: wire.name,
    remark: wire.remark ?? '',
    active: wire.active ?? false,
    usedBytes: BigInt(wire.used_bytes ?? 0),
    limitBytes: wire.limit_bytes === undefined ? undefined : BigInt(wire.limit_bytes),
    createdAt: wire.created_at,
  }
}

export async function listSubaccounts(signal?: AbortSignal) {
  const wire = await request<{ accounts?: SubaccountWire[] }>(path, { signal })
  return (wire.accounts ?? []).map(toSubaccount)
}

export async function createSubaccount({ name, password, remark, active, limitBytes }: CreateSubaccount) {
  const body = { name, password, remark, active, limit_bytes: limitBytes?.toString() }
  return toSubaccount(await request<SubaccountWire>(path, { body }))
}

export async function updateSubaccount(clientId: string, changes: SubaccountChanges) {
  return toSubaccount(await request<SubaccountWire>(accountPath(clientId), {
    method: 'PATCH',
    body: updateBody(changes),
  }))
}

function updateBody(changes: SubaccountChanges) {
  switch (changes.kind) {
    case 'password': return { update_mask: 'password', password: changes.password }
    case 'status': return { update_mask: 'active', active: changes.active }
    case 'details': {
      const mask = []
      if (changes.remark !== undefined) mask.push('remark')
      if (changes.limit) mask.push('limitBytes')
      return { update_mask: mask.join(','), remark: changes.remark, limit_bytes: changes.limit?.bytes?.toString() }
    }
  }
}

export async function revealSubaccountPassword(clientId: string, signal?: AbortSignal) {
  const wire = await request<{ password: string }>(`${accountPath(clientId)}/password`, { signal })
  return wire.password
}

interface ProxyConfig {
  host: string
  port: number
  username: string
  password: string
  explicit_sid?: boolean
}

export function getSubaccountProxyConfig(clientId: string, signal: AbortSignal) {
  return request<ProxyConfig>(`${accountPath(clientId)}/proxy-config`, { signal })
}
