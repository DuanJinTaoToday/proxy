import { computed, onScopeDispose, ref, watch } from 'vue'
import { getAccount } from '@/api/access'
import { ApiError } from '@/api/client'
import { getOverview } from '@/api/dashboard'
import { getTrafficCatalog, type TrafficPackage } from '@/api/products'
import { purchaseTraffic, type TrafficPurchaseRequest } from '@/api/trafficPurchase'
import { sessionEmail } from '@/features/auth'
import { pendingTrafficPurchase } from './pendingTrafficPurchase'
import type { DynamicPurchaseIntent, ParsedDynamicPurchaseIntent } from './purchaseIntent'

const rejectedReasons = new Set([
  'INVALID_WALLET_REQUEST', 'WALLET_REQUEST_CONFLICT', 'WALLET_INSUFFICIENT_FUNDS', 'WALLET_BALANCE_LIMIT',
  'PACKAGE_NOT_FOUND', 'PACKAGE_CHANGED', 'PACKAGE_UNAVAILABLE', 'PRODUCT_SALE_CLOSED',
])

export type DynamicIntentNotice =
  | { kind: 'invalid' }
  | { kind: 'unavailable', packageId: string }
  | { kind: 'closed', packageId: string }
  | { kind: 'changed', packageId: string, previousPriceMinor?: bigint, currentPriceMinor: bigint, versionChanged: boolean }
  | { kind: 'restored', packageId: string }

export interface TrafficCheckoutOptions {
  intent: () => ParsedDynamicPurchaseIntent
  onConsume?: () => void
}

export function useTrafficCheckout(options: TrafficCheckoutOptions) {
  const { intent: intentGetter, onConsume } = options

  const accountEmail = sessionEmail.value
  const initialParsed = intentGetter()
  const selectedId = ref(initialParsed.kind === 'valid' ? initialParsed.intent.packageId : undefined)
  const missingIntentPackageId = ref<string>()
  const intentNotice = ref<DynamicIntentNotice>()
  const data = ref<{
    catalog: Awaited<ReturnType<typeof getTrafficCatalog>>
    walletMinor: bigint
    trafficBytes: bigint
  }>()
  const pending = ref<TrafficPurchaseRequest>()
  const confirmedTier = ref<TrafficPackage>()
  const receipt = ref<Awaited<ReturnType<typeof purchaseTraffic>>>()
  const loadError = ref<unknown>()
  const rejection = ref<string>()
  const loading = ref(false)
  const submitting = ref(false)
  let controller: AbortController | undefined
  let storage: ReturnType<typeof pendingTrafficPurchase> | undefined
  let disposed = false
  let handledIntentKey = ''
  let pendingIntent: DynamicPurchaseIntent | undefined

  const selected = computed(() => {
    const packages = data.value?.catalog.packages ?? []
    if (missingIntentPackageId.value) return undefined
    return packages.find(tier => tier.id === selectedId.value) ?? packages[1] ?? packages[0]
  })

  function applyPendingIntent() {
    if (!pendingIntent) return
    if (pending.value) {
      handledIntentKey = pendingIntent.key
      pendingIntent = undefined
      onConsume?.()
      return
    }
    if (!data.value) return

    const intent = pendingIntent
    handledIntentKey = intent.key
    pendingIntent = undefined

    if (!data.value.catalog.open || !data.value.catalog.canPurchase) {
      intentNotice.value = { kind: 'closed', packageId: intent.packageId }
      return
    }

    const matched = data.value.catalog.packages.find(tier => tier.id === intent.packageId)
    if (!matched) {
      selectedId.value = undefined
      missingIntentPackageId.value = intent.packageId
      intentNotice.value = { kind: 'unavailable', packageId: intent.packageId }
      return
    }

    missingIntentPackageId.value = undefined
    selectedId.value = matched.id
    const priceChanged = intent.snapshotPriceMinor !== undefined && intent.snapshotPriceMinor !== matched.priceMinor
    const versionChanged = intent.snapshotVersion !== undefined && intent.snapshotVersion !== matched.version
    if (priceChanged || versionChanged) {
      intentNotice.value = {
        kind: 'changed',
        packageId: matched.id,
        previousPriceMinor: priceChanged ? intent.snapshotPriceMinor : undefined,
        currentPriceMinor: matched.priceMinor,
        versionChanged,
      }
    } else {
      intentNotice.value = { kind: 'restored', packageId: matched.id }
    }
    onConsume?.()
  }

  function selectPackage(id: string) {
    pendingIntent = undefined
    missingIntentPackageId.value = undefined
    selectedId.value = id
    intentNotice.value = undefined
    onConsume?.()
  }

  async function refresh() {
    controller?.abort()
    const request = new AbortController()
    controller = request
    loading.value = true
    loadError.value = undefined
    try {
      const [{ tenantId }, catalog, overview] = await Promise.all([
        getAccount(request.signal).then(tenant => {
          if (!request.signal.aborted) {
            storage = pendingTrafficPurchase(tenant.tenantId)
            pending.value = storage.read()
            if (pending.value) intentNotice.value = undefined
          }
          return tenant
        }),
        getTrafficCatalog(request.signal),
        getOverview(request.signal),
      ])
      if (request.signal.aborted) return
      data.value = { catalog, walletMinor: overview.walletMinor, trafficBytes: overview.trafficBytes }
      applyPendingIntent()
      return tenantId
    } catch (error) {
      if (!request.signal.aborted) loadError.value = error
    } finally {
      if (controller === request) loading.value = false
    }
  }

  watch(intentGetter, parsed => {
    if (parsed.kind === 'none') {
      pendingIntent = undefined
      return
    }
    if (parsed.key === handledIntentKey) return
    if (pending.value) {
      handledIntentKey = parsed.key
      pendingIntent = undefined
      onConsume?.()
      return
    }
    if (parsed.kind === 'invalid') {
      handledIntentKey = parsed.key
      pendingIntent = undefined
      missingIntentPackageId.value = undefined
      intentNotice.value = { kind: 'invalid' }
      onConsume?.()
      return
    }
    pendingIntent = parsed.intent
    intentNotice.value = undefined
    if (data.value && !loading.value) {
      void refresh()
    }
  }, { immediate: true })

  async function openConfirmation() {
    const tier = selected.value
    if (!tier || !data.value || !data.value.catalog.canPurchase || loading.value || submitting.value || pending.value) return
    rejection.value = undefined
    const tenantId = await refresh()
    if (!tenantId || pending.value || !data.value) return
    const current = data.value.catalog.packages.find(item => item.id === tier.id)
    if (!data.value.catalog.canPurchase || current?.version !== tier.version) {
      rejection.value = 'PACKAGE_CHANGED'
      return
    }
    confirmedTier.value = current
  }

  async function submit() {
    if (loading.value || submitting.value || !storage || sessionEmail.value !== accountEmail) return
    const attemptStorage = storage
    let payload = pending.value
    if (!payload) {
      const tier = confirmedTier.value
      if (!tier || !data.value || data.value.walletMinor < tier.priceMinor) return
      payload = { package_id: tier.id, package_version: tier.version, request_key: crypto.randomUUID() }
      submitting.value = true
      try {
        const existing = await attemptStorage.reserve(payload)
        if (existing) {
          pending.value = existing
          confirmedTier.value = undefined
          submitting.value = false
          return
        }
      } catch (error) {
        loadError.value = error
        confirmedTier.value = undefined
        submitting.value = false
        return
      }
      pending.value = payload
    }

    submitting.value = true
    rejection.value = undefined
    receipt.value = undefined
    try {
      if (disposed || sessionEmail.value !== accountEmail) return
      try {
        receipt.value = await purchaseTraffic(payload)
        intentNotice.value = undefined
        onConsume?.()
      } catch (error) {
        if (!(error instanceof ApiError) || !rejectedReasons.has(error.reason)) return
        rejection.value = error.reason
      }
      try {
        await attemptStorage.clear(payload)
        pending.value = undefined
      } catch {
        // Keep the original request available if the browser cannot clear it.
      }
    } finally {
      confirmedTier.value = undefined
      if (!disposed) await refresh()
      submitting.value = false
    }
  }

  onScopeDispose(() => {
    disposed = true
    controller?.abort()
  })
  void refresh()

  return {
    confirmedTier, data, intentNotice, loadError, loading, openConfirmation, pending, receipt, refresh, rejection,
    selectPackage, selected, selectedId, submit, submitting,
  }
}
