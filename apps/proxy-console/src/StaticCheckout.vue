<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ApiError } from './api/control'
import { buyStaticIP, quoteStaticPurchase, staticOffers } from './api/static-ip'
import type { StaticOffer, StaticQuote } from './api/static-ip'
import { getWallet } from './api/wallet'
import type { Order, WalletTenant } from './api/wallet'
import { money } from './wallet-format'
import { Button } from './ui/button'
import Input from './ui/input/Input.vue'

const { token, tenantId } = defineProps<{ token?: string; tenantId?: string }>()
const emit = defineEmits<{ expired: []; wallet: [WalletTenant] }>()
const route = useRoute()
const controller = new AbortController()
const offers = ref<StaticOffer[]>([])
const selected = ref('')
const region = ref('')
const ipType = ref('')
const termDays = ref<number | ''>('')
const inStock = ref(false)
const rawQuantity = ref<number | string>(Math.max(1, Math.min(2147483647, Number(route.query.quantity) || 1)))
const quantity = computed(() => Number(rawQuantity.value))
const loading = ref(true)
const closed = ref(false)
const error = ref('')
const sending = ref(false)
const order = ref<Order>()
const quote = ref<StaticQuote>()
interface PendingPurchase {
  confirmationToken: string
  key: string
  offer: StaticOffer
  quantity: number
}
const pending = ref<PendingPurchase>()
const storageKey = tenantId ? 'proxy-console.pending-static.' + tenantId : ''
if (storageKey) {
  try {
    const saved = sessionStorage.getItem(storageKey)
    if (saved) pending.value = JSON.parse(saved) as PendingPurchase
  } catch {
    // The submit path requires working session storage before sending money.
  }
}
const offerKey = (offer: StaticOffer) => offer.node_id + '/' + offer.term
const regions = computed(() => [...new Set(offers.value.map(item => item.region).filter(Boolean))])
const ipTypes = computed(() => [...new Set(offers.value.map(item => item.ip_type).filter(Boolean))])
const terms = computed(() => [...new Set(offers.value.map(item => item.term_days))].sort((a, b) => a - b))
const filteredOffers = computed(() => offers.value.filter(item =>
  (!region.value || item.region === region.value) &&
  (!ipType.value || item.ip_type === ipType.value) &&
  (!termDays.value || item.term_days === termDays.value) &&
  (!inStock.value || (item.available ?? 0) > 0),
))
const offer = computed(() => filteredOffers.value.find(item => offerKey(item) === selected.value))
watch([region, ipType, termDays, inStock], () => {
  if (filteredOffers.value.some(item => offerKey(item) === selected.value)) return
  const first = filteredOffers.value[0]
  selected.value = first ? offerKey(first) : ''
}, { flush: 'sync' })
const total = computed(() => offer.value && Number.isInteger(quantity.value) ? BigInt(offer.value.unit_minor) * BigInt(quantity.value) : 0n)
const canSubmit = computed(() => offer.value && Number.isInteger(quantity.value) && quantity.value >= 1 && quantity.value <= (offer.value.available ?? 0))
const checkoutQuery = computed(() => ({ view: 'static', offer: selected.value, quantity: String(quantity.value) }))
watch([selected, quantity], () => { order.value = undefined; quote.value = undefined }, { flush: 'sync' })

function forgetPending() {
  sessionStorage.removeItem(storageKey)
  pending.value = undefined
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await staticOffers(controller.signal)
    offers.value = result.offers ?? []
    closed.value = !result.open
    if (!offer.value && offers.value.length) {
      const requested = route.query.offer
      selected.value = typeof requested === 'string' ? requested : offerKey(offers.value[0]!)
      if (!offer.value) {
        selected.value = ''
        error.value = '原规格已失效，请重新选择。'
      }
    }
      } catch (failure) {
    if (controller.signal.aborted) return
    if (failure instanceof ApiError && failure.reason === 'PRODUCT_SALE_CLOSED') { closed.value = true; offers.value = [] }
    else error.value = '暂时无法查询静态 IP 报价。'
  } finally {
    loading.value = false
  }
}

async function refreshQuote() {
  if (!token || !offer.value || !canSubmit.value || sending.value) return
  sending.value = true
  error.value = ''
  const selection = offer.value
  try {
    quote.value = await quoteStaticPurchase(token, selection, quantity.value)
    selection.unit_minor = quote.value.offer.unit_minor
    selection.available = quote.value.offer.available
  } catch (failure) {
    if (failure instanceof ApiError && failure.status === 401) emit('expired')
    else if (failure instanceof ApiError && failure.reason === 'PRODUCT_SALE_CLOSED') closed.value = true
    else if (failure instanceof ApiError && failure.reason === 'STATIC_STOCK_INSUFFICIENT') {
      selection.available = Number(failure.metadata.available ?? '0')
      error.value = '库存不足，当前可购 ' + selection.available + ' 个。'
    } else error.value = '暂时无法生成确认报价，请重新选择或刷新。'
  } finally {
    sending.value = false
  }
}

async function submit() {
  if (!pending.value && token && !quote.value) {
    await refreshQuote()
    return
  }
  if (!token || sending.value || (!pending.value && (!offer.value || !canSubmit.value))) return
  const attempt = pending.value ?? (quote.value && { key: crypto.randomUUID(), offer: quote.value.offer, quantity: quote.value.quantity, confirmationToken: quote.value.confirmation_token })
  if (!attempt) return
  try {
    sessionStorage.setItem(storageKey, JSON.stringify(attempt))
  } catch {
    error.value = '无法保存本次购买请求。请允许会话存储后重试。'
    return
  }
  pending.value = attempt
  sending.value = true
  error.value = ''
  order.value = undefined
  try {
    order.value = await buyStaticIP(token, attempt.offer, attempt.quantity, attempt.key, attempt.confirmationToken)
    forgetPending()
    const account = await getWallet(token, controller.signal).catch(() => undefined)
    if (account) emit('wallet', account)
  } catch (failure) {
    if (failure instanceof ApiError) {
      switch (failure.reason) {
        case 'PRODUCT_SALE_CLOSED':
          forgetPending()
          closed.value = true
          break
        case 'STATIC_QUOTE_EXPIRED':
          forgetPending()
          quote.value = undefined
          error.value = '确认报价已过期，请重新获取。'
          break
        case 'STATIC_PRICE_CHANGED':
          quote.value = undefined
          forgetPending()
          if (offer.value && failure.metadata.unit_minor) offer.value.unit_minor = failure.metadata.unit_minor
          error.value = '价格已更新，请核对新单价和总价后重新提交。'
          break
        case 'STATIC_STOCK_INSUFFICIENT':
          quote.value = undefined
          forgetPending()
          if (offer.value) offer.value.available = Number(failure.metadata.available ?? '0')
          error.value = '库存不足，当前可购 ' + (failure.metadata.available ?? '0') + ' 个。请调整数量。'
          break
        case 'WALLET_INSUFFICIENT_FUNDS':
          forgetPending()
          error.value = '钱包余额不足，还差 ' + money(failure.metadata.shortfall_minor) + '。'
          break
        case 'STATIC_OFFER_UNAVAILABLE':
          forgetPending()
          await load()
          error.value = '原规格已失效，请重新选择。'
          break
        default:
          if (failure.status === 401) emit('expired')
          else error.value = '暂时无法确认订单结果，请使用当前选择重试查询或查看订单记录。'
      }
    } else {
      error.value = '暂时无法确认订单结果，请使用当前选择重试查询或查看订单记录。'
    }
  } finally {
    sending.value = false
  }
}

onMounted(load)
onUnmounted(() => controller.abort())
</script>

<template>
  <p v-if="loading" role="status">正在加载静态 IP 报价…</p>
  <div v-else-if="pending">
    <p>静态 IP 购买结果待确认。</p>
    <p v-if="error" role="alert">{{ error }}</p>
    <Button type="button" :disabled="sending" @click="submit">使用原请求重试</Button>
  </div>
  <p v-else-if="closed && !order">暂未开放购买</p>
  <div v-else>
    <p v-if="error" role="alert">{{ error }}</p>
    <Button v-if="!offers.length && error" type="button" variant="outline" @click="load">重新加载</Button>
    <p v-else-if="!offers.length">当前没有可选规格。</p>
    <form v-if="offers.length || order" class="checkout" @submit.prevent="submit">
      <fieldset v-if="!closed" class="offer-filters" :disabled="sending">
        <legend>筛选规格</legend>
        <label>地区
          <select v-model="region">
            <option value="">全部地区</option>
            <option v-for="value in regions" :key="value" :value="value">{{ value }}</option>
          </select>
        </label>
        <label>类型
          <select v-model="ipType">
            <option value="">全部类型</option>
            <option v-for="value in ipTypes" :key="value" :value="value">{{ value }}</option>
          </select>
        </label>
        <label>租期
          <select v-model="termDays">
            <option value="">全部租期</option>
            <option v-for="days in terms" :key="days" :value="days">{{ days }} 天</option>
          </select>
        </label>
        <label class="stock-filter"><input v-model="inStock" type="checkbox"> 仅显示有库存</label>
      </fieldset>
      <p v-if="!closed && !filteredOffers.length" role="status">没有符合筛选条件的规格。</p>
      <label v-if="!closed && filteredOffers.length">规格
        <select v-model="selected" :disabled="sending">
          <option value="" disabled>请选择规格</option>
          <option v-for="item in filteredOffers" :key="offerKey(item)" :value="offerKey(item)">
            {{ item.region }} {{ item.ip_type }} · {{ item.term_days }} 天 · {{ money(item.unit_minor) }}/个 · 可购 {{ item.available ?? 0 }} 个
          </option>
        </select>
      </label>
      <label v-if="offer && !closed">数量
        <Input v-model="rawQuantity" type="number" min="1" :max="offer?.available ?? 0" :disabled="sending" />
      </label>
      <p v-if="offer && !closed">
        单价 {{ money(offer.unit_minor) }}，可购 {{ offer.available ?? 0 }} 个。
        <strong>总价 {{ money(String(total)) }}</strong>
      </p>
      <p v-if="order" role="status">
        订单 #{{ order.id }} ·
        {{ order.status === 'succeeded' ? '已交付' : order.status === 'failed' ? '采购失败，已退回预扣款' : '确认中' }}
        <RouterLink :to="{ path: '/', query: { view: 'ips' } }">查看我的 IP</RouterLink>
      </p>
      <Button v-if="token && !closed" type="submit" :disabled="!canSubmit || sending || !!order">
        {{ sending ? '提交中…' : quote ? '确认购买' : '获取确认报价' }}
      </Button>
      <RouterLink v-else-if="canSubmit" :to="{ path: '/', query: checkoutQuery }">购买</RouterLink>
    </form>
  </div>
</template>

<style scoped>
.checkout { display: grid; gap: 12px; justify-items: start; }
.checkout label { display: grid; gap: 4px; }
.checkout select, .checkout input { max-width: 100%; }
.offer-filters { display: flex; flex-wrap: wrap; align-items: end; gap: 8px 12px; min-width: 0; }
.offer-filters .stock-filter { display: flex; align-items: center; }
</style>
