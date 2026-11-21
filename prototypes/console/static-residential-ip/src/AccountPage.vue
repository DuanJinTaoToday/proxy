<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElAlert, ElButton, ElDatePicker, ElEmpty, ElInput, ElOption, ElPagination, ElSelect, ElTable, ElTableColumn, ElTabPane, ElTabs, ElTag } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import OrderDrawer from './OrderDrawer.vue'
import { formatExpiry, formatMoney } from './demo'
import { currentIp, orderKindLabels, orders, orderRefundCents, orderStatusLabels, orderStatusTags, orderTotalCents, type Order, type OrderKind, type OrderStatus } from './state'

// ElTable 的插槽行类型是 DefaultRow，模板里按 Order 读取字段。
type OrderRow = { row: any }
type OrderProduct = 'dynamic' | 'static'
const pageSize = 8
const productLabels: Record<OrderProduct, string> = { dynamic: '动态住宅代理-GB', static: '静态住宅代理' }
const productOf = (order: Order): OrderProduct => order.kind === 'traffic' ? 'dynamic' : 'static'

const tab = ref(location.hash.endsWith('/orders') ? 'orders' : 'profile')
const product = ref<OrderProduct | 'all'>('all')
const status = ref<OrderStatus | 'all'>('all')
const keyword = ref('')
const range = ref<[Date, Date] | null>(null)
const pageNo = ref(1)
const drawerOpen = ref(false)
const drawerOrderId = ref<string>()

const filtered = computed(() => {
  const [from, to] = range.value ?? []
  const term = keyword.value.trim().toUpperCase()
  return orders.value
    .filter(o => (product.value === 'all' || productOf(o) === product.value)
      && (status.value === 'all' || o.status === status.value)
      && (!term || o.id.includes(term))
      && (!from || o.createdAt >= from.getTime())
      && (!to || o.createdAt < to.getTime() + 86_400_000))
    .reverse()
})
const pageRows = computed(() => filtered.value.slice((pageNo.value - 1) * pageSize, pageNo.value * pageSize))
const processingCount = computed(() => orders.value.filter(o => o.status === 'processing').length)

watch([product, status, keyword, range], () => { pageNo.value = 1 })
watch(tab, value => { history.replaceState(null, '', `#account/${value}`) })
const onHash = () => { tab.value = location.hash.endsWith('/orders') ? 'orders' : 'profile' }
window.addEventListener('hashchange', onHash)
onBeforeUnmount(() => window.removeEventListener('hashchange', onHash))

function summary(order: Order) {
  const [first, ...rest] = order.lines
  return rest.length ? `${first.label} 等 ${order.lines.length} 项` : first.label
}

function quantity(order: Order) {
  if (order.kind === 'traffic') return order.credited ?? '—'
  const asked = order.lines.reduce((s, l) => s + l.quantity, 0)
  const delivered = order.lines.reduce((s, l) => s + l.delivered, 0)
  return order.status === 'processing' ? `${asked} 个确认中` : delivered === asked ? `${asked} 个` : `${delivered} / ${asked} 个`
}

function openOrder(order: Order) {
  drawerOrderId.value = order.id
  drawerOpen.value = true
}

function resetFilters() {
  product.value = 'all'
  status.value = 'all'
  keyword.value = ''
  range.value = null
}
</script>

<template>
  <div class="page-head"><h1>我的账户</h1></div>

  <ElTabs v-model="tab" class="page-tabs">
    <ElTabPane label="账户信息" name="profile">
      <div class="profile-grid">
        <section class="details-card profile-card">
          <h2>账户</h2>
          <dl class="profile-fields">
            <div><dt>主账号</dt><dd>chenxl-studio</dd></div>
            <div><dt>手机号</dt><dd>138****6021</dd></div>
          </dl>
        </section>

        <section class="details-card profile-card">
          <h2>当前 IP 地址及归属地</h2>
          <dl class="profile-fields">
            <div><dt>IP 地址</dt><dd class="mono-value">{{ currentIp.address }}</dd></div>
            <div><dt>归属地</dt><dd>{{ currentIp.location }}</dd></div>
          </dl>
          <ElAlert v-if="currentIp.mainland" type="warning" :closable="false" show-icon class="ip-status"
            title="当前网络在中国大陆，无法使用 Proxy" />
          <ElAlert v-else type="success" :closable="false" show-icon class="ip-status" title="可直接使用 Proxy" />
          <p class="ip-notice">所有代理产品均不支持在中国大陆网络环境下使用</p>
        </section>
      </div>
    </ElTabPane>

    <ElTabPane name="orders">
      <template #label>订单记录<span v-if="processingCount" class="tab-count">{{ processingCount }}</span></template>
      <section class="table-card" aria-label="订单记录">
        <div class="table-toolbar">
          <div class="table-filters">
            <ElInput v-model="keyword" class="filter-search" :prefix-icon="Search" clearable placeholder="搜索订单号" />
            <ElSelect v-model="product" class="filter-product" aria-label="按产品筛选">
              <ElOption value="all" label="全部产品" />
              <ElOption v-for="(label, value) in productLabels" :key="value" :value="value" :label="label" />
            </ElSelect>
            <ElSelect v-model="status" class="filter-status" aria-label="按状态筛选">
              <ElOption value="all" label="全部状态" />
              <ElOption v-for="(label, value) in orderStatusLabels" :key="value" :value="value" :label="label" />
            </ElSelect>
            <ElDatePicker v-model="range" type="daterange" class="filter-dates" range-separator="至"
              start-placeholder="开始日期" end-placeholder="结束日期" aria-label="按下单日期筛选" />
          </div>
        </div>

        <ElTable :data="pageRows" row-key="id" class="ip-table order-table" @row-click="openOrder">
          <ElTableColumn label="订单号" min-width="150">
            <template #default="{ row }: OrderRow"><span class="mono order-id">{{ row.id }}</span></template>
          </ElTableColumn>
          <ElTableColumn label="类型" min-width="110">
            <template #default="{ row }: OrderRow">{{ orderKindLabels[row.kind as OrderKind] }}</template>
          </ElTableColumn>
          <ElTableColumn label="内容" min-width="250">
            <template #default="{ row }: OrderRow">
              <span class="order-summary">{{ summary(row) }}</span>
              <span class="remaining">{{ row.term }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="到账 / 交付" min-width="110">
            <template #default="{ row }: OrderRow">{{ quantity(row) }}</template>
          </ElTableColumn>
          <ElTableColumn label="实付" min-width="100" align="right">
            <template #default="{ row }: OrderRow">
              <span class="ledger-amount">{{ formatMoney(orderTotalCents(row) - orderRefundCents(row)) }}</span>
              <span v-if="orderRefundCents(row)" class="remaining">已退 {{ formatMoney(orderRefundCents(row)) }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" min-width="100">
            <template #default="{ row }: OrderRow">
              <ElTag :type="orderStatusTags[row.status as OrderStatus]" size="small" disable-transitions>{{ orderStatusLabels[row.status as OrderStatus] }}</ElTag>
            </template>
          </ElTableColumn>
          <ElTableColumn label="下单时间" min-width="150">
            <template #default="{ row }: OrderRow">{{ formatExpiry(row.createdAt) }}</template>
          </ElTableColumn>
          <ElTableColumn label="" width="70" align="right">
            <template #default="{ row }: OrderRow"><ElButton link type="primary" @click.stop="openOrder(row)">详情</ElButton></template>
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
    </ElTabPane>
  </ElTabs>

  <OrderDrawer v-model="drawerOpen" :order-id="drawerOrderId" />
</template>
