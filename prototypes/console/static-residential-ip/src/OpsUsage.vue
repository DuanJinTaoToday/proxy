<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElOption, ElRadioButton, ElRadioGroup, ElSelect, ElTabPane, ElTabs, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { formatExpiry } from './demo'
import { anomalies, tenantById, usageGaps } from './ops'

// 只读排查视图，字段对应 ListRecordAnomalies / ListUsageGaps。处理动作由网关回放和工程排查完成，不在后台操作。
type Row = { row: any }
const tab = ref('gaps')
const state = ref<'open' | 'resolved' | 'all'>('open')
const gapStatus = ref<'all' | 'pending_replay' | 'possible_gap'>('all')
const gapStatusLabels = { pending_replay: '待回放', possible_gap: '可能缺失' }
const gapKindLabels = { boot: '运行代次', session: '会话' }

const matchState = (resolvedAt?: number) => state.value === 'all' || (state.value === 'open' ? !resolvedAt : Boolean(resolvedAt))
const gapRows = computed(() => usageGaps.filter(g => matchState(g.resolvedAt) && (gapStatus.value === 'all' || g.status === gapStatus.value)))
const anomalyRows = computed(() => anomalies.filter(a => matchState(a.resolvedAt)))
const openGaps = usageGaps.filter(g => !g.resolvedAt).length
const openAnomalies = anomalies.filter(a => !a.resolvedAt).length
</script>

<template>
  <div class="page-head"><h1>用量异常</h1></div>

  <ElTabs v-model="tab" class="page-tabs">
    <ElTabPane name="gaps">
      <template #label>用量缺口<span v-if="openGaps" class="tab-count">{{ openGaps }}</span></template>
    </ElTabPane>
    <ElTabPane name="anomalies">
      <template #label>记录异常<span v-if="openAnomalies" class="tab-count">{{ openAnomalies }}</span></template>
    </ElTabPane>
  </ElTabs>

  <section class="table-card">
    <div class="table-toolbar">
      <div class="table-filters">
        <ElRadioGroup v-model="state" aria-label="按处理状态筛选">
          <ElRadioButton value="open">未解决</ElRadioButton>
          <ElRadioButton value="resolved">已解决</ElRadioButton>
          <ElRadioButton value="all">全部</ElRadioButton>
        </ElRadioGroup>
        <ElSelect v-if="tab === 'gaps'" v-model="gapStatus" class="filter-status" aria-label="按缺口状态筛选">
          <ElOption value="all" label="全部状态" />
          <ElOption v-for="(label, value) in gapStatusLabels" :key="value" :value="value" :label="label" />
        </ElSelect>
      </div>
      <span class="selection">{{ tab === 'gaps' ? '网关退出或会话中断后，未回放的用量可能少扣' : 'Recorder 文件中无法解析的记录' }}</span>
    </div>

    <ElTable v-if="tab === 'gaps'" :data="gapRows" class="ip-table">
      <ElTableColumn label="状态" width="110">
        <template #default="{ row }: Row">
          <ElTag v-if="row.resolvedAt" type="success" size="small" disable-transitions>已解决</ElTag>
          <ElTag v-else :type="row.status === 'possible_gap' ? 'danger' : 'warning'" size="small" disable-transitions>{{ gapStatusLabels[row.status as keyof typeof gapStatusLabels] }}</ElTag>
        </template>
      </ElTableColumn>
      <ElTableColumn label="类型" width="100">
        <template #default="{ row }: Row">{{ gapKindLabels[row.kind as keyof typeof gapKindLabels] }}</template>
      </ElTableColumn>
      <ElTableColumn label="租户" min-width="140">
        <template #default="{ row }: Row"><a class="table-link" :href="`#tenants/${row.tenantId}`">{{ tenantById(row.tenantId).name }}</a></template>
      </ElTableColumn>
      <ElTableColumn label="网关 / 运行代次" min-width="180">
        <template #default="{ row }: Row"><span class="mono">{{ row.gateway }}</span><span class="remaining mono">{{ row.boot }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="会话" min-width="120">
        <template #default="{ row }: Row"><span class="mono">{{ row.session ?? '—' }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="可能缺失起点" min-width="150">
        <template #default="{ row }: Row">{{ formatExpiry(row.fromAt) }}</template>
      </ElTableColumn>
      <ElTableColumn label="发现 / 解决" min-width="160">
        <template #default="{ row }: Row">{{ formatExpiry(row.detectedAt) }}<span class="remaining">{{ row.resolvedAt ? `解决于 ${formatExpiry(row.resolvedAt)}` : '未解决' }}</span></template>
      </ElTableColumn>
    </ElTable>

    <ElTable v-else :data="anomalyRows" class="ip-table">
      <ElTableColumn label="状态" width="90">
        <template #default="{ row }: Row"><ElTag :type="row.resolvedAt ? 'success' : 'danger'" size="small" disable-transitions>{{ row.resolvedAt ? '已解决' : '未解决' }}</ElTag></template>
      </ElTableColumn>
      <ElTableColumn label="异常类型" min-width="150">
        <template #default="{ row }: Row"><span class="mono">{{ row.kind }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="租户" min-width="140">
        <template #default="{ row }: Row"><a class="table-link" :href="`#tenants/${row.tenantId}`">{{ tenantById(row.tenantId).name }}</a></template>
      </ElTableColumn>
      <ElTableColumn label="网关 / 运行代次" min-width="180">
        <template #default="{ row }: Row"><span class="mono">{{ row.gateway }}</span><span class="remaining mono">{{ row.boot }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="文件 / 偏移" min-width="160">
        <template #default="{ row }: Row"><span class="mono">{{ row.file }}</span><span class="remaining mono">@ {{ row.offset.toLocaleString('en-US') }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="发现 / 解决" min-width="160">
        <template #default="{ row }: Row">{{ formatExpiry(row.detectedAt) }}<span class="remaining">{{ row.resolvedAt ? `解决于 ${formatExpiry(row.resolvedAt)}` : '未解决' }}</span></template>
      </ElTableColumn>
    </ElTable>
    <div class="table-footer"><span>只读视图 · 处理由网关回放和工程排查完成</span></div>
  </section>
</template>
