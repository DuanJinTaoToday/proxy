<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { ApiError } from './api/control'
import { getOrder, listEntries, listOrders } from './api/wallet'
import type { Entry, Order } from './api/wallet'
import { dateTime, flowAmount, money, orderKinds, orderStatuses } from './wallet-format'

const props = defineProps<{ token: string; tenantId?: string; revision?: number }>()
const emit = defineEmits<{ expired: [] }>()
const view = ref<'entries' | 'orders'>('entries')
const entries = ref<Entry[]>([])
const orders = ref<Order[]>([])
const order = ref<Order | null>(null)
const orderID = ref('')
const kind = ref('')
const status = ref('')
const cursor = ref('0')
const nextCursor = ref('')
const reload = ref(0)
const loading = ref(false)
const error = ref('')
const orderError = ref('')
const orderHeading = ref<HTMLHeadingElement | null>(null)

const entryKinds: Record<string, string> = {
  recharge: '运营充值', debit: '扣款', hold: '预扣', release: '释放预扣', refund: '退款',
}

function selectView(value: typeof view.value) {
  orderID.value = ''
  if (view.value !== value) cursor.value = '0'
  view.value = value
}

function openOrder(id: string) {
  if (view.value !== 'orders') selectView('orders')
  orderID.value = id
}

function resetList() {
  cursor.value = '0'
  reload.value++
}

function report(err: unknown, fallback: string) {
  if (err instanceof ApiError && err.status === 401) {
    emit('expired')
    return ''
  }
  return fallback
}

watch(() => props.revision, resetList)
watch([view, kind, status, cursor, reload], async (_, __, onCleanup) => {
  const abort = new AbortController()
  onCleanup(() => abort.abort())
  entries.value = []
  orders.value = []
  nextCursor.value = ''
  error.value = ''
  loading.value = true
  try {
    if (view.value === 'entries') {
      const result = await listEntries(props.token, props.tenantId ?? '', cursor.value, abort.signal)
      if (abort.signal.aborted) return
      entries.value = result.entries ?? []
      nextCursor.value = result.next_before_id ?? ''
    } else {
      const result = await listOrders(props.token, props.tenantId ?? '',
        { before_id: cursor.value, kind: kind.value, status: status.value }, abort.signal)
      if (abort.signal.aborted) return
      orders.value = result.orders ?? []
      nextCursor.value = result.next_before_id ?? ''
    }
  } catch (err) {
    if (!abort.signal.aborted) error.value = report(err, '无法加载记录，请重试。')
  } finally {
    if (!abort.signal.aborted) loading.value = false
  }
}, { immediate: true })

watch(orderID, async (id, _, onCleanup) => {
  order.value = null
  orderError.value = ''
  if (!id) return
  const abort = new AbortController()
  onCleanup(() => abort.abort())
  try {
    const result = await getOrder(props.token, props.tenantId ?? '', id, abort.signal)
    if (abort.signal.aborted) return
    order.value = result
    await nextTick()
    orderHeading.value?.focus()
  } catch (err) {
    if (!abort.signal.aborted) orderError.value = report(err, '无法加载订单详情。')
  }
})
</script>

<template>
  <nav class="wallet-tabs" aria-label="钱包与订单">
    <button type="button" :class="{ secondary: view !== 'entries' }"
      :aria-pressed="view === 'entries'" @click="selectView('entries')">钱包流水</button>
    <button type="button" :class="{ secondary: view !== 'orders' }"
      :aria-pressed="view === 'orders'" @click="selectView('orders')">订单记录</button>
  </nav>

  <section v-if="orderID && view === 'orders'" class="panel state order-detail" aria-labelledby="order-title">
    <div class="detail-heading">
      <h2 id="order-title" ref="orderHeading" tabindex="-1">订单 #{{ orderID }}</h2>
      <button class="secondary" type="button" @click="orderID = ''">关闭详情</button>
    </div>
    <p v-if="orderError" class="error" role="alert">{{ orderError }}</p>
    <p v-else-if="!order" role="status">正在加载订单…</p>
    <dl v-else class="record-details">
      <div><dt>商品类型</dt><dd>{{ orderKinds[order.kind] }}</dd></div>
      <div><dt>订单状态</dt><dd>{{ orderStatuses[order.status] }}</dd></div>
      <div><dt>锁定总价</dt><dd>{{ money(order.locked_minor) }}</dd></div>
      <div v-if="order.kind !== 'traffic_package'"><dt>预扣</dt><dd>{{ money(order.prepaid_minor) }}</dd></div>
      <div><dt>已结算</dt><dd>{{ money(order.settled_minor) }}</dd></div>
      <div v-if="order.kind !== 'traffic_package'"><dt>待结算</dt><dd>{{ money(order.unsettled_minor) }}</dd></div>
      <div><dt>已退回</dt><dd>{{ money(order.refunded_minor) }}</dd></div>
      <div><dt>实际到账或交付</dt><dd>{{ order.kind === 'traffic_package' ? flowAmount(order.delivered_bytes) : (order.delivered_count ?? '0') + ' 个 IP' }}</dd></div>
      <div><dt>下单时间（北京时间）</dt><dd>{{ dateTime(order.created_at) }}</dd></div>
    </dl>
    <div v-if="order?.items?.length" class="table-wrap order-items">
      <table>
        <caption>商品明细</caption>
        <thead><tr>
          <th scope="col">规格</th><th scope="col">锁定单价</th><th scope="col">订购数量</th><th scope="col">到账或交付数量</th>
        </tr></thead>
        <tbody><tr v-for="item in order.items" :key="item.position">
          <th scope="row">{{ item.specification || orderKinds[order.kind] }}
            <span v-if="item.capacity_bytes">（{{ flowAmount(item.capacity_bytes) }}）</span>
            <span v-if="item.term_days">（{{ item.term_days }} 天）</span>
          </th>
          <td>{{ money(item.unit_minor) }}</td><td>{{ item.quantity }}</td><td>{{ item.delivered_count ?? 0 }}</td>
        </tr></tbody>
      </table>
    </div>
  </section>

  <section class="panel" aria-labelledby="records-title">
    <div class="section-head"><h2 id="records-title">{{ view === 'entries' ? '钱包流水' : '订单记录' }}</h2></div>
    <div v-if="view === 'orders'" class="filters wallet-filters">
      <label>商品类型
        <select v-model="kind" @change="resetList">
          <option value="">全部类型</option>
          <option v-for="(label, key) in orderKinds" :key="key" :value="key">{{ label }}</option>
        </select>
      </label>
      <label>订单状态
        <select v-model="status" @change="resetList">
          <option value="">全部状态</option>
          <option v-for="(label, key) in orderStatuses" :key="key" :value="key">{{ label }}</option>
        </select>
      </label>
    </div>
    <p v-if="loading" class="state muted" role="status">正在加载…</p>
    <div v-else-if="error" class="state">
      <p class="error" role="alert">{{ error }}</p>
      <button type="button" class="secondary" @click="reload++">重试</button>
    </div>
    <template v-else-if="view === 'entries'">
      <p v-if="!entries.length" class="empty">暂无钱包流水。</p>
      <div v-else class="table-wrap">
        <table>
          <thead><tr>
            <th scope="col">流水编号</th><th scope="col">到账时间（北京时间）</th><th scope="col">类型</th>
            <th scope="col">变动金额</th><th v-if="tenantId" scope="col">变动前余额</th><th scope="col">变动后余额</th>
            <th v-if="tenantId" scope="col">操作人</th><th v-if="tenantId" scope="col">原因</th><th scope="col">关联订单</th>
          </tr></thead>
          <tbody><tr v-for="entry in entries" :key="entry.id">
            <th scope="row">#{{ entry.id }}</th><td>{{ dateTime(entry.created_at) }}</td><td>{{ entryKinds[entry.kind] }}</td>
            <td>{{ money(BigInt(entry.after_minor ?? '0') - BigInt(entry.before_minor ?? '0')) }}</td>
            <td v-if="tenantId">{{ money(entry.before_minor) }}</td><td>{{ money(entry.after_minor) }}</td>
            <td v-if="tenantId">{{ entry.operator_name || entry.operator_id || '—' }}</td>
            <td v-if="tenantId" class="entry-reason">{{ entry.reason || '—' }}</td>
            <td><button v-if="entry.order_id" class="text-button" type="button" @click="openOrder(entry.order_id)">#{{ entry.order_id }}</button><span v-else>—</span></td>
          </tr></tbody>
        </table>
      </div>
    </template>
    <template v-else>
      <p v-if="!orders.length" class="empty">暂无符合条件的订单。</p>
      <div v-else class="table-wrap">
        <table>
          <thead><tr>
            <th scope="col">订单编号</th><th scope="col">商品类型</th><th scope="col">下单时间（北京时间）</th>
            <th scope="col">锁定总价</th><th scope="col">状态</th><th scope="col">操作</th>
          </tr></thead>
          <tbody><tr v-for="item in orders" :key="item.id">
            <th scope="row">#{{ item.id }}</th><td>{{ orderKinds[item.kind] }}</td><td>{{ dateTime(item.created_at) }}</td>
            <td>{{ money(item.locked_minor) }}</td><td><span class="order-status" :data-status="item.status">{{ orderStatuses[item.status] }}</span></td>
            <td><button class="text-button" type="button" :aria-label="'查看订单 #' + item.id" @click="openOrder(item.id)">查看详情</button></td>
          </tr></tbody>
        </table>
      </div>
    </template>
    <div v-if="!loading && !error && (cursor !== '0' || nextCursor)" class="record-pagination">
      <button v-if="cursor !== '0'" type="button" class="secondary" @click="cursor = '0'">返回首页</button>
      <button v-if="nextCursor" type="button" class="secondary" @click="cursor = nextCursor">下一页</button>
    </div>
  </section>
</template>
