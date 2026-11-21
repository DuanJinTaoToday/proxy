<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElButton, ElTable, ElTableColumn, ElTag, ElTooltip } from 'element-plus'
import { ArrowRight, CircleCheck, Clock, Tickets, User, Wallet, Warning } from '@element-plus/icons-vue'
import OpsResolveDialog from './OpsResolveDialog.vue'
import { demoNow, formatExpiry, formatMoney } from './demo'
import { allEntries, billingOnly, canBill, hoursSince, needsAction, openUsageIssues, pendingOrders, pendingStageLabels, stageOf, tenantById, tenants } from './ops'
import { orderKindLabels, orderTotalCents, type Order } from './state'

type Row = { row: any }
const dayKey = (t: number) => new Date(t + 8 * 3600_000).toISOString().slice(0, 10)
const recharges = computed(() => allEntries.value.filter(e => e.kind === 'recharge'))
const todayRecharge = computed(() => recharges.value.filter(e => dayKey(e.at) === dayKey(demoNow)).reduce((s, e) => s + e.amountCents, 0))
const newTenants = tenants.filter(t => demoNow - t.registeredAt <= 7 * 86_400_000).length
const stageTag = { waiting: 'info', review: 'warning', closable: 'danger' } as const

const resolveOpen = ref(false)
const target = ref<{ tenantId?: string, orderId?: string }>({})
function resolve(row: Order & { tenantId: string }) {
  target.value = { tenantId: row.tenantId, orderId: row.id }
  resolveOpen.value = true
}
const progress = (o: Order) => `${o.lines.reduce((s, l) => s + l.delivered, 0)} / ${o.lines.reduce((s, l) => s + l.quantity, 0)}`
</script>

<template>
  <div class="page-head"><h1>工作台</h1></div>

  <div class="stat-grid ops-stats">
    <a class="stat-card" href="#workbench">
      <header><span>待运营核实的订单</span><span class="stat-badge" :class="{ warn: needsAction.length }"><Clock />超过 24 小时</span></header>
      <p class="stat-value">{{ needsAction.length }} <small>笔</small></p>
      <footer><span>另有 {{ pendingOrders.length - needsAction.length }} 笔仍在等待供应商确认</span></footer>
    </a>
    <a class="stat-card" href="#ledger">
      <header><span>今日充值</span><span class="stat-badge"><Wallet />北京时间</span></header>
      <p class="stat-value">{{ formatMoney(todayRecharge) }}</p>
      <footer><span>累计 {{ recharges.length }} 笔充值</span></footer>
    </a>
    <a class="stat-card" href="#tenants">
      <header><span>租户</span><span class="stat-badge"><User />近 7 天新增 {{ newTenants }}</span></header>
      <p class="stat-value">{{ tenants.length }} <small>个</small></p>
      <footer><span>按主账号名或手机号查找</span></footer>
    </a>
    <a class="stat-card" href="#usage">
      <header><span>未解决的用量问题</span><span class="stat-badge" :class="{ warn: openUsageIssues }"><Warning />异常与缺口</span></header>
      <p class="stat-value">{{ openUsageIssues }} <small>项</small></p>
      <footer><span>可能影响客户扣量</span></footer>
    </a>
  </div>

  <section class="table-card" aria-labelledby="pending-title">
    <div class="table-toolbar">
      <div>
        <h2 id="pending-title" class="card-title">确认中的静态订单</h2>
        <p class="card-desc">下单满 24 小时转运营核实，满 72 小时可关闭剩余并退款。</p>
      </div>
    </div>
    <ElTable :data="pendingOrders" row-key="id" class="ip-table">
      <ElTableColumn label="阶段" width="130">
        <template #default="{ row }: Row"><ElTag :type="stageTag[stageOf(row)]" size="small" disable-transitions>{{ pendingStageLabels[stageOf(row)] }}</ElTag></template>
      </ElTableColumn>
      <ElTableColumn label="订单" min-width="200">
        <template #default="{ row }: Row">
          <span class="mono order-id">{{ row.id }}</span>
          <span class="remaining">{{ orderKindLabels[row.kind as Order['kind']] }} · {{ formatExpiry(row.createdAt) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="租户" min-width="150">
        <template #default="{ row }: Row">
          <a class="table-link" :href="`#tenants/${row.tenantId}`">{{ tenantById(row.tenantId).name }}</a>
          <span class="remaining mono">{{ tenantById(row.tenantId).account }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="已等待" width="100" align="right">
        <template #default="{ row }: Row"><span class="ledger-amount">{{ hoursSince(row.createdAt) }} 小时</span></template>
      </ElTableColumn>
      <ElTableColumn label="已交付 / 申请" width="120" align="right">
        <template #default="{ row }: Row">{{ progress(row) }}</template>
      </ElTableColumn>
      <ElTableColumn label="预扣金额" width="110" align="right">
        <template #default="{ row }: Row"><span class="ledger-amount">{{ formatMoney(orderTotalCents(row)) }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="" width="90" align="right">
        <template #default="{ row }: Row">
          <ElTooltip :disabled="canBill" :content="billingOnly" placement="left">
            <span><ElButton link type="primary" :disabled="!canBill" @click="resolve(row)">核实</ElButton></span>
          </ElTooltip>
        </template>
      </ElTableColumn>
      <template #empty>
        <div class="todo-empty"><CircleCheck /><span>没有确认中的订单</span></div>
      </template>
    </ElTable>
    <div class="table-footer">
      <span>共 {{ pendingOrders.length }} 笔</span>
      <a href="#orders" class="reminder-link"><Tickets />查看全部订单<ArrowRight /></a>
    </div>
  </section>

  <OpsResolveDialog v-model="resolveOpen" :tenant-id="target.tenantId" :order-id="target.orderId" />
</template>
