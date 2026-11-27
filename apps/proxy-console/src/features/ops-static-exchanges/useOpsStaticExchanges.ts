import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { ApiError } from '@/api/client'
import { findOpsTenant, getOpsTenant, type OpsTenant } from '@/api/opsTenants'
import {
  getOpsStaticIpExchangeQuota,
  listOpsStaticIpExchangeAudit,
  listOpsStaticIpExchangeQueue,
  listOpsTenantStaticIpExchanges,
  type OperatorStaticIpExchange,
  type OperatorStaticIpExchangeAuditEntry,
  type OperatorStaticIpExchangeQuota,
} from '@/api/opsStaticExchanges'
import { currentOperator, hasOperatorPermission } from '@/features/auth'
import { listTenantPendingIntents, type PendingExchangeIntent } from './exchangeIntent'

export interface SelectedOpsTenant {
  id: string
  name?: string
  account?: string
  email?: string
}

export type TenantSearchStatus = 'idle' | 'loading' | 'not_found' | 'error'

export interface ExchangeStatusBadge {
  labelKey: string
  type: 'success' | 'warning' | 'danger' | 'info'
}

export function canResolveExchange(item: OperatorStaticIpExchange): boolean {
  return item.inQueue && item.status === 'unknown' && !item.resolution
}

export function canCorrectExchange(item: OperatorStaticIpExchange): boolean {
  return item.resolutionConflict && !item.corrected
}

export function exchangeStatusBadge(item: OperatorStaticIpExchange): ExchangeStatusBadge {
  if (item.corrected) {
    return { labelKey: 'opsExchange.status.corrected', type: 'success' }
  }
  if (item.resolutionConflict) {
    return { labelKey: 'opsExchange.status.resolutionConflict', type: 'danger' }
  }
  if (item.status === 'unknown' && !item.resolution) {
    return item.inQueue
      ? { labelKey: 'opsExchange.status.unknownQueued', type: 'warning' }
      : { labelKey: 'opsExchange.status.unknownNotQueued', type: 'info' }
  }
  if (item.status === 'ready') {
    return { labelKey: 'opsExchange.status.ready', type: 'success' }
  }
  if (item.status === 'activating') {
    return { labelKey: 'opsExchange.status.activating', type: 'info' }
  }
  if (item.status === 'support_required') {
    return { labelKey: 'opsExchange.status.support_required', type: 'warning' }
  }
  if (item.status === 'failed') {
    return { labelKey: 'opsExchange.status.failed', type: 'danger' }
  }
  if (item.status === 'pending') {
    return { labelKey: 'opsExchange.status.pending', type: 'info' }
  }
  if (item.status === 'unknown') {
    return { labelKey: 'opsExchange.status.unknown', type: 'warning' }
  }
  return { labelKey: 'opsExchange.status.other', type: 'info' }
}

export function useOpsStaticExchanges() {
  const canRead = computed(() => hasOperatorPermission('read_operations'))
  const canWrite = computed(() => hasOperatorPermission('manage_billing'))
  const operatorUsername = computed(() => currentOperator.value?.username ?? '')

  // Tenant search and selection
  const searchQuery = ref('')
  const searchStatus = ref<TenantSearchStatus>('idle')
  const searchError = shallowRef<unknown>()
  const selectedTenant = shallowRef<SelectedOpsTenant>()

  // Queue state
  const filterQueueByTenant = ref(false)
  const queueItems = shallowRef<OperatorStaticIpExchange[]>([])
  const queueNextCursor = ref<string>()
  const queueLoading = ref(false)
  const queueLoadingMore = ref(false)
  const queueError = shallowRef<unknown>()
  const queueLoaded = ref(false)

  // Selected tenant quota state
  const quota = shallowRef<OperatorStaticIpExchangeQuota>()
  const quotaLoading = ref(false)
  const quotaError = shallowRef<unknown>()
  const quotaLoaded = ref(false)

  // Selected tenant history state
  const historyItems = shallowRef<OperatorStaticIpExchange[]>([])
  const historyNextCursor = ref<string>()
  const historyLoading = ref(false)
  const historyLoadingMore = ref(false)
  const historyError = shallowRef<unknown>()
  const historyLoaded = ref(false)

  // Selected tenant audit state
  const auditEntries = shallowRef<OperatorStaticIpExchangeAuditEntry[]>([])
  const auditNextBeforeId = ref<string>()
  const auditLoading = ref(false)
  const auditLoadingMore = ref(false)
  const auditError = shallowRef<unknown>()
  const auditLoaded = ref(false)

  // Persisted unconfirmed write intents for the selected tenant
  const pendingIntents = shallowRef<PendingExchangeIntent[]>([])

  // Independent abort controllers and monotonic sequence numbers
  let searchController: AbortController | undefined
  let profileController: AbortController | undefined
  let queueController: AbortController | undefined
  let quotaController: AbortController | undefined
  let historyController: AbortController | undefined
  let auditController: AbortController | undefined

  let searchSeq = 0
  let profileSeq = 0
  let queueSeq = 0
  let quotaSeq = 0
  let historySeq = 0
  let auditSeq = 0

  function abortAll() {
    searchController?.abort()
    profileController?.abort()
    queueController?.abort()
    quotaController?.abort()
    historyController?.abort()
    auditController?.abort()
  }

  onScopeDispose(abortAll)

  function refreshPendingIntents() {
    const tenantId = selectedTenant.value?.id
    const operator = operatorUsername.value
    pendingIntents.value = tenantId && operator ? listTenantPendingIntents(operator, tenantId) : []
  }

  function resetTenantData() {
    profileController?.abort()
    quotaController?.abort()
    historyController?.abort()
    auditController?.abort()
    profileSeq += 1
    quotaSeq += 1
    historySeq += 1
    auditSeq += 1

    quota.value = undefined
    quotaLoading.value = false
    quotaError.value = undefined
    quotaLoaded.value = false

    historyItems.value = []
    historyNextCursor.value = undefined
    historyLoading.value = false
    historyLoadingMore.value = false
    historyError.value = undefined
    historyLoaded.value = false

    auditEntries.value = []
    auditNextBeforeId.value = undefined
    auditLoading.value = false
    auditLoadingMore.value = false
    auditError.value = undefined
    auditLoaded.value = false
  }

  async function loadQueue(options: { cursor?: string } = {}) {
    if (!canRead.value) return
    queueController?.abort()
    const controller = new AbortController()
    queueController = controller
    const seq = ++queueSeq
    const isMore = Boolean(options.cursor)
    if (isMore) queueLoadingMore.value = true
    else queueLoading.value = true
    queueError.value = undefined
    try {
      const tenantId = filterQueueByTenant.value ? selectedTenant.value?.id : undefined
      const page = await listOpsStaticIpExchangeQueue({ tenantId, cursor: options.cursor }, controller.signal)
      if (controller.signal.aborted || seq !== queueSeq) return
      queueItems.value = isMore ? [...queueItems.value, ...page.exchanges] : page.exchanges
      queueNextCursor.value = page.nextCursor
      queueLoaded.value = true
    } catch (error) {
      if (controller.signal.aborted || seq !== queueSeq) return
      queueError.value = error
    } finally {
      if (!controller.signal.aborted && seq === queueSeq) {
        queueLoading.value = false
        queueLoadingMore.value = false
      }
    }
  }

  async function loadQuota(tenantId = selectedTenant.value?.id) {
    if (!canRead.value || !tenantId) return
    quotaController?.abort()
    const controller = new AbortController()
    quotaController = controller
    const seq = ++quotaSeq
    quotaLoading.value = true
    quotaError.value = undefined
    try {
      const result = await getOpsStaticIpExchangeQuota(tenantId, controller.signal)
      if (controller.signal.aborted || seq !== quotaSeq || selectedTenant.value?.id !== tenantId) return
      quota.value = result
      quotaLoaded.value = true
    } catch (error) {
      if (controller.signal.aborted || seq !== quotaSeq || selectedTenant.value?.id !== tenantId) return
      quotaError.value = error
    } finally {
      if (!controller.signal.aborted && seq === quotaSeq) {
        quotaLoading.value = false
      }
    }
  }

  async function loadHistory(options: { tenantId?: string, cursor?: string } = {}) {
    const tenantId = options.tenantId ?? selectedTenant.value?.id
    if (!canRead.value || !tenantId) return
    historyController?.abort()
    const controller = new AbortController()
    historyController = controller
    const seq = ++historySeq
    const isMore = Boolean(options.cursor)
    if (isMore) historyLoadingMore.value = true
    else historyLoading.value = true
    historyError.value = undefined
    try {
      const page = await listOpsTenantStaticIpExchanges(tenantId, { cursor: options.cursor }, controller.signal)
      if (controller.signal.aborted || seq !== historySeq || selectedTenant.value?.id !== tenantId) return
      historyItems.value = isMore ? [...historyItems.value, ...page.exchanges] : page.exchanges
      historyNextCursor.value = page.nextCursor
      historyLoaded.value = true
    } catch (error) {
      if (controller.signal.aborted || seq !== historySeq || selectedTenant.value?.id !== tenantId) return
      historyError.value = error
    } finally {
      if (!controller.signal.aborted && seq === historySeq) {
        historyLoading.value = false
        historyLoadingMore.value = false
      }
    }
  }

  async function loadAudit(options: { tenantId?: string, beforeId?: string } = {}) {
    const tenantId = options.tenantId ?? selectedTenant.value?.id
    if (!canRead.value || !tenantId) return
    auditController?.abort()
    const controller = new AbortController()
    auditController = controller
    const seq = ++auditSeq
    const isMore = Boolean(options.beforeId)
    if (isMore) auditLoadingMore.value = true
    else auditLoading.value = true
    auditError.value = undefined
    try {
      const page = await listOpsStaticIpExchangeAudit(tenantId, { beforeId: options.beforeId }, controller.signal)
      if (controller.signal.aborted || seq !== auditSeq || selectedTenant.value?.id !== tenantId) return
      auditEntries.value = isMore ? [...auditEntries.value, ...page.entries] : page.entries
      auditNextBeforeId.value = page.nextBeforeId
      auditLoaded.value = true
    } catch (error) {
      if (controller.signal.aborted || seq !== auditSeq || selectedTenant.value?.id !== tenantId) return
      auditError.value = error
    } finally {
      if (!controller.signal.aborted && seq === auditSeq) {
        auditLoading.value = false
        auditLoadingMore.value = false
      }
    }
  }

  function applySelectedTenant(next: SelectedOpsTenant, fetchProfile: boolean) {
    const changed = selectedTenant.value?.id !== next.id
    if (changed) {
      resetTenantData()
    }
    selectedTenant.value = next
    refreshPendingIntents()
    void loadQuota(next.id)
    void loadHistory({ tenantId: next.id })
    void loadAudit({ tenantId: next.id })
    if (filterQueueByTenant.value) {
      void loadQueue()
    }
    if (fetchProfile && canRead.value) {
      profileController?.abort()
      const controller = new AbortController()
      profileController = controller
      const seq = ++profileSeq
      getOpsTenant(next.id, controller.signal)
        .then(tenant => {
          if (controller.signal.aborted || seq !== profileSeq || selectedTenant.value?.id !== next.id) return
          selectedTenant.value = {
            id: tenant.id,
            name: tenant.name,
            account: tenant.account,
            email: tenant.email,
          }
        })
        .catch(() => {
          // Tenant ID is already displayed; ignore optional profile enrichment failures.
        })
    }
  }

  function selectTenant(tenant: OpsTenant) {
    searchStatus.value = 'idle'
    searchError.value = undefined
    applySelectedTenant({
      id: tenant.id,
      name: tenant.name,
      account: tenant.account,
      email: tenant.email,
    }, false)
  }

  function selectTenantById(tenantId: string) {
    const normalized = tenantId.trim()
    if (!normalized) return
    clearSearchFeedback()
    const existing = selectedTenant.value?.id === normalized ? selectedTenant.value : undefined
    applySelectedTenant(existing ?? { id: normalized }, !existing?.name)
  }

  function clearSelectedTenant() {
    clearSearchFeedback()
    selectedTenant.value = undefined
    resetTenantData()
    refreshPendingIntents()
    if (filterQueueByTenant.value) {
      filterQueueByTenant.value = false
    }
  }

  function clearSearchFeedback() {
    searchController?.abort()
    searchSeq += 1
    searchStatus.value = 'idle'
    searchError.value = undefined
  }

  async function searchTenant() {
    if (!canRead.value) return
    const term = searchQuery.value.trim()
    clearSearchFeedback()
    if (!term) return
    const controller = new AbortController()
    searchController = controller
    const seq = ++searchSeq
    searchStatus.value = 'loading'
    try {
      const tenant = await findOpsTenant(term, controller.signal)
      if (controller.signal.aborted || seq !== searchSeq) return
      selectTenant(tenant)
    } catch (error) {
      if (controller.signal.aborted || seq !== searchSeq) return
      if (error instanceof ApiError && error.status === 404) {
        searchStatus.value = 'not_found'
      } else {
        searchStatus.value = 'error'
        searchError.value = error
      }
    }
  }

  async function refreshAfterMutation() {
    refreshPendingIntents()
    const tasks: Promise<unknown>[] = [loadQueue()]
    const tenantId = selectedTenant.value?.id
    if (tenantId) {
      tasks.push(
        loadQuota(tenantId),
        loadHistory({ tenantId }),
        loadAudit({ tenantId }),
      )
    }
    await Promise.all(tasks)
  }

  watch(filterQueueByTenant, () => {
    if (canRead.value) void loadQueue()
  })

  watch(operatorUsername, () => {
    refreshPendingIntents()
  })

  watch(canRead, allowed => {
    if (allowed && !queueLoaded.value && !queueLoading.value) {
      void loadQueue()
    }
  }, { immediate: true })

  return {
    canRead,
    canWrite,
    operatorUsername,
    searchQuery,
    searchStatus,
    searchError,
    selectedTenant,
    filterQueueByTenant,
    queueItems,
    queueNextCursor,
    queueLoading,
    queueLoadingMore,
    queueError,
    queueLoaded,
    quota,
    quotaLoading,
    quotaError,
    quotaLoaded,
    historyItems,
    historyNextCursor,
    historyLoading,
    historyLoadingMore,
    historyError,
    historyLoaded,
    auditEntries,
    auditNextBeforeId,
    auditLoading,
    auditLoadingMore,
    auditError,
    auditLoaded,
    pendingIntents,
    searchTenant,
    clearSearchFeedback,
    selectTenantById,
    clearSelectedTenant,
    loadQueue,
    loadQuota,
    loadHistory,
    loadAudit,
    refreshPendingIntents,
    refreshAfterMutation,
  }
}
