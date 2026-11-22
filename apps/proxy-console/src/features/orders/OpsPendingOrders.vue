<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElLoading, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { CircleCheck } from '@element-plus/icons-vue'
import { listOpsPendingOrders, type OpsPendingOrder } from '@/api/opsOrders'
import { hasOperatorPermission } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { useOrderFormatting } from './format'
import OpsStaticPurchaseDialog from './OpsStaticPurchaseDialog.vue'

const vLoading = ElLoading.directive
const { t, n } = useI18n()
const { money, dateTime } = useOrderFormatting()
const errorMessage = useErrorMessage()
const records = shallowRef<OpsPendingOrder[]>([])
const failure = shallowRef<unknown>()
const active = shallowRef<OpsPendingOrder>()
const open = ref(false)
const loading = ref(false)
const nextAfterId = ref<string>()
const load = ref<{ afterId?: string }>({})
const purchases = computed(() => records.value.filter(({ order }) => order.kind === 'static_purchase'))

watch(load, async ({ afterId }, _previous, onCleanup) => {
  if (!hasOperatorPermission('read_operations')) return
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  loading.value = true
  failure.value = undefined
  try {
    const page = await listOpsPendingOrders(afterId, controller.signal)
    if (controller.signal.aborted) return
    const pending = page.records.filter(({ order }) => order.status === 'pending')
    records.value = afterId ? [...records.value, ...pending] : pending
    nextAfterId.value = page.nextAfterId
  } catch (error) {
    if (!controller.signal.aborted) failure.value = error
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}, { immediate: true })

function inspect(row: OpsPendingOrder) {
  active.value = row
  open.value = true
}

function changed() {
  load.value = {}
}

function requested({ order }: OpsPendingOrder) {
  return order.items.reduce((total, item) => total + item.quantity, 0)
}

function waited({ order }: OpsPendingOrder) {
  const hours = Math.max(0, Math.floor((Date.now() - order.createdAt.getTime()) / 3_600_000))
  return t('orders.ops.hours', { hours: n(hours) }, hours)
}
</script>

<template>
  <ElAlert v-if="!hasOperatorPermission('read_operations')" type="warning" :title="t('errors.ACCESS_DENIED')" :closable="false" show-icon />
  <ElCard v-else shadow="never" class="pending-card">
    <div class="pending-toolbar">
      <h2 class="pending-title">{{ t('orders.ops.title') }}</h2>
      <p class="pending-note">{{ t('orders.ops.description') }}</p>
    </div>
    <ElAlert v-if="failure" type="error" :title="errorMessage(failure)" :closable="false" show-icon>
      <ElButton @click="load = { ...load }">{{ t('orders.retry') }}</ElButton>
    </ElAlert>
    <ElTable v-loading="loading" :data="purchases" row-key="order.id" class="pending-table"
      :header-cell-style="{ height: 'var(--ops-history-header-height)', fontSize: 'var(--font-size-xs)', fontWeight: 'var(--font-weight-semibold)' }"
      :cell-style="{ height: 'var(--ops-history-row-height)' }">
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn :label="t('orders.ops.stage')" width="130">
        <template #default="{ row }">
          <ElTag :type="row.canClose ? 'danger' : 'warning'" size="small" disable-transitions>{{ t(row.canClose ? 'orders.ops.closable' : 'orders.ops.review') }}</ElTag>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn :label="t('orders.ops.order')" min-width="200">
        <template #default="{ row }">
          <span class="mono pending-order">{{ row.order.id }}</span>
          <span class="pending-meta">{{ t(`orders.kinds.${row.order.kind}`) }} · {{ dateTime(row.order.createdAt, 'short') }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn :label="t('orders.ops.tenant')" min-width="150">
        <template #default="{ row }">
          <RouterLink class="pending-link" :to="{ name: 'ops-tenant', params: { tenantId: row.tenant.id } }">{{ row.tenant.name }}</RouterLink>
          <span class="pending-meta mono">{{ row.tenant.account }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn :label="t('orders.ops.waited')" width="100" align="right">
        <template #default="{ row }"><span class="tabular pending-amount">{{ waited(row) }}</span></template>
      </ElTableColumn>
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn :label="t('orders.ops.progress')" width="120" align="right">
        <template #default="{ row }">{{ t('orders.quantityProgress', { delivered: row.order.deliveredCount, ordered: requested(row) }) }}</template>
      </ElTableColumn>
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn :label="t('orders.prepaid')" width="110" align="right">
        <template #default="{ row }"><span class="tabular pending-amount">{{ money(row.order.prepaidMinor) }}</span></template>
      </ElTableColumn>
      <!-- @vue-generic {OpsPendingOrder} -->
      <ElTableColumn width="90" align="right">
        <template #default="{ row }">
          <ElButton link type="primary" @click="inspect(row)">{{ t('orders.ops.verify') }}</ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <div v-if="!loading && !failure" class="pending-empty"><CircleCheck aria-hidden="true" />{{ t('orders.ops.empty') }}</div>
      </template>
    </ElTable>
    <div class="pending-footer">
      <span>{{ t('orders.ops.total', purchases.length) }}</span>
      <ElButton v-if="nextAfterId" link type="primary" :disabled="loading" @click="load = { afterId: nextAfterId }">{{ t('orders.ops.more') }}</ElButton>
    </div>
  </ElCard>
  <OpsStaticPurchaseDialog v-if="active" :key="active.order.id" v-model="open" :record="active" :waited="waited(active)" @changed="changed" />
</template>

<style scoped>
.pending-card {
  --el-card-padding: 0;
  border-radius: var(--ops-card-radius);
}
.pending-toolbar {
  padding: var(--space-3) var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
}
.pending-title { font-size: var(--font-size-lg); line-height: var(--space-5); font-weight: var(--font-weight-bold); }
.pending-note { color: var(--color-text-secondary); font-size: var(--font-size-xs); margin-block-start: var(--ops-detail-meta-gap); }
.pending-table { width: 100%; font-size: var(--font-size-sm); --el-table-header-bg-color: var(--color-surface-muted); --el-table-row-hover-bg-color: var(--color-surface-muted); }
.pending-meta { display: block; color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.pending-order, .pending-link, .pending-amount { font-weight: var(--font-weight-semibold); }
.pending-link { color: var(--color-primary); }
.pending-link:hover { text-decoration: underline; }
.pending-empty { display: flex; flex-direction: column; align-items: center; gap: var(--space-2); font-size: var(--font-size-sm); }
.pending-empty svg { width: calc(var(--space-6) - var(--space-1)); height: calc(var(--space-6) - var(--space-1)); color: var(--color-focus); }
.pending-footer { display: flex; align-items: center; justify-content: space-between; padding: var(--ops-search-result-padding) var(--space-4); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
</style>
