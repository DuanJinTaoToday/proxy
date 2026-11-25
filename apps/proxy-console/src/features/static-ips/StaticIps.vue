<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert, ElButton, ElEmpty, ElIcon, ElInput, ElMessage, ElOption, ElSelect, ElTable, ElTableColumn, ElTooltip,
  type TableInstance, vLoading,
} from 'element-plus'
import { CircleCheck, Clock, CopyDocument, Download, Loading, Search } from '@element-plus/icons-vue'
import { exportStaticIps, getStaticIpCredentials, listStaticIps, type RenewalSubmission, type StaticIp } from '@/api/staticIps'
import { endpoint, useStaticIpPresentation } from './presentation'
import { readRenewalSubmissions, type RenewalIntent } from './renewalIntent'
import RenewalDialog from './RenewalDialog.vue'

const props = defineProps<{ accountKey: string }>()
const { t, locale } = useI18n()
const { formatDate, errorMessage } = useStaticIpPresentation()
const controller = new AbortController()
const storageKey = `proxy-console.static-renewal.${props.accountKey}`

const statuses = ['active', 'expiring', 'pending', 'expired'] as const
type Status = (typeof statuses)[number]
const dayMs = 86_400_000
const maxRenewalItems = 25
const maxExportItems = 1000

function isStaticIp(row: unknown): row is StaticIp {
  return typeof row === 'object' && row !== null
    && 'id' in row && typeof row.id === 'string'
    && 'ip' in row && typeof row.ip === 'string'
    && 'port' in row && typeof row.port === 'number'
    && 'expiry' in row && row.expiry instanceof Date
    && 'region' in row && typeof row.region === 'string'
    && 'ipType' in row && typeof row.ipType === 'string'
    && 'pendingOrderId' in row && (row.pendingOrderId === undefined || typeof row.pendingOrderId === 'string')
}

const ips = shallowRef<StaticIp[]>([])
const loading = ref(true)
const copying = ref<string>()
const exporting = ref(false)
const listError = shallowRef<unknown>()
const keyword = ref('')
const statusFilter = ref<Status | 'available' | ''>('')
const regionFilter = ref('')
const selected = shallowRef<StaticIp[]>([])
const table = ref<TableInstance>()
const now = ref(Date.now())
const intent = shallowRef<RenewalIntent>()
const savedSubmissions = shallowRef(readRenewalSubmissions(storageKey))

const hasExpiry = (ip: StaticIp) => Number.isFinite(ip.expiry.getTime())
const isAvailable = (ip: StaticIp) => ip.expiry.getTime() > now.value
const daysLeft = (ip: StaticIp) => Math.ceil((ip.expiry.getTime() - now.value) / dayMs)
function statusOf(ip: StaticIp): Status {
  if (ip.pendingOrderId) return 'pending'
  if (!isAvailable(ip)) return 'expired'
  return daysLeft(ip) <= 5 ? 'expiring' : 'active'
}

const activeCount = computed(() => ips.value.filter(isAvailable).length)
const expiring = computed(() => ips.value.filter(ip => statusOf(ip) === 'expiring'))
const regions = computed(() => [...new Set(ips.value.map(ip => ip.region).filter(Boolean))].sort((a, b) => a.localeCompare(b, locale.value)))
const visibleIps = computed(() => {
  const term = keyword.value.trim().toLocaleLowerCase(locale.value)
  return ips.value.filter(ip => {
    const status = statusOf(ip)
    const statusMatches = statusFilter.value === 'available'
      ? isAvailable(ip)
      : !statusFilter.value || statusFilter.value === status
    const regionMatches = !regionFilter.value || regionFilter.value === ip.region
    const searchMatches = !term || [endpoint(ip), ip.region].some(value => value.toLocaleLowerCase(locale.value).includes(term))
    return statusMatches && regionMatches && searchMatches
  }).sort((a, b) => Number(!isAvailable(a)) - Number(!isAvailable(b)) || a.expiry.getTime() - b.expiry.getTime())
})

const bulkProblem = computed(() => {
  if (selected.value.length > maxRenewalItems) return t('staticIps.bulkLimit')
  if (selected.value.some(ip => !canRenew(ip))) return t('staticIps.bulkPending')
  return ''
})
const exportProblem = computed(() => {
  if (selected.value.length > maxExportItems) return t('staticIps.exportLimit')
  if (selected.value.some(ip => !isAvailable(ip))) return t('staticIps.exportExpired')
  return ''
})
const filtersActive = computed(() => Boolean(keyword.value || statusFilter.value || regionFilter.value))

async function loadIps() {
  loading.value = true
  listError.value = undefined
  try {
    const items = await listStaticIps(controller.signal)
    now.value = Date.now()
    table.value?.clearSelection()
    ips.value = items
  } catch (error) {
    if (!controller.signal.aborted) listError.value = error
  } finally {
    loading.value = false
  }
}

function toggleStatusFilter(status: 'available' | 'expiring') {
  statusFilter.value = statusFilter.value === status ? '' : status
}

function clearFilters() {
  keyword.value = ''
  statusFilter.value = ''
  regionFilter.value = ''
}

function canRenew(ip: StaticIp) {
  return isAvailable(ip) && !ip.pendingOrderId
}

function openRenewal(targets: StaticIp[]) {
  const affected = savedSubmissions.value.find(submission => targets.some(ip => submission.allocation_ids.includes(ip.id)))
  if (affected) {
    resumeRenewal(affected)
    return
  }
  if (targets.length && targets.length <= maxRenewalItems && targets.every(canRenew)) {
    intent.value = { kind: 'renew', ips: [...targets] }
  }
}

function openPendingOrder(id: string | undefined) {
  if (id) intent.value = { kind: 'order', id }
}

function resumeRenewal(submission: RenewalSubmission) {
  intent.value = { kind: 'resume', submission }
}

function recordSubmission(payload: RenewalSubmission, unresolved: boolean) {
  const submissions = savedSubmissions.value.filter(submission => submission.request_key !== payload.request_key)
  if (unresolved) submissions.push(payload)
  savedSubmissions.value = submissions
  try {
    if (submissions.length) sessionStorage.setItem(storageKey, JSON.stringify(submissions))
    else sessionStorage.removeItem(storageKey)
  } catch {
    // Vue state still supports retry if the browser disables session storage.
  }
}

function retryLabel(submission: RenewalSubmission, index: number) {
  if (savedSubmissions.value.length === 1) return t('staticIps.retryOriginal')
  const firstIp = ips.value.find(ip => submission.allocation_ids.includes(ip.id))
  return firstIp
    ? t('staticIps.retryOriginalIp', { ip: endpoint(firstIp) })
    : t('staticIps.retryOriginalIndex', { index: index + 1 })
}

async function copyLine(ip: StaticIp) {
  if (copying.value) return
  copying.value = ip.id
  try {
    const config = await getStaticIpCredentials(ip.id, controller.signal)
    if (controller.signal.aborted) return
    const address = endpoint(config)
    try {
      await navigator.clipboard.writeText(`${address}:${config.username}:${config.password}`)
      ElMessage.success(t('staticIps.copied', { endpoint: address }))
    } catch {
      ElMessage.error(t('staticIps.copyFailed'))
    }
  } catch (error) {
    if (!controller.signal.aborted) ElMessage.error(errorMessage(error))
  } finally {
    copying.value = undefined
  }
}

async function downloadSelected() {
  if (exporting.value || !selected.value.length || exportProblem.value) return
  const ids = selected.value.map(ip => ip.id)
  exporting.value = true
  try {
    const contents = await exportStaticIps(ids, controller.signal)
    if (controller.signal.aborted) return
    const url = URL.createObjectURL(new Blob([contents], { type: 'text/plain;charset=utf-8' }))
    const link = document.createElement('a')
    link.href = url
    link.download = 'static-ips.txt'
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
    ElMessage.success(t('staticIps.exported', { count: ids.length }))
  } catch (error) {
    if (!controller.signal.aborted) ElMessage.error(errorMessage(error))
  } finally {
    exporting.value = false
  }
}

const updateClock = window.setInterval(() => { now.value = Date.now() }, 30_000)
onMounted(loadIps)
onUnmounted(() => {
  controller.abort()
  window.clearInterval(updateClock)
})
</script>

<template>
  <section class="ip-workspace">
    <div v-if="savedSubmissions.length" class="page-actions">
      <ElButton
        v-for="(submission, index) in savedSubmissions" :key="submission.request_key" type="warning"
        @click="resumeRenewal(submission)"
      >{{ retryLabel(submission, index) }}</ElButton>
    </div>

    <ElAlert v-if="expiring.length" class="expiry-alert" type="warning" show-icon :closable="false">
      <template #title>
        <span class="expiry-message">
          <span>{{ t('staticIps.expiryAlert', { count: expiring.length }) }}</span>
          <ElButton v-if="expiring.length <= maxRenewalItems" link type="warning" @click="openRenewal(expiring)">
            {{ t('staticIps.renewNow') }}
          </ElButton>
        </span>
      </template>
    </ElAlert>

    <div v-if="!loading && !listError" class="ip-metrics" :aria-label="t('staticIps.summary')">
      <button type="button" class="metric" :aria-label="t('staticIps.active')" :aria-pressed="statusFilter === 'available'"
        @click="toggleStatusFilter('available')">
        <div>
          <h2>{{ t('staticIps.active') }}</h2>
          <p class="metric-value tabular">{{ activeCount }} <span>{{ t('staticIps.unit', activeCount) }}</span></p>
          <p class="metric-hint">{{ t('staticIps.activeHint') }}</p>
        </div>
        <ElIcon class="metric-icon active"><CircleCheck /></ElIcon>
      </button>
      <button type="button" class="metric" :aria-label="t('staticIps.expiring')" :aria-pressed="statusFilter === 'expiring'"
        @click="toggleStatusFilter('expiring')">
        <div>
          <h2>{{ t('staticIps.expiring') }}</h2>
          <p class="metric-value tabular">{{ expiring.length }} <span>{{ t('staticIps.unit', expiring.length) }}</span></p>
          <p class="metric-hint">{{ t('staticIps.expiringHint') }}</p>
        </div>
        <ElIcon class="metric-icon expiring"><Clock /></ElIcon>
      </button>
    </div>

    <ElAlert v-if="listError" type="error" show-icon :closable="false" :title="t('staticIps.listFailed')" role="alert">
      <p>{{ errorMessage(listError) }}</p>
      <ElButton link type="primary" @click="loadIps">{{ t('staticIps.retry') }}</ElButton>
    </ElAlert>

    <section class="ip-table-card" :aria-label="t('staticIps.list')" v-loading="loading">
      <div class="table-toolbar">
        <div class="table-filters">
          <ElInput
            v-model="keyword" class="filter-search" :prefix-icon="Search" clearable
            :aria-label="t('staticIps.search')" :placeholder="t('staticIps.search')"
          />
          <ElSelect v-model="statusFilter" class="filter-status" :empty-values="[null, undefined]" :aria-label="t('staticIps.filterStatus')">
            <ElOption :label="t('staticIps.allStatuses')" value="" />
            <ElOption :label="t('staticIps.availableFilter')" value="available" />
            <ElOption v-for="status in statuses" :key="status" :label="t(`staticIps.status.${status}`)" :value="status" />
          </ElSelect>
          <ElSelect v-model="regionFilter" class="filter-region" :empty-values="[null, undefined]" :aria-label="t('staticIps.filterRegion')">
            <ElOption :label="t('staticIps.allRegions')" value="" />
            <ElOption v-for="region in regions" :key="region" :value="region" :label="region" />
          </ElSelect>
        </div>
        <div class="table-actions">
          <span class="tabular" role="status">{{ t('staticIps.selection', { count: selected.length }) }}</span>
          <ElTooltip :content="bulkProblem" :disabled="!bulkProblem">
            <span><ElButton :disabled="!selected.length || Boolean(bulkProblem)" @click="openRenewal(selected)">{{ t('staticIps.bulkRenew') }}</ElButton></span>
          </ElTooltip>
          <ElTooltip :content="exportProblem" :disabled="!exportProblem">
            <span><ElButton :disabled="!selected.length || Boolean(exportProblem)" :loading="exporting" :icon="Download" @click="downloadSelected">{{ t('staticIps.export') }}</ElButton></span>
          </ElTooltip>
        </div>
      </div>

      <ElTable ref="table" :data="visibleIps" row-key="id" class="ip-table"
        :header-cell-style="{ height: 'var(--tabs-height)', fontSize: 'var(--font-size-xs)' }"
        :cell-style="{ height: 'calc(var(--space-7) + var(--space-2) + var(--border-width))' }"
        :empty-text="loading ? ' ' : undefined" @selection-change="selected = $event">
        <ElTableColumn type="selection" :selectable="isAvailable" reserve-selection width="52" />
        <ElTableColumn :label="t('staticIps.address')" min-width="200">
          <template #default="{ row }">
            <div v-if="isStaticIp(row)" class="endpoint">
              <span>{{ endpoint(row) }}</span>
              <ElTooltip v-if="isAvailable(row)" :content="t('staticIps.copyTooltip')" placement="top">
                <ElButton
                  link type="primary" :icon="CopyDocument" :loading="copying === row.id"
                  :disabled="Boolean(copying) && copying !== row.id"
                  :aria-label="t('staticIps.copy', { endpoint: endpoint(row) })" @click="copyLine(row)"
                />
              </ElTooltip>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.region')" min-width="170">
          <template #default="{ row }">
            <template v-if="isStaticIp(row)">
              <span v-if="row.ipType" class="product-tag">{{ row.ipType }}</span>{{ row.region || t('staticIps.unknownRegion') }}
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.state')" min-width="130">
          <template #default="{ row }">
            <span v-if="isStaticIp(row)" class="status-badge" :class="statusOf(row)">
              <ElIcon v-if="statusOf(row) === 'pending'" class="pending-icon"><Loading /></ElIcon>
              {{ t(`staticIps.status.${statusOf(row)}`) }}
            </span>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.expiresAt')" min-width="180">
          <template #default="{ row }">
            <template v-if="isStaticIp(row)">
              <time v-if="hasExpiry(row)" class="tabular" :datetime="row.expiry.toISOString()">{{ formatDate(row.expiry) }}</time>
              <p v-if="isAvailable(row)" class="secondary tabular" :class="statusOf(row)">{{ t('staticIps.remaining', { days: daysLeft(row) }) }}</p>
              <p v-else-if="row.pendingOrderId && !isAvailable(row)" class="secondary">{{ t('staticIps.servicePaused') }}</p>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.actions')" width="120" fixed="right">
          <template #default="{ row }">
            <template v-if="isStaticIp(row)">
              <ElButton v-if="row.pendingOrderId" size="small" @click="openPendingOrder(row.pendingOrderId)">{{ t('staticIps.check') }}</ElButton>
              <ElButton v-else size="small" :disabled="!canRenew(row)" @click="openRenewal([row])">{{ t('staticIps.renew') }}</ElButton>
            </template>
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty
            v-if="!loading && !listError" :image-size="72"
            :description="t(filtersActive ? 'staticIps.noMatch' : 'staticIps.empty')"
          >
            <ElButton v-if="filtersActive" @click="clearFilters">{{ t('staticIps.clear') }}</ElButton>
          </ElEmpty>
        </template>
      </ElTable>
      <p v-if="!loading && !listError" class="table-footer tabular">
        {{ t(filtersActive ? 'staticIps.filteredCount' : 'staticIps.count', { count: visibleIps.length, total: ips.length }) }}
      </p>
    </section>

    <RenewalDialog
      v-if="intent" :intent="intent" @closed="intent = undefined"
      @submission="recordSubmission" @changed="loadIps"
    />
  </section>
</template>

<style scoped>
.ip-workspace { display: grid; gap: var(--space-4); min-width: 0; }
.page-actions { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: var(--space-2); }
.page-actions button { margin: 0; }
.expiry-alert { border-radius: var(--radius-card); }
.expiry-message { display: flex; align-items: center; gap: var(--space-3); }
.ip-metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: var(--space-4); }
.metric {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--space-3);
  padding: var(--space-4) calc(var(--space-4) + var(--space-1) / 2);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  color: var(--color-text);
  cursor: pointer;
  font: inherit;
  text-align: start;
}
.metric:hover { border-color: var(--color-primary-border); }
.metric[aria-pressed='true'] {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 var(--border-width) var(--color-primary);
}
.metric h2 { font-size: var(--font-size-sm); font-weight: var(--font-weight-semibold); }
.metric-value {
  margin-top: var(--space-1);
  font-size: calc(var(--font-size-display) - var(--space-1) * 1.5);
  font-weight: var(--font-weight-bold);
  line-height: 1.2;
  letter-spacing: -.035em;
}
.metric-value span { font-size: var(--font-size-sm); font-weight: var(--font-weight-medium); color: var(--color-text-secondary); letter-spacing: 0; }
.metric-hint, .secondary, .table-footer { font-size: var(--font-size-xs); color: var(--color-text-secondary); }
.metric-icon {
  flex: none;
  width: var(--control-height);
  height: var(--control-height);
  border-radius: var(--space-2);
  font-size: var(--font-size-xl);
}
.active, .product-tag { color: var(--color-success-text); background: var(--color-success-bg); }
.expiring { color: var(--color-warning-text); background: var(--color-warning-bg); }
.pending { color: var(--color-info-text); background: var(--color-info-bg); }
.expired { color: var(--color-text-secondary); background: var(--color-canvas); }
.secondary { background: none; }
.ip-table-card {
  min-width: 0;
  overflow: hidden;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
}
.table-toolbar, .table-actions, .table-filters { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.table-toolbar {
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
}
.filter-search { width: calc(var(--subaccount-metric-width) + var(--space-4)); max-width: 100%; }
.filter-status, .filter-region { width: var(--business-date-filter-width); max-width: 100%; }
.table-actions { color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.ip-table {
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  --el-table-current-row-bg-color: var(--color-surface-muted);
  font-size: var(--font-size-sm);
}
.table-footer { padding: var(--space-3) var(--space-4); }
.endpoint {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  color: var(--color-text);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  font-variant-numeric: tabular-nums;
}
.product-tag, .status-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  border-radius: var(--radius-control);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
}
.product-tag { margin-inline-end: var(--space-1); padding-inline: var(--space-1); }
.status-badge { padding-inline: var(--space-2); }
.pending-icon { animation: renewal-pending 1.2s linear infinite; }
@keyframes renewal-pending { to { transform: rotate(1turn); } }
@media (prefers-reduced-motion: reduce) {
  .pending-icon { animation: none; }
}
@media (max-width: 680px) {
  .ip-metrics { gap: var(--space-2); }
  .metric { padding-inline: var(--space-3); gap: var(--space-2); }
  .metric-icon { width: var(--space-5); height: var(--space-5); font-size: var(--font-size-md); }
  .table-toolbar { padding-inline: var(--space-3); }
  .table-filters { width: 100%; }
  .filter-search { width: 100%; }
  .filter-status, .filter-region { flex: 1 1 0; min-width: 0; }
}
</style>
