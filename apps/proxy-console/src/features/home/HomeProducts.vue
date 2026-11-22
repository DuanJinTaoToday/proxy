<script setup lang="ts">
import { computed, onMounted, onScopeDispose, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElSkeleton } from 'element-plus'
import { getPublicCatalog } from '@/api/products'
import { listStaticOffers, type StaticPurchaseOffer } from '@/api/staticPricing'
import { formatYuan } from '@/shared/lib/money'
import { formatGB } from '@/shared/lib/trafficUnits'
import { exits, regions, sample } from './demo'

type Catalog = Awaited<ReturnType<typeof getPublicCatalog>>
type StaticPrice = { kind: 'priced', offer: StaticPurchaseOffer } | { kind: 'unavailable' | 'failed' | 'closed' }
type CatalogState = { kind: 'loading' | 'failed' } | { kind: 'ready', catalog: Catalog, staticPrice: StaticPrice }

const { locale, t } = useI18n()
const state = shallowRef<CatalogState>({ kind: 'loading' })
const activeOrbit = ref(0)
const currentOrbit = computed(() => exits[activeOrbit.value])
let request: AbortController | undefined
let rotation: number | undefined

onMounted(() => {
  if (!matchMedia('(prefers-reduced-motion: reduce)').matches) {
    rotation = window.setInterval(() => { activeOrbit.value = (activeOrbit.value + 1) % exits.length }, 2500)
  }
})
onScopeDispose(() => {
  request?.abort()
  clearInterval(rotation)
})

async function getStaticPrice(signal: AbortSignal): Promise<StaticPrice> {
  try {
    const { open, offers } = await listStaticOffers({ shareMode: 'exclusive' }, signal)
    if (!open) return { kind: 'closed' }
    const cheapest = offers.reduce<StaticPurchaseOffer | undefined>((lowest, offer) =>
      offer.available > 0 && (!lowest || offer.unitMinor < lowest.unitMinor) ? offer : lowest, undefined)
    return cheapest ? { kind: 'priced', offer: cheapest } : { kind: 'unavailable' }
  } catch {
    return { kind: 'failed' }
  }
}

async function loadProducts() {
  request?.abort()
  const { signal } = request = new AbortController()
  state.value = { kind: 'loading' }
  try {
    const catalog = await getPublicCatalog(signal)
    const staticPrice: StaticPrice = catalog.staticOpen ? await getStaticPrice(signal) : { kind: 'closed' }
    if (!signal.aborted) state.value = { kind: 'ready', catalog, staticPrice }
  } catch {
    if (!signal.aborted) state.value = { kind: 'failed' }
  }
}

void loadProducts()
</script>

<template>
  <section id="products" class="d-sec choose">
    <div class="wrap">
      <h2 class="statement">{{ t('marketing.dynamicChoice') }}<span>{{ t('marketing.staticChoice') }}</span></h2>
      <ElAlert v-if="state.kind === 'failed'" class="catalog-alert" :title="t('marketing.catalogFailed')" type="error" :closable="false" show-icon>
        <ElButton size="small" @click="loadProducts">{{ t('marketing.retry') }}</ElButton>
      </ElAlert>

      <div class="showcase-grid">
        <article id="dynamic-product" class="stage-card dynamic" aria-labelledby="home-dynamic-title">
          <div class="visual-stage orbit-stage" role="img" :aria-label="t('marketing.rotationSample')">
            <div v-for="ring in 3" :key="ring" class="ring" :class="`ring-${ring}`" />
            <div class="orbit-core"><span>SOCKS5</span><small>{{ t('marketing.gateway') }}</small></div>
            <div v-for="(node, index) in exits" :key="node.sid" class="orbit-pill" :class="{ active: index === activeOrbit }" :style="{ left: node.x, top: node.y }">
              <i /><span>{{ t(`marketing.cities.${node.city}`) }}</span>
            </div>
            <div class="stage-bar">
              <code>{{ sample.username }}<b>-sid-{{ currentOrbit.sid }}</b></code>
              <span class="stage-live">→ {{ t(`marketing.cities.${currentOrbit.city}`) }}</span>
            </div>
          </div>
          <div class="stage-copy">
            <div class="stage-title-row">
              <h3 id="home-dynamic-title">{{ t('marketing.dynamic') }}</h3>
              <span v-if="state.kind === 'ready' && !state.catalog.dynamic.canPurchase" class="closed-tag">{{ t('marketing.closed') }}</span>
            </div>
            <p class="stage-desc">{{ t('marketing.dynamicDesc') }}</p>
            <ElSkeleton v-if="state.kind === 'loading'" :rows="1" :aria-label="t('marketing.loading')" />
            <template v-else-if="state.kind === 'ready'">
              <ul v-if="state.catalog.dynamic.canPurchase" class="package-list">
                <li v-for="tier in state.catalog.dynamic.packages" :key="tier.id" class="stage-footer">
                  <div class="stage-price"><strong>{{ formatYuan(tier.priceMinor, locale) }}</strong><span>{{ formatGB(tier.capacityBytes, locale) }} GB</span></div>
                  <RouterLink class="stage-link" :to="{ name: 'console-dynamic-pricing', query: { package: tier.id } }" :aria-label="t('marketing.buyTier', { capacity: formatGB(tier.capacityBytes, locale) })">{{ t('trafficPricing.buy') }} →</RouterLink>
                </li>
              </ul>
              <p v-else class="stage-footer stage-status">{{ t('marketing.comingPrice') }}</p>
            </template>
          </div>
        </article>

        <article id="static-product" class="stage-card static" aria-labelledby="home-static-title">
          <div class="visual-stage static-stage" role="img" :aria-label="t('marketing.renewalSample')">
            <div class="region-rail">
              <span v-for="region in regions" :key="region" :class="{ active: region === 'KR' }"><b>{{ region }}</b><small>{{ t(`marketing.regions.${region}`) }}</small></span>
            </div>
            <div class="white-ip-card">
              <div class="wic-head"><span class="wic-region">{{ t('marketing.regions.KR') }}</span><span class="wic-warn">{{ t('marketing.expiresSoon') }}</span></div>
              <code class="wic-addr">203.0.113.24:1080</code>
              <p class="wic-sub">{{ t('marketing.renewKeeps') }}</p>
              <div class="wic-btn">{{ t('marketing.renew30') }}</div>
              <div class="wic-dates"><span>{{ t('marketing.oldExpiry', { date: '2026-10-31' }) }}</span><b>→ 2026-11-30</b></div>
            </div>
          </div>
          <div class="stage-copy">
            <div class="stage-title-row">
              <h3 id="home-static-title">{{ t('marketing.static') }}</h3>
              <span v-if="state.kind === 'ready' && state.staticPrice.kind === 'closed'" class="closed-tag">{{ t('marketing.closed') }}</span>
            </div>
            <p class="stage-desc">{{ t('marketing.staticDesc') }}</p>
            <ElSkeleton v-if="state.kind === 'loading'" :rows="1" :aria-label="t('marketing.loading')" />
            <div v-else-if="state.kind === 'ready'" class="stage-footer">
              <template v-if="state.staticPrice.kind === 'priced'">
                <div class="stage-price">
                  <strong>{{ formatYuan(state.staticPrice.offer.unitMinor, locale) }}</strong>
                  <span>{{ t('marketing.staticPrice', { days: state.staticPrice.offer.termDays }) }}</span>
                </div>
                <RouterLink class="stage-link" :to="{ name: 'console-static-pricing' }">{{ t('marketing.seeStatic') }} →</RouterLink>
              </template>
              <template v-else-if="state.staticPrice.kind === 'failed'">
                <span class="stage-status" role="status">{{ t('marketing.staticFailed') }}</span>
                <ElButton size="small" @click="loadProducts">{{ t('marketing.retry') }}</ElButton>
              </template>
              <span v-else class="stage-status">{{ t(state.staticPrice.kind === 'closed' ? 'marketing.comingPrice' : 'marketing.noStatic') }}</span>
            </div>
          </div>
        </article>
      </div>
      <p class="footnote">{{ t('marketing.footnote') }}</p>
    </div>
  </section>
</template>

<style scoped src="./products.css"></style>
