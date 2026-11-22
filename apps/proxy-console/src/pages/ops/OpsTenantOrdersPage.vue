<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Search } from '@element-plus/icons-vue'
import { ElButton, ElEmpty, ElInput, ElOption, ElSelect, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { listOpsTenantOrders } from '@/api/opsTenants'
import { orderKinds, orderStatuses, type OrderSummary } from '@/api/orders'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { readOrderSearch, useOrderFormatting } from '@/features/orders'
import OpsHistoryTable from './OpsHistoryTable.vue'
import { useOpsHistoryPage } from './useOpsHistoryPage'

const props = defineProps<{ tenantId: string }>()
const emit = defineEmits<{ pendingCount: [count: number] }>()
const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const { itemDescription, money, pendingDuration, statusTypes } = useOrderFormatting()
const kind = computed(() => orderKinds.find(value => value === route.query.kind))
const status = computed(() => orderStatuses.find(value => value === route.query.status))
const orderQuery = computed(() => readOrderSearch(route.query.orderQuery))
const orderSearch = ref('')
watch(orderQuery, value => {
  orderSearch.value = value ?? ''
}, { immediate: true })
const history = useOpsHistoryPage(async (beforeId, signal) => {
  const result = await listOpsTenantOrders(props.tenantId, { kind: kind.value, status: status.value, orderQuery: orderQuery.value, beforeId }, signal)
  if (!signal.aborted) emit('pendingCount', result.pendingCount)
  return { rows: result.orders, nextBeforeId: result.nextBeforeId }
})
const { state } = history
watch([kind, status, orderQuery], history.reset, { immediate: true })

function openOrder(row: OrderSummary) {
  void router.push({ query: { ...route.query, order: row.id } })
}

function search() {
  const orderQuery = readOrderSearch(orderSearch.value)
  return router.push({ query: { ...route.query, orderQuery } })
}

function filter(nextKind?: unknown, nextStatus?: unknown, nextQuery = orderQuery.value) {
  return router.push({ query: {
    ...route.query,
    kind: orderKinds.find(value => value === nextKind),
    status: orderStatuses.find(value => value === nextStatus),
    orderQuery: nextQuery || undefined,
  } })
}
</script>

<template>
  <OpsHistoryTable :label="t('users.opsOrders')" :state="state" :page="history.page.value"
    @retry="history.load" @previous="history.previous" @next="history.next">
    <template #filters>
      <form class="ops-order-search" @submit.prevent="search">
        <ElInput v-model="orderSearch" class="ops-order-input" :prefix-icon="Search" clearable maxlength="20" inputmode="numeric"
          :aria-label="t('orders.orderSearch')" :placeholder="t('orders.orderSearch')" @change="search" />
      </form>
      <ElSelect class="ops-order-kind" :model-value="kind ?? 'all'" :aria-label="t('orders.kindFilter')"
        @update:model-value="filter($event, status)">
        <ElOption value="all" :label="t('orders.allKinds')" />
        <ElOption v-for="item in orderKinds" :key="item" :value="item" :label="t(`orders.kinds.${item}`)" />
      </ElSelect>
      <ElSelect class="ops-order-status" :model-value="status ?? 'all'" :aria-label="t('orders.statusFilter')"
        @update:model-value="filter(kind, $event)">
        <ElOption value="all" :label="t('orders.allStatuses')" />
        <ElOption v-for="item in orderStatuses" :key="item" :value="item" :label="t(`orders.status.${item}`)" />
      </ElSelect>
    </template>
    <ElTable v-if="state.kind === 'ready'" :data="state.rows" row-key="id" cell-class-name="ops-history-cell" header-cell-class-name="ops-history-heading" class="ops-history-table ops-orders"
      @row-click="openOrder">
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.number')" min-width="150">
        <template #default="{ row }">
          <ElButton class="ops-order-number mono" link
            @click.stop="openOrder(row)">{{ row.id }}</ElButton>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.type')" min-width="110">
        <template #default="{ row }">{{ t(`orders.kinds.${row.kind}`) }}</template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.contents')" min-width="240">
        <template #default="{ row }">
          <template v-if="row.summaryItem">
            <span>{{ row.itemCount > 1 ? t('orders.itemSummary', { first: itemDescription(row.summaryItem), count: row.itemCount }) : itemDescription(row.summaryItem) }}</span>
            <span v-if="row.kind === 'traffic_package'" class="ops-history-note">{{ t('orders.opsNoExpiry') }}</span>
            <span v-else-if="row.summaryItem.termDays" class="ops-history-note">{{ t('orders.termDays', row.summaryItem.termDays) }}</span>
          </template>
          <span v-else>{{ t('orders.unavailable') }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.paid')" align="right" min-width="100">
        <template #default="{ row }">
          <strong class="ops-order-amount tabular">{{ money(row.settledMinor) }}</strong>
          <span v-if="row.refundedMinor" class="ops-history-note">{{ t('orders.refundNote', { amount: money(row.refundedMinor) }) }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.state')" min-width="110">
        <template #default="{ row }">
          <ElTag :type="row.status === 'pending' ? 'primary' : statusTypes[row.status]" size="small" disable-transitions>{{ t(`orders.status.${row.status}`) }}</ElTag>
          <span v-if="row.status === 'pending'" class="ops-history-note">{{ t('orders.ops.waited', { duration: pendingDuration(row.createdAt) }) }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.placedAt')" min-width="150">
        <template #default="{ row }"><time :datetime="row.createdAt.toISOString()">{{ shanghaiDateTime(row.createdAt, locale, 'short') }}</time></template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty :description="t('orders.noneMatching')" :image-size="72">
          <ElButton v-if="kind || status || orderQuery" @click="filter(undefined, undefined, '')">{{ t('orders.clearFilters') }}</ElButton>
        </ElEmpty>
      </template>
    </ElTable>
  </OpsHistoryTable>
</template>

<style scoped>
.ops-order-input { --el-component-size: var(--control-height); }
.ops-order-search { width: min(100%, var(--ops-history-search-width)); }
.ops-order-kind { width: var(--ops-history-kind-width); }
.ops-order-status { width: var(--ops-history-status-width); }
.ops-orders.ops-orders { cursor: pointer; --el-table-row-hover-bg-color: var(--color-surface-muted); }
.ops-order-number { padding: 0; color: var(--color-text); font-size: var(--font-size-xs); font-weight: var(--font-weight-regular); }
.ops-order-amount { font-weight: var(--font-weight-semibold); }
.ops-order-number:hover { color: var(--color-primary); }
</style>
