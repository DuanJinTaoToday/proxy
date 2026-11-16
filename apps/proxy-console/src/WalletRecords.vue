<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { Badge } from './ui/badge'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import Dialog from './ui/dialog/Dialog.vue'
import DialogContent from './ui/dialog/DialogContent.vue'
import DialogTitle from './ui/dialog/DialogTitle.vue'
import NativeSelect from './ui/native-select/NativeSelect.vue'
import Table from './ui/table/Table.vue'
import TableBody from './ui/table/TableBody.vue'
import TableCaption from './ui/table/TableCaption.vue'
import TableCell from './ui/table/TableCell.vue'
import TableHead from './ui/table/TableHead.vue'
import TableHeader from './ui/table/TableHeader.vue'
import TableRow from './ui/table/TableRow.vue'
import Tabs from './ui/tabs/Tabs.vue'
import TabsContent from './ui/tabs/TabsContent.vue'
import TabsList from './ui/tabs/TabsList.vue'
import TabsTrigger from './ui/tabs/TabsTrigger.vue'
import { ApiError } from './api/control'
import { getOrder, listEntries, listOrders } from './api/wallet'
import type { Entry, Order } from './api/wallet'
import { dateTime, flowAmount, money, orderKinds, orderStatuses } from './wallet-format'

const props = defineProps<{ token: string; tenantId?: string; revision?: number }>()
const emit = defineEmits<{ expired: [] }>()
const view = defineModel<'entries' | 'orders'>('view', { default: 'entries' })
const entries = ref<Entry[]>([])
const recentEntries = ref<Entry[] | null>(null)
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
const orderTrigger = ref<HTMLElement | null>(null)

const entryKinds: Record<string, string> = {
  recharge: '运营充值', debit: '扣款', hold: '预扣', release: '释放预扣', refund: '退款',
}

function selectView(value: typeof view.value) {
  orderID.value = ''
  view.value = value
}

function openOrder(id: string) {
  if (props.tenantId && view.value !== 'orders') selectView('orders')
  orderTrigger.value = document.activeElement instanceof HTMLElement ? document.activeElement : null
  orderID.value = id
}

async function restoreOrderFocus(event: Event) {
  event.preventDefault()
  await nextTick()
  orderTrigger.value?.focus()
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
watch(view, () => {
  cursor.value = '0'
  orderID.value = ''
}, { flush: 'sync' })

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
      if (cursor.value === '0') recentEntries.value = entries.value
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
  <slot name="overview" :recent-entries="recentEntries" />
  <Tabs :model-value="view" class="wallet-records gap-0 min-w-0" @update:model-value="value => { if (value === 'entries' || value === 'orders') selectView(value) }">
    <div class="wallet-record-toolbar">
      <TabsList class="wallet-tabs h-auto" aria-label="钱包与订单">
        <TabsTrigger value="entries">钱包流水</TabsTrigger>
        <TabsTrigger value="orders">订单记录</TabsTrigger>
      </TabsList>
      <slot name="records-help" />
    </div>

    <Dialog :open="Boolean(orderID)" @update:open="open => { if (!open) orderID = '' }">
      <component :is="tenantId ? 'section' : DialogContent" v-if="orderID"
        :class="tenantId ? 'panel state order-detail' : 'wallet-dialog'"
        aria-labelledby="order-title" :aria-describedby="undefined" :show-close-button="tenantId ? undefined : false" @close-auto-focus="restoreOrderFocus">
        <div class="detail-heading">
          <h2 v-if="tenantId" id="order-title" ref="orderHeading" tabindex="-1">订单 #{{ orderID }}</h2>
          <DialogTitle v-else id="order-title" as="h2">订单 #{{ orderID }}</DialogTitle>
          <Button variant="outline" class="secondary" type="button" @click="orderID = ''">关闭详情</Button>
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
        <Table v-if="order?.items?.length" class="order-items-table">
            <TableCaption>商品明细</TableCaption>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">规格</TableHead><TableHead scope="col">锁定单价</TableHead><TableHead scope="col">订购数量</TableHead><TableHead scope="col">到账或交付数量</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="item in order.items" :key="item.position">
                <TableHead scope="row" class="wallet-row-header">{{ item.specification || orderKinds[order.kind] }}
                  <span v-if="item.capacity_bytes">（{{ flowAmount(item.capacity_bytes) }}）</span>
                  <span v-if="item.term_days">（{{ item.term_days }} 天）</span>
                </TableHead>
                <TableCell>{{ money(item.unit_minor) }}</TableCell><TableCell>{{ item.quantity }}</TableCell><TableCell>{{ item.delivered_count ?? 0 }}</TableCell>
              </TableRow>
            </TableBody>
        </Table>
      </component>
    </Dialog>

    <TabsContent :value="view" class="min-w-0">
      <Card class="panel wallet-record-panel gap-0 py-0" role="region" aria-labelledby="records-title">
        <div class="section-head"><h2 id="records-title">{{ view === 'entries' ? '钱包流水' : '订单记录' }}</h2></div>
        <div v-if="view === 'orders'" class="filters wallet-filters">
          <label>商品类型
            <NativeSelect aria-label="商品类型" :model-value="kind" @update:model-value="value => { kind = String(value); resetList() }">
              <option value="">全部类型</option>
              <option v-for="(label, key) in orderKinds" :key="key" :value="key">{{ label }}</option>
            </NativeSelect>
          </label>
          <label>订单状态
            <NativeSelect aria-label="订单状态" :model-value="status" @update:model-value="value => { status = String(value); resetList() }">
              <option value="">全部状态</option>
              <option v-for="(label, key) in orderStatuses" :key="key" :value="key">{{ label }}</option>
            </NativeSelect>
          </label>
        </div>
        <p v-if="loading" class="state muted" role="status">正在加载…</p>
        <div v-else-if="error" class="state">
          <p class="error" role="alert">{{ error }}</p>
          <Button variant="outline" type="button" class="secondary" @click="reload++">重试</Button>
        </div>
        <template v-else-if="view === 'entries'">
          <p v-if="!entries.length" class="empty">暂无钱包流水。</p>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">流水编号</TableHead><TableHead scope="col">到账时间（北京时间）</TableHead><TableHead scope="col">类型</TableHead>
                <TableHead scope="col" class="wallet-amount">变动金额</TableHead><TableHead v-if="tenantId" scope="col">变动前余额</TableHead><TableHead scope="col" class="wallet-amount">变动后余额</TableHead>
                <TableHead v-if="tenantId" scope="col">操作人</TableHead><TableHead v-if="tenantId" scope="col">原因</TableHead><TableHead scope="col">关联订单</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="entry in entries" :key="entry.id">
                <TableHead scope="row" class="wallet-row-header">#{{ entry.id }}</TableHead><TableCell>{{ dateTime(entry.created_at) }}</TableCell><TableCell>{{ entryKinds[entry.kind] }}</TableCell>
                <TableCell class="wallet-amount">{{ money(BigInt(entry.after_minor ?? '0') - BigInt(entry.before_minor ?? '0')) }}</TableCell>
                <TableCell v-if="tenantId">{{ money(entry.before_minor) }}</TableCell><TableCell class="wallet-amount">{{ money(entry.after_minor) }}</TableCell>
                <TableCell v-if="tenantId">{{ entry.operator_name || entry.operator_id || '—' }}</TableCell>
                <TableCell v-if="tenantId" class="entry-reason">{{ entry.reason || '—' }}</TableCell>
                <TableCell><Button variant="link" v-if="entry.order_id" class="text-button" type="button" @click="openOrder(entry.order_id)">#{{ entry.order_id }}</Button><span v-else>—</span></TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </template>
        <template v-else>
          <p v-if="!orders.length" class="empty">暂无符合条件的订单。</p>
          <Table v-else>
            <TableHeader>
              <TableRow>
                <TableHead scope="col">订单编号</TableHead><TableHead scope="col">商品类型</TableHead><TableHead scope="col">下单时间（北京时间）</TableHead>
                <TableHead scope="col" class="wallet-amount">锁定总价</TableHead><TableHead scope="col">状态</TableHead><TableHead scope="col">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="item in orders" :key="item.id">
                <TableHead scope="row" class="wallet-row-header">#{{ item.id }}</TableHead><TableCell>{{ orderKinds[item.kind] }}</TableCell><TableCell>{{ dateTime(item.created_at) }}</TableCell>
                <TableCell class="wallet-amount">{{ money(item.locked_minor) }}</TableCell><TableCell><Badge variant="outline" class="order-status" :data-status="item.status">{{ orderStatuses[item.status] }}</Badge></TableCell>
                <TableCell><Button variant="link" class="text-button" type="button" :aria-label="'查看订单 #' + item.id" @click="openOrder(item.id)">查看详情</Button></TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </template>
        <div v-if="!loading && !error && (cursor !== '0' || nextCursor)" class="record-pagination">
          <Button variant="outline" v-if="cursor !== '0'" type="button" class="secondary" @click="cursor = '0'">返回首页</Button>
          <Button variant="outline" v-if="nextCursor" type="button" class="secondary" @click="cursor = nextCursor">下一页</Button>
        </div>
      </Card>
    </TabsContent>
  </Tabs>
</template>
