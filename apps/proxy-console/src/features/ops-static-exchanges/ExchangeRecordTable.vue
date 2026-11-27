<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElButton, ElLoading, ElTable, ElTableColumn, ElTag } from 'element-plus'
import type { OperatorStaticIpExchange } from '@/api/opsStaticExchanges'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import {
  canCorrectExchange,
  canResolveExchange,
  exchangeStatusBadge,
} from './useOpsStaticExchanges'

const vLoading = ElLoading.directive

defineProps<{
  records: OperatorStaticIpExchange[]
  loading: boolean
  hasError: boolean
  emptyText: string
  showTenant?: boolean
  showSupplier?: boolean
  canWrite: boolean
}>()

const emit = defineEmits<{
  openTenant: [tenantId: string]
  resolve: [row: OperatorStaticIpExchange]
  correct: [row: OperatorStaticIpExchange]
}>()

const { t, locale } = useI18n()

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
  return t('opsExchange.status.other')
}

function rowKey(row: OperatorStaticIpExchange) {
  return `${row.tenantId}:${row.idempotencyKey}`
}
</script>

<template>
  <ElTable
    v-loading="loading"
    :data="records"
    :row-key="rowKey"
    class="exchange-table"
    :header-cell-style="tableHeaderStyle"
    :cell-style="tableCellStyle"
  >
    <!-- @vue-generic {OperatorStaticIpExchange} -->
    <ElTableColumn :label="t('opsExchange.columns.status')" min-width="150">
      <template #default="{ row }">
        <div class="status-cell">
          <ElTag :type="exchangeStatusBadge(row).type" size="small" disable-transitions>
            {{ t(exchangeStatusBadge(row).labelKey) }}
          </ElTag>
          <span v-if="row.resolution" class="cell-meta">{{ resolutionLabel(row.resolution) }}</span>
        </div>
      </template>
    </ElTableColumn>

    <!-- @vue-generic {OperatorStaticIpExchange} -->
    <ElTableColumn v-if="showTenant" :label="t('opsExchange.columns.tenant')" min-width="140">
      <template #default="{ row }">
        <ElButton link type="primary" class="tenant-jump" @click="emit('openTenant', row.tenantId)">
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
    <ElTableColumn v-if="showSupplier" :label="t('opsExchange.columns.supplier')" min-width="160">
      <template #default="{ row }">
        <span class="cell-primary mono">{{ row.supplierOrderId || t('opsExchange.common.none') }}</span>
        <span v-if="row.supplierItemId" class="cell-meta mono">{{ t('opsExchange.labels.supplierItem') }}: {{ row.supplierItemId }}</span>
      </template>
    </ElTableColumn>

    <!-- @vue-generic {OperatorStaticIpExchange} -->
    <ElTableColumn :label="t('opsExchange.columns.time')" min-width="160">
      <template #default="{ row }">
        <template v-if="showTenant">
          <span v-if="row.unknownAt" class="cell-primary">{{ t('opsExchange.labels.unknownAt') }}: {{ formatDate(row.unknownAt) }}</span>
          <span class="cell-meta">{{ t('opsExchange.labels.createdAt') }}: {{ formatDate(row.createdAt) }}</span>
        </template>
        <template v-else>
          <span class="cell-primary">{{ formatDate(row.createdAt) }}</span>
          <span v-if="row.updatedAt" class="cell-meta">{{ t('opsExchange.labels.updatedAt') }}: {{ formatDate(row.updatedAt) }}</span>
        </template>
      </template>
    </ElTableColumn>

    <!-- @vue-generic {OperatorStaticIpExchange} -->
    <ElTableColumn :label="t('opsExchange.columns.actions')" :min-width="showTenant ? 140 : 110" align="right">
      <template #default="{ row }">
        <div class="row-actions">
          <ElButton
            v-if="canResolveExchange(row)"
            link
            type="primary"
            :disabled="!canWrite"
            @click="emit('resolve', row)"
          >
            {{ t('opsExchange.actions.resolve') }}
          </ElButton>
          <ElButton
            v-else-if="canCorrectExchange(row)"
            link
            type="warning"
            :disabled="!canWrite"
            @click="emit('correct', row)"
          >
            {{ t('opsExchange.actions.correct') }}
          </ElButton>
          <span v-else-if="!showTenant" class="cell-meta">{{ t('opsExchange.common.none') }}</span>
          <ElButton v-if="showTenant" link @click="emit('openTenant', row.tenantId)">
            {{ t('opsExchange.queue.openTenant') }}
          </ElButton>
        </div>
      </template>
    </ElTableColumn>

    <template #empty>
      <span v-if="!loading && !hasError" class="table-empty">{{ emptyText }}</span>
    </template>
  </ElTable>
</template>

<style scoped>
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
</style>
