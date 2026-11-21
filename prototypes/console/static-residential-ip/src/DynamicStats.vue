<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElButton, ElDatePicker, ElEmpty, ElMessage, ElOption, ElRadioButton, ElRadioGroup, ElSelect, ElTable, ElTableColumn } from 'element-plus'
import { Download } from '@element-plus/icons-vue'
import UsageChart from './UsageChart.vue'
import { formatExpiry } from './demo'
import { dailyTotals, subAccounts, usageDays, usageUpdatedAt } from './state'

// ElTable 的插槽行类型是 DefaultRow，模板里按 { day, gb } 读取字段。
type DayRow = { row: any }
type Preset = '7' | '30' | 'custom'

const preset = ref<Preset>('7')
const range = ref<[Date, Date] | null>(null)
const account = ref('all')
const accountFilter = computed(() => account.value === 'all' ? undefined : account.value)
const firstDay = usageDays[0]
const lastDay = usageDays.at(-1)!
const dayKey = (d: Date) => new Date(d.getTime() - d.getTimezoneOffset() * 60_000).toISOString().slice(0, 10)

const days = computed(() => {
  if (preset.value !== 'custom' || !range.value) return usageDays.slice(preset.value === '30' ? 0 : -7)
  const [from, to] = range.value.map(dayKey)
  return usageDays.filter(d => d >= from && d <= to)
})
const rows = computed(() => dailyTotals(days.value, accountFilter.value))
const total = computed(() => rows.value.reduce((s, r) => s + r.gb, 0))
const peak = computed(() => rows.value.reduce((m, r) => r.gb > m.gb ? r : m, rows.value[0] ?? { day: '', gb: 0 }))
const accountLabel = computed(() => accountFilter.value ?? '全部子账号')
const tableRows = computed(() => [...rows.value].reverse())

const gb = (value: number) => `${value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} GB`
const bytes = (value: number) => Math.round(value * 1e9).toLocaleString('en-US')
const disabledDate = (d: Date) => dayKey(d) < firstDay || dayKey(d) > lastDay

function onRange(value: [Date, Date] | null) {
  range.value = value
  preset.value = value ? 'custom' : '7'
}

function exportCsv() {
  const header = '业务日,子账号,扣减流量(GB),扣减字节'
  const body = rows.value.map(r => `${r.day},${accountFilter.value ?? '全部'},${r.gb.toFixed(2)},${Math.round(r.gb * 1e9)}`)
  const blob = new Blob(['﻿' + [header, ...body].join('\r\n') + '\r\n'], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `dynamic-usage-${days.value[0]}-${days.value.at(-1)}${accountFilter.value ? `-${accountFilter.value}` : ''}.csv`
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  ElMessage.success(`已导出 ${rows.value.length} 天的用量`)
}
</script>

<template>
  <section class="details-card stats-card">
    <div class="table-toolbar stats-toolbar">
      <div class="table-filters">
        <ElRadioGroup :model-value="preset" aria-label="时间范围" @update:model-value="preset = $event as Preset; range = null">
          <ElRadioButton value="7">近 7 天</ElRadioButton>
          <ElRadioButton value="30">近 30 天</ElRadioButton>
        </ElRadioGroup>
        <ElDatePicker :model-value="range" type="daterange" class="filter-dates" range-separator="至"
          start-placeholder="开始日期" end-placeholder="结束日期" :disabled-date="disabledDate" aria-label="自定义日期"
          @update:model-value="onRange($event as [Date, Date] | null)" />
        <ElSelect v-model="account" class="filter-product" aria-label="按子账号筛选">
          <ElOption value="all" label="全部子账号" />
          <ElOption v-for="a in subAccounts" :key="a.name" :value="a.name" :label="a.active ? a.name : `${a.name}（已禁用）`" />
        </ElSelect>
      </div>
      <ElButton :icon="Download" :disabled="!rows.length" @click="exportCsv">导出 CSV</ElButton>
    </div>

    <template v-if="rows.length">
      <div class="stats-summary">
        <div><span>{{ accountLabel }} · 合计扣减</span><strong>{{ gb(total) }}</strong></div>
        <div><span>日均</span><strong>{{ gb(total / rows.length) }}</strong></div>
        <div><span>最高一天</span><strong>{{ gb(peak.gb) }}</strong><small>{{ peak.day.replaceAll('-', '/') }}</small></div>
      </div>
      <UsageChart :data="rows" />
      <ElTable :data="tableRows" class="ip-table stats-table" max-height="360">
        <ElTableColumn label="业务日" min-width="140">
          <template #default="{ row }: DayRow">{{ row.day.replaceAll('-', '/') }}</template>
        </ElTableColumn>
        <ElTableColumn label="扣减流量" min-width="140" align="right">
          <template #default="{ row }: DayRow"><span class="ledger-amount">{{ gb(row.gb) }}</span></template>
        </ElTableColumn>
        <ElTableColumn label="扣减字节" min-width="180" align="right">
          <template #default="{ row }: DayRow"><span class="mono-value">{{ bytes(row.gb) }}</span></template>
        </ElTableColumn>
      </ElTable>
    </template>
    <ElEmpty v-else description="所选范围内没有数据" :image-size="72" />

    <p class="usage-foot">数据更新于 {{ formatExpiry(usageUpdatedAt) }} · 按天展示实际从流量余额扣减的流量 · 业务日按北京时间 · 1 GB = 10⁹ 字节</p>
  </section>
</template>
