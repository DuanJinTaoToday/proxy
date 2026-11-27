<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElButton } from 'element-plus'
import { Check, Connection } from '@element-plus/icons-vue'
import { unitPrice } from '@/features/catalog'
import { formatYuan } from '@/shared/lib/money'
import { buildDynamicPurchaseIntentQuery } from '@/features/checkout'
import { formatGB } from '@/shared/lib/trafficUnits'
import { usePricingError } from './pricingError'
import type { DynamicPricing, Loadable } from './usePublicPricing'

const props = defineProps<{ state: Loadable<DynamicPricing> }>()
const emit = defineEmits<{ retry: [] }>()

const { t, locale } = useI18n()
const errorText = usePricingError()
const notices = ['billing', 'protocol', 'compliance'] as const

const packages = computed(() => {
  if (props.state.kind !== 'ready' || !props.state.data.onSale) return []
  return [...props.state.data.packages]
    .sort((a, b) => (a.capacityBytes < b.capacityBytes ? -1 : a.capacityBytes > b.capacityBytes ? 1 : 0))
    .map(tier => ({ ...tier, unitMinor: unitPrice(tier) }))
})
</script>

<template>
  <div class="dynamic-shell" :aria-busy="state.kind === 'loading'">
    <aside class="guide">
      <h3>{{ t('publicPricing.dynamic.guideTitle') }}</h3>
      <ul class="guide__list">
        <li v-for="item in notices" :key="item">
          <div class="guide__head">
            <span class="guide__check" aria-hidden="true"><Check /></span>
            <strong>{{ t(`publicPricing.dynamic.guide.${item}.title`) }}</strong>
          </div>
          <p>{{ t(`publicPricing.dynamic.guide.${item}.desc`) }}</p>
        </li>
      </ul>
      <div class="guide__badge">
        <Connection class="guide__badge-icon" aria-hidden="true" />
        <div>
          <strong>{{ t('publicPricing.dynamic.badgeTitle') }}</strong>
          <p>{{ t('publicPricing.dynamic.badgeDesc') }}</p>
        </div>
      </div>
    </aside>

    <div class="tier-area">
      <div v-if="state.kind === 'loading'" class="tier-grid" data-state="loading" :aria-label="t('publicPricing.loading')">
        <div v-for="n in 3" :key="n" class="tier-skeleton" />
      </div>

      <div v-else-if="state.kind === 'failed'" class="state-box" data-state="failed" role="alert">
        <span class="status-dot status-dot--warn" aria-hidden="true" />
        <div class="state-box__body">
          <strong>{{ t('publicPricing.dynamicFailed') }}</strong>
          <p>{{ errorText(state.error) }}</p>
        </div>
        <ElButton size="small" type="primary" @click="emit('retry')">{{ t('publicPricing.retry') }}</ElButton>
      </div>

      <div v-else-if="!state.data.onSale || !packages.length" class="state-box" data-state="closed" role="status">
        <span class="status-dot status-dot--warn" aria-hidden="true" />
        <p>{{ state.data.onSale ? t('publicPricing.noPackages') : t('publicPricing.dynamicClosed') }}</p>
      </div>

      <div v-else class="tier-grid">
        <article
          v-for="tier in packages"
          :key="tier.id"
          class="tier-card"
          :class="{ 'tier-card--featured': tier.discountPercent > 0 }"
        >
          <div class="tier-card__top">
            <span class="tier-card__gem" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M6 4h12l4 5-10 11L2 9l4-5Z" />
                <path d="M2 9h20M8 4l4 16 4-16" opacity=".6" />
              </svg>
            </span>
            <h4 class="tabular">{{ formatGB(tier.capacityBytes, locale) }} GB</h4>
            <span v-if="tier.discountPercent > 0" class="tier-card__discount">
              {{ t('publicPricing.savings', { percent: tier.discountPercent }) }}
            </span>
          </div>

          <div class="tier-card__size">
            <strong class="tabular">{{ formatGB(tier.capacityBytes, locale) }} GB</strong>
            <span class="tier-card__pill">{{ t('publicPricing.dynamic.realCapacity', { size: formatGB(tier.capacityBytes, locale) }) }}</span>
          </div>

          <p class="tier-card__total tabular">{{ formatYuan(tier.priceMinor, locale) }}</p>

          <div class="tier-card__unit">
            <strong class="tabular">{{ formatYuan(tier.unitMinor, locale) }}<span>{{ t('publicPricing.perGbShort') }}</span></strong>
            <span class="tier-card__meta">{{ t('publicPricing.dynamic.socks5Tag') }}</span>
          </div>

          <RouterLink class="tier-card__cta" :to="{ name: 'console-dynamic-pricing', query: buildDynamicPurchaseIntentQuery(tier) }">
            {{ t('publicPricing.buyPackage', { capacity: formatGB(tier.capacityBytes, locale) }) }}
          </RouterLink>
        </article>

        <article class="tier-card tier-card--custom">
          <div class="tier-card__top">
            <span class="tier-card__gem" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
                <path d="M6 4h12l4 5-10 11L2 9l4-5Z" />
              </svg>
            </span>
            <h4>{{ t('publicPricing.dynamic.customTitle') }}</h4>
          </div>
          <p class="tier-card__infinity" aria-hidden="true">∞</p>
          <p class="tier-card__custom-desc">{{ t('publicPricing.dynamic.customDesc') }}</p>
          <a class="tier-card__cta" href="#contact">{{ t('publicPricing.contactSupport') }}</a>
        </article>
      </div>
    </div>
  </div>
</template>

<style scoped>
.dynamic-shell {
  display: grid;
  grid-template-columns: var(--home-pricing-guide-width) minmax(0, 1fr);
  gap: var(--space-6);
  padding: var(--home-card-pad-y);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-radius-xl);
  background: var(--home-light);
  box-shadow: var(--home-card-shadow);
}
.guide {
  display: flex;
  flex-direction: column;
  padding-inline-end: var(--space-6);
  border-inline-end: var(--border-width) solid var(--home-border-subtle);
}
.guide h3 {
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-bold);
  color: var(--home-text-light);
}
.guide__list {
  display: grid;
  gap: var(--space-5);
  margin: var(--space-5) 0 0;
  padding: 0;
  list-style: none;
}
.guide__head {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--home-fs-small);
  color: var(--home-text-light);
}
.guide__check {
  display: inline-grid;
  place-items: center;
  width: var(--home-check-size);
  height: var(--home-check-size);
  border-radius: var(--home-pill);
  background: var(--home-brand);
  color: var(--home-on-brand);
  flex: none;
}
.guide__check svg { width: var(--home-check-glyph); height: var(--home-check-glyph); }
.guide__list p {
  margin-top: var(--space-1);
  padding-inline-start: var(--space-6);
  font-size: var(--home-fs-micro);
  line-height: 1.68;
  color: var(--home-muted-light);
}
.guide__badge {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  margin-top: auto;
  padding: var(--space-4);
  border: var(--border-width) solid var(--home-brand-soft-border);
  border-radius: var(--home-radius-md);
  background: var(--home-brand-soft);
}
.guide__badge-icon {
  width: var(--space-6);
  height: var(--space-6);
  flex: none;
  color: var(--home-brand);
}
.guide__badge strong {
  display: block;
  font-size: var(--home-fs-small);
  color: var(--home-text-light);
}
.guide__badge p {
  margin-top: var(--space-1);
  font-size: var(--home-fs-micro);
  line-height: 1.55;
  color: var(--home-muted-light);
}

.tier-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-4);
}
.tier-card {
  display: flex;
  flex-direction: column;
  padding: var(--space-5);
  border: var(--border-width) solid var(--home-brand-soft-border);
  border-radius: var(--home-radius-lg);
  background: var(--home-blue-wash);
  transition: transform .25s var(--home-ease), box-shadow .25s var(--home-ease), border-color .2s;
}
.tier-card:hover {
  transform: translateY(-2px);
  border-color: var(--home-sky-300);
  box-shadow: var(--home-card-shadow-hover);
}
.tier-card--featured {
  border-color: var(--home-price-border);
  background: var(--home-price-wash);
}
.tier-card--custom {
  border-color: var(--home-border-light);
  background: var(--home-card-gray);
}

.tier-card__top {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.tier-card__gem {
  display: inline-grid;
  place-items: center;
  width: var(--home-pricing-gem-size);
  height: var(--home-pricing-gem-size);
  border-radius: var(--radius-card);
  background: var(--home-brand);
  color: var(--home-on-brand);
  flex: none;
}
.tier-card--featured .tier-card__gem { background: var(--home-price); }
.tier-card__gem svg { width: var(--icon-size); height: var(--icon-size); }
h4 {
  margin: 0;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-bold);
  color: var(--home-text-light);
}
.tier-card__discount {
  margin-inline-start: auto;
  padding: var(--home-tag-pad-y) var(--space-2);
  border-radius: var(--home-pill);
  background: var(--home-price-soft);
  border: var(--border-width) solid var(--home-price-border);
  color: var(--home-price);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-semibold);
}

.tier-card__size {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-4);
}
.tier-card__size strong {
  font-size: var(--home-fs-h3);
  font-weight: var(--font-weight-bold);
  color: var(--home-text-light);
}
.tier-card__pill {
  padding: var(--home-tag-pad-y) var(--space-2);
  border-radius: var(--radius-control);
  background: var(--home-brand-soft);
  color: var(--home-brand);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-medium);
}
.tier-card--featured .tier-card__pill {
  background: var(--home-price-soft);
  color: var(--home-price);
}

.tier-card__total {
  margin-top: var(--space-2);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-medium);
  color: var(--home-muted-light);
}
.tier-card__unit {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
  margin-top: var(--space-4);
  margin-bottom: var(--space-4);
  padding-top: var(--space-3);
  border-top: var(--border-width) solid var(--home-border-subtle);
}
.tier-card__unit strong {
  font-size: var(--home-fs-price);
  font-weight: var(--font-weight-bold);
  color: var(--home-price);
  letter-spacing: -.02em;
}
.tier-card__unit strong span {
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-semibold);
}
.tier-card__meta {
  font-size: var(--home-fs-micro);
  color: var(--home-faint-light);
}

.tier-card__infinity {
  margin-top: var(--space-4);
  font-size: var(--home-fs-h2);
  line-height: 1;
  font-weight: var(--font-weight-bold);
  color: var(--home-text-light);
}
.tier-card__custom-desc {
  margin-top: var(--space-3);
  margin-bottom: var(--space-4);
  font-size: var(--home-fs-small);
  line-height: 1.65;
  color: var(--home-muted-light);
}

.tier-card__cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: var(--home-pricing-cta-height);
  margin-top: auto;
  padding-inline: var(--space-3);
  border-radius: var(--home-radius-sm);
  background: var(--home-brand-soft);
  color: var(--home-brand);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-semibold);
  text-align: center;
  transition: background .2s, color .2s;
}
.tier-card__cta:hover {
  background: var(--home-brand);
  color: var(--home-on-brand);
}
.tier-card--featured .tier-card__cta {
  background: var(--home-price-soft);
  color: var(--home-price);
}
.tier-card--featured .tier-card__cta:hover {
  background: var(--home-price);
  color: var(--home-on-brand);
}

.tier-skeleton {
  height: var(--home-pricing-skeleton-tier-height);
  border-radius: var(--home-radius-lg);
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
  padding: var(--home-card-pad-y);
  border: var(--border-width) dashed var(--home-border-light);
  border-radius: var(--home-radius-lg);
  background: var(--home-light-alt);
  color: var(--home-muted-light);
  font-size: var(--home-fs-small);
}
.state-box__body { display: grid; gap: var(--space-1); }
.state-box__body strong { color: var(--home-text-light); }

@media (max-width: 1080px) {
  .tier-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 860px) {
  .dynamic-shell { grid-template-columns: minmax(0, 1fr); padding: var(--space-5); }
  .guide { padding-inline-end: 0; padding-bottom: var(--space-5); border-inline-end: 0; border-bottom: var(--border-width) solid var(--home-border-subtle); }
  .guide__badge { margin-top: var(--space-5); }
}
@media (max-width: 560px) {
  .tier-grid { grid-template-columns: minmax(0, 1fr); }
}
</style>
