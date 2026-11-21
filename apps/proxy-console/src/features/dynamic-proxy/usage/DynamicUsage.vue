<script setup lang="ts">
import { computed, onScopeDispose, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import dayjs from 'dayjs'
import {
  ElAlert, ElButton, ElCard, ElDatePicker, ElEmpty, ElMessage, ElOption, ElRadioButton,
  ElRadioGroup, ElSelect, ElSkeleton, ElTable, ElTableColumn,
} from 'element-plus'
import { Download } from '@element-plus/icons-vue'
import { listSubaccounts, type Subaccount } from '@/api/subaccounts'
import { exportDailyUsage, getDailyUsage, type DailyUsageDay } from '@/api/usage'
import { dayNumber, shanghaiDateTime, shanghaiToday } from '@/shared/lib/businessDates'
import { formatGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import DailyUsageChart from '@/shared/ui/DailyUsageChart.vue'
import { customRange, selectedAccount, selectedRange, usageFilter, type UsageRange } from './filters'

type LoadState<T> = { kind: 'loading' } | { kind: 'failed', error: unknown } | { kind: 'ready', data: T }
type ExportState = { kind: 'idle' | 'downloading' } | { kind: 'failed', error: unknown }

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const formatError = useErrorMessage()
const range = computed(() => selectedRange(route.query))
const clientId = computed(() => selectedAccount(route.query))
const filter = computed(() => usageFilter(range.value, clientId.value))
const pickerValue = computed<[string, string] | null>(() => {
  const selected = range.value
  return selected.kind === 'custom' ? [selected.start, selected.end] : null
})
const isFutureDate = (date: Date) => dayjs(date).format('YYYY-MM-DD') > shanghaiToday()

function changeFilters(selected: UsageRange, account: string | undefined) {
  void router.push({
    name: 'console-dynamic-usage',
    query: {
      ...route.query,
      range: selected.kind,
      startDate: selected.kind === 'custom' ? selected.start : undefined,
      endDate: selected.kind === 'custom' ? selected.end : undefined,
      subaccountId: account,
    },
  })
}

function choosePreset(value: unknown) {
  if (value === '7' || value === '30') changeFilters({ kind: value }, clientId.value)
}

function chooseDates(value: unknown) {
  if (!Array.isArray(value)) {
    if (!value) choosePreset('7')
    return
  }
  const selected = customRange(value[0], value[1])
  if (selected) changeFilters(selected, clientId.value)
}

function chooseAccount(value: unknown) {
  if (typeof value === 'string') changeFilters(range.value, value === 'all' ? undefined : value)
}

const accounts = shallowRef<LoadState<Subaccount[]>>({ kind: 'loading' })
const usage = shallowRef<LoadState<DailyUsageDay[]>>({ kind: 'loading' })
const download = shallowRef<ExportState>({ kind: 'idle' })
let accountRequest: AbortController | undefined
let usageRequest: AbortController | undefined
let exportRequest: AbortController | undefined

onScopeDispose(() => {
  accountRequest?.abort()
  usageRequest?.abort()
  exportRequest?.abort()
})

async function loadAccounts() {
  accountRequest?.abort()
  const controller = new AbortController()
  accountRequest = controller
  accounts.value = { kind: 'loading' }
  try {
    const data = await listSubaccounts(controller.signal)
    if (!controller.signal.aborted) accounts.value = { kind: 'ready', data }
  } catch (error) {
    if (!controller.signal.aborted) accounts.value = { kind: 'failed', error }
  }
}

async function loadUsage() {
  usageRequest?.abort()
  const controller = new AbortController()
  usageRequest = controller
  usage.value = { kind: 'loading' }
  try {
    const data = await getDailyUsage(filter.value, controller.signal)
    if (!controller.signal.aborted) usage.value = { kind: 'ready', data }
  } catch (error) {
    if (!controller.signal.aborted) usage.value = { kind: 'failed', error }
  }
}

void loadAccounts()
watch([() => filter.value.start, () => filter.value.end, clientId], () => {
  exportRequest?.abort()
  download.value = { kind: 'idle' }
  if (route.name === 'console-dynamic-usage') void loadUsage()
}, { immediate: true })

async function downloadCsv() {
  if (usage.value.kind !== 'ready' || download.value.kind === 'downloading') return
  const controller = new AbortController()
  exportRequest = controller
  const selected = filter.value
  download.value = { kind: 'downloading' }
  try {
    const csv = await exportDailyUsage(selected, controller.signal)
    if (controller.signal.aborted) return
    const url = URL.createObjectURL(new Blob([csv], { type: 'text/csv;charset=utf-8' }))
    const link = document.createElement('a')
    link.href = url
    link.download = `daily-usage-${selected.start}-${selected.end}.csv`
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1_000)
    download.value = { kind: 'idle' }
    ElMessage.success(t('dynamicUsage.csvSaved'))
  } catch (error) {
    if (!controller.signal.aborted) download.value = { kind: 'failed', error }
  }
}

const gb = (bytes: bigint) => t('dynamicUsage.gb', { value: formatGB(bytes, locale.value) })
const bytesLabel = (bytes: bigint) => new Intl.NumberFormat(locale.value).format(bytes)
const days = computed(() => usage.value.kind === 'ready' ? usage.value.data : [])
const tableDays = computed(() => [...days.value].reverse())
const total = computed(() => days.value.reduce((sum, day) => sum + day.bytes, 0n))
const dayCount = computed(() => BigInt(dayNumber(filter.value.end) - dayNumber(filter.value.start) + 1))
const average = computed(() => (total.value + dayCount.value / 2n) / dayCount.value)
const peak = computed(() => days.value.reduce<DailyUsageDay | undefined>(
  (highest, day) => !highest || day.bytes > highest.bytes ? day : highest, undefined,
))
const lastUpdate = computed(() => {
  const latest = days.value.reduce((newest, day) => {
    const timestamp = Date.parse(day.updatedAt ?? '')
    return Number.isFinite(timestamp) ? Math.max(newest, timestamp) : newest
  }, Number.NEGATIVE_INFINITY)
  if (Number.isFinite(latest)) return new Date(latest)
})
const accountOptions = computed(() => accounts.value.kind === 'ready' ? accounts.value.data : [])
const unknownAccount = computed(() => accountOptions.value.some(account => account.clientId === clientId.value) ? undefined : clientId.value)
</script>

<template>
  <ElCard class="dynamic-usage" body-class="dynamic-usage__body" shadow="never">
    <div class="dynamic-usage__toolbar">
      <div class="dynamic-usage__filters">
        <ElRadioGroup :model-value="range.kind" :aria-label="t('dynamicUsage.range')" @update:model-value="choosePreset">
          <ElRadioButton value="7">{{ t('dynamicUsage.last7') }}</ElRadioButton>
          <ElRadioButton value="30">{{ t('dynamicUsage.last30') }}</ElRadioButton>
        </ElRadioGroup>
        <ElDatePicker :model-value="pickerValue" type="daterange" value-format="YYYY-MM-DD" format="YYYY-MM-DD"
          :disabled-date="isFutureDate"
          class="dynamic-usage__dates" :aria-label="t('dynamicUsage.customDates')" :range-separator="t('dynamicUsage.through')"
          :start-placeholder="t('dynamicUsage.startDate')" :end-placeholder="t('dynamicUsage.endDate')"
          @update:model-value="chooseDates" />
        <ElSelect :model-value="clientId ?? 'all'" class="dynamic-usage__accounts" :aria-label="t('dynamicUsage.byAccount')"
          :loading="accounts.kind === 'loading'" @update:model-value="chooseAccount">
          <ElOption value="all" :label="t('dynamicUsage.allAccounts')" />
          <ElOption v-if="unknownAccount" :value="unknownAccount" :label="t('dynamicUsage.selectedAccount')" />
          <ElOption v-for="account in accountOptions" :key="account.clientId" :value="account.clientId"
            :label="account.active ? account.name : t('dynamicUsage.disabledAccount', { name: account.name })" />
        </ElSelect>
      </div>
      <ElButton :icon="Download" :loading="download.kind === 'downloading'" :disabled="usage.kind !== 'ready'"
        @click="downloadCsv">{{ t('dynamicUsage.exportCsv') }}</ElButton>
    </div>

    <ElAlert v-if="accounts.kind === 'failed'" :title="t('dynamicUsage.accountsFailed')" type="error" :closable="false" show-icon>
      <p>{{ formatError(accounts.error) }}</p>
      <ElButton size="small" @click="loadAccounts">{{ t('dynamicUsage.retry') }}</ElButton>
    </ElAlert>
    <ElAlert v-if="download.kind === 'failed'" :title="t('dynamicUsage.csvFailed')" type="error" :closable="false" show-icon>
      <p>{{ formatError(download.error) }}</p>
      <ElButton size="small" @click="downloadCsv">{{ t('dynamicUsage.retry') }}</ElButton>
    </ElAlert>

    <div v-if="usage.kind === 'loading'" role="status" :aria-label="t('dynamicUsage.loading')">
      <ElSkeleton animated :rows="6" />
    </div>
    <ElAlert v-else-if="usage.kind === 'failed'" :title="t('dynamicUsage.loadFailed')" type="error" :closable="false" show-icon>
      <p>{{ formatError(usage.error) }}</p>
      <ElButton size="small" @click="loadUsage">{{ t('dynamicUsage.retry') }}</ElButton>
    </ElAlert>
    <template v-else>
      <dl class="dynamic-usage__summary tabular">
        <div><dt>{{ t('dynamicUsage.total') }}</dt><dd>{{ gb(total) }}</dd></div>
        <template v-if="peak">
          <div><dt>{{ t('dynamicUsage.average') }}</dt><dd>{{ gb(average) }}</dd></div>
          <div>
            <dt>{{ t('dynamicUsage.peak') }}</dt>
            <dd>{{ gb(peak.bytes) }}</dd>
            <dd><time :datetime="peak.date">{{ peak.date }}</time></dd>
          </div>
        </template>
      </dl>

      <template v-if="days.length">
        <DailyUsageChart :days="days" :range="filter" :label="t('dynamicUsage.chartLabel')" :format-bytes="gb" />
        <ElTable :data="tableDays" :aria-label="t('dynamicUsage.dailyDetails')" class="dynamic-usage__table tabular" max-height="360">
          <ElTableColumn prop="date" :label="t('dynamicUsage.businessDay')" min-width="120" />
          <ElTableColumn :label="t('dynamicUsage.debitedTraffic')" min-width="150" align="right">
            <template #default="{ row }">{{ gb(row.bytes) }}</template>
          </ElTableColumn>
          <ElTableColumn :label="t('dynamicUsage.debitedBytes')" min-width="165" align="right">
            <template #default="{ row }">{{ bytesLabel(row.bytes) }}</template>
          </ElTableColumn>
        </ElTable>
      </template>
      <ElEmpty v-else :description="t('dynamicUsage.empty')" :image-size="72" />

      <footer class="dynamic-usage__footer">
        <i18n-t v-if="lastUpdate" keypath="dynamicUsage.updated" tag="span" class="tabular" scope="global">
          <template #time>
            <time :datetime="lastUpdate.toISOString()">{{ shanghaiDateTime(lastUpdate, locale) }}</time>
          </template>
        </i18n-t>
        <span v-else>{{ t('dynamicUsage.unknownUpdate') }}</span>
        <span>{{ t('dynamicUsage.usageBasis') }}</span>
      </footer>
    </template>
  </ElCard>
</template>

<style scoped>
.dynamic-usage__toolbar, .dynamic-usage__filters, .dynamic-usage__footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}
.dynamic-usage__toolbar { justify-content: space-between; }
.dynamic-usage__filters { flex: 1 1 var(--content-narrow-width); min-inline-size: 0; }
.dynamic-usage__dates {
  flex: 0 1 var(--usage-date-range-width);
  min-inline-size: 0;
  max-inline-size: min(100%, var(--usage-date-range-width));
}
.dynamic-usage__accounts { flex: 0 1 var(--usage-account-filter-width); min-inline-size: 0; max-inline-size: 100%; }
.dynamic-usage__summary {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-4) var(--space-6);
  margin: 0;
  padding: var(--space-4);
  background: var(--color-surface-muted);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
}
.dynamic-usage__summary dt, .dynamic-usage__summary time, .dynamic-usage__footer {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
}
.dynamic-usage__summary dd { margin: 0; overflow-wrap: anywhere; }
.dynamic-usage__summary dd:first-of-type {
  font-size: var(--font-size-title);
  font-weight: var(--font-weight-bold);
}
.dynamic-usage__table { width: 100%; }
.dynamic-usage__footer {
  border-block-start: var(--border-width) solid var(--color-border);
  padding-block-start: var(--space-3);
}
.dynamic-usage :deep(.dynamic-usage__body) {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-4);
}
.dynamic-usage p { margin-block: var(--space-2); }
</style>
