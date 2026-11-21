import { request } from './client'

// 对应 api/protos/proxycontrol/v1/access.proto 的租户注册、登录和账号查询。
// 控制面 JSON 使用 proto 字段原名（蛇形），这里映射为前端使用的驼峰字段。

export interface TenantAccess {
  tenantId: string
  name: string
  primaryAccountName: string
}

export interface LoginResult {
  accessToken: string
  expiresInSeconds: number
}

export interface Credentials { phone: string, password: string }

interface TenantAccessWire { tenant_id: string, name: string, primary_account_name: string }
interface LoginResponseWire { access_token: string, expires_in_seconds: number }

const toTenantAccess = (wire: TenantAccessWire): TenantAccess => ({ tenantId: wire.tenant_id, name: wire.name, primaryAccountName: wire.primary_account_name })

export async function register(body: Credentials) {
  return toTenantAccess(await request<TenantAccessWire>('/v1/access/register', { method: 'POST', body }))
}

export async function login(body: Credentials): Promise<LoginResult> {
  const wire = await request<LoginResponseWire>('/v1/access/login', { method: 'POST', body })
  return { accessToken: wire.access_token, expiresInSeconds: wire.expires_in_seconds }
}

export async function getAccount(signal?: AbortSignal) {
  return toTenantAccess(await request<TenantAccessWire>('/v1/tenant/account', { signal }))
}
