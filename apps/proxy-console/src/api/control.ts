export interface Tenant {
  id: string
  name: string
}

export class ApiError extends Error {
  constructor(readonly status: number) {
    super(`Control API returned HTTP ${status}`)
  }
}

async function request<T>(path: string, options: {
  token?: string
  signal?: AbortSignal
  body?: { phone: string; password: string }
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

export async function login(phone: string, password: string) {
  const result = await request<{ accessToken?: string; access_token?: string }>('/v1/access/login', {
    body: { phone, password },
  })
  const token = result.accessToken ?? result.access_token
  if (!token) throw new Error('Login response did not include an access token')
  return token
}

export async function register(phone: string, password: string) {
  await request('/v1/access/register', { body: { phone, password } })
}

export async function getAccount(token: string, signal: AbortSignal): Promise<Tenant> {
  const result = await request<{ tenant_id?: string; name?: string }>('/v1/tenant/account', { token, signal })
  if (!result.tenant_id) throw new Error('Account response did not include a tenant ID')
  return { id: result.tenant_id, name: result.name || result.tenant_id }
}
