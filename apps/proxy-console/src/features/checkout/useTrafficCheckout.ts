import { computed, onScopeDispose, ref } from 'vue'
import { getAccount } from '@/api/access'
import { ApiError } from '@/api/client'
import { getOverview } from '@/api/dashboard'
import { getTrafficCatalog, type TrafficPackage } from '@/api/products'
import { purchaseTraffic, type TrafficPurchaseRequest } from '@/api/trafficPurchase'
import { pendingTrafficPurchase } from './pendingTrafficPurchase'

const rejectedReasons = new Set([
  'INVALID_WALLET_REQUEST', 'WALLET_REQUEST_CONFLICT', 'WALLET_INSUFFICIENT_FUNDS', 'WALLET_BALANCE_LIMIT',
  'PACKAGE_NOT_FOUND', 'PACKAGE_CHANGED', 'PACKAGE_UNAVAILABLE', 'PRODUCT_SALE_CLOSED',
])

export function useTrafficCheckout(initialPackageId: string | undefined) {
  const selectedId = ref(initialPackageId)
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

  const selected = computed(() => {
    const packages = data.value?.catalog.packages ?? []
    return packages.find(tier => tier.id === selectedId.value) ?? packages[1] ?? packages[0]
  })

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
          }
          return tenant
        }),
        getTrafficCatalog(request.signal),
        getOverview(request.signal),
      ])
      if (request.signal.aborted) return
      data.value = { catalog, walletMinor: overview.walletMinor, trafficBytes: overview.trafficBytes }
      return tenantId
    } catch (error) {
      if (!request.signal.aborted) loadError.value = error
    } finally {
      if (controller === request) loading.value = false
    }
  }

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
    if (loading.value || submitting.value || !storage) return
    const attemptStorage = storage
    let payload = pending.value
    if (!payload) {
      const tier = confirmedTier.value
      if (!tier || !data.value || data.value.walletMinor < tier.priceMinor) return
      payload = { package_id: tier.id, package_version: tier.version, request_key: crypto.randomUUID() }
      try {
        attemptStorage.save(payload)
      } catch (error) {
        loadError.value = error
        confirmedTier.value = undefined
        return
      }
      pending.value = payload
    }

    submitting.value = true
    rejection.value = undefined
    receipt.value = undefined
    try {
      const order = await purchaseTraffic(payload)
      attemptStorage.clear(payload)
      pending.value = undefined
      receipt.value = order
    } catch (error) {
      if (error instanceof ApiError && rejectedReasons.has(error.reason)) {
        attemptStorage.clear(payload)
        pending.value = undefined
        rejection.value = error.reason
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

  return { confirmedTier, data, loadError, loading, openConfirmation, pending, receipt, refresh, rejection, selected, selectedId, submit, submitting }
}
