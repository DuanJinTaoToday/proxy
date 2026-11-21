import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { ApiError } from '@/api/client'
import {
  getStaticPurchaseOrder,
  getStaticWalletBalance,
  isStaticPurchaseRequest,
  listStaticOffers,
  makeStaticPurchaseRequest,
  purchaseStaticIP,
  quoteStaticPurchase,
  type StaticCatalogFilter,
  type StaticPurchaseLine,
  type StaticPurchaseOffer,
  type StaticPurchaseOrder,
  type StaticPurchaseQuote,
  type StaticPurchaseRequest,
  type StaticPurchaseSelection,
} from '@/api/staticPricing'

type CatalogState =
  | { kind: 'loading' | 'closed' }
  | { kind: 'failed', error: unknown }
  | { kind: 'open', offers: StaticPurchaseOffer[] }

type RecheckedLine = StaticPurchaseLine & { currentOffer?: StaticPurchaseOffer }
type Checkout =
  | { kind: 'closed' }
  | { kind: 'loading' }
  | { kind: 'confirm', quote: StaticPurchaseQuote, changed?: 'specification' | 'priceOrStock' }
  | { kind: 'blocked', rechecked?: RecheckedLine[], retry?: StaticPurchaseSelection, error: unknown }
  | { kind: 'unresolved', body: StaticPurchaseRequest, error?: unknown }
  | { kind: 'result', order: StaticPurchaseOrder, error?: unknown }

export const offerKey = (offer: StaticPurchaseOffer) => JSON.stringify([offer.nodeId, offer.term])
export const staticSelectionTotal = (lines: StaticPurchaseLine[]) => lines.reduce(
  (total, { offer, quantity }) => total + offer.unitMinor * BigInt(quantity), 0n,
)

function restoreRequest(storageKey: string) {
  try {
    const saved: unknown = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
    return isStaticPurchaseRequest(saved) ? saved : undefined
  } catch {
    return undefined
  }
}

function storeRequest(storageKey: string, body?: StaticPurchaseRequest) {
  try {
    if (body) sessionStorage.setItem(storageKey, JSON.stringify(body))
    else sessionStorage.removeItem(storageKey)
  } catch {
    // The current dialog can still retry using the in-memory request.
  }
}

function readPendingOrders(key: string): string[] {
  try {
    const saved: unknown = JSON.parse(sessionStorage.getItem(key) ?? '[]')
    return Array.isArray(saved) ? saved.filter((id): id is string => typeof id === 'string' && /^[1-9]\d*$/.test(id)) : []
  } catch {
    return []
  }
}

const quoteConflicts = new Set([
  'STATIC_PRICE_CHANGED',
  'STATIC_STOCK_CHANGED',
  'STATIC_STOCK_INSUFFICIENT',
  'STATIC_QUOTE_EXPIRED',
  'STATIC_OFFER_UNAVAILABLE',
])
const safeToConfirmAgain = new Set([
  ...quoteConflicts,
  'PRODUCT_SALE_CLOSED',
  'WALLET_INSUFFICIENT_FUNDS',
  'INVALID_WALLET_REQUEST',
  'ACCESS_DENIED',
])

function reason(error: unknown) {
  return error instanceof ApiError ? error.reason : ''
}

export function useStaticPurchase(accountKey: string, onPurchase: () => void) {
  const storageKey = `proxy-console.static-purchase.${accountKey}`
  const pendingKey = `${storageKey}.pending-orders`
  const pendingOrderIds = shallowRef(readPendingOrders(pendingKey))
  const restored = restoreRequest(storageKey)
  const filters = ref<StaticCatalogFilter>(restored?.share_mode === 'shared'
    ? { shareMode: 'shared', purposeId: restored.purpose_id }
    : { shareMode: 'exclusive' })
  const catalog = shallowRef<CatalogState>({ kind: 'loading' })
  const wallet = shallowRef<{ kind: 'loading' } | { kind: 'ready', balance: bigint } | { kind: 'failed', error: unknown }>({ kind: 'loading' })
  const checkout = shallowRef<Checkout>(restored ? { kind: 'unresolved', body: restored } : { kind: 'closed' })
  const busy = ref(false)
  const lastError = shallowRef<unknown>()
  const lifetime = new AbortController()
  let catalogRequest: AbortController | undefined
  let walletRequest: AbortController | undefined
  onScopeDispose(() => {
    lifetime.abort()
    catalogRequest?.abort()
    walletRequest?.abort()
  })
  const signal = lifetime.signal

  const balance = computed(() => wallet.value.kind === 'ready' ? wallet.value.balance : undefined)

  async function loadWallet() {
    walletRequest?.abort()
    const request = new AbortController()
    walletRequest = request
    wallet.value = { kind: 'loading' }
    try {
      const available = await getStaticWalletBalance(request.signal)
      if (!request.signal.aborted) wallet.value = { kind: 'ready', balance: available }
    } catch (error) {
      if (!request.signal.aborted) wallet.value = { kind: 'failed', error }
    }
  }

  async function loadCatalog() {
    catalogRequest?.abort()
    const request = new AbortController()
    catalogRequest = request
    catalog.value = { kind: 'loading' }
    try {
      const data = await listStaticOffers(filters.value, request.signal)
      if (!request.signal.aborted) catalog.value = data.open ? { kind: 'open', offers: data.offers } : { kind: 'closed' }
    } catch (error) {
      if (!request.signal.aborted) catalog.value = { kind: 'failed', error }
    }
  }

  function updateOffers(lines: StaticPurchaseLine[]) {
    if (catalog.value.kind !== 'open') return
    const updates = new Map(lines.map(({ offer }) => [offerKey(offer), offer]))
    catalog.value = { kind: 'open', offers: catalog.value.offers.map(offer => updates.get(offerKey(offer)) ?? offer) }
  }

  async function refreshBlockedOffer(review: StaticPurchaseSelection, error: unknown, filter: StaticCatalogFilter) {
    let rechecked: RecheckedLine[] | undefined
    let retry: StaticPurchaseSelection | undefined = review
    if (quoteConflicts.has(reason(error))) {
      try {
        const current = await listStaticOffers(filter, signal)
        if (signal.aborted) return
        catalog.value = current.open ? { kind: 'open', offers: current.offers } : { kind: 'closed' }
        const offers = new Map(current.offers.map(offer => [offerKey(offer), offer]))
        rechecked = review.lines.map(line => ({ ...line, currentOffer: offers.get(offerKey(line.offer)) }))
        const valid = rechecked.flatMap(({ currentOffer, quantity }) => currentOffer && currentOffer.available >= quantity
          ? [{ offer: currentOffer, quantity }] : [])
        retry = valid.length === review.lines.length ? { term: review.term, lines: valid } : undefined
      } catch {
        if (signal.aborted) return
      }
    }
    if (reason(error) === 'PRODUCT_SALE_CLOSED') {
      catalog.value = { kind: 'closed' }
      retry = undefined
    }
    checkout.value = { kind: 'blocked', rechecked, retry, error }
  }

  function showOrder(order: StaticPurchaseOrder) {
    const pending = new Set(pendingOrderIds.value)
    if (order.status === 'pending') pending.add(order.id)
    else pending.delete(order.id)
    pendingOrderIds.value = [...pending]
    try {
      if (pending.size) sessionStorage.setItem(pendingKey, JSON.stringify(pendingOrderIds.value))
      else sessionStorage.removeItem(pendingKey)
    } catch {
      // Pending results remain available while the page stays open.
    }
    checkout.value = { kind: 'result', order }
  }

  async function runPurchase(body: StaticPurchaseRequest, review?: StaticPurchaseSelection) {
    storeRequest(storageKey, body)
    try {
      const order = await purchaseStaticIP(body, signal)
      if (signal.aborted) return
      storeRequest(storageKey)
      showOrder(order)
      onPurchase()
      void loadWallet()
      void loadCatalog()
    } catch (error) {
      if (signal.aborted) return
      if (!safeToConfirmAgain.has(reason(error))) {
        checkout.value = { kind: 'unresolved', body, error }
        return
      }
      storeRequest(storageKey)
      if (reason(error) === 'WALLET_INSUFFICIENT_FUNDS') void loadWallet()
      if (review) {
        await refreshBlockedOffer(review, error, filters.value)
      } else {
        checkout.value = { kind: 'closed' }
        lastError.value = error
        void loadCatalog()
      }
    }
  }

  async function reviewPurchase(review: StaticPurchaseSelection, confirm = false) {
    if (busy.value) return
    const filter = filters.value
    lastError.value = undefined
    busy.value = true
    if (!confirm) checkout.value = { kind: 'loading' }
    try {
      const quote = await quoteStaticPurchase(review, filter, signal)
      if (signal.aborted) return
      updateOffers(quote.lines)
      const specificationChanged = quote.lines.some(({ offer }, index) => {
        const previous = review.lines[index]?.offer
        return offer.termDays !== previous?.termDays || offer.region !== previous?.region || offer.isp !== previous?.isp
          || offer.continent !== previous?.continent || offer.line !== previous?.line
      })
      const priceOrStockChanged = quote.lines.some(({ offer }, index) => {
        const previous = review.lines[index]?.offer
        return offer.unitMinor !== previous?.unitMinor || offer.available !== previous?.available
      })
      if (!confirm || specificationChanged || priceOrStockChanged || (balance.value !== undefined && balance.value < staticSelectionTotal(quote.lines))) {
        checkout.value = { kind: 'confirm', quote, changed: specificationChanged ? 'specification' : priceOrStockChanged ? 'priceOrStock' : undefined }
      } else {
        await runPurchase(makeStaticPurchaseRequest(quote, filter), quote)
      }
    } catch (error) {
      if (!signal.aborted) await refreshBlockedOffer(review, error, filter)
    } finally {
      busy.value = false
    }
  }

  async function retryPurchase(body: StaticPurchaseRequest) {
    if (busy.value) return
    busy.value = true
    try {
      await runPurchase(body)
    } finally {
      busy.value = false
    }
  }

  async function refreshOrder(id: string, previous?: StaticPurchaseOrder) {
    if (busy.value) return
    busy.value = true
    lastError.value = undefined
    if (!previous) checkout.value = { kind: 'loading' }
    try {
      const current = await getStaticPurchaseOrder(id, signal)
      if (!signal.aborted) {
        showOrder(current)
        void loadWallet()
      }
    } catch (error) {
      if (signal.aborted) return
      if (previous) {
        checkout.value = { kind: 'result', order: previous, error }
      } else {
        checkout.value = { kind: 'closed' }
        lastError.value = error
      }
    } finally {
      busy.value = false
    }
  }

  function closeCheckout() {
    if (!busy.value && checkout.value.kind !== 'unresolved') checkout.value = { kind: 'closed' }
  }

  watch(filters, loadCatalog, { immediate: true })
  void loadWallet()

  return {
    filters, catalog, wallet, balance, checkout, busy, lastError, pendingOrderIds, loadCatalog, loadWallet,
    reviewPurchase, retryPurchase, refreshOrder, closeCheckout,
  }
}
