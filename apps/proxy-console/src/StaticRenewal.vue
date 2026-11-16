<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ApiError } from './api/control'
import { quoteStaticRenewal, recheckStaticRenewal, renewStatic } from './api/static-ip'
import type { StaticRenewalQuote, StaticRenewalSpec } from './api/static-ip'
import { getWallet, type Order, type WalletTenant } from './api/wallet'
import { dateTime, money } from './wallet-format'
import { Button } from './ui/button'

const props = defineProps<{ token: string; tenantId: string; selected: string[] }>()
const emit = defineEmits<{ updated: []; expired: []; wallet: [WalletTenant] }>()
const term = ref('yp1')
const quote = ref<StaticRenewalQuote>()
const error = ref('')
const order = ref<Order>()
const busy = ref(false)
type Attempt = StaticRenewalSpec & { request_key: string; confirmation_token: string }
const storageKey = 'proxy-console.pending-renewal.' + props.tenantId
const attempt = ref<Attempt>()
try {
  const saved = sessionStorage.getItem(storageKey)
  if (saved) attempt.value = JSON.parse(saved) as Attempt
} catch {
  error.value = '暂时无法读取本次会话的续费记录。'
}
const total = computed(() => quote.value?.lines.reduce((sum, line) => sum + BigInt(line.unit_minor), 0n).toString())
watch([term, () => props.selected.join(',')], () => { quote.value = undefined })

function describe(failure: unknown) {
  if (failure instanceof ApiError && failure.status === 401) {
    emit('expired')
    return ''
  }
  if (failure instanceof ApiError) {
    switch (failure.reason) {
      case 'STATIC_RENEWAL_PENDING': return '所选 IP 有待确认的续费。请在订单记录中核对原订单。'
      case 'STATIC_RENEWAL_EXPIRED': return '所选 IP 已到期，请刷新后重新选择。'
      case 'STATIC_QUOTE_EXPIRED': return '价格、到期时间或报价有效期已变化，请重新获取报价。'
      case 'STATIC_RENEWAL_PRICE_UNAVAILABLE': return '该规格当前没有可用的续费报价。'
      case 'WALLET_INSUFFICIENT_FUNDS': return '钱包余额不足，还差 ' + money(failure.metadata.shortfall_minor) + '。'
    }
  }
  return '暂时无法确认续费结果，请使用原请求重试或查看订单记录。'
}
async function getQuote() {
  if (busy.value || !props.selected.length || props.selected.length > 25) return
  busy.value = true
  quote.value = undefined
  error.value = ''
  order.value = undefined
  const selected = props.selected.join(',')
  try {
    const current = await quoteStaticRenewal(props.token, { supplier_item_ids: props.selected, term: term.value })
    if (props.selected.join(',') === selected) quote.value = current
  } catch (failure) {
    error.value = describe(failure)
  } finally {
    busy.value = false
  }
}
async function submit() {
  if (busy.value || (!attempt.value && !quote.value)) return
  const next = attempt.value ?? {
    supplier_item_ids: quote.value!.lines.map(line => line.supplier_item_id),
    term: term.value, request_key: crypto.randomUUID(), confirmation_token: quote.value!.confirmation_token,
  }
  error.value = ''
  try {
    sessionStorage.setItem(storageKey, JSON.stringify(next))
  } catch {
    error.value = '无法保存本次续费请求，请允许会话存储后重试。'
    return
  }
  attempt.value = next
  busy.value = true
  try {
    order.value = await renewStatic(props.token, next)
    clearAttempt()
    await refresh()
  } catch (failure) {
    error.value = describe(failure)
    if (failure instanceof ApiError && [400, 404, 409].includes(failure.status)) clearAttempt()
  } finally {
    busy.value = false
  }
}
function clearAttempt() {
  sessionStorage.removeItem(storageKey)
  attempt.value = undefined
  quote.value = undefined
}
async function refresh() {
  emit('updated')
  const wallet = await getWallet(props.token, AbortSignal.timeout(5000)).catch(() => undefined)
  if (wallet) emit('wallet', wallet)
}
async function recheck() {
  if (!order.value || busy.value) return
  busy.value = true
  error.value = ''
  try {
    order.value = await recheckStaticRenewal(props.token, order.value.id)
    await refresh()
  } catch (failure) {
    error.value = describe(failure)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section v-if="selected.length || attempt || order" aria-label="保号续费">
    <p v-if="error" role="alert">{{ error }}</p>
    <Button v-if="attempt" type="button" :disabled="busy" @click="submit">用原请求重试续费</Button>
    <template v-else-if="selected.length">
      <label>续费租期
        <select v-model="term" :disabled="busy">
          <option value="yp1">30 天</option>
          <option value="yp3">90 天</option>
          <option value="yp6">180 天</option>
          <option value="np">360 天</option>
        </select>
      </label>
      <p v-if="selected.length > 25">一次最多续费 25 个 IP。</p>
      <Button type="button" variant="outline" :disabled="busy || selected.length > 25" @click="getQuote">获取续费报价</Button>
      <div v-if="quote">
        <ul>
          <li v-for="line in quote.lines" :key="line.supplier_item_id">
            {{ line.ip }} · 原到期 {{ dateTime(line.original_expiry) }} · 顺延 {{ line.term_days }} 天 · {{ money(line.unit_minor) }}
          </li>
        </ul>
        <p>合计 {{ money(total) }}</p>
        <Button type="button" :disabled="busy" @click="submit">确认保号续费</Button>
      </div>
    </template>
    <p v-if="order" role="status">
      续费订单 #{{ order.id }} · 已确认 {{ order.delivered_count ?? '0' }} 项 · 已结算 {{ money(order.settled_minor) }}
      <span v-if="order.status === 'pending'"> · 剩余项待确认</span>
      <span v-else-if="order.status === 'failed'"> · 续费失败</span>
    </p>
    <Button v-if="order?.status === 'pending'" type="button" variant="outline" :disabled="busy" @click="recheck">查单确认续费结果</Button>
  </section>
</template>
