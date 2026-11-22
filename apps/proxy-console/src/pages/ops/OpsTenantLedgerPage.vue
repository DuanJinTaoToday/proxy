<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElButton, ElDatePicker, ElEmpty, ElRadioButton, ElRadioGroup, ElTable, ElTableColumn } from 'element-plus'
import { listOpsTenantWalletEntries } from '@/api/opsTenants'
import type { mapWalletEntry, WalletFilterKind } from '@/api/wallet'
import { readBusinessDateBounds } from '@/shared/lib/businessDateQuery'
import { formatYuan } from '@/shared/lib/money'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import OpsHistoryTable from './OpsHistoryTable.vue'
import { useOpsHistoryPage } from './useOpsHistoryPage'

type WalletEntry = ReturnType<typeof mapWalletEntry>
const props = defineProps<{ tenantId: string }>()
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const money = (minor: bigint) => formatYuan(minor, locale.value)
const filterKinds: WalletFilterKind[] = ['recharge', 'debit', 'refund']
const kind = computed(() => filterKinds.find(value => value === route.query.kind))
const dates = computed(() => readBusinessDateBounds(route.query.start, route.query.end))
const range = computed<[string, string] | undefined>(() => dates.value.start && dates.value.end ? [dates.value.start, dates.value.end] : undefined)

function filterKind(next: unknown) {
  return router.push({ query: { ...route.query, kind: filterKinds.find(value => value === next) } })
}

function filterDates(next: unknown) {
  const { start, end } = Array.isArray(next) ? readBusinessDateBounds(next[0], next[1]) : {}
  return router.push({ query: { ...route.query, start, end } })
}

const history = useOpsHistoryPage(async (beforeId, signal) => {
  const result = await listOpsTenantWalletEntries(props.tenantId, { beforeId, kind: kind.value, ...dates.value }, signal)
  return { rows: result.entries, nextBeforeId: result.nextBeforeId }
})
const { state } = history
watch([kind, () => dates.value.start, () => dates.value.end], history.reset, { immediate: true })
</script>

<template>
  <OpsHistoryTable :label="t('wallet.ledger')" :state="state" :page="history.page.value"
    @retry="history.load" @previous="history.previous" @next="history.next">
    <template #filters>
      <ElRadioGroup :model-value="kind ?? 'all'" :aria-label="t('wallet.kindFilter')" @update:model-value="filterKind">
        <ElRadioButton value="all">{{ t('wallet.all') }}</ElRadioButton>
        <ElRadioButton v-for="item in filterKinds" :key="item" :value="item">{{ t(`wallet.kinds.${item}`) }}</ElRadioButton>
      </ElRadioGroup>
      <div class="ops-ledger-dates">
        <ElDatePicker :model-value="range" type="daterange" value-format="YYYY-MM-DD"
          :style="{ '--el-date-editor-daterange-width': '100%', '--el-component-size': 'var(--control-height)' }"
          :aria-label="t('wallet.dateFilter')" :start-placeholder="t('wallet.startDate')" :end-placeholder="t('wallet.endDate')"
          :range-separator="t('wallet.dateSeparator')" @update:model-value="filterDates" />
      </div>
    </template>
    <ElTable v-if="state.kind === 'ready'" :data="state.rows" row-key="id" class="ops-history-table" cell-class-name="ops-history-cell" header-cell-class-name="ops-history-heading">
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.opsHistoryEntryTime')" min-width="170">
        <template #default="{ row }">
          <span class="ops-ledger-id mono">{{ row.id }}</span>
          <time class="ops-history-note" :datetime="row.createdAt.toISOString()">{{ shanghaiDateTime(row.createdAt, locale, 'short') }}</time>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.kind')" :width="locale === 'zh-CN' ? 80 : undefined" min-width="80">
        <template #default="{ row }"><span class="ops-ledger-kind" :class="`ops-ledger-kind--${row.kind}`">{{ t(`wallet.kinds.${row.kind}`) }}</span></template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.description')" min-width="220">
        <template #default="{ row }">
          <span>{{ row.reason || t(`wallet.notes.${row.kind}`) }}</span>
          <ElButton v-if="row.orderId" class="ops-ledger-order mono" link type="primary"
            @click="router.push({ query: { ...route.query, order: row.orderId } })">{{ row.orderId }}</ElButton>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.opsOperator')" :width="locale === 'zh-CN' ? 90 : undefined" min-width="90">
        <template #default="{ row }">{{ row.operatorName || t('orders.unavailable') }}</template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.amount')" min-width="110" align="right">
        <template #default="{ row }">
          <strong class="ops-ledger-amount tabular" :class="{ 'ops-ledger-credit': row.amountMinor > 0n }">{{ row.amountMinor > 0n ? '+' : '' }}{{ money(row.amountMinor) }}</strong>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.opsBalanceChange')" min-width="170" align="right">
        <template #default="{ row }"><strong class="ops-ledger-amount tabular">{{ money(row.beforeMinor) }} → {{ money(row.afterMinor) }}</strong></template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty :description="t('wallet.noEntries')" :image-size="72">
          <ElButton v-if="kind || dates.start || dates.end"
            @click="router.push({ query: { ...route.query, kind: undefined, start: undefined, end: undefined } })">
            {{ t('wallet.clear') }}
          </ElButton>
        </ElEmpty>
      </template>
    </ElTable>
  </OpsHistoryTable>
</template>

<style scoped>
.ops-ledger-dates { width: min(100%, var(--ops-history-dates-width)); }
.ops-ledger-id, .ops-ledger-order { font-size: var(--font-size-xs); }
.ops-ledger-order { margin-inline-start: var(--space-2); font-weight: var(--font-weight-regular); }
.ops-ledger-kind {
  display: inline-block;
  padding: var(--ops-detail-meta-gap) var(--space-2);
  border-radius: var(--radius-control);
  background: var(--color-canvas);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  line-height: var(--ops-tenant-chip-line-height);
}
.ops-ledger-kind--recharge { background: var(--color-primary-soft); color: var(--color-primary); }
.ops-ledger-kind--hold { background: var(--color-warning-bg); color: var(--color-warning-text); }
.ops-ledger-kind--release { padding-inline: var(--space-1); }
.ops-ledger-kind--release, .ops-ledger-kind--refund { background: var(--color-info-bg); color: var(--color-info-text); }
.ops-ledger-amount { font-weight: var(--font-weight-semibold); }
.ops-ledger-credit { color: var(--color-success-text); }
</style>
