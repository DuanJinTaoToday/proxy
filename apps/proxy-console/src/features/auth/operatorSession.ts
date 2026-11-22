import { computed, shallowRef } from 'vue'
import { ApiError, configureOperatorApi } from '@/api/client'
import { getOperatorAccount, loginOperator, type OperatorPermission } from '@/api/operatorAccess'
import { safeRedirect } from './rules'

const storageKey = 'proxy-console.operator-session'
interface StoredOperator { token: string, expiresAt: number }

function readStored(): StoredOperator | undefined {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(storageKey) ?? 'null')
    if (value && typeof value === 'object' && 'token' in value && 'expiresAt' in value &&
        typeof value.token === 'string' && value.token && typeof value.expiresAt === 'number' &&
        Number.isFinite(value.expiresAt) && value.expiresAt > Date.now()) {
      return { token: value.token, expiresAt: value.expiresAt }
    }
  } catch {
    // A malformed record is treated as an expired session.
  }
}

let stored = readStored()
let expirationTimer: number | undefined
let onExpired = () => {}
const account = shallowRef<Awaited<ReturnType<typeof getOperatorAccount>>>()

export const currentOperator = computed(() => account.value)

export function hasOperatorPermission(permission: OperatorPermission) {
  return account.value?.permissions.includes(permission) ?? false
}

export function hasOperatorSession() {
  return Boolean(stored && stored.expiresAt > Date.now())
}

function clearSession() {
  window.clearTimeout(expirationTimer)
  stored = undefined
  account.value = undefined
  localStorage.removeItem(storageKey)
}

function expireSession() {
  clearSession()
  onExpired()
}

function scheduleExpiration() {
  window.clearTimeout(expirationTimer)
  if (stored) expirationTimer = window.setTimeout(expireSession, Math.max(0, stored.expiresAt - Date.now()))
}

export async function loadOperator() {
  if (!hasOperatorSession()) return false
  if (account.value) return true
  const session = stored
  try {
    const result = await getOperatorAccount()
    if (stored !== session || !hasOperatorSession()) return false
    account.value = result
    return true
  } catch (error) {
    if (stored === session && error instanceof ApiError && (error.status === 401 || error.status === 403)) clearSession()
    return false
  }
}

export async function signInOperator(username: string, password: string) {
  stored = await loginOperator(username.trim(), password)
  account.value = undefined
  try {
    const result = await getOperatorAccount()
    account.value = result
    localStorage.setItem(storageKey, JSON.stringify(stored))
    scheduleExpiration()
  } catch (error) {
    clearSession()
    throw error
  }
}

export function signOutOperator() {
  clearSession()
  window.location.assign('/ops/login')
}

export function safeOperatorRedirect(value: unknown) {
  const path = safeRedirect(value, '/ops/tenants')
  return /^\/ops(?:[/?#]|$)/.test(path) && !/^\/ops\/login(?:[/?#]|$)/i.test(path) ? path : '/ops/tenants'
}

export function installOperatorSession(expired: () => void) {
  onExpired = expired
  scheduleExpiration()
  configureOperatorApi({
    accessToken: () => hasOperatorSession() ? stored?.token : undefined,
    onUnauthorized: (_error, token) => {
      if (stored?.token === token) expireSession()
    },
  })
  window.addEventListener('storage', event => {
    if (event.key !== storageKey) return
    if (!stored) return
    const next = readStored()
    if (stored.token === next?.token) return
    stored = next
    account.value = undefined
    scheduleExpiration()
    if (next) window.location.reload()
    else onExpired()
  })
}
