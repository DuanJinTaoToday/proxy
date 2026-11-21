<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElButton, ElDatePicker, ElEmpty, ElInput, ElPagination, ElRadioButton, ElRadioGroup, ElTable, ElTableColumn } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import OrderDrawer from './OrderDrawer.vue'
import { formatExpiry, formatMoney } from './demo'
import { allEntries, tenantById } from './ops'
import { ledgerKindLabels, type LedgerKind } from './state'

// 运营流水视图比客户多显示操作人和充值原因。在租户详情里传入 tenantId，只看该租户。
const props = defineProps<{ tenantId?: string }>()
type Row = { row: any }
const pageSize = 10

const kind = ref<LedgerKind | 'all'>('all')
const keyword = ref('')
const range = ref<[Date, Date] | null>(null)
const pageNo = ref(1)

const filtered = computed(() => {
  const [from, to] = range.value ?? []
  const term = keyword.value.trim().toLowerCase()
  return allEntries.value.filter(e => (!props.tenantId || e.tenantId === props.tenantId)
    && (kind.value === 'all' || e.kind === kind.value)
    && (!term || e.id.toLowerCase().includes(term) || tenantById(e.tenantId).account.includes(term))
    && (!from || e.at >= from.getTime())
    && (!to || e.at < to.getTime() + 86_400_000))
})
const pageRows = computed(() => filtered.value.slice((pageNo.value - 1) * pageSize, pageNo.value * pageSize))
watch([kind, keyword, range], () => { pageNo.value = 1 })

const drawerOpen = ref(false)
const current = ref<{ orderId?: string, tenantId?: string }>({})
function openOrder(orderId: string, tenantId: string) {
  current.value = { orderId, tenantId }
  drawerOpen.value = true
}

function resetFilters() {
  kind.value = 'all'
  keyword.value = ''
  range.value = null
}
</script>

<template>
  <div v-if="!tenantId" class="page-head"><h1>钱包流水</h1></div>

  <section class="table-card" aria-label="钱包流水">
    <div class="table-toolbar">
      <div class="table-filters">
        <ElInput v-if="!tenantId" v-model="keyword" class="filter-search" :prefix-icon="Search" clearable placeholder="搜索流水号或主账号" />
        <ElRadioGroup v-model="kind" aria-label="按类型筛选">
          <ElRadioButton value="all">全部</ElRadioButton>
          <ElRadioButton v-for="(label, value) in ledgerKindLabels" :key="value" :value="value">{{ label }}</ElRadioButton>
        </ElRadioGroup>
        <ElDatePicker v-model="range" type="daterange" class="filter-dates" range-separator="至"
          start-placeholder="开始日期" end-placeholder="结束日期" aria-label="按日期筛选" />
      </div>
    </div>

    <ElTable :data="pageRows" row-key="id" class="ip-table">
      <ElTableColumn label="流水号 / 时间" min-width="170">
        <template #default="{ row }: Row">
          <span class="mono order-id">{{ row.id }}</span>
          <span class="remaining">{{ formatExpiry(row.at) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn v-if="!tenantId" label="租户" min-width="140">
        <template #default="{ row }: Row">
          <a class="table-link" :href="`#tenants/${row.tenantId}`">{{ tenantById(row.tenantId).name }}</a>
          <span class="remaining mono">{{ tenantById(row.tenantId).account }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="类型" width="80">
        <template #default="{ row }: Row"><span class="status-badge" :class="`ledger-${row.kind}`">{{ ledgerKindLabels[row.kind as LedgerKind] }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="说明" min-width="220">
        <template #default="{ row }: Row">
          <span class="ledger-note">{{ row.reason ?? row.note }}</span>
          <ElButton v-if="row.orderId" link type="primary" class="mono" @click="openOrder(row.orderId, row.tenantId)">{{ row.orderId }}</ElButton>
        </template>
      </ElTableColumn>
      <ElTableColumn label="操作人" width="90">
        <template #default="{ row }: Row">{{ row.operator ?? '—' }}</template>
      </ElTableColumn>
      <ElTableColumn label="变动金额" min-width="110" align="right">
        <template #default="{ row }: Row">
          <span class="ledger-amount" :class="row.amountCents > 0 ? 'amount-in' : 'amount-out'">{{ row.amountCents > 0 ? '+' : '−' }}{{ formatMoney(Math.abs(row.amountCents)) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="变动前 → 后" min-width="170" align="right">
        <template #default="{ row }: Row"><span class="ledger-amount">{{ formatMoney(row.balanceAfterCents - row.amountCents) }} → {{ formatMoney(row.balanceAfterCents) }}</span></template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty description="没有符合条件的流水" :image-size="72">
          <ElButton @click="resetFilters">清除筛选</ElButton>
        </ElEmpty>
      </template>
    </ElTable>
    <div class="table-footer table-footer-pager">
      <span>共 {{ filtered.length }} 条</span>
      <ElPagination v-if="filtered.length > pageSize" v-model:current-page="pageNo" :page-size="pageSize"
        :total="filtered.length" layout="prev, pager, next" small background />
    </div>
  </section>

  <OrderDrawer v-model="drawerOpen" :order-id="current.orderId" :tenant-id="current.tenantId" />
</template>
