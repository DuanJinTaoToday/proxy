import { computed, ref } from 'vue'
import { configureApi } from '@/api/client'
import * as access from '@/api/access'
import type { TenantAccess } from '@/api/access'
import { normalizePhone } from './rules'

// 控制台会话：访问令牌、到期时间和登录手机号保存在本浏览器，刷新后保持登录。
// 控制面的账号接口不返回手机号，顶栏显示的手机号取自本次登录输入。
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

// 另一个标签页登录或退出时同步。
window.addEventListener('storage', event => {
  if (event.key !== storageKey) return
  stored.value = readStored()
  account.value = undefined
})

export const currentAccount = computed(() => account.value)
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

// 注册成功后用同一组账密登录，客户直接进入控制台。
export async function registerAndSignIn(phone: string, password: string) {
  await access.register({ phone: normalizePhone(phone), password })
  await signIn(phone, password)
}

export async function loadAccount() {
  if (!account.value) account.value = await access.getAccount()
  return account.value
}

// 退出和会话失效都整页跳转到登录页，内存里各模块缓存的业务数据随之清空。
export function signOut(loginPath: string) {
  clearSession()
  window.location.assign(loginPath)
}

export function installSession(onExpired: () => void) {
  configureApi({
    accessToken: () => stored.value?.token,
    onUnauthorized: () => {
      clearSession()
      onExpired()
    },
  })
}
