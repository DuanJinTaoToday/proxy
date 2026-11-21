<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert, ElButton, ElCard, ElDialog, ElEmpty, ElInput, ElInputNumber, ElOption,
  ElRadioButton, ElRadioGroup, ElSelect, ElSkeleton, ElTag,
} from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { maxStaticPurchaseLines, maxStaticPurchaseQuantity, type StaticPurchaseLine, type StaticPurchaseOffer } from '@/api/staticPricing'
import { formatYuan } from '@/shared/lib/formatYuan'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { offerKey, staticSelectionTotal, useStaticPurchase } from './useStaticPurchase'

const props = defineProps<{ accountKey: string }>()
const { t, te, locale } = useI18n()
const errorMessage = useErrorMessage()
const {
  filters, catalog, wallet, balance, checkout, busy, lastError, loadCatalog, loadWallet,
  reviewPurchase, retryPurchase, refreshOrder, closeCheckout,
} = useStaticPurchase(props.accountKey)

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
const keyword = ref('')
const term = ref('')
const cart = ref(new Map<string, StaticPurchaseLine>())

const offers = computed(() => catalog.value.kind === 'open' ? catalog.value.offers : [])
const regions = computed(() => [...new Set(offers.value.map(offer => offer.region).filter(Boolean))]
  .sort((a, b) => a.localeCompare(b, locale.value)))
const terms = computed(() => [...new Map(offers.value.map(offer => [offer.term, { code: offer.term, days: offer.termDays }])).values()]
  .sort((a, b) => a.days - b.days))
const currentTerm = computed(() => term.value || terms.value[0]?.code)
const termOffers = computed(() => offers.value.filter(offer => offer.term === currentTerm.value))
const visibleOffers = computed(() => {
  const search = keyword.value.trim().toLocaleLowerCase(locale.value)
  return termOffers.value.filter(offer => (!region.value || offer.region === region.value)
    && (!search || [offer.region, offer.isp].some(label => label.toLocaleLowerCase(locale.value).includes(search))))
    .sort((first, second) => Number(second.available > 0) - Number(first.available > 0))
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
const quantityExceeded = computed(() => cartLines.value.reduce((total, line) => total + line.quantity, 0) > maxStaticPurchaseQuantity)
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

watch(() => checkout.value.kind, kind => {
  if (kind === 'result') cart.value.clear()
})

function setShareMode(value: string | number | boolean | undefined) {
  if (value !== 'exclusive' && value !== 'shared') return
  resetSelection()
  filters.value = { shareMode: value }
}
function setPurpose(value: number) {
  filters.value = { shareMode: 'shared', purposeId: value || undefined }
}
function resetSelection() {
  region.value = ''
  keyword.value = ''
  term.value = ''
  cart.value.clear()
}
function changeTerm(value: string) {
  term.value = value
}
function selectNode(offer: StaticPurchaseOffer) {
  if (cart.value.delete(offer.nodeId)) return
  if (cart.value.size >= maxStaticPurchaseLines || offer.available <= 0) return
  term.value = offer.term
  cart.value.set(offer.nodeId, { offer, quantity: 1 })
}
function changeQuantity(line: StaticPurchaseLine, value: number | undefined) {
  if (value !== undefined && Number.isInteger(value) && value > 0 && line.offer.available > 0) {
    cart.value.set(line.offer.nodeId, { offer: line.offer, quantity: Math.min(value, line.offer.available) })
  }
}
function startReview() {
  if (currentTerm.value && canReview.value) void reviewPurchase({ term: currentTerm.value, lines: cartLines.value })
}
function shortfall(total: bigint) {
  return balance.value !== undefined && balance.value < total ? total - balance.value : 0n
}
const money = (value: bigint) => formatYuan(value, locale.value)
const count = (value: number | bigint) => new Intl.NumberFormat(locale.value).format(value)
const termDays = (days: number) => t('staticPricing.termDays', { days: count(days) }, days)
const offerRegion = (offer: StaticPurchaseOffer) => offer.region || t('staticPricing.unknownRegion')
function checkoutError(error: unknown) {
  if (error instanceof ApiError) {
    const key = `staticPricing.errors.${error.reason}`
    if (te(key)) return t(key)
  }
  return errorMessage(error)
}
</script>

<template>
  <ElCard v-if="catalog.kind === 'closed'" shadow="never">
    <ElEmpty :description="t('staticPricing.closed')" />
  </ElCard>
  <ElAlert v-if="lastError" class="pricing-notice" type="warning" :title="checkoutError(lastError)" :closable="false" show-icon />
  <div v-if="catalog.kind !== 'closed'" class="pricing">
    <ElCard shadow="never" class="pricing-catalog">
      <ElRadioGroup :model-value="filters.shareMode" :aria-label="t('staticPricing.product')" @change="setShareMode">
        <ElRadioButton value="exclusive">{{ t('staticPricing.exclusive') }}</ElRadioButton>
        <ElRadioButton value="shared">{{ t('staticPricing.shared') }}</ElRadioButton>
      </ElRadioGroup>
      <p class="muted product-hint">
        {{ filters.shareMode === 'exclusive' ? t('staticPricing.exclusiveHint') : t('staticPricing.sharedHint') }}
      </p>

      <div class="catalog-heading">
        <h2>{{ t('staticPricing.chooseRegion') }}</h2>
        <ElInput v-model="keyword" class="filter-control" :prefix-icon="Search" clearable
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
        </label>
        <label class="filter">
          <span>{{ t('staticPricing.region') }}</span>
          <ElSelect v-model="region" class="filter-control" :aria-label="t('staticPricing.region')">
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
      <ElEmpty v-else-if="!visibleOffers.length" :description="t('staticPricing.empty')" />
      <div v-else class="offers">
        <ElCard v-for="offer in visibleOffers" :key="offerKey(offer)" shadow="never"
          class="offer" :class="{ 'offer--selected': cart.has(offer.nodeId) }">
          <div class="offer-heading">
            <div>
              <h3>{{ offerRegion(offer) }}</h3>
              <p v-if="offer.isp" class="muted">{{ offer.isp }}</p>
            </div>
            <ElTag v-if="!offer.available" type="info">{{ t('staticPricing.soldOut') }}</ElTag>
          </div>
          <dl class="summary-list tabular">
            <div><dt>{{ t('staticPricing.unitPrice') }}</dt><dd>{{ money(offer.unitMinor) }}</dd></div>
            <div><dt>{{ t('staticPricing.stock') }}</dt><dd>{{ count(offer.available) }}</dd></div>
          </dl>
          <ElButton :type="cart.has(offer.nodeId) ? 'primary' : 'default'" plain
            :disabled="!cart.has(offer.nodeId) && (offer.available <= 0 || cart.size >= maxStaticPurchaseLines)"
            :aria-pressed="cart.has(offer.nodeId)"
            :aria-label="t('staticPricing.selectOffer', { region: offerRegion(offer) })"
            @click="selectNode(offer)">
            {{ cart.has(offer.nodeId) ? t('staticPricing.selected') : t('staticPricing.select') }}
          </ElButton>
        </ElCard>
      </div>
    </ElCard>

    <ElCard shadow="never" class="pricing-summary">
      <template #header>
        <div class="selection">
          <h2>{{ t('staticPricing.summary') }}</h2>
          <ElButton v-if="cart.size" text @click="cart.clear()">{{ t('staticPricing.clear') }}</ElButton>
        </div>
      </template>
      <div class="catalog-feedback">
        <label class="filter">
          <span>{{ t('staticPricing.term') }}</span>
          <ElSelect :model-value="currentTerm" :aria-label="t('staticPricing.term')" :disabled="!terms.length" @change="changeTerm">
            <ElOption v-for="option in terms" :key="option.code" :value="option.code"
              :label="termDays(option.days)" />
          </ElSelect>
        </label>
        <p v-if="filters.shareMode === 'shared'" class="muted">{{ t('staticPricing.summaryPurpose', { purpose: purposeLabel }) }}</p>
        <section v-for="line in cartLines" :key="line.offer.nodeId" class="cart-line">
          <div class="selection">
            <div>
              <h3>{{ offerRegion(line.offer) }}</h3>
              <p v-if="line.offer.isp" class="muted">{{ line.offer.isp }}</p>
            </div>
            <ElButton text :aria-label="t('staticPricing.removeOffer', { region: offerRegion(line.offer) })"
              @click="cart.delete(line.offer.nodeId)">{{ t('staticPricing.remove') }}</ElButton>
          </div>
          <label class="filter">
            <span>{{ t('staticPricing.quantity') }}</span>
            <ElInputNumber :model-value="line.quantity" :min="1" :max="Math.max(line.quantity, line.available ?? 0)"
              :disabled="!line.available" :precision="0"
              :aria-label="cart.size === 1 ? t('staticPricing.quantity') : t('staticPricing.regionQuantity', { region: offerRegion(line.offer) })"
              @change="changeQuantity(line, $event)" />
          </label>
          <dl v-if="line.available !== undefined" class="summary-list tabular">
            <div><dt>{{ t('staticPricing.subtotal') }}</dt><dd>{{ money(line.offer.unitMinor * BigInt(line.quantity)) }}</dd></div>
          </dl>
          <p v-if="catalog.kind === 'open' && line.available === undefined" class="muted">{{ t('staticPricing.unavailableLine') }}</p>
          <p v-else-if="line.available !== undefined && line.quantity > line.available" class="muted">
            {{ t('staticPricing.adjustQuantity', { stock: count(line.available) }) }}
          </p>
        </section>
        <p v-if="!cart.size" class="muted">{{ t('staticPricing.selectRegion') }}</p>
        <p v-else-if="cart.size === maxStaticPurchaseLines" class="muted">{{ t('staticPricing.lineLimit', { count: count(maxStaticPurchaseLines) }) }}</p>
        <ElAlert v-if="quantityExceeded" type="warning" :closable="false" show-icon
          :title="t('staticPricing.quantityLimit', { count: count(maxStaticPurchaseQuantity) })" />
        <dl class="summary-list tabular">
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
        <ElButton type="primary" :disabled="!canReview" @click="startReview">{{ t('staticPricing.buy') }}</ElButton>
      </div>
    </ElCard>
  </div>

  <ElDialog :model-value="checkout.kind !== 'closed'" :title="dialogTitle" width="var(--form-dialog-width)"
    class="static-purchase-dialog" align-center :show-close="dismissible" :close-on-press-escape="dismissible"
    :close-on-click-modal="false" @close="closeCheckout">
    <ElSkeleton v-if="checkout.kind === 'loading'" animated :rows="3" />
    <div v-else-if="checkout.kind === 'confirm' || checkout.kind === 'blocked'" class="dialog-content">
      <ElAlert v-if="checkout.kind === 'blocked'" type="warning" :title="checkoutError(checkout.error)" :closable="false" show-icon />
      <ElAlert v-else-if="checkout.changed" type="warning"
        :title="checkout.changed === 'term' ? t('staticPricing.termChanged') : t('staticPricing.changed')" :closable="false" show-icon />
      <p v-if="checkout.kind === 'confirm'">{{ t('staticPricing.confirmHint') }}</p>
      <dl class="summary-list tabular">
        <div><dt>{{ t('staticPricing.product') }}</dt><dd>{{ productLabel }}</dd></div>
        <div v-if="filters.shareMode === 'shared'"><dt>{{ t('staticPricing.purpose') }}</dt><dd>{{ purposeLabel }}</dd></div>
      </dl>
      <section v-for="line in reviewLines" :key="line.offer.nodeId" class="quote-line">
        <h3>{{ offerRegion(line.offer) }}</h3>
        <p v-if="line.offer.isp" class="muted">{{ line.offer.isp }}</p>
        <dl v-if="line.currentOffer" class="summary-list tabular">
          <div><dt>{{ t('staticPricing.term') }}</dt><dd>{{ termDays(line.currentOffer.termDays) }}</dd></div>
          <div><dt>{{ t('staticPricing.quantity') }}</dt><dd>{{ count(line.quantity) }}</dd></div>
          <div><dt>{{ t('staticPricing.stock') }}</dt><dd>{{ count(line.currentOffer.available) }}</dd></div>
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
          @click="reviewPurchase(checkout.retry)">
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
          <ElButton v-if="checkout.order.status === 'pending'" :loading="busy" @click="refreshOrder(checkout.order)">
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
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: var(--space-5);
}
.pricing-catalog { flex: 3 1 var(--static-catalog-width); min-width: 0; }
.pricing-summary { flex: 1 1 var(--static-checkout-width); min-width: 0; }
.pricing-notice { margin-bottom: var(--space-4); }
.muted { color: var(--color-text-secondary); }
.product-hint, .catalog-heading, .filters { margin-top: var(--space-4); }
h2 { font-size: var(--font-size-lg); }
h3 { font-size: var(--font-size-md); }
.filters, .catalog-heading, .offer-heading, .selection, .dialog-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}
.catalog-heading, .offer-heading, .selection { justify-content: space-between; }
.filters { margin-bottom: var(--space-4); }
.filter { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2); }
.filter-control { width: var(--static-offer-width); max-width: 100%; }
.pricing-summary .filter { justify-content: space-between; }
.pricing-summary .filter > :last-child { flex: 1 1 55%; }
.offers { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--static-offer-width)), 1fr)); gap: var(--space-3); }
.offer--selected {
  --el-card-border-color: var(--color-primary-border);
  --el-card-bg-color: var(--color-primary-soft);
}
.summary-list, .catalog-feedback, .dialog-content { display: grid; gap: var(--space-3); }
.cart-line, .quote-line {
  display: grid;
  gap: var(--space-2);
  border-top: var(--border-width) solid var(--color-border);
  padding-block: var(--space-3);
}
.summary-list { margin: 0; }
.offer .summary-list { margin-block: var(--space-3); }
.summary-list div { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--space-2); }
.summary-list dt { color: var(--color-text-secondary); }
.summary-list dd { margin: 0; }
.grand-total { color: var(--color-primary); font-size: var(--font-size-xl); font-weight: var(--font-weight-semibold); }
.dialog-actions { justify-content: flex-end; }
.dialog-actions > * { margin: 0; }
</style>

<style>
.static-purchase-dialog { max-width: calc(100vw - var(--page-padding) * 2); }
</style>
