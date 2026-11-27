import { request } from './client'

// 对应 api/protos/proxycontrol/v1/access.proto 的租户注册、登录、找回密码和账号查询。
// 控制面 JSON 使用 proto 字段原名（蛇形），这里映射为前端使用的驼峰字段。

export interface TenantAccess {
  tenantId: string
  name: string
  primaryAccountName: string
  email: string
}

export interface LoginResult {
  accessToken: string
  expiresInSeconds: number
}

export interface Credentials { email: string, password: string }
export interface Registration extends Credentials {
  verificationCode: string
  inviteCode?: string
  linkInviteCode?: string
}

interface TenantAccessWire { tenant_id: string, name: string, primary_account_name: string, email?: string }
interface LoginResponseWire { access_token: string, expires_in_seconds: number }

const toTenantAccess = (wire: TenantAccessWire): TenantAccess => ({
  tenantId: wire.tenant_id,
  name: wire.name,
  primaryAccountName: wire.primary_account_name,
  email: wire.email ?? '',
})

// 公开接口不携带当前会话，避免旧令牌被拒绝时误触发退出。
// 发送验证码和找回密码对未注册、已注册的邮箱返回相同结果。
export async function requestRegisterCode(email: string) {
  await request<unknown>('/v1/access/register-code', { body: { email }, anonymous: true })
}

export async function register({ email, password, verificationCode, inviteCode, linkInviteCode }: Registration) {
  const body: Record<string, string> = {
    email,
    password,
    verification_code: verificationCode,
  }
  if (inviteCode !== undefined) body.invite_code = inviteCode
  if (linkInviteCode !== undefined) body.link_invite_code = linkInviteCode
  return toTenantAccess(await request<TenantAccessWire>('/v1/access/register', {
    body,
    anonymous: true,
  }))
}

export async function login(body: Credentials): Promise<LoginResult> {
  const wire = await request<LoginResponseWire>('/v1/access/login', { body, anonymous: true })
  return { accessToken: wire.access_token, expiresInSeconds: wire.expires_in_seconds }
}

export async function requestPasswordReset(email: string) {
  await request<unknown>('/v1/access/forgot-password', { body: { email }, anonymous: true })
}

// 邮件链接中的一次性令牌：重置密码，或完成运营发起的账号恢复。
export async function resetPassword(token: string, password: string) {
  await request<unknown>('/v1/access:reset-password', { body: { token, password }, anonymous: true })
}

export async function recoverEmailAccount(token: string, password: string) {
  await request<unknown>('/v1/access:recover-email', { body: { token, password }, anonymous: true })
}

export async function getAccount(signal?: AbortSignal) {
  return toTenantAccess(await request<TenantAccessWire>('/v1/tenant/account', { signal }))
}
