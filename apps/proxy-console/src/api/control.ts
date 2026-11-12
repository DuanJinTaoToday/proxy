export interface Tenant {
  id: string
  name: string
}

export interface UsageAmount {
  input_bytes?: string
  output_bytes?: string
  total_bytes?: string
  billable_bytes?: string
}

export interface DailyUsage {
  days?: { date: string; amount?: UsageAmount }[]
  last_received_at?: string
}

export interface Balance {
  available_bytes?: string
  overdraft_bytes?: string
  admission_threshold_bytes?: string
  warning_threshold_bytes?: string
  warning_since?: string
  grants?: {
    grant_id: string
    granted_bytes?: string
    used_bytes?: string
    starts_at?: string
    expires_at?: string
  }[]
}

export class ApiError extends Error {
  constructor(readonly status: number) {
    super(`Control API returned HTTP ${status}`)
  }
}

async function request<T>(path: string, options: {
  token?: string
  signal?: AbortSignal
  body?: { username: string; password: string }
} = {}): Promise<T> {
  const headers = new Headers({ Accept: 'application/json' })
  if (options.token) headers.set('Authorization', `Bearer ${options.token}`)
  if (options.body) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, {
    method: options.body ? 'POST' : 'GET',
    headers,
    signal: options.signal,
    body: options.body && JSON.stringify(options.body),
    cache: 'no-store',
  })
  if (!response.ok) throw new ApiError(response.status)
  return response.json()
}

export async function login(username: string, password: string) {
  const result = await request<{ accessToken?: string; access_token?: string }>('/v1/access/login', {
    body: { username, password },
  })
  const token = result.accessToken ?? result.access_token
  if (!token) throw new Error('Login response did not include an access token')
  return token
}

export async function listTenants(token: string, signal: AbortSignal): Promise<Tenant[]> {
  const result = await request<{
    tenants?: { tenantId?: string; tenant_id?: string; name?: string }[]
  }>('/v1/access/tenants', { token, signal })
  return (result.tenants ?? []).map((tenant) => {
    const id = tenant.tenantId ?? tenant.tenant_id
    if (!id) throw new Error('Tenant response did not include an ID')
    return { id, name: tenant.name || id }
  })
}

export function getDailyUsage(tenant: string, start: string, end: string, token: string, signal: AbortSignal) {
  const dates = new URLSearchParams({ start_date: start, end_date: end })
  return request<DailyUsage>(`/v1/tenants/${encodeURIComponent(tenant)}/usage/daily?${dates}`, { token, signal })
}

export function getBalance(tenant: string, token: string, signal: AbortSignal) {
  return request<Balance>(`/v1/tenants/${encodeURIComponent(tenant)}/balance`, { token, signal })
}
