<script setup lang="ts">
import { computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElButton, ElEmpty, ElOption, ElSelect, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { listOpsTenantOrders } from '@/api/opsTenants'
import { orderKinds, orderStatuses, type OrderSummary } from '@/api/orders'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { useOrderFormatting } from '@/features/orders'
import OpsHistoryTable from './OpsHistoryTable.vue'
import { useOpsHistoryPage } from './useOpsHistoryPage'

const props = defineProps<{ tenantId: string }>()
const emit = defineEmits<{ pendingCount: [count: number] }>()
const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const { itemDescription, money, statusTypes } = useOrderFormatting()
const kind = computed(() => orderKinds.find(value => value === route.query.kind))
const status = computed(() => orderStatuses.find(value => value === route.query.status))
const history = useOpsHistoryPage(async (beforeId, signal) => {
  const result = await listOpsTenantOrders(props.tenantId, { kind: kind.value, status: status.value, beforeId }, signal)
  if (!signal.aborted) emit('pendingCount', result.pendingCount)
  return { rows: result.orders, nextBeforeId: result.nextBeforeId }
})
const { state } = history
watch([kind, status], history.reset, { immediate: true })

function openOrder(row: OrderSummary) {
  void router.push({ query: { ...route.query, order: row.id } })
}

function filter(nextKind?: unknown, nextStatus?: unknown) {
  return router.push({ query: {
    ...route.query,
    kind: orderKinds.find(value => value === nextKind),
    status: orderStatuses.find(value => value === nextStatus),
  } })
}
</script>

<template>
  <OpsHistoryTable :label="t('users.opsOrders')" :state="state" :page="history.page.value"
    @retry="history.load" @previous="history.previous" @next="history.next">
    <template #filters>
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
          <strong class="tabular">{{ money(row.settledMinor) }}</strong>
          <span v-if="row.refundedMinor" class="ops-history-note">{{ t('orders.refundNote', { amount: money(row.refundedMinor) }) }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.state')" min-width="110">
        <template #default="{ row }">
          <ElTag :type="statusTypes[row.status]" size="small" disable-transitions>{{ t(`orders.status.${row.status}`) }}</ElTag>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OrderSummary} -->
      <ElTableColumn :label="t('orders.placedAt')" min-width="150">
        <template #default="{ row }"><time :datetime="row.createdAt.toISOString()">{{ shanghaiDateTime(row.createdAt, locale, 'short') }}</time></template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty :description="t('orders.noneMatching')" :image-size="72">
          <ElButton v-if="kind || status" @click="filter()">{{ t('orders.clearFilters') }}</ElButton>
        </ElEmpty>
      </template>
    </ElTable>
  </OpsHistoryTable>
</template>

<style scoped>
.ops-order-kind { width: var(--ops-history-kind-width); }
.ops-order-status { width: var(--ops-history-status-width); }
.ops-orders { cursor: pointer; }
.ops-order-number { color: var(--color-text); font-size: var(--font-size-xs); font-weight: var(--font-weight-regular); }
.ops-order-number:hover { color: var(--color-primary); }
</style>
