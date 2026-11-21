<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  ElAlert, ElButton, ElCard, ElDatePicker, ElEmpty, ElRadioButton, ElRadioGroup, ElSkeleton, ElTable,
  ElTableColumn, ElTag, type TagProps,
} from 'element-plus'
import { listWalletEntries, mapWalletEntry, type WalletFilterKind } from '@/api/wallet'
import { readBusinessDate, readBusinessDateBounds } from '@/shared/lib/businessDateQuery'
import { formatYuan, shanghaiDateTime } from '@/shared/lib/formatters'

const { t, locale } = useI18n()
const route = useRoute()
const router = useRouter()
const filterKinds: WalletFilterKind[] = ['recharge', 'debit', 'refund']
type WalletEntry = ReturnType<typeof mapWalletEntry>
const tagTypes: Record<WalletEntry['kind'], TagProps['type']> = {
  recharge: 'success',
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

function setStartDate(value: unknown) {
  const next = readBusinessDate(value)
  return setFilters(kind.value, next, next && end.value && next > end.value ? undefined : end.value)
}

function setEndDate(value: unknown) {
  const next = readBusinessDate(value)
  return setFilters(kind.value, next && start.value && next < start.value ? undefined : start.value, next)
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
  <ElCard class="wallet-ledger" role="region" aria-labelledby="wallet-ledger-title" shadow="never">
    <div class="wallet-ledger__heading">
      <h2 id="wallet-ledger-title">{{ t('wallet.ledger') }}</h2>
      <span>{{ t('wallet.businessDay') }}</span>
    </div>
    <div class="wallet-ledger__toolbar">
      <ElRadioGroup
        :model-value="kind ?? 'all'" :aria-label="t('wallet.kindFilter')"
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
          :aria-label="t('wallet.startDate')" :placeholder="t('wallet.startDate')" @update:model-value="setStartDate"
        />
        <span>{{ t('wallet.dateSeparator') }}</span>
        <ElDatePicker
          class="wallet-ledger__date-input" :model-value="end" type="date" value-format="YYYY-MM-DD"
          :aria-label="t('wallet.endDate')" :placeholder="t('wallet.endDate')" @update:model-value="setEndDate"
        />
      </div>
    </div>

    <div v-if="state.kind === 'loading'" role="status" :aria-label="t('wallet.loadingLedger')">
      <ElSkeleton animated :rows="5" />
    </div>
    <ElAlert v-else-if="state.kind === 'failed'" :title="t('wallet.ledgerFailed')" type="error" :closable="false" show-icon>
      <ElButton size="small" @click="load">{{ t('wallet.retry') }}</ElButton>
    </ElAlert>
    <ElTable v-else-if="state.kind === 'ready'" :data="state.page.entries" row-key="id">
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.entryId')" min-width="110">
        <template #default="{ row }"><span class="tabular">{{ row.id }}</span></template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.time')" min-width="170">
        <template #default="{ row }">
          <time :datetime="row.createdAt.toISOString()">{{ shanghaiDateTime(row.createdAt, locale) }}</time>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.kind')" min-width="110">
        <template #default="{ row }">
          <ElTag :type="tagTypes[row.kind]" size="small">{{ t(`wallet.kinds.${row.kind}`) }}</ElTag>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {WalletEntry} -->
      <ElTableColumn :label="t('wallet.description')" min-width="225">
        <template #default="{ row }">
          <span>{{ t(`wallet.notes.${row.kind}`) }}</span>
          <RouterLink
            v-if="row.orderId" class="wallet-ledger__order tabular"
            :to="{ name: 'console-account-orders', query: { detail: row.orderId } }"
          >
            {{ t('wallet.orderLink', { id: row.orderId }) }}
          </RouterLink>
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
      <ElTableColumn :label="t('wallet.balanceAfter')" min-width="135" align="right">
        <template #default="{ row }">
          <span class="wallet-ledger__amount tabular">{{ formatYuan(row.afterMinor, locale) }}</span>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty :description="t('wallet.noEntries')" :image-size="72">
          <ElButton v-if="hasFilters" @click="setFilters()">{{ t('wallet.clear') }}</ElButton>
        </ElEmpty>
      </template>
    </ElTable>

    <nav
      v-if="cursors.length > 1 || (state.kind === 'ready' && state.page.nextBeforeId)"
      class="wallet-ledger__pager" :aria-label="t('wallet.pagination')"
    >
      <span class="tabular">{{ t('wallet.page', { page: cursors.length }) }}</span>
      <div class="wallet-ledger__controls">
        <ElButton size="small" :disabled="cursors.length === 1 || state.kind === 'loading'" @click="previousPage">
          {{ t('wallet.previous') }}
        </ElButton>
        <ElButton size="small" :disabled="state.kind !== 'ready' || !state.page.nextBeforeId" @click="nextPage">
          {{ t('wallet.next') }}
        </ElButton>
      </div>
    </nav>
  </ElCard>
</template>

<style scoped>
.wallet-ledger { min-inline-size: 0; }
.wallet-ledger__heading,
.wallet-ledger__toolbar,
.wallet-ledger__pager,
.wallet-ledger__controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}
.wallet-ledger__heading,
.wallet-ledger__pager { justify-content: space-between; }
.wallet-ledger__heading { margin-block-end: var(--space-3); }
.wallet-ledger__heading h2 { font-size: var(--font-size-lg); font-weight: var(--font-weight-semibold); }
.wallet-ledger__heading span,
.wallet-ledger__pager > span { color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.wallet-ledger__toolbar { margin-block-end: var(--space-4); }
.wallet-ledger__dates {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  max-inline-size: 100%;
}
.wallet-ledger__date-input { width: min(100%, var(--business-date-filter-width)); }
.wallet-ledger__order {
  display: inline-block;
  margin-inline-start: var(--space-2);
  color: var(--color-primary);
  text-decoration: underline;
  overflow-wrap: anywhere;
}
.wallet-ledger__amount { font-weight: var(--font-weight-semibold); }
.wallet-ledger__amount--positive { color: var(--color-success-text); }
.wallet-ledger__pager { margin-block-start: var(--space-3); }
.wallet-ledger__controls .el-button { margin-inline: 0; }
</style>
