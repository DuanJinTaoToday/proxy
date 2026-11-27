import { computed, ref } from 'vue'
import { configureApi } from '@/api/client'
import * as access from '@/api/access'
import type { TenantAccess } from '@/api/access'
import { emailAddress, normalizeEmail } from './rules'

// 控制台会话：访问令牌、到期时间和登录邮箱保存在本浏览器 localStorage，刷新和新标签页保持登录。
// 登录邮箱同时作为各页面本地数据的账号键；账号接口返回的邮箱优先用于展示。
// 改用邮箱登录前保存的手机号会话没有邮箱，视为无效，需要重新登录。
interface StoredSession { token: string, expiresAt: number, email: string }

const storageKey = 'proxy-console.session'

function readStored(): StoredSession | undefined {
  try {
    const value = JSON.parse(localStorage.getItem(storageKey) ?? 'null')
    return value && typeof value.token === 'string' && value.token && typeof value.email === 'string' && value.email && value.expiresAt > Date.now() ? value : undefined
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
  accountFailed.value = false
  if (hadSession && !stored.value) onExpired()
  if (hadSession && stored.value) void loadAccount()
})

export const currentAccount = computed(() => account.value)
export const accountLoadFailed = computed(() => accountFailed.value)
export const sessionEmail = computed(() => stored.value?.email ?? '')
export const accountEmail = computed(() => account.value?.email || sessionEmail.value)

export function hasSession() {
  if (stored.value && stored.value.expiresAt <= Date.now()) clearSession()
  return Boolean(stored.value)
}

// 重置密码或恢复账号成功后，控制面已使旧会话失效，本地会话随之清除，不触发跳转。
export function clearSession() {
  stored.value = undefined
  account.value = undefined
  localStorage.removeItem(storageKey)
}

export async function signIn(email: string, password: string) {
  const address = normalizeEmail(email)
  const result = await access.login({ email: address, password })
  stored.value = { token: result.accessToken, expiresAt: Date.now() + result.expiresInSeconds * 1000, email: address }
  localStorage.setItem(storageKey, JSON.stringify(stored.value))
  account.value = undefined
}

export interface RegisterInviteOptions {
  inviteCode?: string
  linkInviteCode?: string
}

export async function register(email: string, password: string, verificationCode: string, invite?: RegisterInviteOptions) {
  await access.register({
    email: normalizeEmail(email),
    password,
    verificationCode: verificationCode.trim(),
    inviteCode: invite?.inviteCode !== undefined ? invite.inviteCode.trim() : undefined,
    linkInviteCode: invite?.linkInviteCode !== undefined ? invite.linkInviteCode.trim() : undefined,
  })
}

// 验证码按输入原样（去首尾空白）投递；控制面自行归一化账号标识。找回密码总是发往账号已核实的地址，只需标识。
export const requestRegisterCode = (email: string) => access.requestRegisterCode(emailAddress(email))
export const requestPasswordReset = (email: string) => access.requestPasswordReset(normalizeEmail(email))

// 读取失败（网络等）时记录状态，由顶栏提示并重试；会话失效由请求层统一处理。
export async function loadAccount() {
  const token = stored.value?.token
  if (!token || account.value) return
  accountFailed.value = false
  try {
    const result = await access.getAccount()
    if (stored.value?.token === token) account.value = result
  } catch {
    if (stored.value?.token === token) accountFailed.value = true
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
