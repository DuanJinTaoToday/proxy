<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElButton, ElDatePicker, ElEmpty, ElPagination, ElPopover, ElRadioButton, ElRadioGroup, ElTable, ElTableColumn } from 'element-plus'
import { Wallet, Top, Bottom, Service } from '@element-plus/icons-vue'
import OrderDrawer from './OrderDrawer.vue'
import { formatExpiry, formatMoney } from './demo'
import { balanceCents, ledger, ledgerKindLabels, type LedgerKind } from './state'

// ElTable 的插槽行类型是 DefaultRow，模板里按 LedgerEntry 读取字段。
type EntryRow = { row: any }
const pageSize = 8
const lowBalanceCents = 5000

const kind = ref<LedgerKind | 'all'>('all')
const range = ref<[Date, Date] | null>(null)
const pageNo = ref(1)
const drawerOpen = ref(false)
const drawerOrderId = ref<string>()

const totalRecharge = computed(() => ledger.value.filter(e => e.kind === 'recharge').reduce((s, e) => s + e.amountCents, 0))
const totalSpend = computed(() => -ledger.value.filter(e => e.kind !== 'recharge').reduce((s, e) => s + e.amountCents, 0))

const filtered = computed(() => {
  const [from, to] = range.value ?? []
  return ledger.value
    .filter(e => (kind.value === 'all' || e.kind === kind.value)
      && (!from || e.at >= from.getTime())
      && (!to || e.at < to.getTime() + 86_400_000))
    .reverse()
})
const pageRows = computed(() => filtered.value.slice((pageNo.value - 1) * pageSize, pageNo.value * pageSize))

watch([kind, range], () => { pageNo.value = 1 })

function openOrder(id: string) {
  drawerOrderId.value = id
  drawerOpen.value = true
}

function resetFilters() {
  kind.value = 'all'
  range.value = null
}
</script>

<template>
  <div class="page-head"><h1>钱包</h1></div>

  <div class="wallet-overview">
    <section class="balance-card">
      <div class="balance-label"><Wallet />可用余额</div>
      <p class="balance-value">{{ formatMoney(balanceCents) }}</p>
      <p class="balance-hint" :class="{ low: balanceCents < lowBalanceCents }">
        {{ balanceCents < lowBalanceCents ? '余额偏低，可能无法完成下次购买或续费。' : '购买流量包、静态 IP 和续费都从余额扣款。' }}
      </p>
      <ElPopover placement="bottom-start" trigger="click" :width="280">
        <template #reference><ElButton type="primary" :icon="Service">联系运营充值</ElButton></template>
        <strong>如何充值</strong>
        <p class="notification-copy">首版暂不支持在线支付。请联系客户经理转账，运营确认到账后为钱包充值，充值记录会出现在下方流水中。</p>
        <p class="notification-copy">客户经理：王经理 · 示例微信 proxy-sales</p>
      </ElPopover>
    </section>
    <section class="metric">
      <div><h2>累计充值</h2><p class="metric-value">{{ formatMoney(totalRecharge) }}</p><p class="metric-hint">运营确认到账的全部金额</p></div>
      <span class="metric-icon available"><Top /></span>
    </section>
    <section class="metric">
      <div><h2>累计支出</h2><p class="metric-value">{{ formatMoney(totalSpend) }}</p><p class="metric-hint">全部扣款减去退款</p></div>
      <span class="metric-icon expiring"><Bottom /></span>
    </section>
  </div>

  <section class="table-card" aria-label="钱包流水">
    <div class="table-toolbar">
      <div class="table-filters">
        <ElRadioGroup v-model="kind" aria-label="按类型筛选">
          <ElRadioButton value="all">全部</ElRadioButton>
          <ElRadioButton v-for="(label, value) in ledgerKindLabels" :key="value" :value="value">{{ label }}</ElRadioButton>
        </ElRadioGroup>
        <ElDatePicker v-model="range" type="daterange" class="filter-dates" range-separator="至"
          start-placeholder="开始日期" end-placeholder="结束日期" aria-label="按日期筛选" />
      </div>
      <span class="selection">业务日按北京时间</span>
    </div>

    <ElTable :data="pageRows" row-key="id" class="ip-table">
      <ElTableColumn label="时间" min-width="150">
        <template #default="{ row }: EntryRow">{{ formatExpiry(row.at) }}</template>
      </ElTableColumn>
      <ElTableColumn label="类型" width="90">
        <template #default="{ row }: EntryRow"><span class="status-badge" :class="`ledger-${row.kind}`">{{ ledgerKindLabels[row.kind as LedgerKind] }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="说明" min-width="240">
        <template #default="{ row }: EntryRow">
          <span class="ledger-note">{{ row.note }}</span>
          <ElButton v-if="row.orderId" link type="primary" class="mono" @click="openOrder(row.orderId)">{{ row.orderId }}</ElButton>
        </template>
      </ElTableColumn>
      <ElTableColumn label="变动金额" min-width="120" align="right">
        <template #default="{ row }: EntryRow">
          <span class="ledger-amount" :class="row.amountCents > 0 ? 'amount-in' : 'amount-out'">{{ row.amountCents > 0 ? '+' : '−' }}{{ formatMoney(Math.abs(row.amountCents)) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="变动后余额" min-width="120" align="right">
        <template #default="{ row }: EntryRow"><span class="ledger-amount">{{ formatMoney(row.balanceAfterCents) }}</span></template>
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

  <OrderDrawer v-model="drawerOpen" :order-id="drawerOrderId" />
</template>
