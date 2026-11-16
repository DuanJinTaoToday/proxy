<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ApiError } from './api/control'
import { pendingStaticPurchases, resolveStaticPurchase, resolveStaticRenewal } from './api/static-ip'
import type { PendingStaticPurchase } from './api/static-ip'
import { dateTime } from './wallet-format'
import { Button } from './ui/button'

const { token, canResolve } = defineProps<{ token: string; canResolve: boolean }>()
const emit = defineEmits<{ expired: []; resolved: [string] }>()
const controller = new AbortController()
const pending = ref<PendingStaticPurchase[]>([])
const next = ref('')
const active = ref<PendingStaticPurchase>()
const evidence = ref('')
const supplierID = ref('')
const renewalItem = ref('')
const loading = ref(false)
const submitting = ref(false)
const error = ref('')
const result = ref('')

function report(failure: unknown, message: string) {
  if (failure instanceof ApiError && failure.status === 401) emit('expired')
  else error.value = message
}

async function load(after = '') {
  loading.value = true
  error.value = ''
  try {
    const page = await pendingStaticPurchases(token, after, controller.signal)
    pending.value = page.purchases ?? []
    next.value = page.next_after_id ?? ''
    active.value = undefined
  } catch (failure) {
    if (!controller.signal.aborted) report(failure, '暂时无法查询待核实订单。')
  } finally {
    loading.value = false
  }
}

function select(order: PendingStaticPurchase) {
  active.value = order
  supplierID.value = order.supplier_order_id ?? ''
  renewalItem.value = order.pending_renewal_items?.[0] ?? ''
  evidence.value = ''
  result.value = ''
  error.value = ''
}

async function resolve(closeRemaining: boolean, confirmedUndeliverable = false) {
  const order = active.value
  if (!order || submitting.value || !evidence.value.trim() ||
      (closeRemaining ? !order.can_close && !confirmedUndeliverable : !renewalItem.value && !supplierID.value.trim())) return
  submitting.value = true
  error.value = ''
  result.value = ''
  try {
    const updated = renewalItem.value
      ? await resolveStaticRenewal(token, order, {
          supplier_item_id: renewalItem.value, evidence: evidence.value.trim(),
          close_unknown: closeRemaining && !confirmedUndeliverable, confirmed_failure: confirmedUndeliverable,
        })
      : await resolveStaticPurchase(token, order.tenant_id, order.order_id, {
      supplier_order_id: supplierID.value.trim(), evidence: evidence.value.trim(), close_remaining: closeRemaining, confirmed_undeliverable: confirmedUndeliverable,
    })
    result.value = updated.status === 'pending' ? '已同步交付，剩余部分继续核实。' : '订单已结算。'
    emit('resolved', order.tenant_id)
    await load()
  } catch (failure) {
    report(failure, '暂时无法更新订单，请核对皇冠订单号并刷新待核实列表。')
  } finally {
    submitting.value = false
  }
}

onMounted(() => load())
onUnmounted(() => controller.abort())
</script>

<template>
  <section class="panel wallet-section" aria-labelledby="static-operations-title">
    <h2 id="static-operations-title">待核实静态 IP 订单</h2>
    <Button type="button" variant="outline" :disabled="loading || submitting" @click="load()">刷新</Button>
    <p v-if="loading" role="status">正在加载待核实订单…</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <p v-if="result" role="status">{{ result }}</p>
    <p v-if="!loading && !error && !pending.length">暂无超过 24 小时的待核实订单。</p>
    <ul v-if="!loading">
      <li v-for="order in pending" :key="order.order_id">
        <Button v-if="canResolve" type="button" variant="link" :disabled="submitting" @click="select(order)">核实{{ order.pending_renewal_items?.length ? '续费' : '购买' }} #{{ order.order_id }}</Button>
        <span v-else>订单 #{{ order.order_id }} ·</span>
        {{ dateTime(order.created_at) }} · 已交付 {{ order.delivered_count ?? 0 }} 个
      </li>
    </ul>
    <Button v-if="next" type="button" variant="outline" :disabled="loading || submitting" @click="load(next)">下一页</Button>
    <form v-if="active" class="resolution" @submit.prevent="resolve(false)">
      <h3>订单 #{{ active.order_id }}</h3>
      <label v-if="active.pending_renewal_items?.length">续费明细
        <select v-model="renewalItem"><option v-for="item in active.pending_renewal_items" :key="item">{{ item }}</option></select>
      </label>
      <label v-else>皇冠订单号 <input v-model="supplierID" maxlength="128" :readonly="!!active.supplier_order_id" :disabled="submitting" /></label>
      <label>核实依据 <textarea v-model="evidence" required maxlength="500" :disabled="submitting" /></label>
      <Button type="submit" :disabled="submitting || (!renewalItem && !supplierID.trim()) || !evidence.trim()">查询皇冠并同步交付</Button>
      <Button type="button" variant="outline" :disabled="submitting || !evidence.trim()" @click="resolve(true, true)">已确认{{ renewalItem ? '续费失败' : '无法交付' }}，退回预扣</Button>
      <template v-if="active.can_close">
        <p>提交后会关闭剩余交付并退回对应预扣款。随后补发的 IP 由运营处理。</p>
        <Button type="button" variant="outline" :disabled="submitting || !evidence.trim()" @click="resolve(true)">关闭剩余交付并退款</Button>
      </template>
    </form>
  </section>
</template>

<style scoped>
.resolution, .resolution label { display: grid; gap: 8px; justify-items: start; }
</style>
