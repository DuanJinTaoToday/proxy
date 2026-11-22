<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElButton, ElDatePicker, ElEmpty, ElRadioButton, ElRadioGroup, ElTable, ElTableColumn, ElTag } from 'element-plus'
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
const tags = { recharge: 'success', debit: 'info', hold: 'warning', release: 'info', refund: 'primary', unknown: 'info' } as const
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
      <ElDatePicker class="ops-ledger-dates" :model-value="range" type="daterange" value-format="YYYY-MM-DD"
        :aria-label="t('wallet.dateFilter')" :start-placeholder="t('wallet.startDate')" :end-placeholder="t('wallet.endDate')"
        :range-separator="t('wallet.dateSeparator')" @update:model-value="filterDates" />
    </template>
    <ElTable v-if="state.kind === 'ready'" :data="state.rows" row-key="id" class="ops-history-table" cell-class-name="ops-history-cell" header-cell-class-name="ops-history-heading">
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.opsEntryTime')" min-width="170">
        <template #default="{ row }">
          <strong class="mono">{{ row.id }}</strong>
          <time class="ops-history-note" :datetime="row.createdAt.toISOString()">{{ shanghaiDateTime(row.createdAt, locale, 'short') }}</time>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.kind')" min-width="80">
        <template #default="{ row }"><ElTag :type="tags[row.kind]" size="small" disable-transitions>{{ t(`wallet.kinds.${row.kind}`) }}</ElTag></template>
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
      <ElTableColumn :label="t('wallet.opsOperator')" min-width="90">
        <template #default="{ row }">{{ row.operatorName || t('orders.unavailable') }}</template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.amount')" min-width="110" align="right">
        <template #default="{ row }">
          <strong class="tabular" :class="{ 'ops-ledger-credit': row.amountMinor > 0n }">{{ row.amountMinor > 0n ? '+' : '' }}{{ money(row.amountMinor) }}</strong>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.opsBalanceChange')" min-width="170" align="right">
        <template #default="{ row }"><strong class="tabular">{{ money(row.beforeMinor) }} → {{ money(row.afterMinor) }}</strong></template>
      </ElTableColumn>
      <template #empty><ElEmpty :description="t('wallet.noEntries')" :image-size="72" /></template>
    </ElTable>
  </OpsHistoryTable>
</template>

<style scoped>
.ops-ledger-dates { max-width: min(100%, var(--ops-history-dates-width)); }
.ops-ledger-order { margin-inline-start: var(--space-2); }
.ops-ledger-credit { color: var(--color-success-text); }
</style>
