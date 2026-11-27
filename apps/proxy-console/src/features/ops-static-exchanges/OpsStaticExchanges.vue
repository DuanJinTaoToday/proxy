<script setup lang="ts">
import { ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert,
  ElButton,
  ElCard,
  ElCheckbox,
  ElEmpty,
  ElInput,
  ElLoading,
  ElTable,
  ElTableColumn,
  ElTag,
} from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import type {
  OperatorStaticIpExchange,
  OperatorStaticIpExchangeAuditEntry,
  OperatorStaticIpExchangeQuota,
} from '@/api/opsStaticExchanges'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import type { PendingExchangeIntent } from './exchangeIntent'
import ExchangeOperationDialog, { type ExchangeDialogMode } from './ExchangeOperationDialog.vue'
import {
  canCorrectExchange,
  canResolveExchange,
  exchangeStatusBadge,
  useOpsStaticExchanges,
} from './useOpsStaticExchanges'

const vLoading = ElLoading.directive
const { t, te, locale } = useI18n()
const errorMessage = useErrorMessage()

const {
  canRead,
  canWrite,
  searchQuery,
  searchStatus,
  searchError,
  selectedTenant,
  filterQueueByTenant,
  queueItems,
  queueNextCursor,
  queueLoading,
  queueLoadingMore,
  queueError,
  quota,
  quotaLoading,
  quotaError,
  historyItems,
  historyNextCursor,
  historyLoading,
  historyLoadingMore,
  historyError,
  auditEntries,
  auditNextBeforeId,
  auditLoading,
  auditLoadingMore,
  auditError,
  pendingIntents,
  searchTenant,
  clearSearchFeedback,
  selectTenantById,
  clearSelectedTenant,
  loadQueue,
  loadQuota,
  loadHistory,
  loadAudit,
  refreshPendingIntents,
  refreshAfterMutation,
} = useOpsStaticExchanges()

const dialogOpen = ref(false)
const dialogMode = ref<ExchangeDialogMode>('adjust')
const dialogTenantId = ref('')
const dialogTenantName = ref<string>()
const dialogExchange = shallowRef<OperatorStaticIpExchange>()
const dialogInitialIntent = shallowRef<PendingExchangeIntent>()

const tableHeaderStyle = {
  height: 'var(--ops-history-header-height)',
  fontSize: 'var(--font-size-xs)',
  fontWeight: 'var(--font-weight-semibold)',
}
const tableCellStyle = {
  height: 'var(--ops-history-row-height)',
}

function formatDate(date?: Date) {
  return date ? shanghaiDateTime(date, locale.value, 'short') : t('opsExchange.common.none')
}

function resolutionLabel(resolution: string) {
  if (resolution === 'executed') return t('opsExchange.resolution.executed')
  if (resolution === 'not_executed') return t('opsExchange.resolution.notExecuted')
  return resolution
}

function auditKindLabel(kind: string) {
  const key = `opsExchange.auditKind.${kind}`
  return te(key) ? t(key) : (kind || t('opsExchange.common.none'))
}

function formatQuotaInline(snapshot?: OperatorStaticIpExchangeQuota) {
  if (!snapshot) return t('opsExchange.common.none')
  return `${snapshot.total} / ${snapshot.remaining} / ${snapshot.reserved} / ${snapshot.used}`
}

function pendingIntentSummary(intent: PendingExchangeIntent) {
  if (intent.action === 'adjust_quota') {
    const delta = intent.payload.delta > 0 ? `+${intent.payload.delta}` : String(intent.payload.delta)
    return t('opsExchange.pendingBanner.adjustItem', { delta, requestKey: intent.requestKey })
  }
  const key = intent.action === 'correct_exchange'
    ? 'opsExchange.pendingBanner.correctItem'
    : 'opsExchange.pendingBanner.resolveItem'
  return t(key, { idempotencyKey: intent.idempotencyKey, requestKey: intent.requestKey })
}

function openAdjustDialog() {
  if (!canWrite.value || !selectedTenant.value) return
  dialogMode.value = 'adjust'
  dialogTenantId.value = selectedTenant.value.id
  dialogTenantName.value = selectedTenant.value.name
  dialogExchange.value = undefined
  dialogInitialIntent.value = undefined
  dialogOpen.value = true
}

function openResolutionDialog(row: OperatorStaticIpExchange, mode: 'resolve' | 'correct') {
  if (!canWrite.value) return
  const tenantId = row.tenantId || selectedTenant.value?.id || ''
  if (!tenantId) return
  if (selectedTenant.value?.id !== tenantId) {
    selectTenantById(tenantId)
  }
  dialogMode.value = mode
  dialogTenantId.value = tenantId
  dialogTenantName.value = selectedTenant.value?.id === tenantId ? selectedTenant.value.name : undefined
  dialogExchange.value = row
  dialogInitialIntent.value = undefined
  dialogOpen.value = true
}

function openPendingIntentDialog(intent: PendingExchangeIntent) {
  if (!canWrite.value) return
  dialogMode.value = intent.action === 'adjust_quota'
    ? 'adjust'
    : intent.action === 'correct_exchange'
      ? 'correct'
      : 'resolve'
  dialogTenantId.value = intent.tenantId
  dialogTenantName.value = selectedTenant.value?.id === intent.tenantId ? selectedTenant.value.name : undefined
  if (intent.action === 'adjust_quota') {
    dialogExchange.value = undefined
  } else {
    dialogExchange.value =
      historyItems.value.find(item => item.idempotencyKey === intent.idempotencyKey)
      ?? queueItems.value.find(item => item.idempotencyKey === intent.idempotencyKey)
  }
  dialogInitialIntent.value = intent
  dialogOpen.value = true
}

function handleMutationCompleted() {
  void refreshAfterMutation()
}
</script>

<template>
  <ElAlert
    v-if="!canRead"
    type="warning"
    :title="t('errors.ACCESS_DENIED')"
    :closable="false"
    show-icon
  />

  <div v-else class="ops-exchange">
    <ElAlert
      v-if="!canWrite"
      type="info"
      :title="t('opsExchange.common.readOnlyHint')"
      :closable="false"
      show-icon
    />

    <!-- Tenant search & selection -->
    <ElCard shadow="never" class="exchange-card">
      <div class="section-header">
        <div>
          <h2 class="section-title">{{ t('opsExchange.search.title') }}</h2>
          <p class="section-note">{{ t('opsExchange.search.description') }}</p>
        </div>
      </div>
      <div class="section-body">
        <form class="search-row" @submit.prevent="searchTenant">
          <ElInput
            v-model="searchQuery"
            clearable
            maxlength="254"
            :prefix-icon="Search"
            :aria-label="t('opsExchange.search.label')"
            :placeholder="t('opsExchange.search.placeholder')"
            @input="clearSearchFeedback"
          />
          <ElButton
            type="primary"
            native-type="submit"
            :loading="searchStatus === 'loading'"
          >
            {{ t('opsExchange.search.submit') }}
          </ElButton>
        </form>

        <ElEmpty
          v-if="searchStatus === 'not_found'"
          class="search-feedback"
          :description="t('opsExchange.search.notFound')"
          :image-size="48"
        />
        <ElAlert
          v-else-if="searchStatus === 'error'"
          class="search-feedback"
          type="error"
          :title="errorMessage(searchError)"
          :closable="false"
          show-icon
        >
          <ElButton size="small" @click="searchTenant">{{ t('opsExchange.common.retry') }}</ElButton>
        </ElAlert>

        <div v-if="selectedTenant" class="selected-tenant">
          <div class="selected-tenant__info">
            <span class="selected-tenant__label">{{ t('opsExchange.search.selected') }}</span>
            <strong v-if="selectedTenant.name" class="selected-tenant__name">{{ selectedTenant.name }}</strong>
            <span class="selected-tenant__meta">
              {{ t('opsExchange.search.tenantId') }}: <span class="mono">{{ selectedTenant.id }}</span>
              <template v-if="selectedTenant.account">
                · {{ t('opsExchange.search.tenantAccount') }}: <span class="mono">{{ selectedTenant.account }}</span>
              </template>
              <template v-if="selectedTenant.email">
                · {{ selectedTenant.email }}
              </template>
            </span>
          </div>
          <ElButton size="small" @click="clearSelectedTenant">
            {{ t('opsExchange.search.clearTenant') }}
          </ElButton>
        </div>
      </div>
    </ElCard>

    <!-- Global / filtered anomaly queue -->
    <ElCard shadow="never" class="exchange-card">
      <div class="section-header">
        <div>
          <h2 class="section-title">{{ t('opsExchange.queue.title') }}</h2>
          <p class="section-note">{{ t('opsExchange.queue.description') }}</p>
        </div>
        <div class="section-controls">
          <ElCheckbox v-if="selectedTenant" v-model="filterQueueByTenant">
            {{ t('opsExchange.queue.filterSelected') }}
          </ElCheckbox>
          <ElButton size="small" :loading="queueLoading" @click="loadQueue()">
            {{ t('opsExchange.common.refresh') }}
          </ElButton>
        </div>
      </div>

      <ElAlert
        v-if="queueError"
        type="error"
        :title="errorMessage(queueError)"
        :closable="false"
        show-icon
        class="table-alert"
      >
        <ElButton size="small" @click="loadQueue()">{{ t('opsExchange.common.retry') }}</ElButton>
      </ElAlert>

      <ElTable
        v-loading="queueLoading"
        :data="queueItems"
        :row-key="(row: OperatorStaticIpExchange) => `${row.tenantId}:${row.idempotencyKey}`"
        class="exchange-table"
        :header-cell-style="tableHeaderStyle"
        :cell-style="tableCellStyle"
      >
        <!-- @vue-generic {OperatorStaticIpExchange} -->
        <ElTableColumn :label="t('opsExchange.columns.status')" min-width="150">
          <template #default="{ row }">
            <div class="status-cell">
              <ElTag :type="exchangeStatusBadge(row).type" size="small" disable-transitions>
                {{ exchangeStatusBadge(row).labelKey ? t(exchangeStatusBadge(row).labelKey) : exchangeStatusBadge(row).raw }}
              </ElTag>
              <span v-if="row.resolution" class="cell-meta">{{ resolutionLabel(row.resolution) }}</span>
            </div>
          </template>
        </ElTableColumn>

        <!-- @vue-generic {OperatorStaticIpExchange} -->
        <ElTableColumn :label="t('opsExchange.columns.tenant')" min-width="140">
          <template #default="{ row }">
            <ElButton link type="primary" class="tenant-jump" @click="selectTenantById(row.tenantId)">
              <span class="mono">{{ row.tenantId }}</span>
            </ElButton>
          </template>
        </ElTableColumn>

        <!-- @vue-generic {OperatorStaticIpExchange} -->
        <ElTableColumn :label="t('opsExchange.columns.requestAndAllocation')" min-width="210">
          <template #default="{ row }">
            <span class="cell-primary mono">{{ row.idempotencyKey }}</span>
            <span class="cell-meta mono">
              {{ t('opsExchange.labels.allocationId') }}: {{ row.allocationId || t('opsExchange.common.none') }}
              <template v-if="row.nodeId"> · {{ t('opsExchange.labels.nodeId') }}: {{ row.nodeId }}</template>
            </span>
          </template>
        </ElTableColumn>

        <!-- @vue-generic {OperatorStaticIpExchange} -->
        <ElTableColumn :label="t('opsExchange.columns.ips')" min-width="160">
          <template #default="{ row }">
            <span class="cell-primary mono">{{ row.oldIp || t('opsExchange.common.none') }}</span>
            <span class="cell-meta mono">→ {{ row.newIp || t('opsExchange.common.none') }}</span>
          </template>
        </ElTableColumn>

        <!-- @vue-generic {OperatorStaticIpExchange} -->
        <ElTableColumn :label="t('opsExchange.columns.time')" min-width="160">
          <template #default="{ row }">
            <span v-if="row.unknownAt" class="cell-primary">{{ t('opsExchange.labels.unknownAt') }}: {{ formatDate(row.unknownAt) }}</span>
            <span class="cell-meta">{{ t('opsExchange.labels.createdAt') }}: {{ formatDate(row.createdAt) }}</span>
          </template>
        </ElTableColumn>

        <!-- @vue-generic {OperatorStaticIpExchange} -->
        <ElTableColumn :label="t('opsExchange.columns.actions')" min-width="140" align="right">
          <template #default="{ row }">
            <div class="row-actions">
              <ElButton
                v-if="canResolveExchange(row)"
                link
                type="primary"
                :disabled="!canWrite"
                @click="openResolutionDialog(row, 'resolve')"
              >
                {{ t('opsExchange.actions.resolve') }}
              </ElButton>
              <ElButton
                v-else-if="canCorrectExchange(row)"
                link
                type="warning"
                :disabled="!canWrite"
                @click="openResolutionDialog(row, 'correct')"
              >
                {{ t('opsExchange.actions.correct') }}
              </ElButton>
              <ElButton link @click="selectTenantById(row.tenantId)">
                {{ t('opsExchange.queue.openTenant') }}
              </ElButton>
            </div>
          </template>
        </ElTableColumn>

        <template #empty>
          <span v-if="!queueLoading && !queueError" class="table-empty">{{ t('opsExchange.queue.empty') }}</span>
        </template>
      </ElTable>

      <div class="table-footer">
        <span>{{ t('opsExchange.common.totalShown', { count: queueItems.length }) }}</span>
        <ElButton
          v-if="queueNextCursor"
          link
          type="primary"
          :loading="queueLoadingMore"
          :disabled="queueLoading"
          @click="loadQueue({ cursor: queueNextCursor })"
        >
          {{ t('opsExchange.common.loadMore') }}
        </ElButton>
      </div>
    </ElCard>

    <!-- Selected tenant workspace -->
    <template v-if="selectedTenant">
      <ElAlert
        v-if="pendingIntents.length > 0"
        type="warning"
        :title="t('opsExchange.pendingBanner.title')"
        :closable="false"
        show-icon
      >
        <p class="pending-banner-desc">{{ t('opsExchange.pendingBanner.description') }}</p>
        <ul class="pending-intent-list">
          <li v-for="intent in pendingIntents" :key="`${intent.action}-${intent.requestKey}`" class="pending-intent-item">
            <span class="mono">{{ pendingIntentSummary(intent) }}</span>
            <ElButton
              size="small"
              type="warning"
              :disabled="!canWrite"
              @click="openPendingIntentDialog(intent)"
            >
              {{ t('opsExchange.actions.retryOriginal') }}
            </ElButton>
          </li>
        </ul>
      </ElAlert>

      <!-- Quota card -->
      <ElCard shadow="never" class="exchange-card">
        <div class="section-header">
          <div>
            <h2 class="section-title">{{ t('opsExchange.quota.title') }}</h2>
            <p class="section-note mono">{{ selectedTenant.name ? `${selectedTenant.name} · ${selectedTenant.id}` : selectedTenant.id }}</p>
          </div>
          <div class="section-controls">
            <ElButton size="small" :loading="quotaLoading" @click="loadQuota()">
              {{ t('opsExchange.common.refresh') }}
            </ElButton>
            <ElButton
              type="primary"
              size="small"
              :disabled="!canWrite || quotaLoading || Boolean(quotaError) || !quota"
              @click="openAdjustDialog"
            >
              {{ t('opsExchange.quota.adjustButton') }}
            </ElButton>
          </div>
        </div>

        <div class="section-body">
          <ElAlert
            v-if="quotaError"
            type="error"
            :title="errorMessage(quotaError)"
            :closable="false"
            show-icon
          >
            <ElButton size="small" @click="loadQuota()">{{ t('opsExchange.common.retry') }}</ElButton>
          </ElAlert>

          <dl v-else v-loading="quotaLoading" class="quota-grid">
            <div class="quota-stat">
              <dt>{{ t('opsExchange.quota.total') }}</dt>
              <dd class="tabular">{{ quota?.total ?? 0 }}</dd>
            </div>
            <div class="quota-stat">
              <dt>{{ t('opsExchange.quota.remaining') }}</dt>
              <dd class="tabular">{{ quota?.remaining ?? 0 }}</dd>
            </div>
            <div class="quota-stat">
              <dt>{{ t('opsExchange.quota.reserved') }}</dt>
              <dd class="tabular">{{ quota?.reserved ?? 0 }}</dd>
            </div>
            <div class="quota-stat">
              <dt>{{ t('opsExchange.quota.used') }}</dt>
              <dd class="tabular">{{ quota?.used ?? 0 }}</dd>
            </div>
          </dl>
        </div>
      </ElCard>

      <!-- Tenant exchange history -->
      <ElCard shadow="never" class="exchange-card">
        <div class="section-header">
          <div>
            <h2 class="section-title">{{ t('opsExchange.history.title') }}</h2>
          </div>
          <div class="section-controls">
            <ElButton size="small" :loading="historyLoading" @click="loadHistory()">
              {{ t('opsExchange.common.refresh') }}
            </ElButton>
          </div>
        </div>

        <ElAlert
          v-if="historyError"
          type="error"
          :title="errorMessage(historyError)"
          :closable="false"
          show-icon
          class="table-alert"
        >
          <ElButton size="small" @click="loadHistory()">{{ t('opsExchange.common.retry') }}</ElButton>
        </ElAlert>

        <ElTable
          v-loading="historyLoading"
          :data="historyItems"
          row-key="idempotencyKey"
          class="exchange-table"
          :header-cell-style="tableHeaderStyle"
          :cell-style="tableCellStyle"
        >
          <!-- @vue-generic {OperatorStaticIpExchange} -->
          <ElTableColumn :label="t('opsExchange.columns.status')" min-width="150">
            <template #default="{ row }">
              <div class="status-cell">
                <ElTag :type="exchangeStatusBadge(row).type" size="small" disable-transitions>
                  {{ exchangeStatusBadge(row).labelKey ? t(exchangeStatusBadge(row).labelKey) : exchangeStatusBadge(row).raw }}
                </ElTag>
                <span v-if="row.resolution" class="cell-meta">{{ resolutionLabel(row.resolution) }}</span>
              </div>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchange} -->
          <ElTableColumn :label="t('opsExchange.columns.requestAndAllocation')" min-width="210">
            <template #default="{ row }">
              <span class="cell-primary mono">{{ row.idempotencyKey }}</span>
              <span class="cell-meta mono">
                {{ t('opsExchange.labels.allocationId') }}: {{ row.allocationId || t('opsExchange.common.none') }}
                <template v-if="row.nodeId"> · {{ t('opsExchange.labels.nodeId') }}: {{ row.nodeId }}</template>
              </span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchange} -->
          <ElTableColumn :label="t('opsExchange.columns.ips')" min-width="160">
            <template #default="{ row }">
              <span class="cell-primary mono">{{ row.oldIp || t('opsExchange.common.none') }}</span>
              <span class="cell-meta mono">→ {{ row.newIp || t('opsExchange.common.none') }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchange} -->
          <ElTableColumn :label="t('opsExchange.columns.supplier')" min-width="160">
            <template #default="{ row }">
              <span class="cell-primary mono">{{ row.supplierOrderId || t('opsExchange.common.none') }}</span>
              <span v-if="row.supplierItemId" class="cell-meta mono">{{ t('opsExchange.labels.supplierItem') }}: {{ row.supplierItemId }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchange} -->
          <ElTableColumn :label="t('opsExchange.columns.time')" min-width="160">
            <template #default="{ row }">
              <span class="cell-primary">{{ formatDate(row.createdAt) }}</span>
              <span v-if="row.updatedAt" class="cell-meta">{{ t('opsExchange.labels.updatedAt') }}: {{ formatDate(row.updatedAt) }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchange} -->
          <ElTableColumn :label="t('opsExchange.columns.actions')" min-width="110" align="right">
            <template #default="{ row }">
              <ElButton
                v-if="canResolveExchange(row)"
                link
                type="primary"
                :disabled="!canWrite"
                @click="openResolutionDialog(row, 'resolve')"
              >
                {{ t('opsExchange.actions.resolve') }}
              </ElButton>
              <ElButton
                v-else-if="canCorrectExchange(row)"
                link
                type="warning"
                :disabled="!canWrite"
                @click="openResolutionDialog(row, 'correct')"
              >
                {{ t('opsExchange.actions.correct') }}
              </ElButton>
              <span v-else class="cell-meta">{{ t('opsExchange.common.none') }}</span>
            </template>
          </ElTableColumn>

          <template #empty>
            <span v-if="!historyLoading && !historyError" class="table-empty">{{ t('opsExchange.history.empty') }}</span>
          </template>
        </ElTable>

        <div class="table-footer">
          <span>{{ t('opsExchange.common.totalShown', { count: historyItems.length }) }}</span>
          <ElButton
            v-if="historyNextCursor"
            link
            type="primary"
            :loading="historyLoadingMore"
            :disabled="historyLoading"
            @click="loadHistory({ cursor: historyNextCursor })"
          >
            {{ t('opsExchange.common.loadMore') }}
          </ElButton>
        </div>
      </ElCard>

      <!-- Tenant audit log -->
      <ElCard shadow="never" class="exchange-card">
        <div class="section-header">
          <div>
            <h2 class="section-title">{{ t('opsExchange.audit.title') }}</h2>
          </div>
          <div class="section-controls">
            <ElButton size="small" :loading="auditLoading" @click="loadAudit()">
              {{ t('opsExchange.common.refresh') }}
            </ElButton>
          </div>
        </div>

        <ElAlert
          v-if="auditError"
          type="error"
          :title="errorMessage(auditError)"
          :closable="false"
          show-icon
          class="table-alert"
        >
          <ElButton size="small" @click="loadAudit()">{{ t('opsExchange.common.retry') }}</ElButton>
        </ElAlert>

        <ElTable
          v-loading="auditLoading"
          :data="auditEntries"
          row-key="id"
          class="exchange-table"
          :header-cell-style="tableHeaderStyle"
          :cell-style="tableCellStyle"
        >
          <!-- @vue-generic {OperatorStaticIpExchangeAuditEntry} -->
          <ElTableColumn :label="t('opsExchange.columns.auditKind')" min-width="130">
            <template #default="{ row }">
              <span class="cell-primary">{{ auditKindLabel(row.kind) }}</span>
              <span class="cell-meta">{{ formatDate(row.createdAt) }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchangeAuditEntry} -->
          <ElTableColumn :label="t('opsExchange.columns.actor')" min-width="120">
            <template #default="{ row }">
              <span class="cell-primary mono">{{ row.actorId || t('opsExchange.common.none') }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchangeAuditEntry} -->
          <ElTableColumn :label="t('opsExchange.columns.quotaChange')" min-width="190">
            <template #default="{ row }">
              <span class="cell-primary tabular">{{ t('opsExchange.labels.before') }}: {{ formatQuotaInline(row.before) }}</span>
              <span class="cell-meta tabular">{{ t('opsExchange.labels.after') }}: {{ formatQuotaInline(row.after) }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchangeAuditEntry} -->
          <ElTableColumn :label="t('opsExchange.columns.outcomeAndIp')" min-width="160">
            <template #default="{ row }">
              <span class="cell-primary">{{ row.outcome ? resolutionLabel(row.outcome) : t('opsExchange.common.none') }}</span>
              <span v-if="row.observedIp" class="cell-meta mono">{{ t('opsExchange.labels.observedIp') }}: {{ row.observedIp }}</span>
              <span v-if="row.supplierRecordId" class="cell-meta mono">{{ t('opsExchange.labels.supplierRecord') }}: {{ row.supplierRecordId }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchangeAuditEntry} -->
          <ElTableColumn :label="t('opsExchange.columns.evidence')" min-width="180">
            <template #default="{ row }">
              <span class="cell-evidence">{{ row.evidence || t('opsExchange.common.none') }}</span>
            </template>
          </ElTableColumn>

          <!-- @vue-generic {OperatorStaticIpExchangeAuditEntry} -->
          <ElTableColumn :label="t('opsExchange.columns.requestKey')" min-width="180">
            <template #default="{ row }">
              <span v-if="row.requestKey" class="cell-primary mono">{{ row.requestKey }}</span>
              <span v-if="row.idempotencyKey" class="cell-meta mono">{{ row.idempotencyKey }}</span>
              <span v-if="!row.requestKey && !row.idempotencyKey" class="cell-meta">{{ t('opsExchange.common.none') }}</span>
            </template>
          </ElTableColumn>

          <template #empty>
            <span v-if="!auditLoading && !auditError" class="table-empty">{{ t('opsExchange.audit.empty') }}</span>
          </template>
        </ElTable>

        <div class="table-footer">
          <span>{{ t('opsExchange.common.totalShown', { count: auditEntries.length }) }}</span>
          <ElButton
            v-if="auditNextBeforeId"
            link
            type="primary"
            :loading="auditLoadingMore"
            :disabled="auditLoading"
            @click="loadAudit({ beforeId: auditNextBeforeId })"
          >
            {{ t('opsExchange.common.loadMore') }}
          </ElButton>
        </div>
      </ElCard>
    </template>

    <ElCard v-else shadow="never" class="exchange-card">
      <div class="section-body">
        <ElEmpty :description="t('opsExchange.quota.selectTenantHint')" :image-size="56" />
      </div>
    </ElCard>

    <ExchangeOperationDialog
      v-if="dialogTenantId"
      v-model="dialogOpen"
      :mode="dialogMode"
      :tenant-id="dialogTenantId"
      :tenant-name="dialogTenantName"
      :quota="selectedTenant?.id === dialogTenantId ? quota : undefined"
      :exchange="dialogExchange"
      :initial-intent="dialogInitialIntent"
      @completed="handleMutationCompleted"
      @intents-changed="refreshPendingIntents"
    />
  </div>
</template>

<style scoped>
.ops-exchange {
  display: flex;
  flex-direction: column;
  gap: var(--space-4);
  min-width: 0;
  max-width: 100%;
}
.exchange-card {
  --el-card-padding: 0;
  border-radius: var(--ops-card-radius);
  min-width: 0;
  max-width: 100%;
  overflow: hidden;
}
.section-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
}
.section-title {
  margin: 0;
  font-size: var(--font-size-lg);
  line-height: var(--space-5);
  font-weight: var(--font-weight-bold);
}
.section-note {
  margin: var(--ops-detail-meta-gap) 0 0;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  line-height: normal;
  overflow-wrap: anywhere;
}
.section-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}
.section-body {
  padding: var(--space-4);
}
.search-row {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  max-width: var(--content-narrow-width);
}
.search-row :deep(.el-input) {
  flex: 1 1 220px;
  min-width: 0;
}
.search-feedback {
  margin-block-start: var(--space-3);
}
.selected-tenant {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-block-start: var(--space-3);
  padding: var(--space-3) var(--space-4);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface-muted);
}
.selected-tenant__info {
  display: flex;
  flex-direction: column;
  gap: var(--ops-detail-meta-gap);
  min-width: 0;
  overflow-wrap: anywhere;
}
.selected-tenant__label {
  font-size: var(--font-size-xs);
  color: var(--color-text-secondary);
}
.selected-tenant__name {
  font-size: var(--font-size-md);
}
.selected-tenant__meta {
  font-size: var(--font-size-xs);
  color: var(--color-text-secondary);
}
.quota-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: var(--space-3);
  margin: 0;
}
.quota-stat {
  padding: var(--space-3);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface-muted);
  min-width: 0;
}
.quota-stat dt {
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}
.quota-stat dd {
  margin: var(--space-1) 0 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
}
.table-alert {
  margin: var(--space-3) var(--space-4);
  width: auto;
}
.exchange-table {
  width: 100%;
  font-size: var(--font-size-sm);
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
}
.status-cell {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-1);
}
.cell-primary {
  display: block;
  font-size: var(--font-size-xs);
  overflow-wrap: anywhere;
}
.cell-meta {
  display: block;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  overflow-wrap: anywhere;
}
.cell-evidence {
  display: block;
  font-size: var(--font-size-xs);
  overflow-wrap: anywhere;
  white-space: pre-wrap;
}
.tenant-jump {
  padding: 0;
  font-size: var(--font-size-xs);
  max-width: 100%;
  overflow-wrap: anywhere;
  white-space: normal;
  text-align: start;
}
.row-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-2);
}
.table-empty {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
}
.table-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--ops-search-result-padding) var(--space-4);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}
.pending-banner-desc {
  margin: 0 0 var(--space-2);
}
.pending-intent-list {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}
.pending-intent-item {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  overflow-wrap: anywhere;
}
</style>
