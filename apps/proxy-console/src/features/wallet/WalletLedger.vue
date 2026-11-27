<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  ElAlert, ElButton, ElCard, ElDatePicker, ElEmpty, ElRadioButton, ElRadioGroup, ElSkeleton, ElTable,
  ElTableColumn, ElTag, type TagProps,
} from 'element-plus'
import { listWalletEntries, mapWalletEntry, type WalletFilterKind } from '@/api/wallet'
import { readBusinessDateBounds, updateBusinessDateBounds } from '@/shared/lib/businessDateQuery'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { formatYuan } from '@/shared/lib/money'

const emit = defineEmits<{ openOrder: [orderId: string] }>()
const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const filterKinds: WalletFilterKind[] = ['recharge', 'commission_transfer', 'debit', 'refund']
type WalletEntry = ReturnType<typeof mapWalletEntry>
const tagTypes: Record<WalletEntry['kind'], TagProps['type']> = {
  recharge: 'success',
  commission_transfer: 'success',
  debit: 'info',
  hold: 'warning',
  release: 'info',
  refund: 'primary',
  unknown: 'info',
}
const kind = computed(() => filterKinds.find(value => value === route.query.kind))
const dates = computed(() => readBusinessDateBounds(route.query.start, route.query.end))
const start = computed(() => dates.value.start)
const end = computed(() => dates.value.end)
const hasFilters = computed(() => Boolean(kind.value || start.value || end.value))

function setFilters(nextKind?: unknown, nextStart?: string, nextEnd?: string) {
  return router.push({
    name: 'console-wallet',
    query: {
      ...route.query,
      kind: filterKinds.find(value => value === nextKind),
      start: nextStart,
      end: nextEnd,
    },
  })
}

function setDate(field: 'start' | 'end', value: unknown) {
  const { start, end } = updateBusinessDateBounds(dates.value, field, value)
  return setFilters(kind.value, start, end)
}

type LedgerState =
  | { kind: 'loading' | 'failed' }
  | { kind: 'ready', page: Awaited<ReturnType<typeof listWalletEntries>> }
const state = shallowRef<LedgerState>({ kind: 'loading' })
const cursors = shallowRef<(string | undefined)[]>([undefined])
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

async function load() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  try {
    const page = await listWalletEntries({
      kind: kind.value,
      start_date: start.value,
      end_date: end.value,
      before_id: cursors.value.at(-1),
    }, controller.signal)
    if (!controller.signal.aborted) state.value = { kind: 'ready', page }
  } catch {
    if (!controller.signal.aborted) state.value = { kind: 'failed' }
  }
}

watch([kind, start, end], () => {
  cursors.value = [undefined]
  void load()
}, { immediate: true })

function nextPage() {
  if (state.value.kind !== 'ready' || !state.value.page.nextBeforeId) return
  cursors.value = [...cursors.value, state.value.page.nextBeforeId]
  void load()
}

function previousPage() {
  if (cursors.value.length < 2) return
  cursors.value = cursors.value.slice(0, -1)
  void load()
}
</script>

<template>
  <ElCard class="wallet-ledger" role="region" :aria-label="t('wallet.ledger')" :body-style="{ padding: 0 }" shadow="never">
    <div class="wallet-ledger__toolbar">
      <div class="wallet-ledger__filters">
        <ElRadioGroup
          class="wallet-ledger__kind-filter" :model-value="kind ?? 'all'" :aria-label="t('wallet.kindFilter')"
          @update:model-value="setFilters($event, start, end)"
        >
          <ElRadioButton value="all">{{ t('wallet.all') }}</ElRadioButton>
          <ElRadioButton v-for="filter in filterKinds" :key="filter" :value="filter">
            {{ t(`wallet.kinds.${filter}`) }}
          </ElRadioButton>
        </ElRadioGroup>
        <div class="wallet-ledger__dates" role="group" :aria-label="t('wallet.dateFilter')">
          <ElDatePicker
            class="wallet-ledger__date-input" :model-value="start" type="date" value-format="YYYY-MM-DD"
            :aria-label="t('wallet.startDate')" :placeholder="t('wallet.startDate')" @update:model-value="setDate('start', $event)"
          />
          <span>{{ t('wallet.dateSeparator') }}</span>
          <ElDatePicker
            class="wallet-ledger__date-input" :model-value="end" type="date" value-format="YYYY-MM-DD"
            :aria-label="t('wallet.endDate')" :placeholder="t('wallet.endDate')" @update:model-value="setDate('end', $event)"
          />
        </div>
      </div>
      <span class="wallet-ledger__business-day">{{ t('wallet.businessDay') }}</span>
    </div>

    <div v-if="state.kind === 'loading'" class="wallet-ledger__feedback" role="status" :aria-label="t('wallet.loadingLedger')">
      <ElSkeleton animated :rows="5" />
    </div>
    <ElAlert v-else-if="state.kind === 'failed'" class="wallet-ledger__feedback" :title="t('wallet.ledgerFailed')" type="error" :closable="false" show-icon>
      <ElButton size="small" @click="load">{{ t('wallet.retry') }}</ElButton>
    </ElAlert>
    <ElTable v-else-if="state.kind === 'ready'" :data="state.page.entries" row-key="id" class="wallet-ledger__table" header-row-class-name="wallet-ledger__table-heading" row-class-name="wallet-ledger__table-row">
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.time')" min-width="150">
        <template #default="{ row }">
          <time :datetime="row.createdAt.toISOString()">{{ shanghaiDateTime(row.createdAt, locale, 'compact') }}</time>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.kind')" :min-width="locale === 'zh-CN' ? 108 : 168">
        <template #default="{ row }">
          <ElTag :type="tagTypes[row.kind]" size="small" :class="`wallet-ledger__kind--${row.kind}`" disable-transitions>{{ t(`wallet.kinds.${row.kind}`) }}</ElTag>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.description')" min-width="240">
        <template #default="{ row }">
          <span>{{ t(`wallet.notes.${row.kind}`) }}</span>
          <ElButton
            v-if="row.orderId" class="wallet-ledger__order tabular" link type="primary"
            @click="emit('openOrder', row.orderId)"
          >
            {{ t('wallet.orderLink', { id: row.orderId }) }}
          </ElButton>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.amount')" min-width="130" align="right">
        <template #default="{ row }">
          <span class="wallet-ledger__amount tabular" :class="{ 'wallet-ledger__amount--positive': row.amountMinor > 0n }">
            {{ row.amountMinor > 0n ? '+' : '' }}{{ formatYuan(row.amountMinor, locale) }}
          </span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.balanceBefore')" min-width="135" align="right">
        <template #default="{ row }">
          <span class="wallet-ledger__amount tabular">{{ formatYuan(row.beforeMinor, locale) }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.balanceAfter')" min-width="135" align="right">
        <template #default="{ row }">
          <span class="wallet-ledger__amount tabular">{{ formatYuan(row.afterMinor, locale) }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.entryId')" min-width="110">
        <template #default="{ row }"><span class="tabular">{{ row.id }}</span></template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty :description="t('wallet.noEntries')" :image-size="72">
          <ElButton v-if="hasFilters" @click="setFilters()">{{ t('wallet.clear') }}</ElButton>
        </ElEmpty>
      </template>
    </ElTable>

    <nav v-if="state.kind === 'ready'" class="wallet-ledger__pager" :aria-label="t('wallet.pagination')">
      <span class="tabular">{{ t('wallet.pageEntries', { count: state.page.entries.length }) }}</span>
      <div v-if="cursors.length > 1 || state.page.nextBeforeId" class="wallet-ledger__controls">
        <span class="tabular">{{ t('wallet.page', { page: cursors.length }) }}</span>
        <ElButton size="small" :disabled="cursors.length === 1" @click="previousPage">
          {{ t('wallet.previous') }}
        </ElButton>
        <ElButton size="small" :disabled="!state.page.nextBeforeId" @click="nextPage">
          {{ t('wallet.next') }}
        </ElButton>
      </div>
    </nav>
  </ElCard>
</template>

<style scoped>
.wallet-ledger { min-inline-size: 0; border-radius: var(--dashboard-card-radius); }
.wallet-ledger__toolbar,
.wallet-ledger__filters,
.wallet-ledger__pager,
.wallet-ledger__controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
.wallet-ledger__toolbar, .wallet-ledger__pager { justify-content: space-between; }
.wallet-ledger__toolbar {
  padding: var(--space-3) var(--space-4);
  border-block-end: var(--border-width) solid var(--color-border);
}
.wallet-ledger__filters { min-inline-size: 0; max-inline-size: 100%; }
.wallet-ledger__feedback { margin: var(--space-4); }
.wallet-ledger__business-day, .wallet-ledger__pager { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.wallet-ledger__kind-filter {
  display: flex;
  flex-wrap: wrap;
  row-gap: var(--space-1);
  max-inline-size: 100%;
}
.wallet-ledger__dates {
  display: flex;
  align-items: center;
  gap: var(--space-1);
  max-inline-size: 100%;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-control);
  color: var(--color-text-secondary);
  overflow: hidden;
}
.wallet-ledger__date-input {
  --el-date-editor-width: min(38vw, var(--tenant-date-field-width));
  --el-input-height: calc(var(--control-height) - 2 * var(--border-width));
  --el-input-border-color: transparent;
}
.wallet-ledger__table {
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  font-size: var(--font-size-sm);
}
.wallet-ledger__table :deep(.wallet-ledger__table-heading) {
  height: var(--tenant-table-heading-height);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}
.wallet-ledger__table :deep(.wallet-ledger__table-row) { height: var(--tenant-table-row-height); }
.wallet-ledger__order {
  margin-inline-start: var(--space-2);
  white-space: normal;
  overflow-wrap: anywhere;
}
.wallet-ledger__amount { font-weight: var(--font-weight-semibold); }
.wallet-ledger__amount--positive { color: var(--color-success-text); }
.wallet-ledger__kind--debit { --el-tag-bg-color: var(--color-canvas); --el-tag-text-color: var(--color-text-secondary); }
.wallet-ledger__kind--refund { --el-tag-bg-color: var(--color-info-bg); --el-tag-text-color: var(--color-info-text); }
.wallet-ledger__pager { padding: var(--space-3) var(--space-4); }
.wallet-ledger__controls .el-button { margin-inline: 0; }
@media (max-width: 600px) {
  .wallet-ledger__dates {
    inline-size: 100%;
    min-inline-size: 0;
  }
  .wallet-ledger__date-input {
    flex: 1 1 0;
    min-inline-size: 0;
  }
}
</style>
