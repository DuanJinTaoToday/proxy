<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert, ElButton, ElCard, ElDialog, ElEmpty, ElInput, ElInputNumber, ElOption,
  ElRadioButton, ElRadioGroup, ElSelect, ElSkeleton, ElTag,
} from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { formatYuan } from '@/shared/lib/formatYuan'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { offerKey, useStaticPurchase } from './useStaticPurchase'

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
const nodeId = ref('')
const quantity = ref(1)

const offers = computed(() => catalog.value.kind === 'open' ? catalog.value.offers : [])
const regions = computed(() => [...new Set(offers.value.map(offer => offer.region).filter(Boolean))]
  .sort((a, b) => a.localeCompare(b, locale.value)))
const terms = computed(() => [...new Map(offers.value.map(offer => [offer.term, { code: offer.term, days: offer.termDays }])).values()]
  .sort((a, b) => a.days - b.days))
const currentTerm = computed(() => terms.value.find(option => option.code === term.value) ?? terms.value[0])
const termOffers = computed(() => offers.value.filter(offer => offer.term === currentTerm.value?.code))
const visibleOffers = computed(() => {
  const search = keyword.value.trim().toLocaleLowerCase(locale.value)
  return termOffers.value.filter(offer => (!region.value || offer.region === region.value)
    && (!search || [offer.region, offer.isp].some(label => label.toLocaleLowerCase(locale.value).includes(search))))
    .sort((first, second) => Number(second.available > 0) - Number(first.available > 0))
})
const selected = computed(() => termOffers.value.find(offer => offer.nodeId === nodeId.value))
const selectedTotal = computed(() => selected.value ? selected.value.unitMinor * BigInt(quantity.value) : 0n)
const selectedShortfall = computed(() => shortfall(selectedTotal.value))
const canReview = computed(() => selected.value && quantity.value > 0 && quantity.value <= selected.value.available
  && selectedShortfall.value === 0n && !busy.value)
const confirmTotal = computed(() => checkout.value.kind === 'confirm'
  ? checkout.value.quote.offer.unitMinor * BigInt(checkout.value.quote.quantity) : 0n)
const confirmShortfall = computed(() => shortfall(confirmTotal.value))
const dialogTitle = computed(() => {
  if (checkout.value.kind === 'result') return t('staticPricing.result')
  if (checkout.value.kind === 'unresolved') return t('staticPricing.unresolved')
  return t('staticPricing.confirm')
})
const dismissible = computed(() => !busy.value && checkout.value.kind !== 'unresolved')

function setShareMode(value: string | number | boolean | undefined) {
  if (value !== 'exclusive' && value !== 'shared') return
  resetSelection()
  filters.value = { shareMode: value }
}
function setPurpose(value: number) {
  resetSelection()
  filters.value = { shareMode: 'shared', purposeId: value || undefined }
}
function resetSelection() {
  region.value = ''
  keyword.value = ''
  term.value = ''
  nodeId.value = ''
  quantity.value = 1
}
function changeTerm(value: string) {
  term.value = value
  if (selected.value?.available) quantity.value = Math.min(quantity.value, selected.value.available)
}
function selectNode(value: string) {
  if (nodeId.value !== value) quantity.value = 1
  nodeId.value = value
}
function changeQuantity(value: number | undefined) {
  if (value !== undefined && Number.isInteger(value) && value > 0) quantity.value = value
}
function startReview() {
  if (selected.value && canReview.value) void reviewPurchase({ offer: selected.value, quantity: quantity.value })
}
function shortfall(total: bigint) {
  return balance.value !== undefined && balance.value < total ? total - balance.value : 0n
}
const money = (value: bigint) => formatYuan(value, locale.value)
const count = (value: number | bigint) => new Intl.NumberFormat(locale.value).format(value)
const termDays = (days: number) => t('staticPricing.termDays', { days: count(days) }, days)
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
          class="offer" :class="{ 'offer--selected': nodeId === offer.nodeId }">
          <div class="offer-heading">
            <div>
              <h3>{{ offer.region || t('staticPricing.unknownRegion') }}</h3>
              <p v-if="offer.isp" class="muted">{{ offer.isp }}</p>
            </div>
            <ElTag v-if="!offer.available" type="info">{{ t('staticPricing.soldOut') }}</ElTag>
          </div>
          <dl class="summary-list tabular">
            <div><dt>{{ t('staticPricing.unitPrice') }}</dt><dd>{{ money(offer.unitMinor) }}</dd></div>
            <div><dt>{{ t('staticPricing.stock') }}</dt><dd>{{ count(offer.available) }}</dd></div>
          </dl>
          <ElButton :type="nodeId === offer.nodeId ? 'primary' : 'default'" plain
            :disabled="offer.available <= 0" :aria-pressed="nodeId === offer.nodeId"
            :aria-label="t('staticPricing.selectOffer', { region: offer.region || t('staticPricing.unknownRegion') })"
            @click="selectNode(offer.nodeId)">
            {{ nodeId === offer.nodeId ? t('staticPricing.selected') : t('staticPricing.select') }}
          </ElButton>
        </ElCard>
      </div>
    </ElCard>

    <ElCard shadow="never" class="pricing-summary">
      <template #header><h2>{{ t('staticPricing.summary') }}</h2></template>
      <div class="catalog-feedback">
        <label class="filter">
          <span>{{ t('staticPricing.term') }}</span>
          <ElSelect :model-value="currentTerm?.code" :aria-label="t('staticPricing.term')" :disabled="!terms.length" @change="changeTerm">
            <ElOption v-for="option in terms" :key="option.code" :value="option.code"
              :label="termDays(option.days)" />
          </ElSelect>
        </label>
        <p v-if="filters.shareMode === 'shared'" class="muted">{{ t('staticPricing.summaryPurpose', { purpose: purposeLabel }) }}</p>
        <template v-if="selected">
          <div class="selection">
            <strong>{{ selected.region || t('staticPricing.unknownRegion') }}</strong>
            <ElButton text @click="nodeId = ''">{{ t('staticPricing.clear') }}</ElButton>
          </div>
          <label class="filter">
            <span>{{ t('staticPricing.quantity') }}</span>
            <ElInputNumber :model-value="quantity" :min="1" :max="Math.max(1, selected.available)"
              :disabled="selected.available <= 0" :precision="0" :aria-label="t('staticPricing.quantity')"
              @change="changeQuantity" />
          </label>
        </template>
        <p v-else class="muted">{{ t('staticPricing.selectRegion') }}</p>
        <dl class="summary-list tabular">
          <div>
            <dt>{{ t('staticPricing.balance') }}</dt>
            <dd v-if="wallet.kind === 'ready'">{{ money(wallet.balance) }}</dd>
            <dd v-else-if="wallet.kind === 'loading'" role="status">{{ t('staticPricing.loading') }}</dd>
            <dd v-else><ElButton link @click="loadWallet">{{ t('staticPricing.reloadBalance') }}</ElButton></dd>
          </div>
          <div><dt>{{ t('staticPricing.total') }}</dt><dd class="grand-total">{{ money(selectedTotal) }}</dd></div>
        </dl>
        <ElAlert v-if="selectedShortfall > 0n" class="tabular" type="warning" :closable="false" show-icon
          :title="t('staticPricing.shortfall', { amount: money(selectedShortfall) })" />
        <ElButton type="primary" :disabled="!canReview" @click="startReview">{{ t('staticPricing.buy') }}</ElButton>
      </div>
    </ElCard>
  </div>

  <ElDialog :model-value="checkout.kind !== 'closed'" :title="dialogTitle" width="var(--form-dialog-width)"
    class="static-purchase-dialog" align-center :show-close="dismissible" :close-on-press-escape="dismissible"
    :close-on-click-modal="false" @close="closeCheckout">
    <ElSkeleton v-if="checkout.kind === 'loading'" animated :rows="3" />
    <div v-else-if="checkout.kind === 'confirm'" class="dialog-content">
      <ElAlert v-if="checkout.changed" type="warning" :title="t('staticPricing.changed')" :closable="false" show-icon />
      <p>{{ t('staticPricing.confirmHint') }}</p>
      <div>
        <strong>{{ checkout.quote.offer.region || t('staticPricing.unknownRegion') }}</strong>
        <p v-if="checkout.quote.offer.isp" class="muted">{{ checkout.quote.offer.isp }}</p>
      </div>
      <dl class="summary-list tabular">
        <div><dt>{{ t('staticPricing.product') }}</dt><dd>{{ productLabel }}</dd></div>
        <div v-if="filters.shareMode === 'shared'"><dt>{{ t('staticPricing.purpose') }}</dt><dd>{{ purposeLabel }}</dd></div>
        <div><dt>{{ t('staticPricing.term') }}</dt><dd>{{ termDays(checkout.quote.offer.termDays) }}</dd></div>
        <div><dt>{{ t('staticPricing.quantity') }}</dt><dd>{{ count(checkout.quote.quantity) }}</dd></div>
        <div><dt>{{ t('staticPricing.stock') }}</dt><dd>{{ count(checkout.quote.offer.available) }}</dd></div>
        <div><dt>{{ t('staticPricing.unitPrice') }}</dt><dd>{{ money(checkout.quote.offer.unitMinor) }}</dd></div>
        <div v-if="balance !== undefined && confirmShortfall === 0n">
          <dt>{{ t('staticPricing.afterPayment') }}</dt><dd>{{ money(balance - confirmTotal) }}</dd>
        </div>
        <div><dt>{{ t('staticPricing.payable') }}</dt><dd class="grand-total">{{ money(confirmTotal) }}</dd></div>
      </dl>
      <ElAlert v-if="confirmShortfall > 0n" type="warning" class="tabular" :closable="false" show-icon
        :title="t('staticPricing.shortfall', { amount: money(confirmShortfall) })" />
    </div>
    <div v-else-if="checkout.kind === 'blocked'" class="dialog-content">
      <ElAlert type="warning" :title="checkoutError(checkout.error)" :closable="false" show-icon />
      <dl v-if="checkout.currentOffer" class="summary-list tabular">
        <div><dt>{{ t('staticPricing.stock') }}</dt><dd>{{ count(checkout.currentOffer.available) }}</dd></div>
        <div><dt>{{ t('staticPricing.unitPrice') }}</dt><dd>{{ money(checkout.currentOffer.unitMinor) }}</dd></div>
        <div><dt>{{ t('staticPricing.payable') }}</dt><dd>{{ money(checkout.currentOffer.unitMinor * BigInt(checkout.review.quantity)) }}</dd></div>
      </dl>
    </div>
    <div v-else-if="checkout.kind === 'unresolved'" class="dialog-content">
      <ElAlert type="warning" :title="t('staticPricing.unresolvedHint')" :closable="false" show-icon />
      <p v-if="checkout.error">{{ checkoutError(checkout.error) }}</p>
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
        <ElButton v-if="checkout.kind === 'blocked'" type="primary" :loading="busy"
          :disabled="checkout.currentOffer !== undefined && checkout.currentOffer.available < checkout.review.quantity"
          @click="reviewPurchase({ offer: checkout.currentOffer ?? checkout.review.offer, quantity: checkout.review.quantity })">
          {{ t('staticPricing.retryQuote') }}
        </ElButton>
        <ElButton v-if="checkout.kind === 'unresolved'" type="primary" :loading="busy" @click="retryPurchase(checkout.body)">
          {{ t('staticPricing.retryPurchase') }}
        </ElButton>
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
