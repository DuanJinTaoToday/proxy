import { computed, ref } from 'vue'
import { configureApi } from '@/api/client'
import * as access from '@/api/access'
import type { TenantAccess } from '@/api/access'
import { normalizePhone } from './rules'

// 控制台会话：访问令牌、到期时间和登录手机号保存在本浏览器 localStorage，刷新和新标签页保持登录。
// 控制面以 Bearer 令牌鉴权，且账号接口不返回手机号，顶栏显示的手机号取自本次登录输入。
interface StoredSession { token: string, expiresAt: number, phone: string }

const storageKey = 'proxy-console.session'

function readStored(): StoredSession | undefined {
  try {
    const value = JSON.parse(localStorage.getItem(storageKey) ?? 'null')
    return value && typeof value.token === 'string' && value.token && value.expiresAt > Date.now() ? value : undefined
  } catch {
    return undefined
  }
}

const stored = ref(readStored())
const account = ref<TenantAccess>()
const accountFailed = ref(false)
let onExpired = () => {}

// 另一个标签页登录或退出时同步；会话被清空时当前页面同样回到登录页。
window.addEventListener('storage', event => {
  if (event.key !== storageKey) return
  const hadSession = Boolean(stored.value)
  stored.value = readStored()
  account.value = undefined
  if (hadSession && !stored.value) onExpired()
})

export const currentAccount = computed(() => account.value)
export const accountLoadFailed = computed(() => accountFailed.value)
export const sessionPhone = computed(() => stored.value?.phone ?? '')

export function hasSession() {
  if (stored.value && stored.value.expiresAt <= Date.now()) clearSession()
  return Boolean(stored.value)
}

function clearSession() {
  stored.value = undefined
  account.value = undefined
  localStorage.removeItem(storageKey)
}

// 表单允许输入空格、连字符和 +86，提交前统一为 11 位号码，控制面再按同一号码去重。
export async function signIn(phone: string, password: string) {
  const number = normalizePhone(phone)
  const result = await access.login({ phone: number, password })
  stored.value = { token: result.accessToken, expiresAt: Date.now() + result.expiresInSeconds * 1000, phone: number }
  localStorage.setItem(storageKey, JSON.stringify(stored.value))
  account.value = undefined
}

export async function register(phone: string, password: string) {
  await access.register({ phone: normalizePhone(phone), password })
}

// 读取失败（网络等）时记录状态，由顶栏提示并重试；会话失效由请求层统一处理。
export async function loadAccount() {
  if (account.value) return
  accountFailed.value = false
  try {
    account.value = await access.getAccount()
  } catch {
    accountFailed.value = true
  }
}

// 退出整页跳转到登录页，内存里各模块缓存的业务数据随之清空。
export function signOut(loginPath: string) {
  clearSession()
  window.location.assign(loginPath)
}

// expired 在会话失效（当前令牌被拒绝，或在其他标签页退出）时调用。
export function installSession(expired: () => void) {
  onExpired = expired
  configureApi({
    accessToken: () => stored.value?.token,
    onUnauthorized: (_error, token) => {
      // 旧请求在重新登录后才返回 401 时，不影响新会话。
      if (stored.value?.token !== token) return
      clearSession()
      onExpired()
    },
  })
}
