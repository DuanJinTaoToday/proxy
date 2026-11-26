<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElButton } from 'element-plus'
import { Box, Delete, Minus, Plus } from '@element-plus/icons-vue'
import type { StaticPurchaseOffer } from '@/api/staticPricing'
import { formatYuan } from '@/shared/lib/money'
import { flagForRegion } from '@/shared/lib/regionFlags'
import FlagIcon from '@/shared/ui/FlagIcon.vue'
import { usePricingError } from './pricingError'
import { sharedPurposes, type Loadable, type SharedPurpose, type StaticPricing } from './usePublicPricing'

const props = defineProps<{
  state: Loadable<StaticPricing>
  shareMode: 'exclusive' | 'shared'
  purpose: SharedPurpose
}>()
const emit = defineEmits<{
  retry: []
  'update:purpose': [value: SharedPurpose]
}>()

const { t, locale } = useI18n()
const router = useRouter()
const errorText = usePricingError()

const selectedTerm = ref('')
const cart = ref<Map<string, { offer: StaticPurchaseOffer, qty: number }>>(new Map())

function extractTerms(offers: StaticPurchaseOffer[]) {
  const map = new Map<string, number>()
  for (const offer of offers) {
    if (!map.has(offer.term)) map.set(offer.term, offer.termDays)
  }
  return [...map.entries()]
    .map(([term, days]) => ({ term, days }))
    .sort((a, b) => a.days - b.days)
}

const sellableOffers = computed(() => {
  if (props.state.kind !== 'ready' || !props.state.data.open) return []
  return props.state.data.offers.filter(offer => offer.available > 0)
})

const terms = computed(() => extractTerms(sellableOffers.value))

watch(
  [() => props.shareMode, () => props.purpose],
  () => {
    cart.value = new Map()
  },
)

watch(
  terms,
  availableTerms => {
    if (!availableTerms.length) {
      selectedTerm.value = ''
      cart.value = new Map()
      return
    }
    if (!availableTerms.some(item => item.term === selectedTerm.value)) {
      selectedTerm.value = availableTerms.find(item => item.days === 30)?.term ?? availableTerms[0]?.term ?? ''
      cart.value = new Map()
    }
  },
  { immediate: true },
)

const visibleOffers = computed(() =>
  sellableOffers.value.filter(offer => !selectedTerm.value || offer.term === selectedTerm.value),
)

function switchTerm(term: string) {
  if (selectedTerm.value === term) return
  selectedTerm.value = term
  cart.value = new Map()
}

function keyOf(offer: StaticPurchaseOffer) {
  return `${offer.nodeId}::${offer.term}`
}

function qtyOf(offer: StaticPurchaseOffer) {
  return cart.value.get(keyOf(offer))?.qty ?? 0
}

function addOffer(offer: StaticPurchaseOffer) {
  const next = new Map(cart.value)
  const key = keyOf(offer)
  const existing = next.get(key)
  const nextQty = Math.min((existing?.qty ?? 0) + 1, offer.available)
  next.set(key, { offer, qty: nextQty })
  cart.value = next
}

function stepOffer(key: string, delta: number) {
  const next = new Map(cart.value)
  const item = next.get(key)
  if (!item) return
  const nextQty = item.qty + delta
  if (nextQty <= 0) next.delete(key)
  else next.set(key, { ...item, qty: Math.min(nextQty, item.offer.available) })
  cart.value = next
}

function clearCart() {
  cart.value = new Map()
}

const cartItems = computed(() => [...cart.value.entries()].map(([key, item]) => ({ key, ...item })))
const totalCount = computed(() => cartItems.value.reduce((sum, item) => sum + item.qty, 0))
const totalMinor = computed(() => cartItems.value.reduce((sum, item) => sum + item.offer.unitMinor * BigInt(item.qty), 0n))

function checkout() {
  if (!cartItems.value.length) return
  const query: LocationQueryRaw = {
    shareMode: props.shareMode,
    term: selectedTerm.value || undefined,
    items: cartItems.value.map(item => `${item.offer.nodeId}:${item.qty}`).join(','),
  }
  if (props.shareMode === 'shared' && props.purpose) {
    query.purpose = String(props.purpose)
  }
  void router.push({ name: 'console-static-pricing', query })
}
</script>

<template>
  <div class="static-layout" :aria-busy="state.kind === 'loading'">
    <section class="catalog-card">
      <div class="catalog-card__header">
        <h3>{{ t('publicPricing.static.regionsTitle') }}</h3>
        <div v-if="shareMode === 'shared'" class="purpose-bar" role="group" :aria-label="t('publicPricing.purpose')">
          <button
            v-for="id in sharedPurposes"
            :key="id"
            type="button"
            class="purpose-chip"
            :class="{ 'is-active': purpose === id }"
            :aria-pressed="purpose === id"
            @click="emit('update:purpose', id)"
          >
            {{ t(`publicPricing.purposes.${id}`) }}
          </button>
        </div>
      </div>

      <div v-if="terms.length" class="term-bar" role="group" :aria-label="t('publicPricing.term')">
        <button
          v-for="item in terms"
          :key="item.term"
          type="button"
          class="term-pill"
          :class="{ 'is-active': selectedTerm === item.term }"
          :aria-pressed="selectedTerm === item.term"
          @click="switchTerm(item.term)"
        >
          {{ t('publicPricing.termDays', { days: item.days }, item.days) }}
        </button>
      </div>

      <div v-if="state.kind === 'loading'" class="offer-grid" data-state="loading" :aria-label="t('publicPricing.loading')">
        <div v-for="n in 6" :key="n" class="offer-skeleton" />
      </div>

      <div v-else-if="state.kind === 'failed'" class="state-box" data-state="failed" role="alert">
        <span class="status-dot status-dot--warn" aria-hidden="true" />
        <div class="state-box__body">
          <strong>{{ t('publicPricing.staticFailed') }}</strong>
          <p>{{ errorText(state.error) }}</p>
        </div>
        <ElButton size="small" type="primary" @click="emit('retry')">{{ t('publicPricing.retry') }}</ElButton>
      </div>

      <div v-else-if="!state.data.open || !visibleOffers.length" class="state-box" data-state="closed" role="status">
        <span class="status-dot status-dot--warn" aria-hidden="true" />
        <p>{{ t('publicPricing.static.unavailable') }}</p>
      </div>

      <div v-else class="offer-grid">
        <button
          v-for="offer in visibleOffers"
          :key="keyOf(offer)"
          type="button"
          class="offer-row"
          :class="{ 'is-selected': qtyOf(offer) > 0 }"
          @click="addOffer(offer)"
        >
          <span class="offer-row__strip">{{ offer.isp || t('publicPricing.static.defaultTag') }}</span>
          <span class="offer-row__body">
            <FlagIcon :code="flagForRegion(offer.region)" :size="30" />
            <span class="offer-row__info">
              <strong class="offer-row__region">{{ offer.region || offer.nodeId }}</strong>
              <span class="offer-row__meta">
                <i class="status-dot" aria-hidden="true" />
                <span>{{ t('publicPricing.available', { count: offer.available }) }}</span>
                <span v-if="offer.line">· {{ offer.line }}</span>
              </span>
            </span>
          </span>
          <span class="offer-row__price tabular">
            <strong>{{ formatYuan(offer.unitMinor, locale) }}<span>{{ t('publicPricing.static.perIp') }}</span></strong>
            <span v-if="qtyOf(offer) > 0" class="offer-row__qty">×{{ qtyOf(offer) }}</span>
          </span>
        </button>
      </div>
    </section>

    <aside class="cart-card" :aria-label="t('publicPricing.static.cartTitle')">
      <div class="cart-card__top">
        <h3>{{ t('publicPricing.static.cartTitle') }}</h3>
        <button type="button" class="clear-btn" :disabled="!cartItems.length" @click="clearCart">
          <Delete aria-hidden="true" />{{ t('publicPricing.static.clearAll') }}
        </button>
      </div>

      <div v-if="terms.length" class="term-bar term-bar--compact" role="group" :aria-label="t('publicPricing.term')">
        <button
          v-for="item in terms"
          :key="`side-${item.term}`"
          type="button"
          class="term-pill"
          :class="{ 'is-active': selectedTerm === item.term }"
          :aria-pressed="selectedTerm === item.term"
          @click="switchTerm(item.term)"
        >
          {{ t('publicPricing.termDays', { days: item.days }, item.days) }}
        </button>
      </div>

      <div class="cart-head">
        <span>{{ t('publicPricing.static.colRegion') }}</span>
        <span>{{ t('publicPricing.static.colTerm') }}</span>
        <span>{{ t('publicPricing.static.colPrice') }}</span>
        <span>{{ t('publicPricing.static.colQty') }}</span>
      </div>

      <div v-if="!cartItems.length" class="cart-empty">
        <Box class="cart-empty__icon" aria-hidden="true" />
        <p>{{ t('publicPricing.static.emptyHint') }}</p>
      </div>

      <ul v-else class="cart-list">
        <li v-for="item in cartItems" :key="item.key" class="cart-item">
          <span class="cart-item__region">{{ item.offer.region || item.offer.nodeId }}</span>
          <span class="cart-item__term">{{ t('publicPricing.termDays', { days: item.offer.termDays }, item.offer.termDays) }}</span>
          <span class="cart-item__price tabular">{{ formatYuan(item.offer.unitMinor, locale) }}</span>
          <span class="stepper">
            <button type="button" :aria-label="t('publicPricing.static.decrease')" @click="stepOffer(item.key, -1)"><Minus /></button>
            <strong class="tabular">{{ item.qty }}</strong>
            <button type="button" :aria-label="t('publicPricing.static.increase')" :disabled="item.qty >= item.offer.available" @click="stepOffer(item.key, 1)"><Plus /></button>
          </span>
        </li>
      </ul>

      <div class="cart-summary">
        <span>{{ t('publicPricing.static.totalCount') }}: <strong class="tabular">{{ totalCount }}</strong></span>
        <span>{{ t('publicPricing.static.totalPrice') }}: <strong class="cart-summary__price tabular">{{ formatYuan(totalMinor, locale) }}</strong></span>
      </div>

      <button type="button" class="checkout-btn" :disabled="!cartItems.length" @click="checkout">
        {{ t('publicPricing.buyNow') }}
      </button>
    </aside>
  </div>
</template>

<style scoped>
.static-layout {
  display: grid;
  grid-template-columns: minmax(0, 1.65fr) minmax(var(--home-pricing-cart-min-width), 1fr);
  gap: var(--space-5);
  align-items: start;
}

.catalog-card,
.cart-card {
  padding: var(--space-6);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-radius-xl);
  background: var(--home-light);
  box-shadow: var(--home-card-shadow);
}
.cart-card {
  position: sticky;
  top: calc(var(--home-header-height) + var(--space-5));
}

.catalog-card__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
h3 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  color: var(--home-text-light);
}

.purpose-bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.purpose-chip {
  min-height: var(--home-pricing-gem-size);
  padding: 0 var(--space-3);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-pill);
  background: var(--home-light);
  color: var(--home-muted-light);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: background .2s, color .2s, border-color .2s;
}
.purpose-chip:hover { border-color: var(--home-sky-300); color: var(--home-brand); }
.purpose-chip.is-active {
  border-color: var(--home-brand);
  background: var(--home-brand-soft);
  color: var(--home-brand);
  font-weight: var(--font-weight-semibold);
}

.term-bar {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  margin-top: var(--space-4);
}
.term-bar--compact { margin-top: var(--space-3); }
.term-pill {
  min-width: var(--home-term-pill-min-w);
  min-height: var(--home-pricing-gem-size);
  padding: 0 var(--space-4);
  border: 0;
  border-radius: var(--home-pill);
  background: var(--home-card-gray);
  color: var(--home-muted-light);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-medium);
  cursor: pointer;
  transition: background .2s, color .2s;
}
.term-pill:hover { color: var(--home-brand); background: var(--home-brand-soft); }
.term-pill.is-active {
  background: var(--home-brand);
  color: var(--home-on-brand);
  font-weight: var(--font-weight-semibold);
}

.offer-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-3);
  margin-top: var(--space-5);
}
.offer-row {
  display: flex;
  align-items: stretch;
  gap: var(--space-3);
  width: 100%;
  padding: 0 var(--space-4) 0 0;
  overflow: hidden;
  border: var(--border-width) solid var(--home-border-subtle);
  border-radius: var(--home-radius-md);
  background: var(--home-light);
  text-align: start;
  cursor: pointer;
  transition: border-color .2s, box-shadow .2s, transform .2s var(--home-ease);
}
.offer-row:hover {
  border-color: var(--home-sky-300);
  box-shadow: var(--home-card-shadow);
  transform: translateY(-1px);
}
.offer-row.is-selected {
  border-color: var(--home-brand);
  background: var(--home-brand-soft);
}
.offer-row__strip {
  display: flex;
  align-items: center;
  justify-content: center;
  width: var(--home-pricing-strip-width);
  flex: none;
  padding: var(--space-2) var(--space-1);
  background: var(--home-brand-soft);
  color: var(--home-brand);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-semibold);
  writing-mode: vertical-rl;
  letter-spacing: .08em;
}
.offer-row.is-selected .offer-row__strip {
  background: var(--home-brand);
  color: var(--home-on-brand);
}
.offer-row__body {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex: 1;
  min-width: 0;
  padding-block: var(--space-4);
}
.offer-row__info {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  min-width: 0;
}
.offer-row__region {
  font-size: var(--home-fs-body);
  font-weight: var(--font-weight-semibold);
  color: var(--home-text-light);
  overflow-wrap: anywhere;
}
.offer-row__meta {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--home-fs-micro);
  color: var(--home-faint-light);
}
.offer-row__meta .status-dot { width: var(--home-dot-xs); height: var(--home-dot-xs); box-shadow: none; }
.offer-row__price {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  justify-content: center;
  gap: var(--space-1);
  flex: none;
  padding-block: var(--space-3);
}
.offer-row__price strong {
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-bold);
  color: var(--home-price);
  white-space: nowrap;
}
.offer-row__price strong span {
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-medium);
}
.offer-row__qty {
  padding: var(--home-cart-qty-pad-y) var(--space-2);
  border-radius: var(--home-pill);
  background: var(--home-brand);
  color: var(--home-on-brand);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-semibold);
}

.cart-card__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
}
.clear-btn {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2);
  border: 0;
  border-radius: var(--home-radius-sm);
  background: transparent;
  color: var(--home-price);
  font-size: var(--home-fs-micro);
  cursor: pointer;
}
.clear-btn svg { width: var(--home-icon-sm); height: var(--home-icon-sm); }
.clear-btn:disabled { opacity: .4; cursor: not-allowed; }
.clear-btn:not(:disabled):hover { background: var(--home-price-soft); }

.cart-head {
  display: grid;
  grid-template-columns: 1.2fr .75fr .85fr var(--home-cart-qty-col);
  gap: var(--space-2);
  margin-top: var(--space-4);
  padding: var(--space-2) var(--space-3);
  border-radius: var(--home-radius-sm);
  background: var(--home-light-alt);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-medium);
  color: var(--home-faint-light);
}
.cart-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-3);
  min-height: var(--home-cart-empty-height);
  color: var(--home-faint-light);
  font-size: var(--home-fs-small);
  text-align: center;
}
.cart-empty__icon { width: var(--home-btn-circle); height: var(--home-btn-circle); opacity: .45; }

.cart-list {
  display: grid;
  gap: var(--space-2);
  margin: var(--space-3) 0 0;
  padding: 0;
  list-style: none;
  max-height: var(--home-cart-list-max-height);
  overflow-y: auto;
}
.cart-item {
  display: grid;
  grid-template-columns: 1.2fr .75fr .85fr var(--home-cart-qty-col);
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-2) var(--space-3);
  border-bottom: var(--border-width) solid var(--home-border-subtle);
  font-size: var(--home-fs-micro);
  color: var(--home-text-light);
}
.cart-item__region { font-weight: var(--font-weight-medium); overflow-wrap: anywhere; }
.cart-item__term { color: var(--home-muted-light); }
.cart-item__price { color: var(--home-price); font-weight: var(--font-weight-semibold); }

.stepper {
  display: inline-flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-1);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--radius-control);
  padding: var(--home-cart-stepper-pad);
  background: var(--home-light);
}
.stepper button {
  display: inline-grid;
  place-items: center;
  width: var(--home-cart-stepper-btn);
  height: var(--home-cart-stepper-btn);
  border: 0;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--home-text-light);
  cursor: pointer;
}
.stepper button:hover:not(:disabled) { background: var(--home-card-gray); }
.stepper button:disabled { opacity: .35; cursor: not-allowed; }
.stepper svg { width: var(--home-cart-stepper-glyph); height: var(--home-cart-stepper-glyph); }
.stepper strong { min-width: var(--home-cart-stepper-min-w); text-align: center; font-size: var(--home-fs-micro); }

.cart-summary {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-3);
  margin-top: var(--space-4);
  padding-top: var(--space-4);
  border-top: var(--border-width) solid var(--home-border-subtle);
  font-size: var(--home-fs-small);
  color: var(--home-text-light);
}
.cart-summary__price {
  font-size: var(--home-fs-h3);
  font-weight: var(--font-weight-bold);
  color: var(--home-price);
}

.checkout-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: var(--home-pricing-checkout-height);
  margin-top: var(--space-4);
  border: 0;
  border-radius: var(--home-radius-md);
  background: var(--home-brand);
  color: var(--home-on-brand);
  font-size: var(--home-fs-body);
  font-weight: var(--font-weight-semibold);
  box-shadow: var(--home-brand-shadow);
  cursor: pointer;
  transition: background .2s, transform .2s var(--home-ease), opacity .2s;
}
.checkout-btn:hover:not(:disabled) {
  background: var(--home-brand-hover);
  transform: translateY(-1px);
}
.checkout-btn:disabled {
  opacity: .45;
  box-shadow: none;
  cursor: not-allowed;
}

.offer-skeleton {
  height: var(--home-pricing-skeleton-offer-height);
  border-radius: var(--home-radius-md);
  background: linear-gradient(90deg, var(--home-card-gray), var(--home-light-alt), var(--home-card-gray));
  background-size: 200% 100%;
  animation: shimmer 1.4s linear infinite;
}
@keyframes shimmer { to { background-position: -200% 0; } }

.state-box {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
  margin-top: var(--space-5);
  padding: var(--home-card-pad-y);
  border: var(--border-width) dashed var(--home-border-light);
  border-radius: var(--home-radius-lg);
  background: var(--home-light-alt);
  color: var(--home-muted-light);
  font-size: var(--home-fs-small);
}
.state-box__body { display: grid; gap: var(--space-1); }
.state-box__body strong { color: var(--home-text-light); }

@media (max-width: 1040px) {
  .static-layout { grid-template-columns: minmax(0, 1fr); }
  .cart-card { position: static; }
}
@media (max-width: 640px) {
  .catalog-card, .cart-card { padding: var(--space-4); }
  .offer-grid { grid-template-columns: minmax(0, 1fr); }
  .cart-head, .cart-item { grid-template-columns: 1fr .65fr .75fr var(--home-cart-qty-col-mobile); }
}
</style>
