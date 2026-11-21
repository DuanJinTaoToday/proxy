<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElButton, ElEmpty, ElInput, ElOption, ElPagination, ElSelect, ElTable, ElTableColumn, ElTag, ElTooltip } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import OrderDrawer from './OrderDrawer.vue'
import OpsResolveDialog from './OpsResolveDialog.vue'
import { formatExpiry, formatMoney } from './demo'
import { allOrders, billingOnly, canBill, hoursSince, tenantById } from './ops'
import { orderKindLabels, orderRefundCents, orderStatusLabels, orderStatusTags, orderTotalCents, type Order, type OrderKind, type OrderStatus } from './state'

// 运营订单列表。在租户详情里传入 tenantId，只看该租户。
const props = defineProps<{ tenantId?: string }>()
type Row = { row: any }
type OpsOrder = Order & { tenantId: string }
const pageSize = 10

const keyword = ref('')
const kind = ref<OrderKind | 'all'>('all')
const status = ref<OrderStatus | 'all'>('all')
const pageNo = ref(1)

const filtered = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  return allOrders.value.filter(o => (!props.tenantId || o.tenantId === props.tenantId)
    && (kind.value === 'all' || o.kind === kind.value)
    && (status.value === 'all' || o.status === status.value)
    && (!term || o.id.toLowerCase().includes(term) || tenantById(o.tenantId).account.includes(term)))
})
const pageRows = computed(() => filtered.value.slice((pageNo.value - 1) * pageSize, pageNo.value * pageSize))
watch([keyword, kind, status], () => { pageNo.value = 1 })

const drawerOpen = ref(false)
const current = ref<OpsOrder>()
const resolveOpen = ref(false)
function openOrder(row: OpsOrder) {
  current.value = row
  drawerOpen.value = true
}

function summary(order: Order) {
  const [first, ...rest] = order.lines
  return rest.length ? `${first.label} 等 ${order.lines.length} 项` : first.label
}

function resetFilters() {
  keyword.value = ''
  kind.value = 'all'
  status.value = 'all'
}
</script>

<template>
  <div v-if="!tenantId" class="page-head"><h1>订单</h1></div>

  <section class="table-card" aria-label="订单">
    <div class="table-toolbar">
      <div class="table-filters">
        <ElInput v-model="keyword" class="filter-search" :prefix-icon="Search" clearable :placeholder="tenantId ? '搜索订单号' : '搜索订单号或主账号'" />
        <ElSelect v-model="kind" class="filter-product" aria-label="按类型筛选">
          <ElOption value="all" label="全部类型" />
          <ElOption v-for="(label, value) in orderKindLabels" :key="value" :value="value" :label="label" />
        </ElSelect>
        <ElSelect v-model="status" class="filter-status" aria-label="按状态筛选">
          <ElOption value="all" label="全部状态" />
          <ElOption v-for="(label, value) in orderStatusLabels" :key="value" :value="value" :label="label" />
        </ElSelect>
      </div>
    </div>

    <ElTable :data="pageRows" row-key="id" class="ip-table order-table" @row-click="openOrder">
      <ElTableColumn label="订单号" min-width="150">
        <template #default="{ row }: Row"><span class="mono order-id">{{ row.id }}</span></template>
      </ElTableColumn>
      <ElTableColumn v-if="!tenantId" label="租户" min-width="140">
        <template #default="{ row }: Row">
          <a class="table-link" :href="`#tenants/${row.tenantId}`" @click.stop>{{ tenantById(row.tenantId).name }}</a>
          <span class="remaining mono">{{ tenantById(row.tenantId).account }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="类型" min-width="110">
        <template #default="{ row }: Row">{{ orderKindLabels[row.kind as OrderKind] }}</template>
      </ElTableColumn>
      <ElTableColumn label="内容" min-width="240">
        <template #default="{ row }: Row">
          <span class="order-summary">{{ summary(row) }}</span>
          <span class="remaining">{{ row.term }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="实付" min-width="100" align="right">
        <template #default="{ row }: Row">
          <span class="ledger-amount">{{ formatMoney(orderTotalCents(row) - orderRefundCents(row)) }}</span>
          <span v-if="orderRefundCents(row)" class="remaining">已退 {{ formatMoney(orderRefundCents(row)) }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="状态" min-width="110">
        <template #default="{ row }: Row">
          <ElTag :type="orderStatusTags[row.status as OrderStatus]" size="small" disable-transitions>{{ orderStatusLabels[row.status as OrderStatus] }}</ElTag>
          <span v-if="row.status === 'processing'" class="remaining">已等待 {{ hoursSince(row.createdAt) }} 小时</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="下单时间" min-width="150">
        <template #default="{ row }: Row">{{ formatExpiry(row.createdAt) }}</template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty description="没有符合条件的订单" :image-size="72">
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

  <OrderDrawer v-model="drawerOpen" :order-id="current?.id" :tenant-id="current?.tenantId">
    <template #default="{ order }">
      <template v-if="order.resolutions?.length">
        <h3 class="drawer-heading">运营核实记录</h3>
        <ul class="drawer-ledger resolution-list">
          <li v-for="(r, i) in order.resolutions" :key="i">
            <span><strong>{{ r.closed ? '关闭剩余' : '同步交付' }}</strong> · {{ r.operator }} · {{ formatExpiry(r.at) }}</span>
            <span class="remaining">{{ r.evidence }}<template v-if="r.supplierRef"> · 皇冠单号 {{ r.supplierRef }}</template></span>
          </li>
        </ul>
      </template>
      <div v-if="order.status === 'processing'" class="drawer-actions">
        <ElTooltip :disabled="canBill" :content="billingOnly" placement="top">
          <span><ElButton type="primary" :disabled="!canBill" @click="resolveOpen = true">核实订单</ElButton></span>
        </ElTooltip>
      </div>
    </template>
  </OrderDrawer>
  <OpsResolveDialog v-model="resolveOpen" :tenant-id="current?.tenantId" :order-id="current?.id" />
</template>
