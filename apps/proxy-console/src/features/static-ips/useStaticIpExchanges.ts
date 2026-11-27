import { ref, shallowRef } from 'vue'
import type { StaticIp } from '@/api/staticIps'
import {
  getStaticIpExchangeQuota,
  type StaticIpExchangeQuota,
  type StaticIpExchangeRecord,
  type StaticIpExchangeStatus,
  type StaticIpExchangeSubmission,
} from '@/api/staticIpExchanges'
import {
  exchangeStorageKey,
  readExchangeSubmissions,
  writeExchangeSubmissions,
  type ExchangeIntent,
} from './exchangeIntent'

export interface ExchangeEligibilityContext {
  isAvailable: boolean
  hasUnresolvedRenewal: boolean
}

export function useStaticIpExchanges(accountKey: string, signal: AbortSignal) {
  const storageKey = exchangeStorageKey(accountKey)

  const quota = shallowRef<StaticIpExchangeQuota>()
  const quotaLoading = ref(true)
  const quotaError = shallowRef<unknown>()
  const savedExchanges = shallowRef<StaticIpExchangeSubmission[]>(readExchangeSubmissions(storageKey))
  const resolvedExchanges = new Map<string, number>()
  const exchangeIntent = shallowRef<ExchangeIntent>()
  const lastExchangedId = ref<string>()

  function resolutionKey(allocationId: string, idempotencyKey: string) {
    return `${allocationId}:${idempotencyKey}`
  }

  function isResolvedStatus(status: StaticIpExchangeStatus) {
    return status === 'activating' || status === 'ready' || status === 'support_required' || status === 'failed'
  }

  async function loadQuota() {
    quotaLoading.value = true
    quotaError.value = undefined
    try {
      const next = await getStaticIpExchangeQuota(signal)
      if (!signal.aborted) quota.value = next
    } catch (error) {
      if (!signal.aborted) {
        quota.value = undefined
        quotaError.value = error
      }
    } finally {
      if (!signal.aborted) quotaLoading.value = false
    }
  }

  function recordExchangeSubmission(payload: StaticIpExchangeSubmission, unresolved: boolean) {
    const next = savedExchanges.value.filter(
      item => item.idempotency_key !== payload.idempotency_key && item.allocation_id !== payload.allocation_id,
    )
    if (unresolved) next.push(payload)
    else resolvedExchanges.delete(resolutionKey(payload.allocation_id, payload.idempotency_key))
    savedExchanges.value = next
    writeExchangeSubmissions(storageKey, next)
  }

  function noteResolvedExchange(record: StaticIpExchangeRecord, listSequence: number) {
    if (!isResolvedStatus(record.status)) return
    const hasSaved = savedExchanges.value.some(
      item => item.allocation_id === record.allocationId && item.idempotency_key === record.idempotencyKey,
    )
    if (hasSaved) resolvedExchanges.set(resolutionKey(record.allocationId, record.idempotencyKey), listSequence)
  }

  function reconcileSavedExchanges(items: StaticIp[], listSequence: number) {
    if (!savedExchanges.value.length) return
    const next = savedExchanges.value.filter(submission => {
      const match = items.find(ip => ip.id === submission.allocation_id)
      if (!match) return true
      const key = resolutionKey(submission.allocation_id, submission.idempotency_key)
      if (match.exchangeIdempotencyKey === submission.idempotency_key && match.exchangeStatus) {
        resolvedExchanges.delete(key)
        return false
      }
      const resolvedAt = resolvedExchanges.get(key)
      if (resolvedAt !== undefined && listSequence > resolvedAt) {
        resolvedExchanges.delete(key)
        return false
      }
      return true
    })
    if (next.length !== savedExchanges.value.length) {
      savedExchanges.value = next
      writeExchangeSubmissions(storageKey, next)
    }
  }

  function unresolvedExchangeFor(ip: StaticIp) {
    return savedExchanges.value.find(submission => submission.allocation_id === ip.id)
  }

  function isExchangeBlockingRenewal(ip: StaticIp) {
    return ip.exchangeStatus === 'pending'
      || ip.exchangeStatus === 'unknown'
      || Boolean(unresolvedExchangeFor(ip))
  }

  function exchangeDisabledReason(ip: StaticIp, context: ExchangeEligibilityContext): string {
    if (!context.isAvailable) return 'staticIps.exchange.disabled.expired'
    if (ip.shareMode !== 'exclusive') return 'staticIps.exchange.disabled.exclusiveOnly'
    if (!ip.nodeId) return 'staticIps.exchange.disabled.unavailable'
    if (ip.pendingOrderId || context.hasUnresolvedRenewal) return 'staticIps.exchange.disabled.renewalPending'
    if (ip.exchangeStatus === 'pending' || ip.exchangeStatus === 'unknown' || unresolvedExchangeFor(ip)) {
      return 'staticIps.exchange.disabled.exchangePending'
    }
    if (ip.exchangeStatus === 'activating') return 'staticIps.exchange.disabled.activating'
    if (quotaLoading.value) return 'staticIps.exchange.disabled.quotaLoading'
    if (quotaError.value || !quota.value) return 'staticIps.exchange.disabled.quotaError'
    if (quota.value.remaining <= 0) return 'staticIps.exchange.disabled.noQuota'
    return ''
  }

  function canExchange(ip: StaticIp, context: ExchangeEligibilityContext) {
    return !exchangeDisabledReason(ip, context)
  }

  function openExchange(ip: StaticIp, context: ExchangeEligibilityContext) {
    const unresolved = unresolvedExchangeFor(ip)
    if (unresolved) {
      resumeExchange(unresolved, ip)
      return
    }
    if (!canExchange(ip, context)) return
    exchangeIntent.value = { kind: 'exchange', ip }
  }

  function inspectExchange(ip: StaticIp) {
    const unresolved = unresolvedExchangeFor(ip)
    if (unresolved) {
      resumeExchange(unresolved, ip)
      return
    }
    if (ip.exchangeIdempotencyKey) {
      exchangeIntent.value = { kind: 'inspect', ip, idempotencyKey: ip.exchangeIdempotencyKey }
    }
  }

  function resumeExchange(submission: StaticIpExchangeSubmission, ip?: StaticIp) {
    exchangeIntent.value = { kind: 'resume', submission, ip }
  }

  return {
    quota,
    quotaLoading,
    quotaError,
    savedExchanges,
    exchangeIntent,
    lastExchangedId,
    loadQuota,
    recordExchangeSubmission,
    noteResolvedExchange,
    reconcileSavedExchanges,
    unresolvedExchangeFor,
    isExchangeBlockingRenewal,
    exchangeDisabledReason,
    canExchange,
    openExchange,
    inspectExchange,
    resumeExchange,
  }
}
