<script setup lang="ts">
import { onScopeDispose, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElSkeleton } from 'element-plus'
import { getPublicCatalog } from '@/api/products'
import { listStaticOffers, type StaticPurchaseOffer } from '@/api/staticPricing'
import { formatYuan } from '@/shared/lib/money'
import { formatGB } from '@/shared/lib/trafficUnits'
import LinkButton from '@/shared/ui/LinkButton.vue'

type PublicCatalog = Awaited<ReturnType<typeof getPublicCatalog>>
type CatalogueState =
  | { kind: 'loading' }
  | { kind: 'failed' }
  | { kind: 'ready', catalog: PublicCatalog, staticOffer?: StaticPurchaseOffer }

const { t, locale } = useI18n()
const state = shallowRef<CatalogueState>({ kind: 'loading' })
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

async function loadProducts() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  try {
    const catalog = await getPublicCatalog(controller.signal)
    const stock = catalog.staticOpen ? await listStaticOffers({ shareMode: 'exclusive' }, controller.signal).catch(() => undefined) : undefined
    const staticOffer = stock?.offers.reduce<StaticPurchaseOffer | undefined>((lowest, offer) => {
      return offer.available > 0 && (!lowest || offer.unitMinor < lowest.unitMinor) ? offer : lowest
    }, undefined)
    if (!controller.signal.aborted) state.value = { kind: 'ready', catalog, staticOffer }
  } catch {
    if (!controller.signal.aborted) state.value = { kind: 'failed' }
  }
}

const money = (amount: bigint) => formatYuan(amount, locale.value)
void loadProducts()
</script>

<template>
  <section class="home-hero">
    <h1 class="home-hero__title">{{ t('home.title') }}</h1>
    <p class="home-hero__subtitle">{{ t('home.subtitle') }}</p>
    <LinkButton to="/console" type="primary" size="large">{{ t('home.console') }}</LinkButton>
  </section>

  <section class="home-products" :aria-label="t('home.products')">
    <ElSkeleton v-if="state.kind === 'loading'" :rows="3" />
    <ElAlert v-else-if="state.kind === 'failed'" :title="t('home.catalogError')" type="error" :closable="false" show-icon>
      <ElButton size="small" @click="loadProducts">{{ t('auth.retry') }}</ElButton>
    </ElAlert>
    <template v-else>
      <ElCard shadow="never" role="region" aria-labelledby="home-dynamic-title">
        <h2 id="home-dynamic-title">{{ t('nav.dynamic') }}</h2>
        <p>{{ t('home.dynamicDescription') }}</p>
        <ul v-if="state.catalog.dynamic.canPurchase" class="home-products__tiers">
          <li v-for="tier in state.catalog.dynamic.packages" :key="tier.id">
            <span>{{ t('home.trafficTier', { capacity: formatGB(tier.capacityBytes, locale), price: money(tier.priceMinor) }) }}</span>
            <LinkButton :to="{ name: 'console-dynamic-pricing', query: { package: tier.id } }" type="primary" size="small">{{ t('trafficPricing.buy') }}</LinkButton>
          </li>
        </ul>
        <p v-else>{{ t('trafficPricing.closed') }}</p>
      </ElCard>

      <ElCard shadow="never" role="region" aria-labelledby="home-static-title">
        <h2 id="home-static-title">{{ t('nav.static') }}</h2>
        <p>{{ t('home.staticDescription') }}</p>
        <template v-if="state.catalog.staticOpen">
          <p v-if="state.staticOffer">{{ t('home.staticFrom', { price: money(state.staticOffer.unitMinor), days: state.staticOffer.termDays }) }}</p>
          <LinkButton :to="{ name: 'console-static-pricing' }" type="primary" size="small">{{ t('home.staticPricing') }}</LinkButton>
        </template>
        <p v-else>{{ t('trafficPricing.closed') }}</p>
      </ElCard>
    </template>
  </section>
</template>

<style scoped>
.home-hero { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-4); max-width: var(--content-narrow-width); }
.home-hero__title { font-size: var(--font-size-display); font-weight: var(--font-weight-bold); line-height: var(--line-height-tight); }
.home-hero__subtitle { color: var(--color-text-secondary); font-size: var(--font-size-lg); }
.home-products { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--wallet-summary-card-width)), 1fr)); gap: var(--space-4); margin-block-start: var(--space-6); }
.home-products h2 { margin-block-end: var(--space-2); font-size: var(--font-size-xl); }
.home-products p { margin-block-end: var(--space-3); color: var(--color-text-secondary); }
.home-products__tiers { padding: 0; margin: 0; list-style: none; }
.home-products__tiers li { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: var(--space-2); padding-block: var(--space-2); border-block-start: var(--border-width) solid var(--color-border); }
</style>
