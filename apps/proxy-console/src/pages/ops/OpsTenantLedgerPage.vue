<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElButton, ElEmpty, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { listOpsTenantWalletEntries } from '@/api/opsTenants'
import type { mapWalletEntry } from '@/api/wallet'
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
const history = useOpsHistoryPage(async (beforeId, signal) => {
  const result = await listOpsTenantWalletEntries(props.tenantId, beforeId, signal)
  return { rows: result.entries, nextBeforeId: result.nextBeforeId }
})
const { state } = history
void history.load()
</script>

<template>
  <OpsHistoryTable :label="t('wallet.ledger')" :state="state" :page="history.page.value"
    @retry="history.load" @previous="history.previous" @next="history.next">
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
.ops-ledger-order { margin-inline-start: var(--space-2); }
.ops-ledger-credit { color: var(--color-success-text); }
</style>
