<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElEmpty, ElRadioButton, ElRadioGroup, ElSkeleton, ElTabPane, ElTabs, ElTag } from 'element-plus'
import { hasSession } from '@/features/auth'
import { unitPrice } from '@/features/catalog'
import { formatYuan } from '@/shared/lib/money'
import { formatGB } from '@/shared/lib/trafficUnits'
import LinkButton from '@/shared/ui/LinkButton.vue'
import PageHeader from '@/shared/ui/PageHeader.vue'
import StaticOfferCatalog from './StaticOfferCatalog.vue'
import { usePricingError } from './pricingError'
import { sharedPurposes, staticCategoryOf, usePublicPricing, type Loadable, type SharedPurpose } from './usePublicPricing'

// 官网游客价格页：动态流量包、独享静态 IP、共享静态 IP（按用途）三类报价，各自加载、失败和重试。
type Category = 'dynamic' | 'exclusive' | 'shared'
type Status = { type: 'success' | 'info' | 'warning' | 'danger', key: string }
const categories: Category[] = ['dynamic', 'exclusive', 'shared']

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const errorText = usePricingError()
const signedIn = hasSession()
const { dynamic, statics } = usePublicPricing()

// 当前分类与用途写入 URL，便于分享、刷新和切换语言后保留。
const category = computed<Category>(() => categories.find(name => name === route.query.category) ?? 'dynamic')
const purpose = computed<SharedPurpose>(() => sharedPurposes.find(id => id !== 0 && String(id) === route.query.purpose) ?? 0)
function replaceQuery(next: LocationQueryRaw) {
  void router.replace({ query: { ...route.query, ...next } })
}
function selectCategory(name: string | number) {
  const next = categories.find(item => item === name)
  if (next) replaceQuery({ category: next === 'dynamic' ? undefined : next, purpose: next === 'shared' ? route.query.purpose : undefined })
}
function selectPurpose(value: string | number | boolean | undefined) {
  const next = sharedPurposes.find(id => id === value)
  if (next !== undefined) replaceQuery({ category: 'shared', purpose: next ? String(next) : undefined })
}

const dynamicState = computed(() => dynamic.state.value)
const exclusive = statics.exclusive
const exclusiveState = computed(() => exclusive.state.value)
const shared = computed(() => statics[staticCategoryOf('shared', purpose.value)])
const sharedState = computed(() => shared.value.state.value)

function statusOf<T>(state: Loadable<T>, onSale: (data: T) => boolean, available: (data: T) => number): Status | undefined {
  if (state.kind === 'loading') return undefined
  if (state.kind === 'failed') return { type: 'danger', key: 'publicPricing.status.failed' }
  if (!onSale(state.data)) return { type: 'info', key: 'publicPricing.status.closed' }
  return available(state.data) > 0
    ? { type: 'success', key: 'publicPricing.status.onSale' }
    : { type: 'warning', key: 'publicPricing.status.unavailable' }
}
const inStock = (offers: { available: number }[]) => offers.filter(offer => offer.available > 0).length
const staticStatus = (state: Loadable<{ open: boolean, offers: { available: number }[] }>) =>
  statusOf(state, data => data.open, data => inStock(data.offers))
const statuses = computed<Record<Category, Status | undefined>>(() => {
  // 共享页签汇总各用途：任一用途可购即为在售，否则依次提示加载失败、未开售或无可售报价。
  const sharedStatuses = sharedPurposes.map(id => statics[staticCategoryOf('shared', id)].state.value)
  const sharedSummary = sharedStatuses.map(staticStatus)
  const sharedStatus = sharedSummary.find(status => status?.type === 'success')
    ?? (sharedSummary.includes(undefined) ? undefined : sharedSummary.find(status => status?.type === 'danger')
      ?? sharedSummary.find(status => status?.type === 'warning') ?? sharedSummary[0])
  return {
    dynamic: statusOf(dynamicState.value, data => data.onSale, data => data.packages.length),
    exclusive: staticStatus(exclusiveState.value),
    shared: sharedStatus,
  }
})

const money = (minor: bigint) => formatYuan(minor, locale.value)
const capacity = (bytes: bigint) => formatGB(bytes, locale.value)
const packages = computed(() => dynamicState.value.kind === 'ready'
  ? dynamicState.value.data.packages.map(tier => ({ ...tier, unitMinor: unitPrice(tier) }))
  : [])
</script>

<template>
  <div class="public-pricing">
    <PageHeader :title="t('publicPricing.title')">
      <template #actions>
        <LinkButton v-if="signedIn" :to="{ name: 'console-dashboard' }" type="primary">{{ t('publicPricing.openConsole') }}</LinkButton>
        <LinkButton v-else :to="`/${locale}/register`" type="primary">{{ t('publicPricing.register') }}</LinkButton>
      </template>
    </PageHeader>
    <p class="lead">{{ t('publicPricing.lead') }}</p>

    <ElTabs :model-value="category" class="pricing-tabs" @tab-change="selectCategory">
      <ElTabPane v-for="name in categories" :key="name" :name="name" lazy>
        <template #label>
          <span class="tab-label">
            {{ t(`publicPricing.categories.${name}`) }}
            <ElTag v-if="statuses[name]" :type="statuses[name]?.type" size="small" disable-transitions>{{ t(statuses[name]?.key ?? '') }}</ElTag>
          </span>
        </template>

        <section v-if="name === 'dynamic'" id="pricing-dynamic" class="category" :aria-label="t('publicPricing.categories.dynamic')"
          :aria-busy="dynamicState.kind === 'loading'">
          <p class="hint">{{ t('publicPricing.dynamicHint') }}</p>
          <ElSkeleton v-if="dynamicState.kind === 'loading'" animated :rows="4" :aria-label="t('publicPricing.loading')" />
          <div v-else-if="dynamicState.kind === 'failed'" class="catalog-state" data-state="failed" role="alert">
            <ElAlert type="error" :title="t('publicPricing.dynamicFailed')" :description="errorText(dynamicState.error)" :closable="false" show-icon />
            <ElButton type="primary" @click="dynamic.load">{{ t('publicPricing.retry') }}</ElButton>
          </div>
          <ElEmpty v-else-if="!dynamicState.data.onSale" data-state="closed" :image-size="72" :description="t('publicPricing.dynamicClosed')" />
          <ElEmpty v-else-if="!packages.length" data-state="empty" :image-size="72" :description="t('publicPricing.noPackages')" />
          <ul v-else class="packages tabular">
            <li v-for="tier in packages" :key="tier.id" class="package">
              <span v-if="tier.discountPercent > 0" class="savings">{{ t('publicPricing.savings', { percent: tier.discountPercent }) }}</span>
              <strong class="package-size">{{ capacity(tier.capacityBytes) }} <small>GB</small></strong>
              <strong class="package-price">{{ money(tier.priceMinor) }}</strong>
              <span class="muted">{{ t('publicPricing.perGb', { amount: money(tier.unitMinor) }) }}</span>
              <LinkButton class="package-buy" :to="{ name: 'console-dynamic-pricing', query: { package: tier.id } }">
                {{ t('publicPricing.buyPackage', { capacity: capacity(tier.capacityBytes) }) }}
              </LinkButton>
            </li>
          </ul>
        </section>

        <section v-else-if="name === 'exclusive'" id="pricing-exclusive" class="category" :aria-label="t('publicPricing.categories.exclusive')">
          <p class="hint">{{ t('publicPricing.exclusiveHint') }}</p>
          <StaticOfferCatalog :state="exclusiveState" share-mode="exclusive" @retry="exclusive.load" />
        </section>

        <section v-else id="pricing-shared" class="category" :aria-label="t('publicPricing.categories.shared')">
          <p class="hint">{{ t('publicPricing.sharedHint') }}</p>
          <div class="purpose">
            <span class="purpose-label">{{ t('publicPricing.purpose') }}</span>
            <ElRadioGroup :model-value="purpose" :aria-label="t('publicPricing.purpose')" @change="selectPurpose">
              <ElRadioButton v-for="id in sharedPurposes" :key="id" :value="id">{{ t(`publicPricing.purposes.${id}`) }}</ElRadioButton>
            </ElRadioGroup>
            <span class="muted purpose-hint">{{ t('publicPricing.purposeHint') }}</span>
          </div>
          <StaticOfferCatalog :state="sharedState" share-mode="shared" @retry="shared.load" />
        </section>
      </ElTabPane>
    </ElTabs>

    <p class="footnote muted">{{ t('publicPricing.footnote') }}</p>
  </div>
</template>

<style scoped>
.public-pricing { --page-heading-gap: 0px; display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-4); min-width: 0; }
.lead { max-width: var(--content-narrow-width); color: var(--color-text-secondary); font-size: var(--font-size-lg); }
.muted { color: var(--color-text-secondary); }
.tab-label { display: inline-flex; align-items: center; gap: var(--space-2); }
.category, .catalog-state { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-4); min-width: 0; }
.catalog-state { justify-items: start; gap: var(--space-3); }
.hint { color: var(--color-text-secondary); }
.purpose { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); }
.purpose-label { font-weight: var(--font-weight-semibold); }
.purpose-hint { font-size: var(--font-size-xs); }
.packages {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(min(100%, var(--static-offer-width)), 1fr));
  gap: var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}
.package {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: start;
  gap: var(--space-1);
  min-width: 0;
  padding: var(--space-5);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
}
.package-size { font-size: var(--font-size-metric); line-height: var(--line-height-tight); overflow-wrap: anywhere; }
.package-size small { color: var(--color-text-secondary); font-size: var(--font-size-md); }
.package-price { font-size: var(--font-size-brand); overflow-wrap: anywhere; }
.package .muted { font-size: var(--font-size-xs); }
.package-buy { margin-top: var(--space-3); }
.savings {
  position: absolute;
  inset-block-start: var(--space-3);
  inset-inline-end: var(--space-3);
  padding-inline: var(--space-2);
  border-radius: var(--radius-control);
  background: var(--color-primary-soft);
  color: var(--color-success-text);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
}
.footnote { font-size: var(--font-size-sm); }
@media (max-width: 767px) {
  .lead { font-size: var(--font-size-md); }
  .package { padding: var(--space-4); }
  .savings { position: static; align-self: end; }
}
</style>
