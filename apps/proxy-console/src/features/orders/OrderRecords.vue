<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter, type LocationQueryRaw } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElDatePicker, ElEmpty, ElInput, ElOption, ElSelect, ElSkeleton, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { listOrders, orderProducts, orderStatuses, type OrderSummary } from '@/api/orders'
import { readBusinessDateBounds, updateBusinessDateBounds } from '@/shared/lib/businessDateQuery'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import OrderDrawer from './OrderDrawer.vue'
import { readOrderId, readOrderSearch, useOrderFormatting } from './format'

const emit = defineEmits<{ pendingCount: [count: number] }>()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const errorMessage = useErrorMessage()
const { dateTime, delivery, itemDescription, money, statusTypes } = useOrderFormatting()

const product = computed(() => orderProducts.find(value => value === route.query.product))
const status = computed(() => orderStatuses.find(value => value === route.query.status))
const orderQuery = computed(() => readOrderSearch(route.query.order))
const dates = computed(() => readBusinessDateBounds(route.query.start, route.query.end))
const start = computed(() => dates.value.start)
const end = computed(() => dates.value.end)
const detailId = computed(() => readOrderId(route.query.detail))
const orderSearch = ref('')
const invalidSearch = ref(false)

watch(orderQuery, value => {
  orderSearch.value = value ?? ''
  invalidSearch.value = false
}, { immediate: true })

function editFilters(changed: LocationQueryRaw) {
  void router.push({ query: { ...route.query, ...changed } })
}

function searchOrders() {
  const searched = orderSearch.value.trim()
  const query = readOrderSearch(searched)
  invalidSearch.value = searched !== '' && query === undefined
  if (invalidSearch.value) return
  orderSearch.value = query ?? ''
  editFilters({ order: query })
}

function clearOrderSearch() {
  orderSearch.value = ''
  searchOrders()
}

function changeDate(field: 'start' | 'end', value: unknown) {
  editFilters(updateBusinessDateBounds(dates.value, field, value))
}

function resetFilters() {
  orderSearch.value = ''
  invalidSearch.value = false
  editFilters({ product: undefined, status: undefined, order: undefined, start: undefined, end: undefined })
}

function openOrder(order: OrderSummary) {
  void router.push({ query: { ...route.query, detail: order.id } })
}

function closeOrder() {
  void router.replace({ query: { ...route.query, detail: undefined } })
}

const cursorHistory = ref<(string | undefined)[]>([undefined])
const cursor = computed(() => cursorHistory.value.at(-1))
const retry = ref(0)
type ListState =
  | { kind: 'loading' }
  | { kind: 'failed', error: unknown }
  | { kind: 'ready', data: Awaited<ReturnType<typeof listOrders>> }
const state = shallowRef<ListState>({ kind: 'loading' })

watch([product, status, orderQuery, start, end], () => {
  cursorHistory.value = [undefined]
})

watch([product, status, orderQuery, start, end, cursor, retry], async ([product, status, orderQuery, start, end, beforeId], _previous, onCleanup) => {
  const request = new AbortController()
  onCleanup(() => request.abort())
  state.value = { kind: 'loading' }
  try {
    const data = await listOrders({ product, status, orderQuery, start, end, beforeId }, request.signal)
    if (!request.signal.aborted) {
      state.value = { kind: 'ready', data }
      emit('pendingCount', data.pendingCount)
    }
  } catch (error) {
    if (!request.signal.aborted) state.value = { kind: 'failed', error }
  }
}, { immediate: true })

function nextPage() {
  if (state.value.kind !== 'ready' || !state.value.data.nextBeforeId) return
  cursorHistory.value.push(state.value.data.nextBeforeId)
}

const hasFilters = computed(() => Boolean(product.value || status.value || orderQuery.value || start.value || end.value))
</script>

<template>
  <ElCard class="order-records" :body-style="{ padding: 0 }" shadow="never" role="region" :aria-label="t('tabs.accountOrders')">
    <div class="order-filters">
      <form class="order-search" @submit.prevent="searchOrders">
        <ElInput v-model="orderSearch" :prefix-icon="Search" :placeholder="t('orders.orderSearch')"
          :aria-label="t('orders.orderSearch')" :aria-invalid="invalidSearch"
          :aria-describedby="invalidSearch ? 'order-search-error' : undefined"
          clearable @clear="clearOrderSearch" />
        <ElButton native-type="submit">{{ t('orders.search') }}</ElButton>
      </form>
      <ElSelect :model-value="product ?? 'all'" class="select-filter product-filter" :aria-label="t('orders.productFilter')"
        @update:model-value="editFilters({ product: $event === 'all' ? undefined : $event })">
        <ElOption value="all" :label="t('orders.allProducts')" />
        <ElOption v-for="value in orderProducts" :key="value" :value="value" :label="t(`orders.products.${value}`)" />
      </ElSelect>
      <ElSelect :model-value="status ?? 'all'" class="select-filter" :aria-label="t('orders.statusFilter')"
        @update:model-value="editFilters({ status: $event === 'all' ? undefined : $event })">
        <ElOption value="all" :label="t('orders.allStatuses')" />
        <ElOption v-for="value in orderStatuses" :key="value" :value="value" :label="t(`orders.status.${value}`)" />
      </ElSelect>
      <div class="order-dates" role="group" :aria-label="t('orders.dateFilter')">
        <ElDatePicker :model-value="start" type="date" value-format="YYYY-MM-DD" class="date-filter"
          :aria-label="t('orders.startDate')" :placeholder="t('orders.startDate')"
          @update:model-value="changeDate('start', $event)" />
        <ElDatePicker :model-value="end" type="date" value-format="YYYY-MM-DD" class="date-filter"
          :aria-label="t('orders.endDate')" :placeholder="t('orders.endDate')"
          @update:model-value="changeDate('end', $event)" />
      </div>
    </div>
    <p v-if="invalidSearch" id="order-search-error" class="search-error" role="alert">{{ t('orders.invalidOrderId') }}</p>

    <div v-if="state.kind === 'loading'" class="order-feedback" role="status" :aria-label="t('orders.loading')">
      <ElSkeleton animated :rows="5" />
    </div>
    <ElAlert v-else-if="state.kind === 'failed'" class="order-feedback" type="error" :title="t('orders.listFailed')" show-icon :closable="false">
      <p>{{ errorMessage(state.error) }}</p>
      <ElButton size="small" @click="retry++">{{ t('orders.retry') }}</ElButton>
    </ElAlert>
    <template v-else>
      <ElTable :data="state.data.orders" row-key="id" class="order-table" header-row-class-name="order-table-heading" row-class-name="order-table-row" @row-click="openOrder">
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.number')" min-width="150">
          <template #default="{ row }">
            <span class="mono">{{ t('orders.orderNumber', { id: row.id }) }}</span>
          </template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.type')" min-width="110">
          <template #default="{ row }">{{ t(`orders.kinds.${row.kind}`) }}</template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.contents')" min-width="250">
          <template #default="{ row }">
            <template v-if="row.summaryItem">
              <span class="order-summary tabular">{{ row.itemCount > 1
                ? t('orders.itemSummary', { first: itemDescription(row.summaryItem), count: row.itemCount }, row.itemCount)
                : itemDescription(row.summaryItem) }}</span>
              <span v-if="row.summaryItem.termDays" class="order-note tabular">
                {{ t('orders.termDays', row.summaryItem.termDays) }}
              </span>
            </template>
            <span v-else>{{ t('orders.unavailable') }}</span>
          </template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.delivery')" min-width="110">
          <template #default="{ row }"><span class="tabular">{{ delivery(row) }}</span></template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.paid')" min-width="100" align="right">
          <template #default="{ row }">
            <span class="tabular">{{ money(row.settledMinor) }}</span>
            <span v-if="row.refundedMinor > 0n" class="order-note tabular">
              {{ t('orders.refundNote', { amount: money(row.refundedMinor) }) }}
            </span>
          </template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.state')" min-width="100">
          <template #default="{ row }">
            <ElTag :type="statusTypes[row.status]" size="small" disable-transitions>{{ t(`orders.status.${row.status}`) }}</ElTag>
          </template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn :label="t('orders.placedAt')" min-width="150">
          <template #default="{ row }">
            <time :datetime="row.createdAt.toISOString()">{{ dateTime(row.createdAt) }}</time>
          </template>
        </ElTableColumn>
        <!-- @vue-generic {OrderSummary} -->
        <ElTableColumn width="70" align="right">
          <template #default="{ row }">
            <ElButton link type="primary" @click.stop="openOrder(row)">{{ t('orders.details') }}</ElButton>
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty :image-size="72" :description="t(hasFilters ? 'orders.noneMatching' : 'orders.emptyOrders')">
            <ElButton v-if="hasFilters" @click="resetFilters">{{ t('orders.clearFilters') }}</ElButton>
          </ElEmpty>
        </template>
      </ElTable>
      <nav class="order-pagination" :aria-label="t('orders.pagination')">
        <span>{{ t('orders.pageEntries', { count: state.data.orders.length }) }}</span>
        <div v-if="cursorHistory.length > 1 || state.data.nextBeforeId" class="order-pagination__controls">
          <ElButton size="small" :disabled="cursorHistory.length < 2" @click="cursorHistory.pop()">{{ t('orders.previous') }}</ElButton>
          <ElButton size="small" :disabled="!state.data.nextBeforeId" @click="nextPage">{{ t('orders.next') }}</ElButton>
        </div>
      </nav>
    </template>
  </ElCard>
  <OrderDrawer :order-id="detailId" @close="closeOrder" />
</template>

<style scoped>
.order-records { border-radius: var(--dashboard-card-radius); min-inline-size: 0; }
.order-filters, .order-search, .order-dates, .order-pagination, .order-pagination__controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
.order-filters { padding: var(--space-3) var(--space-4); border-block-end: var(--border-width) solid var(--color-border); }
.order-search { max-width: 100%; }
.order-search .el-input { width: var(--tenant-order-search-width); max-width: 100%; }
.select-filter { width: var(--tenant-order-status-width); max-width: 100%; }
.product-filter { width: var(--tenant-order-product-width); }
.date-filter { --el-date-editor-width: min(40vw, var(--business-date-filter-width)); }
.search-error { margin: var(--space-3) var(--space-4); color: var(--color-danger-text); }
.order-feedback { margin: var(--space-4); }
.order-table {
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  font-size: var(--font-size-sm);
}
.order-table :deep(.order-table-heading) { height: var(--tenant-table-heading-height); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.order-table :deep(.order-table-row) { height: var(--tenant-table-row-height); cursor: pointer; }
.order-summary { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.order-note { display: block; color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.order-pagination { justify-content: space-between; padding: var(--space-3) var(--space-4); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.order-pagination__controls .el-button { margin-inline: 0; }
</style>
