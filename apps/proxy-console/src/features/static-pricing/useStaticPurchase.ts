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
  type StaticPurchaseOffer,
  type StaticPurchaseOrder,
  type StaticPurchaseQuote,
} from '@/api/staticPricing'

type CatalogState =
  | { kind: 'loading' | 'closed' }
  | { kind: 'failed', error: unknown }
  | { kind: 'open', offers: StaticPurchaseOffer[] }

type PurchaseRequest = ReturnType<typeof makeStaticPurchaseRequest>
type Review = { offer: StaticPurchaseOffer, quantity: number }
type Checkout =
  | { kind: 'closed' }
  | { kind: 'loading' }
  | { kind: 'confirm', quote: StaticPurchaseQuote, changed: boolean }
  | { kind: 'blocked', currentOffer?: StaticPurchaseOffer, review: Review, error: unknown }
  | { kind: 'unresolved', body: PurchaseRequest, error?: unknown }
  | { kind: 'result', order: StaticPurchaseOrder, error?: unknown }

export const offerKey = (offer: StaticPurchaseOffer) => JSON.stringify([offer.nodeId, offer.term])

function restoreRequest(storageKey: string) {
  try {
    const saved: unknown = JSON.parse(sessionStorage.getItem(storageKey) ?? 'null')
    return isStaticPurchaseRequest(saved) ? saved : undefined
  } catch {
    return undefined
  }
}

function storeRequest(storageKey: string, body?: PurchaseRequest) {
  try {
    if (body) sessionStorage.setItem(storageKey, JSON.stringify(body))
    else sessionStorage.removeItem(storageKey)
  } catch {
    // The current dialog can still retry using the in-memory request.
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

export function useStaticPurchase(accountKey: string) {
  const storageKey = `proxy-console.static-purchase.${accountKey}`
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

  function updateOffer(offer: StaticPurchaseOffer) {
    if (catalog.value.kind !== 'open') return
    const key = offerKey(offer)
    catalog.value = { kind: 'open', offers: catalog.value.offers.map(old => offerKey(old) === key ? offer : old) }
  }

  async function refreshBlockedOffer(review: Review, error: unknown, filter: StaticCatalogFilter) {
    let currentOffer: StaticPurchaseOffer | undefined
    if (quoteConflicts.has(reason(error))) {
      try {
        const current = await listStaticOffers(filter, signal)
        if (signal.aborted) return
        catalog.value = current.open ? { kind: 'open', offers: current.offers } : { kind: 'closed' }
        currentOffer = current.offers.find(offer => offerKey(offer) === offerKey(review.offer))
      } catch {
        if (signal.aborted) return
      }
    }
    if (reason(error) === 'PRODUCT_SALE_CLOSED') catalog.value = { kind: 'closed' }
    checkout.value = { kind: 'blocked', review, currentOffer, error }
  }

  async function runPurchase(body: PurchaseRequest, review?: Review) {
    storeRequest(storageKey, body)
    try {
      const order = await purchaseStaticIP(body, signal)
      if (signal.aborted) return
      storeRequest(storageKey)
      checkout.value = { kind: 'result', order }
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

  async function reviewPurchase(review: Review, confirm = false) {
    if (busy.value) return
    const filter = filters.value
    lastError.value = undefined
    busy.value = true
    if (!confirm) checkout.value = { kind: 'loading' }
    try {
      const quote = await quoteStaticPurchase({ ...review, filter, signal })
      if (signal.aborted) return
      updateOffer(quote.offer)
      const changed = quote.offer.unitMinor !== review.offer.unitMinor || quote.offer.available !== review.offer.available
      if (!confirm || changed) {
        checkout.value = { kind: 'confirm', quote, changed }
      } else if (balance.value !== undefined && balance.value < quote.offer.unitMinor * BigInt(quote.quantity)) {
        checkout.value = { kind: 'confirm', quote, changed: false }
      } else {
        await runPurchase(makeStaticPurchaseRequest(quote, filter), quote)
      }
    } catch (error) {
      if (!signal.aborted) await refreshBlockedOffer(review, error, filter)
    } finally {
      busy.value = false
    }
  }

  async function retryPurchase(body: PurchaseRequest) {
    if (busy.value) return
    busy.value = true
    try {
      await runPurchase(body)
    } finally {
      busy.value = false
    }
  }

  async function refreshOrder(order: StaticPurchaseOrder) {
    if (busy.value) return
    busy.value = true
    try {
      const current = await getStaticPurchaseOrder(order.id, signal)
      if (!signal.aborted) {
        checkout.value = { kind: 'result', order: current }
        void loadWallet()
      }
    } catch (error) {
      if (!signal.aborted) checkout.value = { kind: 'result', order, error }
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
    filters, catalog, wallet, balance, checkout, busy, lastError, loadCatalog, loadWallet,
    reviewPurchase, retryPurchase, refreshOrder, closeCheckout,
  }
}
