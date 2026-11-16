export class ApiError extends Error {
  constructor(readonly status: number, readonly reason = '', readonly metadata: Record<string, string> = {}) {
    super(`Control API returned HTTP ${status}`)
  }
}

interface RequestOptions {
  token?: string
  signal?: AbortSignal
  body?: object
  method?: 'PATCH' | 'PUT'
}

async function fetchControl(path: string, options: RequestOptions, accept: string) {
  const headers = new Headers({ Accept: accept })
  if (options.token) headers.set('Authorization', `Bearer ${options.token}`)
  if (options.body) headers.set('Content-Type', 'application/json')
  const response = await fetch(path, {
    method: options.method ?? (options.body ? 'POST' : 'GET'),
    headers,
    signal: options.signal,
    body: options.body && JSON.stringify(options.body),
    cache: 'no-store',
  })
  if (!response.ok) {
    const error = await response.json().catch(() => ({})) as { reason?: string; metadata?: Record<string, string> }
    throw new ApiError(response.status, error?.reason, error?.metadata)
  }
  return response
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return (await fetchControl(path, options, 'application/json')).json()
}

export async function requestCSV(path: string, options: RequestOptions) {
  return (await fetchControl(path, options, 'text/csv')).blob()
}

export async function login(phone: string, password: string) {
  return signIn('/v1/access/login', { phone, password })
}

export async function loginOperator(username: string, password: string) {
  return signIn('/v1/operations/access/login', { username, password })
}

async function signIn(path: string, body: object) {
  const result = await request<{ accessToken?: string; access_token?: string }>(path, { body })
  const token = result.accessToken ?? result.access_token
  if (!token) throw new Error('Login response did not include an access token')
  return token
}

export async function register(phone: string, password: string) {
  await request('/v1/access/register', { body: { phone, password } })
}
