import { computed, onScopeDispose, ref, watch, type ComputedRef, type Ref } from 'vue'
import { ApiError } from '@/api/client'
import { transferTenantCommission } from '@/api/commissions'
import { currentAccount } from '@/features/auth'

export type CommissionTransferNotice =
  | { kind: 'invalid' | 'exceeds' | 'uncertain' | 'storage' | 'rejected' | 'insufficient' }
  | { kind: 'success', amountMinor: bigint }

export interface CommissionTransferProps {
  tenantId: string
  availableMinor?: bigint
}

export interface CommissionTransferState {
  amount: Ref<string>
  busy: Ref<boolean>
  pendingAmount: ComputedRef<bigint | undefined>
  notice: Ref<CommissionTransferNotice | undefined>
  submit: () => Promise<void>
  retry: () => Promise<void>
}

interface PendingTransfer {
  requestKey: string
  amountMinor: bigint
}

const STORAGE_PREFIX = 'proxy-console.commission-transfer.'
const DECIMAL_AMOUNT_PATTERN = /^\d+(?:\.\d{1,2})?$/
const POSITIVE_INT_PATTERN = /^[1-9]\d*$/

const INSUFFICIENT_REASON = 'COMMISSION_BALANCE_INSUFFICIENT'
const REJECTED_REASONS = new Set([
  'INVALID_WALLET_REQUEST',
  'WALLET_REQUEST_CONFLICT',
  'WALLET_BALANCE_LIMIT',
])

function parseAmountMinor(raw: string): bigint | undefined {
  const trimmed = raw.trim()
  if (!DECIMAL_AMOUNT_PATTERN.test(trimmed)) return undefined
  const [majorPart, fractionPart = ''] = trimmed.split('.')
  const minor = BigInt(majorPart) * 100n + BigInt(fractionPart.padEnd(2, '0'))
  return minor > 0n ? minor : undefined
}

function parsePendingPayload(raw: string): PendingTransfer {
  const parsed: unknown = JSON.parse(raw)
  if (
    typeof parsed === 'object'
    && parsed !== null
    && 'request_key' in parsed
    && typeof parsed.request_key === 'string'
    && parsed.request_key.trim() !== ''
    && 'amount_minor' in parsed
    && typeof parsed.amount_minor === 'string'
    && POSITIVE_INT_PATTERN.test(parsed.amount_minor)
  ) {
    return {
      requestKey: parsed.request_key,
      amountMinor: BigInt(parsed.amount_minor),
    }
  }
  throw new Error('Invalid pending commission transfer')
}

function pendingCommissionTransfer(tenantId: string) {
  const key = `${STORAGE_PREFIX}${tenantId}`

  function read(): PendingTransfer | undefined {
    const raw = localStorage.getItem(key)
    if (raw === null) return undefined
    return parsePendingPayload(raw)
  }

  return {
    read,
    async reserve(payload: PendingTransfer): Promise<PendingTransfer | undefined> {
      return navigator.locks.request(key, () => {
        const existing = read()
        if (!existing) {
          localStorage.setItem(key, JSON.stringify({
            request_key: payload.requestKey,
            amount_minor: payload.amountMinor.toString(),
          }))
        }
        return existing
      })
    },
    async clear(payload: PendingTransfer): Promise<void> {
      await navigator.locks.request(key, () => {
        if (read()?.requestKey === payload.requestKey) {
          localStorage.removeItem(key)
        }
      })
    },
  }
}

export function useCommissionTransfer(
  props: CommissionTransferProps,
  onRefresh: () => void,
): CommissionTransferState {
  const amount = ref('')
  const busy = ref(false)
  const pending = ref<PendingTransfer | undefined>(undefined)
  const notice = ref<CommissionTransferNotice | undefined>(undefined)
  const pendingAmount = computed<bigint | undefined>(() => pending.value?.amountMinor)

  let activeController: AbortController | undefined
  let disposed = false

  function isSameAccount(tenantId: string): boolean {
    return !disposed
      && tenantId !== ''
      && props.tenantId === tenantId
      && currentAccount.value?.tenantId === tenantId
  }

  function abortActive() {
    if (!activeController) return
    activeController.abort()
    activeController = undefined
    busy.value = false
  }

  watch(
    () => props.tenantId,
    tenantId => {
      abortActive()
      amount.value = ''
      notice.value = undefined
      pending.value = undefined
      if (!tenantId) return
      try {
        pending.value = pendingCommissionTransfer(tenantId).read()
      } catch {
        notice.value = { kind: 'storage' }
      }
    },
    { immediate: true },
  )

  watch(
    () => currentAccount.value?.tenantId,
    accountTenantId => {
      if (accountTenantId !== props.tenantId) {
        abortActive()
      }
    },
  )

  onScopeDispose(() => {
    disposed = true
    activeController?.abort()
    activeController = undefined
  })

  async function clearPending(
    tenantId: string,
    storage: ReturnType<typeof pendingCommissionTransfer>,
    payload: PendingTransfer,
    controller: AbortController,
  ): Promise<boolean> {
    try {
      await storage.clear(payload)
      return true
    } catch {
      if (!isSameAccount(tenantId) || controller.signal.aborted) return false
      notice.value = { kind: 'storage' }
      return false
    }
  }

  async function executeTransfer(
    tenantId: string,
    storage: ReturnType<typeof pendingCommissionTransfer>,
    payload: PendingTransfer,
    controller: AbortController,
  ): Promise<void> {
    if (!isSameAccount(tenantId) || controller.signal.aborted) return
    try {
      await transferTenantCommission(
        { requestKey: payload.requestKey, amountMinor: payload.amountMinor },
        controller.signal,
      )
    } catch (error) {
      if (!isSameAccount(tenantId) || controller.signal.aborted) return
      if (error instanceof ApiError && error.status >= 400 && error.status < 500) {
        if (error.reason === INSUFFICIENT_REASON) {
          const cleared = await clearPending(tenantId, storage, payload, controller)
          if (!isSameAccount(tenantId) || controller.signal.aborted) return
          if (cleared) {
            pending.value = undefined
            notice.value = { kind: 'insufficient' }
          }
          onRefresh()
          return
        }
        if (REJECTED_REASONS.has(error.reason)) {
          const cleared = await clearPending(tenantId, storage, payload, controller)
          if (!isSameAccount(tenantId) || controller.signal.aborted) return
          if (cleared) {
            pending.value = undefined
            notice.value = { kind: 'rejected' }
          }
          return
        }
      }
      notice.value = { kind: 'uncertain' }
      return
    }

    if (!isSameAccount(tenantId) || controller.signal.aborted) return
    const cleared = await clearPending(tenantId, storage, payload, controller)
    if (!isSameAccount(tenantId) || controller.signal.aborted) return
    if (cleared) {
      pending.value = undefined
      amount.value = ''
      notice.value = { kind: 'success', amountMinor: payload.amountMinor }
    }
    onRefresh()
  }

  async function submit(): Promise<void> {
    if (busy.value || pending.value) return
    const tenantId = props.tenantId
    if (!isSameAccount(tenantId)) return

    const amountMinor = parseAmountMinor(amount.value)
    if (amountMinor === undefined) {
      notice.value = { kind: 'invalid' }
      return
    }
    if (props.availableMinor === undefined) return
    if (amountMinor > props.availableMinor) {
      notice.value = { kind: 'exceeds' }
      return
    }

    const storage = pendingCommissionTransfer(tenantId)
    const payload: PendingTransfer = {
      requestKey: crypto.randomUUID(),
      amountMinor,
    }
    const controller = new AbortController()
    activeController = controller
    busy.value = true
    notice.value = undefined

    try {
      let existing: PendingTransfer | undefined
      try {
        existing = await storage.reserve(payload)
      } catch {
        if (isSameAccount(tenantId) && !controller.signal.aborted) {
          notice.value = { kind: 'storage' }
        }
        return
      }
      if (!isSameAccount(tenantId) || controller.signal.aborted) return
      if (existing) {
        pending.value = existing
        return
      }
      pending.value = payload
      await executeTransfer(tenantId, storage, payload, controller)
    } finally {
      if (activeController === controller) {
        activeController = undefined
        busy.value = false
      }
    }
  }

  async function retry(): Promise<void> {
    if (busy.value) return
    const tenantId = props.tenantId
    const payload = pending.value
    if (!payload || !isSameAccount(tenantId)) return

    const storage = pendingCommissionTransfer(tenantId)
    const controller = new AbortController()
    activeController = controller
    busy.value = true
    notice.value = undefined

    try {
      await executeTransfer(tenantId, storage, payload, controller)
    } finally {
      if (activeController === controller) {
        activeController = undefined
        busy.value = false
      }
    }
  }

  return {
    amount,
    busy,
    pendingAmount,
    notice,
    submit,
    retry,
  }
}
