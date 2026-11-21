<script setup lang="ts">
import { computed } from 'vue'
import { ElAlert, ElDescriptions, ElDescriptionsItem, ElDrawer, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { formatExpiry, formatMoney } from './demo'
import { books, ledgerKindLabels, liveTenantId, orderKindLabels, orderRefundCents, orderStatusLabels, orderStatusTags, orderTotalCents } from './state'

// 客户控制台只看自己的订单；运营后台传入 tenantId 查看任一租户，并通过插槽追加核实记录和操作。
const props = defineProps<{ orderId?: string, tenantId?: string }>()
const open = defineModel<boolean>({ required: true })

const book = computed(() => books.value[props.tenantId ?? liveTenantId])
const order = computed(() => book.value?.orders.find(o => o.id === props.orderId))
const entries = computed(() => book.value?.ledger.filter(entry => entry.orderId === props.orderId) ?? [])
const total = computed(() => order.value ? orderTotalCents(order.value) : 0)
const refund = computed(() => order.value ? orderRefundCents(order.value) : 0)
// ElTable 的插槽行类型是 DefaultRow，模板里按 OrderLine 读取字段。
type LineRow = { row: any }
</script>

<template>
  <ElDrawer v-model="open" title="订单详情" size="min(560px, 100vw)">
    <template v-if="order">
      <ElAlert v-if="order.status === 'processing'" class="drawer-alert" type="info" show-icon :closable="false"
        title="供应商正在确认结果，金额已预扣"
        description="确认成功后按实际结果结算，失败的款项退回钱包。确认期间请勿重复提交同一 IP 的续费。" />
      <ElDescriptions :column="2" border size="small">
        <ElDescriptionsItem label="订单号" :span="2"><span class="mono">{{ order.id }}</span></ElDescriptionsItem>
        <ElDescriptionsItem label="类型">{{ orderKindLabels[order.kind] }}</ElDescriptionsItem>
        <ElDescriptionsItem label="状态">
          <ElTag :type="orderStatusTags[order.status]" size="small" disable-transitions>{{ orderStatusLabels[order.status] }}</ElTag>
        </ElDescriptionsItem>
        <ElDescriptionsItem label="下单时间">{{ formatExpiry(order.createdAt) }}</ElDescriptionsItem>
        <ElDescriptionsItem label="租期">{{ order.term ?? '—' }}</ElDescriptionsItem>
        <ElDescriptionsItem v-if="order.credited" label="实际到账流量" :span="2">{{ order.credited }}</ElDescriptionsItem>
      </ElDescriptions>

      <h3 class="drawer-heading">成交明细 · 锁定价格</h3>
      <ElTable :data="order.lines" size="small">
        <ElTableColumn prop="label" label="项目" min-width="200" />
        <ElTableColumn label="交付 / 申请" width="100" align="right">
          <template #default="{ row }: LineRow">
            <span :class="{ 'text-warning': order.status !== 'processing' && row.delivered < row.quantity }">{{ order.status === 'processing' ? '确认中' : `${row.delivered} / ${row.quantity}` }}</span>
          </template>
        </ElTableColumn>
        <ElTableColumn label="单价" width="90" align="right">
          <template #default="{ row }: LineRow">{{ formatMoney(row.unitCents) }}</template>
        </ElTableColumn>
      </ElTable>

      <dl class="summary-total compact">
        <div><dt>预扣金额</dt><dd>{{ formatMoney(total) }}</dd></div>
        <div v-if="refund"><dt>未交付退款</dt><dd class="amount-in">+{{ formatMoney(refund) }}</dd></div>
        <div class="grand"><dt>{{ order.status === 'processing' ? '已预扣' : '实付' }}</dt><dd>{{ formatMoney(total - refund) }}</dd></div>
      </dl>

      <h3 class="drawer-heading">关联钱包流水</h3>
      <ul class="drawer-ledger">
        <li v-for="entry in entries" :key="entry.id">
          <span><strong>{{ ledgerKindLabels[entry.kind] }}</strong> · {{ formatExpiry(entry.at) }}</span>
          <span :class="entry.amountCents > 0 ? 'amount-in' : 'amount-out'">{{ entry.amountCents > 0 ? '+' : '−' }}{{ formatMoney(Math.abs(entry.amountCents)) }}</span>
        </li>
      </ul>
      <slot :order="order" />
    </template>
  </ElDrawer>
</template>
