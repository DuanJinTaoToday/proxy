<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert, ElButton, ElCheckbox, ElEmpty, ElInput, ElOption, ElPagination, ElRadioButton, ElRadioGroup,
  ElSelect, ElSkeleton, ElTable, ElTableColumn, ElTag,
} from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import type { StaticPurchaseOffer } from '@/api/staticPricing'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { formatYuan } from '@/shared/lib/money'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { usePricingError } from './pricingError'
import type { Loadable, StaticPricing } from './usePublicPricing'

// 一个静态报价分类（独享或某一共享用途）：状态、筛选、排序与分页都在分类内完成。
const props = defineProps<{ state: Loadable<StaticPricing>, shareMode: 'exclusive' | 'shared' }>()
defineEmits<{ retry: [] }>()

const { t, locale } = useI18n()
const errorText = usePricingError()
const pageSize = 20
type SortKey = 'price-asc' | 'price-desc' | 'stock' | 'region'
const sortKeys: SortKey[] = ['price-asc', 'price-desc', 'stock', 'region']

const term = ref('')
const continent = ref('')
const region = ref('')
const keyword = ref('')
const inStockOnly = ref(false)
const sort = ref<SortKey>('price-asc')
const page = ref(1)

const catalog = computed(() => props.state.kind === 'ready' && props.state.data.open ? props.state.data : undefined)
const offers = computed(() => catalog.value?.offers ?? [])
const inStock = computed(() => offers.value.filter(offer => offer.available > 0).length)

function optionsOf(field: 'continent' | 'region') {
  return [...new Set(offers.value.map(offer => offer[field]).filter(Boolean))].sort((a, b) => a.localeCompare(b, locale.value))
}
const continents = computed(() => optionsOf('continent'))
const regions = computed(() => optionsOf('region'))
const terms = computed(() => [...new Map(offers.value.map(offer => [offer.term, { code: offer.term, days: offer.termDays }])).values()]
  .sort((a, b) => a.days - b.days))
const hasLines = computed(() => offers.value.some(offer => offer.line))

// 换用途或重新加载后，已选筛选值不在新目录中时回到“全部”，避免误显示为无结果。
watch(offers, () => {
  if (term.value && !terms.value.some(option => option.code === term.value)) term.value = ''
  if (continent.value && !continents.value.includes(continent.value)) continent.value = ''
  if (region.value && !regions.value.includes(region.value)) region.value = ''
})

const compareMinor = (a: bigint, b: bigint) => a < b ? -1 : a > b ? 1 : 0
const visible = computed(() => {
  const search = keyword.value.trim().toLocaleLowerCase(locale.value)
  const rows = offers.value.filter(offer => (!term.value || offer.term === term.value)
    && (!continent.value || offer.continent === continent.value)
    && (!region.value || offer.region === region.value)
    && (!inStockOnly.value || offer.available > 0)
    && (!search || [offer.region, offer.continent, offer.isp, offer.line]
      .some(label => label.toLocaleLowerCase(locale.value).includes(search))))
  const order = (a: StaticPurchaseOffer, b: StaticPurchaseOffer) => {
    switch (sort.value) {
      case 'price-desc': return compareMinor(b.unitMinor, a.unitMinor)
      case 'stock': return b.available - a.available
      case 'region': return a.region.localeCompare(b.region, locale.value) || a.termDays - b.termDays
      default: return compareMinor(a.unitMinor, b.unitMinor)
    }
  }
  // 售罄的报价始终排在可购买报价之后。
  return rows.sort((a, b) => Number(b.available > 0) - Number(a.available > 0) || order(a, b))
})
const pageRows = computed(() => visible.value.slice((page.value - 1) * pageSize, page.value * pageSize))
const filtered = computed(() => Boolean(term.value || continent.value || region.value || keyword.value.trim() || inStockOnly.value))
watch([term, continent, region, keyword, inStockOnly, sort, offers], () => { page.value = 1 })

function resetFilters() {
  term.value = ''
  continent.value = ''
  region.value = ''
  keyword.value = ''
  inStockOnly.value = false
}

const count = (value: number) => new Intl.NumberFormat(locale.value).format(value)
const money = (value: bigint) => formatYuan(value, locale.value)
const termDays = (days: number) => t('publicPricing.termDays', { days: count(days) }, days)
const offerKey = (offer: StaticPurchaseOffer) => `${offer.nodeId}:${offer.term}`
const quotedAt = computed(() => {
  const value = catalog.value?.quotedAt
  const date = value ? new Date(value) : undefined
  return date && !Number.isNaN(date.getTime()) ? shanghaiDateTime(date, locale.value) : undefined
})
</script>

<template>
  <div class="static-catalog" :aria-busy="state.kind === 'loading'">
    <ElSkeleton v-if="state.kind === 'loading'" animated :rows="6" :aria-label="t('publicPricing.loading')" />
    <div v-else-if="state.kind === 'failed'" class="catalog-state" data-state="failed" role="alert">
      <ElAlert type="error" :title="t('publicPricing.staticFailed')" :description="errorText(state.error)" :closable="false" show-icon />
      <ElButton type="primary" @click="$emit('retry')">{{ t('publicPricing.retry') }}</ElButton>
    </div>
    <ElEmpty v-else-if="!state.data.open" data-state="closed" :image-size="72"
      :description="t(shareMode === 'exclusive' ? 'publicPricing.exclusiveClosed' : 'publicPricing.sharedClosed')" />
    <ElEmpty v-else-if="!offers.length" data-state="empty" :image-size="72" :description="t('publicPricing.noOffers')" />
    <template v-else>
      <ElAlert v-if="!inStock" data-state="sold-out" class="catalog-notice" type="warning" :closable="false" show-icon
        :title="t('publicPricing.allSoldOut')" :description="t('publicPricing.allSoldOutHint')" />

      <div class="toolbar" role="search" :aria-label="t('publicPricing.filters')">
        <div class="toolbar-row">
          <ElInput v-model="keyword" class="toolbar-search" :prefix-icon="Search" clearable
            :aria-label="t('publicPricing.search')" :placeholder="t('publicPricing.search')" />
          <ElSelect v-if="continents.length > 1" v-model="continent" class="toolbar-select" :empty-values="[null, undefined]"
            :aria-label="t('publicPricing.continent')">
            <ElOption value="" :label="t('publicPricing.allContinents')" />
            <ElOption v-for="name in continents" :key="name" :value="name" :label="name" />
          </ElSelect>
          <ElSelect v-model="region" class="toolbar-select" filterable :empty-values="[null, undefined]"
            :aria-label="t('publicPricing.region')">
            <ElOption value="" :label="t('publicPricing.allRegions')" />
            <ElOption v-for="name in regions" :key="name" :value="name" :label="name" />
          </ElSelect>
          <ElSelect v-model="sort" class="toolbar-select" :aria-label="t('publicPricing.sort')">
            <ElOption v-for="key in sortKeys" :key="key" :value="key" :label="t(`publicPricing.sortBy.${key}`)" />
          </ElSelect>
          <ElCheckbox v-model="inStockOnly">{{ t('publicPricing.inStockOnly') }}</ElCheckbox>
        </div>
        <div v-if="terms.length > 1" class="toolbar-row">
          <span class="toolbar-label">{{ t('publicPricing.term') }}</span>
          <ElRadioGroup v-model="term" :aria-label="t('publicPricing.term')">
            <ElRadioButton value="">{{ t('publicPricing.allTerms') }}</ElRadioButton>
            <ElRadioButton v-for="option in terms" :key="option.code" :value="option.code">{{ termDays(option.days) }}</ElRadioButton>
          </ElRadioGroup>
        </div>
      </div>

      <p class="result-count muted" role="status">
        {{ t('publicPricing.resultCount', { shown: count(visible.length), total: count(offers.length), inStock: count(inStock) }) }}
      </p>

      <ElEmpty v-if="!visible.length" data-state="no-match" :image-size="56" :description="t('publicPricing.noMatch')">
        <ElButton v-if="filtered" @click="resetFilters">{{ t('publicPricing.clearFilters') }}</ElButton>
      </ElEmpty>
      <div v-else class="table-scroll">
        <ElTable :data="pageRows" :row-key="offerKey" class="offer-table tabular" :aria-label="t('publicPricing.staticTable')">
          <ElTableColumn :label="t('publicPricing.region')" min-width="168" fixed>
            <template #default="{ row }">
              <strong class="cell-title">{{ row.region || t('publicPricing.unknownRegion') }}</strong>
              <span v-if="row.continent" class="cell-sub">{{ row.continent }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="t('publicPricing.type')" min-width="150">
            <template #default="{ row }">
              <span class="cell-title">{{ t(shareMode === 'exclusive' ? 'publicPricing.exclusiveShort' : 'publicPricing.sharedShort') }}</span>
              <span v-if="row.isp" class="cell-sub">{{ row.isp }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn v-if="hasLines" :label="t('publicPricing.line')" min-width="110">
            <template #default="{ row }">{{ row.line || '—' }}</template>
          </ElTableColumn>
          <ElTableColumn :label="t('publicPricing.term')" min-width="100">
            <template #default="{ row }">{{ termDays(row.termDays) }}</template>
          </ElTableColumn>
          <ElTableColumn :label="t('publicPricing.availability')" min-width="130">
            <template #default="{ row }">
              <ElTag v-if="row.available > 0" type="success" disable-transitions>{{ t('publicPricing.available', { count: count(row.available) }) }}</ElTag>
              <ElTag v-else type="info" disable-transitions>{{ t('publicPricing.soldOut') }}</ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="t('publicPricing.pricePerIp')" min-width="130" align="right">
            <template #default="{ row }">
              <strong class="price">{{ money(row.unitMinor) }}</strong>
              <span class="cell-sub">{{ t('publicPricing.perIpTerm', { days: count(row.termDays) }, row.termDays) }}</span>
            </template>
          </ElTableColumn>
        </ElTable>
      </div>
      <ElPagination v-if="visible.length > pageSize" v-model:current-page="page" class="pagination" :page-size="pageSize"
        :total="visible.length" layout="total, prev, pager, next" :pager-count="5" size="small" background />

      <div class="catalog-footer">
        <p class="muted">
          <template v-if="quotedAt">{{ t('publicPricing.quotedAt', { time: quotedAt }) }}</template>
          {{ t('publicPricing.staticNote') }}
        </p>
        <LinkButton v-if="inStock" :to="{ name: 'console-static-pricing' }" type="primary">{{ t('publicPricing.buyStatic') }}</LinkButton>
      </div>
    </template>
  </div>
</template>

<style scoped>
.static-catalog, .catalog-state, .toolbar { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-3); min-width: 0; }
.static-catalog { gap: var(--space-4); }
.catalog-state { justify-items: start; }
.muted { color: var(--color-text-secondary); }
.toolbar-row { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-3); }
.toolbar-search { width: var(--usage-date-range-width); max-width: 100%; }
.toolbar-select { width: calc(var(--account-name-width) + var(--space-4)); max-width: 100%; }
.toolbar-label { font-weight: var(--font-weight-semibold); }
.result-count { font-size: var(--font-size-sm); }
.table-scroll { min-width: 0; }
.cell-title { display: block; overflow-wrap: anywhere; }
.cell-sub { display: block; color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.price { display: block; font-size: var(--font-size-lg); font-weight: var(--font-weight-semibold); }
.pagination { justify-content: flex-end; flex-wrap: wrap; }
.catalog-footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-3); font-size: var(--font-size-sm); }
.catalog-footer p { flex: 1 1 var(--wallet-summary-card-width); }
@media (max-width: 767px) {
  .toolbar-search, .toolbar-select { width: 100%; }
  .pagination { justify-content: center; }
}
</style>
