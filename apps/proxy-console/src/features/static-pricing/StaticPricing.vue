<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert, ElButton, ElCard, ElDialog, ElEmpty, ElInput, ElInputNumber, ElOption,
  ElRadioButton, ElRadioGroup, ElSelect, ElSkeleton,
} from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import {
  maxStaticPurchaseLines,
  maxStaticPurchaseQuantity,
  type StaticCatalogFilter,
  type StaticPurchaseLine,
  type StaticPurchaseOffer,
  type StaticPurchaseSelection,
} from '@/api/staticPricing'
import { formatYuan } from '@/shared/lib/money'
import type { ParsedStaticPurchaseIntent, StaticPurchaseIntent, StaticPurchaseSnapshot } from '@/features/checkout'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { offerKey, staticSelectionTotal, useStaticPurchase } from './useStaticPurchase'

type IntentBanner =
  | { kind: 'invalid' }
  | { kind: 'blocked', reason: 'unavailable' | 'insufficientStock' | 'closed' }
  | { kind: 'changed' }

const panelPadding = { padding: 'calc(var(--space-4) + var(--space-1))' }
const props = defineProps<{
  accountKey: string
  intent?: ParsedStaticPurchaseIntent
}>()
const emit = defineEmits<{
  'consume-intent': []
}>()
const { t, te, locale } = useI18n()
const errorMessage = useErrorMessage()
const cart = ref(new Map<string, StaticPurchaseLine>())
const intentBanner = ref<IntentBanner>()
const intentSnapshots = ref(new Map<string, StaticPurchaseSnapshot>())
let handledIntentKey = ''
let pendingIntent: StaticPurchaseIntent | undefined

const initialFilter = props.intent?.kind === 'valid' ? props.intent.intent.filter : undefined
const {
  filters, catalog, wallet, balance, checkout, busy, lastError, pendingOrderIds, loadCatalog, loadWallet,
  reviewPurchase, retryPurchase, refreshOrder, closeCheckout,
} = useStaticPurchase(props.accountKey, () => {
  cart.value.clear()
  intentBanner.value = undefined
  intentSnapshots.value = new Map()
  emit('consume-intent')
}, initialFilter)

const purposes = [
  { id: 8, key: 'staticPricing.purposeTikTok' },
  { id: 9, key: 'staticPricing.purposeSora' },
  { id: 10, key: 'staticPricing.purposeChatGPT' },
]
const productLabel = computed(() => t(filters.value.shareMode === 'exclusive' ? 'staticPricing.exclusive' : 'staticPricing.shared'))
const purposeLabel = computed(() => {
  const selectedPurpose = filters.value.shareMode === 'shared' ? filters.value.purposeId : undefined
  const purpose = purposes.find(option => option.id === selectedPurpose)
  return t(purpose?.key ?? 'staticPricing.anyPurpose')
})
const region = ref('')
const continent = ref('')
const proxyLine = ref('')
const keyword = ref('')
const term = ref('')

const offers = computed(() => catalog.value.kind === 'open' ? catalog.value.offers : [])
function catalogOptions(field: 'continent' | 'region' | 'line') {
  return [...new Set(offers.value.map(offer => offer[field]).filter(Boolean))]
    .sort((a, b) => a.localeCompare(b, locale.value))
}
const regions = computed(() => catalogOptions('region'))
const continents = computed(() => catalogOptions('continent'))
const proxyLines = computed(() => catalogOptions('line'))
const terms = computed(() => [...new Map(offers.value.map(offer => [offer.term, { code: offer.term, days: offer.termDays }])).values()]
  .sort((a, b) => a.days - b.days))
const defaultTerm = 'yp1'
const currentTerm = computed(() => {
  if (term.value && cart.value.size > 0 && !terms.value.some(option => option.code === term.value)) {
    return term.value
  }
  return (terms.value.find(option => option.code === term.value)
    ?? terms.value.find(option => option.code === defaultTerm)
    ?? terms.value[0])?.code
})
const termOffers = computed(() => offers.value.filter(offer => offer.term === currentTerm.value))
const visibleOffers = computed(() => {
  const search = keyword.value.trim().toLocaleLowerCase(locale.value)
  return termOffers.value.filter(offer => (!region.value || offer.region === region.value)
    && (!continent.value || offer.continent === continent.value)
    && (!proxyLine.value || proxyLines.value.length < 2 || offer.line === proxyLine.value)
    && (!search || [offer.region, offer.isp, offer.line].some(label => label.toLocaleLowerCase(locale.value).includes(search))))
    .sort((first, second) => Number(second.available > 0) - Number(first.available > 0))
})
const offerGroups = computed(() => {
  const groups = new Map<string, StaticPurchaseOffer[]>()
  for (const offer of visibleOffers.value) {
    const group = groups.get(offer.continent) ?? []
    group.push(offer)
    groups.set(offer.continent, group)
  }
  return [...groups]
})
const cartLines = computed(() => {
  const current = new Map(termOffers.value.map(offer => [offer.nodeId, offer]))
  return [...cart.value.values()].map(({ offer, quantity }) => {
    const available = current.get(offer.nodeId)
    return { offer: available ?? offer, quantity, available: available?.available }
  })
})
const missingOffers = computed(() => cartLines.value.some(line => line.available === undefined))
const insufficientStock = computed(() => cartLines.value.some(line => line.quantity > (line.available ?? 0)))
const selectedQuantity = computed(() => cartLines.value.reduce((total, line) => total + line.quantity, 0))
const quantityExceeded = computed(() => selectedQuantity.value > maxStaticPurchaseQuantity)
const selectedTotal = computed(() => staticSelectionTotal(cartLines.value))
const selectedShortfall = computed(() => shortfall(selectedTotal.value))
const canReview = computed(() => cartLines.value.length > 0 && !insufficientStock.value && !quantityExceeded.value
  && selectedShortfall.value === 0n && !busy.value)
const reviewLines = computed(() => {
  const state = checkout.value
  if (state.kind === 'confirm') return state.quote.lines.map(line => ({ ...line, currentOffer: line.offer }))
  return state.kind === 'blocked' ? state.rechecked ?? [] : []
})
const confirmTotal = computed(() => checkout.value.kind === 'confirm'
  ? staticSelectionTotal(checkout.value.quote.lines) : 0n)
const confirmShortfall = computed(() => shortfall(confirmTotal.value))
const dialogTitle = computed(() => {
  if (checkout.value.kind === 'result') return t('staticPricing.result')
  if (checkout.value.kind === 'unresolved') return t('staticPricing.unresolved')
  return t('staticPricing.confirm')
})
const dismissible = computed(() => !busy.value && checkout.value.kind !== 'unresolved')
const requestConflict = computed(() => checkout.value.kind === 'unresolved' && checkout.value.error instanceof ApiError
  && checkout.value.error.reason === 'WALLET_REQUEST_CONFLICT')

function sameCatalogFilter(first: StaticCatalogFilter, second: StaticCatalogFilter) {
  if (first.shareMode !== second.shareMode) return false
  return first.shareMode === 'exclusive' || first.purposeId === (second.shareMode === 'shared' ? second.purposeId : undefined)
}

function dismissIntent(clearSnapshots = false) {
  pendingIntent = undefined
  intentBanner.value = undefined
  if (clearSnapshots) intentSnapshots.value = new Map()
  emit('consume-intent')
}

async function applyPendingIntent() {
  if (!pendingIntent) return
  if (checkout.value.kind === 'unresolved') {
    handledIntentKey = pendingIntent.key
    pendingIntent = undefined
    emit('consume-intent')
    return
  }
  if (catalog.value.kind === 'loading' || catalog.value.kind === 'failed') return
  if (!sameCatalogFilter(filters.value, pendingIntent.filter)) return

  const intent = pendingIntent
  handledIntentKey = intent.key
  pendingIntent = undefined

  if (catalog.value.kind === 'closed') {
    intentBanner.value = { kind: 'blocked', reason: 'closed' }
    return
  }

  const matchingOffers = new Map(
    catalog.value.offers
      .filter(offer => offer.term === intent.term)
      .map(offer => [offer.nodeId, offer]),
  )
  const snapshots = new Map<string, StaticPurchaseSnapshot>()
  const nextCart = new Map<string, StaticPurchaseLine>()
  let hasMissing = false
  let hasInsufficientStock = false
  let hasSnapshotChange = false

  for (const line of intent.lines) {
    if (line.snapshotUnitMinor !== undefined || line.snapshotAvailable !== undefined) {
      snapshots.set(line.nodeId, { unitMinor: line.snapshotUnitMinor, available: line.snapshotAvailable })
    }
    const matched = matchingOffers.get(line.nodeId)
    if (!matched) {
      hasMissing = true
      nextCart.set(line.nodeId, {
        offer: {
          nodeId: line.nodeId,
          region: '',
          isp: '',
          continent: '',
          line: '',
          term: intent.term,
          termDays: 0,
          unitMinor: line.snapshotUnitMinor ?? 0n,
          available: 0,
        },
        quantity: line.quantity,
      })
      continue
    }
    if (matched.available < line.quantity) hasInsufficientStock = true
    if (
      (line.snapshotUnitMinor !== undefined && matched.unitMinor !== line.snapshotUnitMinor)
      || (line.snapshotAvailable !== undefined && matched.available !== line.snapshotAvailable)
    ) {
      hasSnapshotChange = true
    }
    nextCart.set(line.nodeId, { offer: matched, quantity: line.quantity })
  }

  term.value = intent.term
  cart.value = nextCart
  intentSnapshots.value = snapshots

  if (hasMissing || hasInsufficientStock) {
    intentBanner.value = { kind: 'blocked', reason: hasMissing ? 'unavailable' : 'insufficientStock' }
    return
  }

  if (hasSnapshotChange) intentBanner.value = { kind: 'changed' }
  await reviewPurchase({ term: intent.term, lines: [...nextCart.values()] }, false, snapshots)
  if (checkout.value.kind === 'confirm') {
    emit('consume-intent')
    if (checkout.value.changed) intentBanner.value = { kind: 'changed' }
  }
}

watch(() => props.intent, intent => {
  if (!intent || intent.kind === 'none') {
    pendingIntent = undefined
    return
  }
  if (intent.key === handledIntentKey) return
  if (checkout.value.kind === 'unresolved') {
    handledIntentKey = intent.key
    pendingIntent = undefined
    emit('consume-intent')
    return
  }
  if (intent.kind === 'invalid') {
    handledIntentKey = intent.key
    pendingIntent = undefined
    intentBanner.value = { kind: 'invalid' }
    emit('consume-intent')
    return
  }
  pendingIntent = intent.intent
  intentBanner.value = undefined
  if (!sameCatalogFilter(filters.value, intent.intent.filter)) {
    resetSelection()
    filters.value = intent.intent.filter
  } else if (catalog.value.kind !== 'loading') {
    void loadCatalog()
  }
}, { immediate: true })

watch(catalog, () => {
  void applyPendingIntent()
})

function setShareMode(value: string | number | boolean | undefined) {
  if (value !== 'exclusive' && value !== 'shared') return
  dismissIntent(true)
  resetSelection()
  filters.value = { shareMode: value }
}
function setPurpose(value: number) {
  dismissIntent(true)
  resetCatalogFilters()
  filters.value = { shareMode: 'shared', purposeId: value || undefined }
}
function resetSelection() {
  resetCatalogFilters()
  term.value = ''
  cart.value.clear()
  intentSnapshots.value = new Map()
}
function resetCatalogFilters() {
  region.value = ''
  continent.value = ''
  proxyLine.value = ''
  keyword.value = ''
}
function changeTerm(value: string | number | boolean | undefined) {
  if (typeof value === 'string') {
    dismissIntent(true)
    term.value = value
  }
}
function selectNode(offer: StaticPurchaseOffer) {
  if (cart.value.size >= maxStaticPurchaseLines || offer.available <= 0) return
  dismissIntent()
  term.value = offer.term
  cart.value.set(offer.nodeId, { offer, quantity: 1 })
}
function changeQuantity(offer: StaticPurchaseOffer, value: number | undefined) {
  dismissIntent()
  if (value === 0) cart.value.delete(offer.nodeId)
  if (value !== undefined && Number.isInteger(value) && value > 0 && offer.available > 0) {
    cart.value.set(offer.nodeId, { offer, quantity: Math.min(value, offer.available) })
  }
}
function removeLine(nodeId: string) {
  dismissIntent()
  cart.value.delete(nodeId)
  if (!cart.value.size && term.value && !terms.value.some(option => option.code === term.value)) {
    term.value = ''
  }
}
function clearCart() {
  dismissIntent(true)
  cart.value.clear()
  if (term.value && !terms.value.some(option => option.code === term.value)) {
    term.value = ''
  }
}
async function startReview() {
  if (!currentTerm.value || !canReview.value) return
  const snapshots = intentSnapshots.value.size ? intentSnapshots.value : undefined
  await reviewPurchase({ term: currentTerm.value, lines: cartLines.value }, false, snapshots)
  if (checkout.value.kind === 'confirm') emit('consume-intent')
}
async function retryBlockedQuote(selection: StaticPurchaseSelection) {
  const snapshots = intentSnapshots.value.size ? intentSnapshots.value : undefined
  await reviewPurchase(selection, false, snapshots)
  if (checkout.value.kind === 'confirm') emit('consume-intent')
}
function snapshotFor(nodeId: string) {
  return checkout.value.kind === 'confirm' ? checkout.value.snapshots?.get(nodeId) : undefined
}
const cartLineTitle = (offer: StaticPurchaseOffer, available: number | undefined) =>
  offer.region || (available === undefined ? t('staticPricing.nodeFallback', { id: offer.nodeId }) : offerRegion(offer))
function shortfall(total: bigint) {
  return balance.value !== undefined && balance.value < total ? total - balance.value : 0n
}
const money = (value: bigint) => formatYuan(value, locale.value)
const count = (value: number | bigint) => new Intl.NumberFormat(locale.value).format(value)
const termDays = (days: number) => t('staticPricing.termDays', { days: count(days) }, days)
const offerRegion = (offer: StaticPurchaseOffer) => offer.region || t('staticPricing.unknownRegion')
const offerLine = (offer: StaticPurchaseOffer) => [offer.isp, offer.line].filter(Boolean).join(' · ')
function checkoutError(error: unknown) {
  if (error instanceof ApiError) {
    const key = `staticPricing.errors.${error.reason}`
    if (te(key)) return t(key)
  }
  return errorMessage(error)
}
</script>

<template>
  <ElCard v-if="catalog.kind === 'closed'" class="pricing-closed" shadow="never">
    <ElEmpty :description="t('staticPricing.closed')" :image-size="80" />
    <div class="closed-actions">
      <LinkButton :to="{ name: 'pricing', params: { locale } }">{{ t('staticPricing.backToPricing') }}</LinkButton>
    </div>
  </ElCard>
  <div v-if="pendingOrderIds.length" class="pending-orders pricing-notice">
    <ElButton v-for="id in pendingOrderIds" :key="id" :disabled="busy" @click="refreshOrder(id)">
      {{ t('staticPricing.pendingOrder', { id }) }}
    </ElButton>
  </div>
  <ElAlert
    v-if="intentBanner && catalog.kind !== 'closed'"
    class="pricing-notice"
    type="warning"
    :title="t(
      intentBanner.kind === 'invalid'
        ? 'staticPricing.intentInvalid'
        : intentBanner.kind === 'changed'
          ? 'staticPricing.intentChanged'
          : intentBanner.reason === 'unavailable'
            ? 'staticPricing.intentUnavailable'
            : 'staticPricing.intentInsufficientStock',
    )"
    :closable="false"
    show-icon
  >
    <div v-if="intentBanner.kind === 'blocked' && intentBanner.reason === 'unavailable'" class="intent-alert-action">
      <LinkButton size="small" :to="{ name: 'pricing', params: { locale } }">{{ t('staticPricing.backToPricing') }}</LinkButton>
    </div>
  </ElAlert>
  <ElAlert v-if="lastError" class="pricing-notice" type="warning" :title="checkoutError(lastError)" :closable="false" show-icon />
  <div v-if="catalog.kind !== 'closed'" class="pricing">
    <ElCard shadow="never" class="pricing-catalog" :body-style="panelPadding">
      <div class="product-switch">
        <ElRadioGroup size="large" :model-value="filters.shareMode" :aria-label="t('staticPricing.product')" @change="setShareMode">
          <ElRadioButton value="exclusive">{{ t('staticPricing.exclusive') }}</ElRadioButton>
          <ElRadioButton value="shared">{{ t('staticPricing.shared') }}</ElRadioButton>
        </ElRadioGroup>
        <p class="muted product-hint">
          {{ filters.shareMode === 'exclusive' ? t('staticPricing.exclusiveHint') : t('staticPricing.sharedHint') }}
        </p>
      </div>

      <div class="catalog-heading">
        <h2>{{ t('staticPricing.chooseRegion') }}</h2>
        <ElInput v-model="keyword" class="catalog-search" :prefix-icon="Search" clearable
          :aria-label="t('staticPricing.search')" :placeholder="t('staticPricing.search')" />
      </div>
      <div class="filters">
        <label v-if="filters.shareMode === 'shared'" class="filter">
          <span>{{ t('staticPricing.purpose') }}</span>
          <ElSelect class="filter-control" :model-value="filters.purposeId ?? 0"
            :aria-label="t('staticPricing.purpose')" @change="setPurpose">
            <ElOption :value="0" :label="t('staticPricing.anyPurpose')" />
            <ElOption v-for="purpose in purposes" :key="purpose.id" :value="purpose.id" :label="t(purpose.key)" />
          </ElSelect>
          <span class="muted purpose-hint">{{ t('staticPricing.purposeHint') }}</span>
        </label>
        <div v-if="proxyLines.length > 1" class="filter">
          <span>{{ t('staticPricing.proxyLine') }}</span>
          <ElRadioGroup v-model="proxyLine" :aria-label="t('staticPricing.proxyLine')">
            <ElRadioButton value="">{{ t('staticPricing.allProxyLines') }}</ElRadioButton>
            <ElRadioButton v-for="name in proxyLines" :key="name" :value="name">{{ name }}</ElRadioButton>
          </ElRadioGroup>
        </div>
        <div v-if="continents.length" class="filter">
          <span>{{ t('staticPricing.continent') }}</span>
          <ElRadioGroup v-model="continent" :aria-label="t('staticPricing.continent')">
            <ElRadioButton value="">{{ t('staticPricing.allContinents') }}</ElRadioButton>
            <ElRadioButton v-for="name in continents" :key="name" :value="name">{{ name }}</ElRadioButton>
          </ElRadioGroup>
        </div>
        <label class="filter">
          <span>{{ t('staticPricing.region') }}</span>
          <ElSelect v-model="region" class="filter-control" :empty-values="[null, undefined]" :aria-label="t('staticPricing.region')">
            <ElOption value="" :label="t('staticPricing.allRegions')" />
            <ElOption v-for="name in regions" :key="name" :value="name" :label="name" />
          </ElSelect>
        </label>
      </div>

      <ElSkeleton v-if="catalog.kind === 'loading'" animated :rows="5" />
      <div v-else-if="catalog.kind === 'failed'" class="catalog-feedback">
        <ElAlert type="error" :title="checkoutError(catalog.error)" :closable="false" show-icon />
        <ElButton @click="loadCatalog">{{ t('staticPricing.retry') }}</ElButton>
      </div>
      <ElEmpty v-else-if="!visibleOffers.length" :description="t('staticPricing.empty')" :image-size="72" />
      <div v-else class="offer-groups">
        <section v-for="[group, items] in offerGroups" :key="group" class="offer-group">
          <h3 v-if="group || offerGroups.length > 1">{{ group || t('staticPricing.unknownContinent') }}</h3>
          <div class="offers">
            <div v-for="offer in items" :key="offerKey(offer)" class="offer"
              :class="{ 'offer--selected': cart.has(offer.nodeId), 'offer--sold-out': !offer.available }">
              <span v-if="offer.line" class="line-tag">{{ offer.line }}</span>
              <div v-if="cart.has(offer.nodeId)" class="offer-pick">
                <div class="offer-text">
                  <h3>{{ offerRegion(offer) }} <span v-if="offer.isp" class="isp">{{ offer.isp }}</span></h3>
                  <ElInputNumber :model-value="cart.get(offer.nodeId)?.quantity" :min="0" size="small"
                    :max="Math.max(cart.get(offer.nodeId)?.quantity ?? 0, offer.available)" :precision="0"
                    :disabled="!offer.available"
                    :aria-label="t('staticPricing.cardQuantity', { region: offerRegion(offer) })"
                    @change="changeQuantity(offer, $event)" />
                </div>
              </div>
              <button v-else type="button" class="offer-pick"
                :disabled="offer.available <= 0 || cart.size >= maxStaticPurchaseLines"
                :aria-label="t('staticPricing.selectOffer', { region: offerRegion(offer) })"
                @click="selectNode(offer)">
                <span class="offer-text">
                  <strong>{{ offerRegion(offer) }} <span v-if="offer.isp" class="isp">{{ offer.isp }}</span></strong>
                  <span class="offer-meta">
                    <span>{{ offer.available ? t('staticPricing.availableCount', { count: count(offer.available) }) : t('staticPricing.soldOut') }}</span>
                    <strong class="unit-price tabular">{{ t('staticPricing.pricePerIp', { price: money(offer.unitMinor) }) }}</strong>
                  </span>
                </span>
              </button>
            </div>
          </div>
        </section>
      </div>
    </ElCard>

    <ElCard shadow="never" class="pricing-summary" :body-style="panelPadding">
      <aside class="summary-content" :aria-label="t('staticPricing.summary')">
        <div class="selection summary-heading">
          <h2>{{ t('staticPricing.summaryProduct', { product: productLabel }) }}</h2>
          <ElButton link :disabled="!cart.size" @click="clearCart">{{ t('staticPricing.clear') }}</ElButton>
        </div>
        <div class="summary-terms">
          <span>{{ t('staticPricing.term') }}</span>
          <ElRadioGroup :model-value="currentTerm" :aria-label="t('staticPricing.term')" :disabled="!terms.length" @change="changeTerm">
            <ElRadioButton v-for="option in terms" :key="option.code" :value="option.code">{{ termDays(option.days) }}</ElRadioButton>
          </ElRadioGroup>
        </div>
        <div class="summary-lines">
          <section v-for="line in cartLines" :key="line.offer.nodeId" class="cart-line">
            <div class="selection">
              <div>
                <h3>{{ cartLineTitle(line.offer, line.available) }}</h3>
                <p v-if="line.offer.isp || line.offer.line" class="muted">{{ offerLine(line.offer) }}</p>
                <p v-if="line.available !== undefined" class="muted tabular">
                  {{ t('staticPricing.pricePerIp', { price: money(line.offer.unitMinor) }) }}
                </p>
              </div>
              <ElInputNumber :model-value="line.quantity" :min="0" :max="Math.max(line.quantity, line.available ?? 0)"
                size="small" :disabled="!line.available" :precision="0"
                :aria-label="cart.size === 1 ? t('staticPricing.quantity') : t('staticPricing.regionQuantity', { region: cartLineTitle(line.offer, line.available) })"
                @change="changeQuantity(line.offer, $event)" />
              <ElButton v-if="!line.available" link :aria-label="t('staticPricing.removeOffer', { region: cartLineTitle(line.offer, line.available) })"
                @click="removeLine(line.offer.nodeId)">{{ t('staticPricing.remove') }}</ElButton>
            </div>
            <p v-if="catalog.kind === 'open' && line.available === undefined" class="muted">
              {{ t('staticPricing.unavailableLineWithStock', { stock: count(0) }) }}
            </p>
            <p v-else-if="line.available !== undefined && line.quantity > line.available" class="muted">
              {{ t('staticPricing.adjustQuantity', { stock: count(line.available) }) }}
            </p>
          </section>
          <ElEmpty v-if="!cart.size" :description="t('staticPricing.selectRegion')" :image-size="56" />
          <p v-else-if="cart.size === maxStaticPurchaseLines" class="muted">{{ t('staticPricing.lineLimit', { count: count(maxStaticPurchaseLines) }) }}</p>
        </div>
        <ElAlert v-if="quantityExceeded" type="warning" :closable="false" show-icon
          :title="t('staticPricing.quantityLimit', { count: count(maxStaticPurchaseQuantity) })" />
        <dl class="summary-list summary-total tabular">
          <div><dt>{{ t('staticPricing.ipCount') }}</dt><dd>{{ count(selectedQuantity) }}</dd></div>
          <div v-if="currentTerm"><dt>{{ t('staticPricing.term') }}</dt><dd>{{ termDays(termOffers[0]?.termDays ?? 0) }}</dd></div>
          <div v-if="filters.shareMode === 'shared'"><dt>{{ t('staticPricing.purpose') }}</dt><dd>{{ purposeLabel }}</dd></div>
          <div>
            <dt>{{ t('staticPricing.balance') }}</dt>
            <dd v-if="wallet.kind === 'ready'">{{ money(wallet.balance) }}</dd>
            <dd v-else-if="wallet.kind === 'loading'" role="status">{{ t('staticPricing.loading') }}</dd>
            <dd v-else><ElButton link @click="loadWallet">{{ t('staticPricing.reloadBalance') }}</ElButton></dd>
          </div>
          <div v-if="!missingOffers"><dt>{{ t('staticPricing.total') }}</dt><dd class="grand-total">{{ money(selectedTotal) }}</dd></div>
        </dl>
        <ElAlert v-if="selectedShortfall > 0n && !insufficientStock" class="tabular" type="warning" :closable="false" show-icon
          :title="t('staticPricing.shortfall', { amount: money(selectedShortfall) })" />
        <ElButton type="primary" size="large" :disabled="!canReview" @click="startReview">{{ t('staticPricing.buy') }}</ElButton>
      </aside>
    </ElCard>
  </div>

  <ElDialog :model-value="checkout.kind !== 'closed'" :title="dialogTitle" width="var(--order-drawer-width)"
    class="static-purchase-dialog" align-center :show-close="dismissible" :close-on-press-escape="dismissible"
    :close-on-click-modal="false" @close="closeCheckout">
    <ElSkeleton v-if="checkout.kind === 'loading'" animated :rows="3" />
    <div v-else-if="checkout.kind === 'confirm' || checkout.kind === 'blocked'" class="dialog-content">
      <ElAlert v-if="checkout.kind === 'blocked'" type="warning" :title="checkoutError(checkout.error)" :closable="false" show-icon />
      <ElAlert v-else-if="checkout.changed" type="warning"
        :title="checkout.changed === 'specification' ? t('staticPricing.specificationChanged') : t('staticPricing.changed')" :closable="false" show-icon />
      <p v-if="checkout.kind === 'confirm'">{{ t('staticPricing.confirmHint') }}</p>
      <dl class="summary-list tabular">
        <div><dt>{{ t('staticPricing.product') }}</dt><dd>{{ productLabel }}</dd></div>
        <div v-if="filters.shareMode === 'shared'"><dt>{{ t('staticPricing.purpose') }}</dt><dd>{{ purposeLabel }}</dd></div>
      </dl>
      <section v-for="line in reviewLines" :key="line.offer.nodeId" class="quote-line">
        <h3>{{ offerRegion(line.offer) }}</h3>
        <p v-if="line.offer.isp || line.offer.line" class="muted">{{ offerLine(line.offer) }}</p>
        <dl v-if="line.currentOffer" class="summary-list tabular">
          <div><dt>{{ t('staticPricing.term') }}</dt><dd>{{ termDays(line.currentOffer.termDays) }}</dd></div>
          <div><dt>{{ t('staticPricing.quantity') }}</dt><dd>{{ count(line.quantity) }}</dd></div>
          <div v-if="snapshotFor(line.offer.nodeId)?.available !== undefined && snapshotFor(line.offer.nodeId)?.available !== line.currentOffer.available" class="muted">
            <dt>{{ t('staticPricing.previousStock') }}</dt><dd>{{ count(snapshotFor(line.offer.nodeId)!.available!) }}</dd>
          </div>
          <div><dt>{{ t('staticPricing.stock') }}</dt><dd>{{ count(line.currentOffer.available) }}</dd></div>
          <div v-if="snapshotFor(line.offer.nodeId)?.unitMinor !== undefined && snapshotFor(line.offer.nodeId)?.unitMinor !== line.currentOffer.unitMinor" class="muted">
            <dt>{{ t('staticPricing.previousUnitPrice') }}</dt><dd>{{ money(snapshotFor(line.offer.nodeId)!.unitMinor!) }}</dd>
          </div>
          <div><dt>{{ t('staticPricing.unitPrice') }}</dt><dd>{{ money(line.currentOffer.unitMinor) }}</dd></div>
          <div v-if="reviewLines.length > 1">
            <dt>{{ t('staticPricing.subtotal') }}</dt><dd>{{ money(line.currentOffer.unitMinor * BigInt(line.quantity)) }}</dd>
          </div>
        </dl>
        <p v-else class="muted">{{ t('staticPricing.unavailableLine') }}</p>
      </section>
      <template v-if="checkout.kind === 'confirm'">
        <dl class="summary-list tabular">
          <div v-if="balance !== undefined && confirmShortfall === 0n">
            <dt>{{ t('staticPricing.afterPayment') }}</dt><dd>{{ money(balance - confirmTotal) }}</dd>
          </div>
          <div><dt>{{ t('staticPricing.payable') }}</dt><dd class="grand-total">{{ money(confirmTotal) }}</dd></div>
        </dl>
        <ElAlert v-if="confirmShortfall > 0n" type="warning" class="tabular" :closable="false" show-icon
          :title="t('staticPricing.shortfall', { amount: money(confirmShortfall) })" />
      </template>
    </div>
    <div v-else-if="checkout.kind === 'unresolved'" class="dialog-content">
      <ElAlert type="warning" :title="requestConflict ? t('staticPricing.requestConflict') : t('staticPricing.unresolvedHint')"
        :closable="false" show-icon />
      <p v-if="checkout.error && !requestConflict">{{ checkoutError(checkout.error) }}</p>
    </div>
    <div v-else-if="checkout.kind === 'result'" class="dialog-content tabular">
      <h3>{{ t(`staticPricing.status.${checkout.order.status}`) }}</h3>
      <p>{{ t('staticPricing.orderNumber', { id: checkout.order.id }) }}</p>
      <p>{{ t('staticPricing.delivered', { count: count(checkout.order.deliveredCount) }, checkout.order.deliveredCount === 1n ? 1 : 2) }}</p>
      <p>{{ t('staticPricing.refunded', { amount: money(checkout.order.refundedMinor) }) }}</p>
      <ElAlert v-if="checkout.error" type="error" :title="checkoutError(checkout.error)" :closable="false" show-icon />
    </div>

    <template #footer>
      <div class="dialog-actions">
        <ElButton v-if="checkout.kind === 'confirm' || checkout.kind === 'blocked'" :disabled="busy" @click="closeCheckout">
          {{ t('staticPricing.back') }}
        </ElButton>
        <ElButton v-if="checkout.kind === 'confirm'" type="primary" :disabled="confirmShortfall > 0n" :loading="busy"
          @click="reviewPurchase(checkout.quote, true)">
          {{ t('staticPricing.pay') }}
        </ElButton>
        <ElButton v-if="checkout.kind === 'blocked' && checkout.retry" type="primary" :loading="busy"
          @click="retryBlockedQuote(checkout.retry)">
          {{ t('staticPricing.retryQuote') }}
        </ElButton>
        <template v-if="checkout.kind === 'unresolved'">
          <LinkButton v-if="requestConflict" :to="{ name: 'console-static-ips' }" type="primary">{{ t('staticPricing.viewIps') }}</LinkButton>
          <ElButton v-else type="primary" :loading="busy" @click="retryPurchase(checkout.body)">
            {{ t('staticPricing.retryPurchase') }}
          </ElButton>
        </template>
        <template v-if="checkout.kind === 'result'">
          <ElButton :disabled="busy" @click="closeCheckout">{{ t('staticPricing.done') }}</ElButton>
          <ElButton v-if="checkout.order.status === 'pending'" :loading="busy" @click="refreshOrder(checkout.order.id, checkout.order)">
            {{ t('staticPricing.refreshOrder') }}
          </ElButton>
          <LinkButton v-if="checkout.order.deliveredCount > 0n" :to="{ name: 'console-static-ips' }" type="primary">
            {{ t('staticPricing.viewIps') }}
          </LinkButton>
        </template>
      </div>
    </template>
  </ElDialog>
</template>

<style scoped>
.pricing {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, var(--static-checkout-width));
  align-items: start;
  gap: var(--space-4);
}
.pricing-catalog, .pricing-summary, .pricing-closed { --el-card-border-radius: var(--radius-card); min-width: 0; }
.pricing-summary { position: sticky; top: var(--space-4); }
.pricing-notice { margin-bottom: var(--space-4); }
.closed-actions { display: flex; justify-content: center; padding-bottom: var(--space-4); }
.intent-alert-action { margin-top: var(--space-2); }
.muted { color: var(--color-text-secondary); }
h2 { font-size: var(--font-size-lg); }
h3 { font-size: var(--font-size-md); }
.catalog-heading, .product-switch, .selection, .dialog-actions, .pending-orders {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}
.catalog-heading, .product-switch {
  margin-block-end: var(--space-4);
  padding-block-end: var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
}
.catalog-heading, .selection { justify-content: space-between; }
.product-hint, .offer-group > h3 { font-size: var(--font-size-sm); }
.catalog-search { width: var(--wallet-summary-card-width); max-width: 100%; }
.filters { display: grid; gap: var(--space-3); margin-bottom: var(--space-5); }
.filter { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); }
.filter > :first-child { min-width: var(--space-7); font-weight: var(--font-weight-semibold); }
.filter-control { width: calc(var(--account-name-width) + var(--space-4)); max-width: 100%; }
.purpose-hint { font-size: var(--font-size-xs); }
.offer-groups { display: grid; gap: var(--space-5); }
.offer-group { display: grid; gap: var(--space-2); }
.offer-group > h3 { color: var(--color-text-secondary); font-weight: var(--font-weight-semibold); }
.offers {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, var(--static-offer-width)), 1fr));
  gap: var(--space-3);
}
.offer {
  position: relative;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
}
.offer:not(.offer--sold-out):hover { border-color: var(--color-primary-border); }
.offer--selected {
  border-color: var(--color-primary);
  background: var(--color-surface-muted);
  box-shadow: 0 0 0 var(--border-width) var(--color-primary);
}
.offer--sold-out { opacity: .64; }
.line-tag {
  position: absolute;
  inset-block-start: 0;
  inset-inline-start: 0;
  border-radius: var(--radius-card) 0 var(--radius-control) 0;
  padding-inline: var(--space-2);
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  pointer-events: none;
}
.offer-pick {
  display: flex;
  align-items: center;
  width: 100%;
  height: 100%;
  min-height: calc(var(--space-7) + var(--space-5));
  padding: var(--space-5) var(--space-3) var(--space-3);
  border: 0;
  border-radius: inherit;
  background: none;
  color: inherit;
  font: inherit;
  text-align: start;
}
button.offer-pick { cursor: pointer; }
button.offer-pick:disabled { cursor: not-allowed; }
.offer-text { display: grid; gap: var(--space-1); min-width: 0; overflow-wrap: anywhere; }
.isp, .offer-meta { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.isp {
  padding-inline: var(--space-1);
  border-radius: var(--space-1);
  background: var(--color-canvas);
  font-weight: var(--font-weight-medium);
}
.offer-meta { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.unit-price { color: var(--color-text); font-weight: var(--font-weight-semibold); }
.summary-content, .summary-list, .catalog-feedback, .dialog-content { display: grid; gap: var(--space-3); }
.summary-content { gap: var(--space-4); }
.summary-heading { padding-block-end: var(--space-3); border-bottom: var(--border-width) solid var(--color-border); }
.summary-terms { display: grid; gap: var(--space-2); font-weight: var(--font-weight-semibold); }
.summary-terms > *, .filter > * { min-width: 0; }
.summary-lines { display: grid; align-content: center; gap: var(--space-2); min-height: calc(var(--space-7) * 2 + var(--space-5)); }
.cart-line, .quote-line { display: grid; gap: var(--space-2); }
.cart-line { font-size: var(--font-size-xs); }
.cart-line h3 { font-size: var(--font-size-sm); overflow-wrap: anywhere; }
.cart-line :is(h3, p) { max-width: 100%; }
.quote-line {
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  padding: var(--space-3);
}
.summary-list { margin: 0; }
.summary-list div { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--space-2); }
.summary-list dt { color: var(--color-text-secondary); }
.summary-list dd { margin: 0; font-weight: var(--font-weight-semibold); }
.summary-total {
  gap: var(--space-2);
  padding-top: var(--space-3);
  border-top: var(--border-width) solid var(--color-border);
  font-size: var(--font-size-sm);
}
.grand-total { color: var(--color-text); font-size: var(--font-size-title); font-weight: var(--font-weight-semibold); }
.dialog-actions { justify-content: flex-end; }
.dialog-actions > *, .pending-orders > * { margin: 0; }
@media (max-width: 950px) {
  .pricing { grid-template-columns: minmax(0, 1fr); }
  .pricing-summary { position: static; }
}
</style>

<style>
.static-purchase-dialog {
  --el-dialog-border-radius: var(--radius-card);
  --el-dialog-padding-primary: var(--space-5);
  --el-dialog-box-shadow: var(--shadow-overlay);
  max-width: calc(100vw - var(--space-4) * 2);
}
</style>
