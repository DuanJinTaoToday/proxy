<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { HomeFooter, SupportChannels } from '@/features/home'
import DynamicPricingPanel from './DynamicPricingPanel.vue'
import StaticPricingPanel from './StaticPricingPanel.vue'
import { sharedPurposes, staticCategoryOf, usePublicPricing, type SharedPurpose } from './usePublicPricing'

// 官网公开报价页：动态住宅流量包、静态独享 IP、静态共享 IP（按用途）三类报价，各自加载、失败和重试。
type Category = 'dynamic' | 'exclusive' | 'shared'
const categories: Category[] = ['dynamic', 'exclusive', 'shared']

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const { dynamic, statics } = usePublicPricing()
const tabButtons = ref<HTMLButtonElement[]>([])

const category = computed<Category>(() => categories.find(name => name === route.query.category) ?? 'dynamic')
const purpose = computed<SharedPurpose>(() => sharedPurposes.find(id => id !== 0 && String(id) === route.query.purpose) ?? 0)

function replaceQuery(next: LocationQueryRaw) {
  void router.replace({ query: { ...route.query, ...next } })
}

function selectCategory(next: Category) {
  if (next === category.value) return
  replaceQuery({
    category: next === 'dynamic' ? undefined : next,
    purpose: next === 'shared' ? route.query.purpose : undefined,
  })
}

function selectPurpose(next: SharedPurpose) {
  replaceQuery({ category: 'shared', purpose: next ? String(next) : undefined })
}

function onTabKeydown(event: KeyboardEvent, index: number) {
  let target = -1
  if (event.key === 'ArrowRight') target = (index + 1) % categories.length
  else if (event.key === 'ArrowLeft') target = (index - 1 + categories.length) % categories.length
  else if (event.key === 'Home') target = 0
  else if (event.key === 'End') target = categories.length - 1
  if (target < 0) return
  event.preventDefault()
  selectCategory(categories[target])
  tabButtons.value[target]?.focus()
}

const dynamicState = computed(() => dynamic.state.value)
const activeStatic = computed(() => (
  category.value === 'shared'
    ? statics[staticCategoryOf('shared', purpose.value)]
    : statics.exclusive
))
const activeStaticState = computed(() => activeStatic.value.state.value)
</script>

<template>
  <div class="homepage pricing-page">
    <section class="pricing-hero">
      <div class="wrap pricing-hero__inner">
        <h1>{{ t(`publicPricing.tabs.${category}.title`) }}</h1>
        <p class="pricing-hero__sub">{{ t(`publicPricing.tabs.${category}.sub`) }}</p>

        <div class="cat-tabs" role="tablist" :aria-label="t('publicPricing.categoryLabel')">
          <button
            v-for="(cat, index) in categories"
            :key="cat"
            :id="`pricing-tab-${cat}`"
            ref="tabButtons"
            type="button"
            role="tab"
            :aria-selected="category === cat"
            :aria-controls="`pricing-panel-${cat}`"
            :tabindex="category === cat ? 0 : -1"
            class="cat-tabs__btn"
            :class="{ 'is-active': category === cat }"
            @click="selectCategory(cat)"
            @keydown="onTabKeydown($event, index)"
          >
            {{ t(`publicPricing.categories.${cat}`) }}
          </button>
        </div>
      </div>
    </section>

    <section class="pricing-body">
      <div class="wrap">
        <div
          v-if="category === 'dynamic'"
          id="pricing-panel-dynamic"
          role="tabpanel"
          aria-labelledby="pricing-tab-dynamic"
        >
          <DynamicPricingPanel :state="dynamicState" @retry="dynamic.load" />
        </div>
        <div
          v-else
          :id="`pricing-panel-${category}`"
          role="tabpanel"
          :aria-labelledby="`pricing-tab-${category}`"
        >
          <StaticPricingPanel
            :state="activeStaticState"
            :share-mode="category"
            :purpose="purpose"
            @update:purpose="selectPurpose"
            @retry="activeStatic.load"
          />
        </div>
        <p class="pricing-footnote">{{ t('publicPricing.footnote') }}</p>
      </div>
    </section>

    <HomeFooter />
    <SupportChannels variant="dock" />
  </div>
</template>

<style scoped>
.pricing-page { background: var(--home-light-alt); }
.pricing-hero {
  padding: calc(var(--home-header-height) + var(--home-pricing-hero-pad-top)) 0 var(--home-pricing-hero-pad-bottom);
  background: linear-gradient(180deg, var(--home-ice-start) 0%, var(--home-ice-mid) 56%, var(--home-ice-end) 100%);
}
.pricing-hero__inner {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}
h1 {
  font-size: var(--home-fs-h2);
  line-height: 1.2;
  font-weight: var(--font-weight-bold);
  letter-spacing: -.025em;
  color: var(--home-text-light);
}
.pricing-hero__sub {
  margin-top: var(--space-3);
  max-width: 40em;
  font-size: var(--home-fs-body);
  color: var(--home-muted-light);
}

.cat-tabs {
  display: inline-flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: var(--space-1);
  margin-top: var(--space-6);
  padding: var(--space-1);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-pill);
  background: var(--home-light);
  box-shadow: var(--home-card-shadow);
}
.cat-tabs__btn {
  min-height: var(--home-btn-sm-height);
  padding: 0 var(--space-5);
  border: 0;
  border-radius: var(--home-pill);
  background: transparent;
  color: var(--home-muted-light);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-semibold);
  cursor: pointer;
  transition: background .2s, color .2s;
}
.cat-tabs__btn:hover { color: var(--home-brand); }
.cat-tabs__btn.is-active {
  background: var(--home-brand);
  color: var(--home-on-brand);
}

.pricing-body {
  padding: var(--space-3) 0 var(--home-pricing-body-pad-bottom);
}
.pricing-footnote {
  margin-top: var(--space-5);
  font-size: var(--home-fs-micro);
  color: var(--home-faint-light);
  text-align: center;
}

@media (max-width: 560px) {
  .cat-tabs { width: 100%; border-radius: var(--home-radius-lg); }
  .cat-tabs__btn { flex: 1 1 auto; padding: 0 var(--space-3); }
}
</style>
