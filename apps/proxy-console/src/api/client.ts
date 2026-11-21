// 控制面的统一请求入口：附带登录凭据、支持取消、把错误归一为 ApiError。契约以仓库 api/proxycontrol/openapi.yaml 为准。

export class ApiError extends Error {
  constructor(
    readonly status: number,
    // 控制面返回的稳定错误码，界面按它翻译。网络失败为 NETWORK。
    readonly reason: string,
    message: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

type QueryValue = string | number | boolean | undefined | null
export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  query?: Record<string, QueryValue | QueryValue[]>
  body?: unknown
  signal?: AbortSignal
}

interface ApiHooks {
  accessToken: () => string | undefined
  // token 为被拒绝的那次请求携带的凭据，会话层据此判断是否仍是当前会话。
  onUnauthorized: (error: ApiError, token: string) => void
}

const hooks: ApiHooks = { accessToken: () => undefined, onUnauthorized: () => {} }

// 认证模块注册凭据来源和失效处理，API 层本身不依赖页面或会话实现。
export function configureApi(next: Partial<ApiHooks>) {
  Object.assign(hooks, next)
}

function buildUrl(path: string, query: RequestOptions['query']) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query ?? {})) {
    for (const item of Array.isArray(value) ? value : [value]) {
      if (item !== undefined && item !== null && item !== '') params.append(key, String(item))
    }
  }
  const search = params.toString()
  return search ? `${path}?${search}` : path
}

// protojson 把 int64 编码为字符串。万一收到超出安全整数范围的数字，保留原始文本，避免字节数丢精度。
function parseJson(text: string) {
  return JSON.parse(text, (_key, value, context?: { source?: string }) =>
    typeof value === 'number' && Number.isInteger(value) && !Number.isSafeInteger(value) && context?.source ? context.source : value)
}

async function readResponse(path: string, options: RequestOptions, accept: string): Promise<string> {
  const headers: Record<string, string> = { Accept: accept }
  const token = hooks.accessToken()
  if (token) headers.Authorization = `Bearer ${token}`
  if (options.body !== undefined) headers['Content-Type'] = 'application/json'

  let response: Response
  try {
    response = await fetch(buildUrl(path, options.query), {
      method: options.method ?? (options.body === undefined ? 'GET' : 'POST'),
      headers,
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal: options.signal,
    })
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') throw error
    throw new ApiError(0, 'NETWORK', error instanceof Error ? error.message : 'network error')
  }

  const text = await response.text()
  if (!response.ok) {
    let payload: { reason?: string, message?: string } = {}
    try { payload = text ? parseJson(text) : {} } catch { /* 非 JSON 错误体按状态码处理 */ }
    const error = new ApiError(response.status, payload.reason || 'UNKNOWN', payload.message || response.statusText)
    // 只有携带凭据的请求被拒绝才表示会话失效；登录接口的账密错误同为 401，不应触发退出。
    if (response.status === 401 && token) hooks.onUnauthorized(error, token)
    throw error
  }
  return text
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const text = await readResponse(path, options, 'application/json')
  return (text ? parseJson(text) : undefined) as T
}

export function requestText(path: string, options: RequestOptions = {}): Promise<string> {
  return readResponse(path, options, 'text/plain')
}
