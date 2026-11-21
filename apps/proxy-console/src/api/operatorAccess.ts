import { request } from './client'

export type OperatorPermission = 'read_operations' | 'manage_billing' | 'manage_tenant_accounts'

export async function loginOperator(username: string, password: string) {
  const result = await request<{ access_token: string, expires_in_seconds: number }>('/v1/operations/access/login', {
    body: { username, password },
    anonymous: true,
  })
  return { token: result.access_token, expiresAt: Date.now() + result.expires_in_seconds * 1000 }
}

export async function getOperatorAccount() {
  const result = await request<{ username: string, permissions?: OperatorPermission[] }>('/v1/operations/access/account')
  return { username: result.username, permissions: result.permissions ?? [] }
}
