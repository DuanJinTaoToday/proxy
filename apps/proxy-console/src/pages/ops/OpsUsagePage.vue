<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElOption, ElRadioButton, ElRadioGroup, ElSelect, ElSkeleton, ElTabPane, ElTabs, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { gapStatuses, listOpsUsageFindings, resolutions, type Finding, type FindingsTab } from '@/api/opsUsage'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { useOpsHistoryPage } from './useOpsHistoryPage'
import './opsHistory.css'

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const tabs: FindingsTab[] = ['gaps', 'anomalies']
const counts = reactive<Record<FindingsTab, bigint>>({ gaps: 0n, anomalies: 0n })
const tab = computed(() => tabs.find(value => value === route.query.tab) ?? 'gaps')
const resolution = computed(() => resolutions.find(value => value === route.query.resolution) ?? 'open')
const status = computed(() => gapStatuses.find(value => value === route.query.status))
const gatewayId = computed(() => readScope(route.query.gateway_id))
const bootId = computed(() => readScope(route.query.boot_id))
const tenantId = computed(() => readScope(route.query.tenant_id))
const scope = computed(() => ({
  gatewayId: gatewayId.value,
  bootId: bootId.value,
  tenantId: tenantId.value,
}))

function readScope(value: unknown) {
  return typeof value === 'string' ? value : undefined
}

function query(patch: Record<string, string | undefined>) {
  return router.push({ query: { ...route.query, ...patch } })
}

const { state, page, load, reset, previous, next } = useOpsHistoryPage<Finding>(async (cursor, signal) => {
  const currentTab = tab.value
  const result = await listOpsUsageFindings(currentTab, { ...scope.value, resolution: resolution.value, status: status.value }, cursor, signal)
  if (!cursor && !signal.aborted) counts[currentTab] = result.openCount
  return result
})

watch([tab, resolution, status, scope], reset, { immediate: true })
watch(scope, async (currentScope, _, onCleanup) => {
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  counts.gaps = 0n
  counts.anomalies = 0n
  const otherTab = tab.value === 'gaps' ? 'anomalies' : 'gaps'
  try {
    const result = await listOpsUsageFindings(otherTab, { ...currentScope, resolution: 'open' }, undefined, controller.signal, 1)
    if (!controller.signal.aborted) counts[otherTab] = result.openCount
  } catch {
    // The tab shows its own request error when opened.
  }
}, { immediate: true })

const dateTime = computed(() => new Intl.DateTimeFormat(locale.value, {
  timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit',
  hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
}))
const time = (date: Date) => dateTime.value.format(date)
const cellStyle = { height: 'var(--ops-history-row-height)' }
const statusLabel = (row: Finding) => row.resolvedAt ? 'resolved' : row.type === 'gap' ? row.status ?? 'open' : 'open'
function tagType(row: Finding) {
  if (row.resolvedAt) return 'success'
  return row.type === 'gap' && row.status === 'pending_replay' ? 'warning' : 'danger'
}
</script>

<template>
  <h1 class="ops-page-title">{{ t('nav.opsUsage') }}</h1>

  <ElTabs :model-value="tab" class="ops-page-tabs ops-usage-tabs" @update:model-value="query({ tab: $event === 'anomalies' ? 'anomalies' : undefined })">
    <ElTabPane v-for="item in tabs" :key="item" :name="item">
      <template #label>
        <span :class="{ 'ops-usage-tab-active': tab === item }">{{ t('opsUsage.' + item) }}</span>
        <span v-if="counts[item]" class="ops-usage-count">{{ counts[item].toLocaleString(locale) }}</span>
      </template>
    </ElTabPane>
  </ElTabs>

  <ElCard class="ops-history" shadow="never" role="region" :aria-label="t('opsUsage.' + tab)">
    <div class="ops-history-toolbar ops-usage-toolbar">
      <div class="ops-usage-filters">
        <ElRadioGroup :model-value="resolution" :aria-label="t('opsUsage.resolutionFilter')"
          @update:model-value="query({ resolution: resolutions.find(value => value === $event && value !== 'open') })">
          <ElRadioButton v-for="item in resolutions" :key="item" :value="item">{{ t('opsUsage.' + item) }}</ElRadioButton>
        </ElRadioGroup>
        <ElSelect v-if="tab === 'gaps'" :model-value="status ?? 'all'" class="ops-usage-status" :aria-label="t('opsUsage.statusFilter')"
          @update:model-value="query({ status: gapStatuses.find(value => value === $event) })">
          <ElOption value="all" :label="t('opsUsage.allStatuses')" />
          <ElOption v-for="item in gapStatuses" :key="item" :value="item" :label="t('opsUsage.' + item)" />
        </ElSelect>
      </div>
      <span class="ops-usage-hint">{{ t(tab === 'gaps' ? 'opsUsage.gapsHint' : 'opsUsage.anomaliesHint') }}</span>
    </div>

    <ElSkeleton v-if="state.kind === 'loading'" class="ops-history-placeholder" animated :rows="5" />
    <div v-else-if="state.kind === 'failed'" class="ops-history-placeholder">
      <ElAlert type="error" :title="errorMessage(state.error)" show-icon :closable="false">
        <ElButton size="small" @click="load">{{ t('orders.retry') }}</ElButton>
      </ElAlert>
    </div>
    <ElTable v-else :key="tab" :data="state.rows" :empty-text="t('opsUsage.empty')" class="ops-history-table ops-usage-table"
      :cell-style="cellStyle" header-cell-class-name="ops-history-heading">
      <!-- @vue-generic {Finding} -->
      <ElTableColumn :label="t('opsUsage.status')" :width="tab === 'gaps' ? 110 : 90">
        <template #default="{ row }">
          <ElTag :type="tagType(row)" size="small" disable-transitions>{{ t('opsUsage.' + statusLabel(row)) }}</ElTag>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {Finding} -->
      <ElTableColumn :label="t(tab === 'gaps' ? 'opsUsage.kind' : 'opsUsage.anomalyKind')" :width="tab === 'gaps' ? 100 : undefined" min-width="150">
        <template #default="{ row }">
          <span v-if="row.type === 'anomaly'" class="mono">{{ row.kind }}</span>
          <template v-else>{{ t('opsUsage.' + row.kind) }}</template>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {Finding} -->
      <ElTableColumn :label="t('opsUsage.tenant')" min-width="140">
        <template #default="{ row }">
          <RouterLink v-if="row.tenantId" class="ops-usage-link" :to="{ name: 'ops-tenant', params: { tenantId: row.tenantId } }">
            {{ row.tenantName || row.tenantId }}
          </RouterLink>
          <template v-else>—</template>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {Finding} -->
      <ElTableColumn :label="t('opsUsage.gatewayBoot')" min-width="180">
        <template #default="{ row }">
          <span class="mono">{{ row.gatewayId }}</span>
          <span class="ops-history-note mono">{{ row.bootId }}</span>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {Finding} -->
      <ElTableColumn :label="t(tab === 'gaps' ? 'opsUsage.session' : 'opsUsage.fileOffset')" :min-width="tab === 'gaps' ? 120 : 160">
        <template #default="{ row }">
          <span v-if="row.type === 'gap'" class="mono">{{ row.sessionId || '—' }}</span>
          <template v-else>
            <span class="mono">{{ row.fileId }}</span>
            <span class="ops-history-note mono">@ {{ row.offset.toLocaleString(locale) }}</span>
          </template>
        </template>
      </ElTableColumn>
      <!-- @vue-generic {Finding} -->
      <ElTableColumn v-if="tab === 'gaps'" :label="t('opsUsage.fromAt')" min-width="150">
        <template #default="{ row }">{{ row.type === 'gap' ? time(row.fromAt) : '' }}</template>
      </ElTableColumn>
      <!-- @vue-generic {Finding} -->
      <ElTableColumn :label="t('opsUsage.detectedResolved')" min-width="160">
        <template #default="{ row }">
          {{ time(row.detectedAt) }}
          <span class="ops-history-note">{{ row.resolvedAt ? t('opsUsage.resolvedAt', { time: time(row.resolvedAt) }) : t('opsUsage.open') }}</span>
        </template>
      </ElTableColumn>
    </ElTable>

    <div class="ops-history-footer ops-usage-footer">
      <span>{{ t('opsUsage.readOnly') }}</span>
      <nav v-if="page > 1 || (state.kind === 'ready' && state.nextBeforeId)" :aria-label="t('opsUsage.' + tab)" class="ops-usage-filters">
        <span>{{ t('wallet.page', { page }) }}</span>
        <ElButton size="small" :disabled="page === 1 || state.kind === 'loading'" @click="previous">{{ t('orders.previous') }}</ElButton>
        <ElButton size="small" :disabled="state.kind !== 'ready' || !state.nextBeforeId" @click="next">{{ t('orders.next') }}</ElButton>
      </nav>
    </div>
  </ElCard>
</template>

<style scoped>
.ops-usage-tabs {
  --el-tabs-header-height: var(--ops-history-header-height);
  margin-block-end: var(--ops-usage-tabs-gap);
  line-height: normal;
}
.ops-usage-tab-active { font-weight: var(--font-weight-bold); }
.ops-history .ops-usage-table {
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  --el-table-current-row-bg-color: var(--color-surface-muted);
}
.ops-usage-table .mono { font-size: var(--font-size-xs); }
.ops-usage-count {
  display: inline-grid;
  place-items: center;
  min-inline-size: var(--ops-usage-count-size);
  block-size: var(--ops-usage-count-size);
  margin-inline-start: var(--ops-usage-count-gap);
  padding-inline: var(--space-1);
  border-radius: var(--ops-usage-count-size);
  color: var(--color-primary);
  background: var(--color-primary-soft);
  font-size: var(--ops-usage-count-font);
  font-weight: var(--font-weight-bold);
}
.ops-usage-toolbar { justify-content: space-between; gap: var(--space-3); }
.ops-usage-filters { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-2); }
.ops-usage-status { width: var(--ops-history-status-width); }
.ops-usage-hint {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
  line-height: normal;
  margin-inline-end: var(--space-1);
}
.ops-usage-link { color: var(--color-primary); font-weight: var(--font-weight-semibold); text-decoration: none; }
.ops-usage-link:hover { text-decoration: underline; }
.ops-usage-footer {
  padding-block: var(--ops-usage-footer-padding);
  font-size: var(--font-size-xs);
  line-height: normal;
}
</style>
