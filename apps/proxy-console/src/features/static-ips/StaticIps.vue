<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import {
  ElAlert, ElButton, ElCard, ElEmpty, ElIcon, ElInput, ElMessage, ElOption, ElSelect, ElTable, ElTableColumn, ElTag, ElTooltip,
  type TableInstance, vLoading,
} from 'element-plus'
import { CircleCheck, Clock, CopyDocument, Download, Search } from '@element-plus/icons-vue'
import { exportStaticIps, getStaticIpCredentials, listStaticIps, type RenewalSubmission, type StaticIp } from '@/api/staticIps'
import { endpoint, useStaticIpPresentation } from './presentation'
import { readRenewalSubmissions, type RenewalIntent } from './renewalIntent'
import RenewalDialog from './RenewalDialog.vue'

const props = defineProps<{ accountPhone: string }>()
const { t, locale } = useI18n()
const { formatDate, errorMessage } = useStaticIpPresentation()
const router = useRouter()
const controller = new AbortController()
const storageKey = `proxy-console.static-renewal.${props.accountPhone}`

const statuses = ['active', 'expiring', 'pending', 'expired'] as const
type Status = (typeof statuses)[number]
const statusType = { active: 'success', expiring: 'warning', pending: 'info', expired: 'info' } as const
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
  const affected = savedSubmissions.value.find(submission => targets.some(ip => submission.supplier_item_ids.includes(ip.id)))
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
  const firstIp = ips.value.find(ip => submission.supplier_item_ids.includes(ip.id))
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
    <div class="page-actions">
      <ElButton
        v-for="(submission, index) in savedSubmissions" :key="submission.request_key" type="warning"
        @click="resumeRenewal(submission)"
      >{{ retryLabel(submission, index) }}</ElButton>
      <ElButton type="primary" @click="router.push({ name: 'console-static-pricing' })">{{ t('staticIps.buy') }}</ElButton>
    </div>

    <ElAlert
      v-if="expiring.length" type="warning" show-icon :closable="false"
      :title="t('staticIps.expiryAlert', { count: expiring.length })"
    >
      <ElButton v-if="expiring.length <= maxRenewalItems" link type="warning" @click="openRenewal(expiring)">
        {{ t('staticIps.renewNow') }}
      </ElButton>
    </ElAlert>

    <div v-if="ips.length" class="ip-metrics" :aria-label="t('staticIps.summary')">
      <ElCard shadow="never">
        <div class="metric">
          <div>
            <h2><ElButton text class="metric-filter" :aria-pressed="statusFilter === 'available'"
              @click="toggleStatusFilter('available')">{{ t('staticIps.active') }}</ElButton></h2>
            <strong class="metric-value tabular">{{ activeCount }}</strong>
          </div>
          <ElIcon class="metric-icon active"><CircleCheck /></ElIcon>
        </div>
      </ElCard>
      <ElCard shadow="never">
        <div class="metric">
          <div>
            <h2><ElButton text class="metric-filter" :aria-pressed="statusFilter === 'expiring'"
              @click="toggleStatusFilter('expiring')">{{ t('staticIps.expiring') }}</ElButton></h2>
            <strong class="metric-value tabular">{{ expiring.length }}</strong>
            <p>{{ t('staticIps.expiringHint') }}</p>
          </div>
          <ElIcon class="metric-icon expiring"><Clock /></ElIcon>
        </div>
      </ElCard>
    </div>

    <ElAlert v-if="listError" type="error" show-icon :closable="false" :title="t('staticIps.listFailed')" role="alert">
      <p>{{ errorMessage(listError) }}</p>
      <ElButton link type="primary" @click="loadIps">{{ t('staticIps.retry') }}</ElButton>
    </ElAlert>

    <ElCard shadow="never" class="ip-table-card" v-loading="loading">
      <div class="table-toolbar">
        <div class="table-filters">
          <ElInput
            v-model="keyword" :prefix-icon="Search" clearable
            :aria-label="t('staticIps.search')" :placeholder="t('staticIps.search')"
          />
          <ElSelect v-model="statusFilter" :empty-values="[null, undefined]" :aria-label="t('staticIps.filterStatus')">
            <ElOption :label="t('staticIps.allStatuses')" value="" />
            <ElOption :label="t('staticIps.availableFilter')" value="available" />
            <ElOption v-for="status in statuses" :key="status" :label="t(`staticIps.status.${status}`)" :value="status" />
          </ElSelect>
          <ElSelect v-model="regionFilter" :empty-values="[null, undefined]" :aria-label="t('staticIps.filterRegion')">
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

      <ElTable ref="table" :data="visibleIps" row-key="id" :empty-text="loading ? ' ' : undefined" @selection-change="selected = $event">
        <ElTableColumn type="selection" :selectable="isAvailable" reserve-selection width="48" />
        <ElTableColumn :label="t('staticIps.address')" min-width="205">
          <template #default="{ row }">
            <div v-if="isStaticIp(row)" class="endpoint">
              <span class="mono">{{ endpoint(row) }}</span>
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
        <ElTableColumn :label="t('staticIps.region')" min-width="140">
          <template #default="{ row }">
            <template v-if="isStaticIp(row)">
              <div>{{ row.region || t('staticIps.unknownRegion') }}</div>
              <p v-if="row.ipType" class="secondary">{{ row.ipType }}</p>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.state')" min-width="142">
          <template #default="{ row }">
            <ElTag v-if="isStaticIp(row)" :type="statusType[statusOf(row)]">{{ t(`staticIps.status.${statusOf(row)}`) }}</ElTag>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.expiresAt')" min-width="180">
          <template #default="{ row }">
            <template v-if="isStaticIp(row)">
              <time v-if="hasExpiry(row)" class="tabular" :datetime="row.expiry.toISOString()">{{ formatDate(row.expiry) }}</time>
              <p v-if="isAvailable(row) && daysLeft(row) <= 5" class="secondary tabular">{{ t('staticIps.remaining', { days: daysLeft(row) }) }}</p>
              <p v-else-if="row.pendingOrderId && !isAvailable(row)" class="secondary">{{ t('staticIps.servicePaused') }}</p>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('staticIps.actions')" min-width="130">
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
    </ElCard>

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
.ip-metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--subaccount-metric-width)), 1fr)); gap: var(--space-4); }
.metric { display: flex; justify-content: space-between; align-items: flex-start; gap: var(--space-3); }
.metric-filter { font-size: var(--font-size-md); font-weight: var(--font-weight-medium); padding-inline: 0; }
.metric-filter[aria-pressed='true'] { color: var(--color-primary); }
.metric p, .secondary, .table-footer { font-size: var(--font-size-xs); color: var(--color-text-secondary); }
.metric-value { font-size: var(--font-size-title); color: var(--color-text); }
.metric-icon { border-radius: var(--radius-control); box-sizing: content-box; padding: var(--space-2); font-size: var(--font-size-xl); }
.metric-icon.active { background: var(--color-success-bg); color: var(--color-success-text); }
.metric-icon.expiring { background: var(--color-warning-bg); color: var(--color-warning-text); }
.ip-table-card { min-width: 0; }
.table-toolbar, .table-actions, .table-filters { display: flex; align-items: center; gap: var(--space-2); flex-wrap: wrap; }
.table-toolbar { justify-content: space-between; margin-block-end: var(--space-4); }
.table-filters { flex: 1 1 auto; }
.table-filters > * { flex: 1 1 var(--account-name-width); max-width: var(--subaccount-metric-width); min-width: 0; }
.table-actions { color: var(--color-text-secondary); }
.table-footer { margin-block-start: var(--space-3); }
.endpoint { display: flex; align-items: center; gap: var(--space-2); overflow-wrap: anywhere; }
</style>
