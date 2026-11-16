<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { ApiError } from './api/control'
import { listProducts, type TrafficPackage } from './api/product'
import { getWallet, purchaseTrafficPackage, type Order, type WalletTenant } from './api/wallet'
import { flowAmount, money } from './wallet-format'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'

const props = defineProps<{ token: string; account: WalletTenant }>()
const emit = defineEmits<{ expired: []; wallet: [WalletTenant] }>()
const route = useRoute()
const controller = new AbortController()
const packages = ref<TrafficPackage[]>([])
const selection = ref('')
const loading = ref(true)
const loadError = ref(false)
const submitting = ref(false)
const error = ref('')
const order = ref<Order>()
const balancePending = ref(false)
const selected = computed(() => packages.value.find(pkg => pkg.id === selection.value))

interface PendingPurchase {
  key: string
  package: TrafficPackage
}

const storageKey = 'proxy-console.pending-traffic.' + props.account.tenant_id
const pending = ref<PendingPurchase>()
try {
  const saved = sessionStorage.getItem(storageKey)
  if (saved) pending.value = JSON.parse(saved) as PendingPurchase
} catch {
  error.value = '无法读取本次会话的购买记录。'
}

const confirmation = computed(() => pending.value?.package ?? selected.value)

function restoreSelection() {
  const requested = route.query.package
  if (typeof requested === 'string') selection.value = requested
}

async function loadCatalog() {
  loading.value = true
  loadError.value = false
  try {
    const products = await listProducts(controller.signal)
    const dynamic = products.find(product => product.kind === 'dynamic')
    packages.value = dynamic?.can_purchase ? dynamic.packages ?? [] : []
  } catch {
    if (!controller.signal.aborted) loadError.value = true
  } finally {
    loading.value = false
  }
}

async function refreshWallet() {
  balancePending.value = true
  try {
    emit('wallet', await getWallet(props.token, controller.signal))
    balancePending.value = false
  } catch (failure) {
    if (failure instanceof ApiError && failure.status === 401) emit('expired')
  }
}

function clearPending(attempt: PendingPurchase) {
  try {
    if (sessionStorage.getItem(storageKey) === JSON.stringify(attempt)) sessionStorage.removeItem(storageKey)
  } catch {
    error.value = '无法清除本次会话的购买记录。'
  }
  pending.value = undefined
}

async function submit() {
  if (submitting.value || !confirmation.value) return
  let attempt: PendingPurchase
  try {
    attempt = pending.value ?? { key: crypto.randomUUID(), package: confirmation.value }
    sessionStorage.setItem(storageKey, JSON.stringify(attempt))
  } catch {
    error.value = '无法保存购买请求，请允许此网站使用会话存储后重试。'
    return
  }
  pending.value = attempt
  submitting.value = true
  error.value = ''
  order.value = undefined
  try {
    order.value = await purchaseTrafficPackage(props.token, {
      package_id: attempt.package.id, package_version: attempt.package.version, request_key: attempt.key,
    })
    clearPending(attempt)
    selection.value = ''
    await refreshWallet()
  } catch (failure) {
    if (failure instanceof ApiError && failure.status === 401) {
      emit('expired')
      return
    }
    if (!(failure instanceof ApiError)) {
      error.value = '暂时无法确认成交结果，请用原请求重试。'
      return
    }
    switch (failure.reason) {
      case 'WALLET_INSUFFICIENT_FUNDS':
        clearPending(attempt)
        const shortfall = failure.metadata.shortfall_minor
        error.value = (shortfall ? `钱包余额不足，还差 ${money(shortfall)}。` : '钱包余额不足。') +
          '请联系运营充值后重新确认。'
        selection.value = ''
        await refreshWallet()
        break
      case 'PACKAGE_CHANGED':
        clearPending(attempt)
        error.value = '套餐报价已更新，请核对新报价后重新确认。'
        await loadCatalog()
        break
      case 'PRODUCT_SALE_CLOSED':
      case 'PACKAGE_UNAVAILABLE':
      case 'PACKAGE_NOT_FOUND':
        clearPending(attempt)
        error.value = '该套餐当前无法购买。'
        await loadCatalog()
        break
      case 'INVALID_WALLET_REQUEST':
      case 'WALLET_REQUEST_CONFLICT':
        clearPending(attempt)
        error.value = '提交失败，请重新确认后提交。'
        break
      default:
        error.value = '暂时无法确认成交结果，请用原请求重试。'
    }
  } finally {
    submitting.value = false
  }
}

watch(() => route.query.package, restoreSelection, { immediate: true })
onMounted(loadCatalog)
onUnmounted(() => controller.abort())
</script>

<template>
  <Card class="m-4 max-w-2xl md:m-6">
    <CardHeader>
      <h2 class="text-lg font-semibold">固定流量套餐</h2>
      <p class="text-sm text-muted-foreground">钱包余额 {{ money(account.available_minor) }} · 可用流量 {{ flowAmount(account.available_traffic_bytes) }}</p>
      <p v-if="balancePending && !submitting" class="text-sm" role="status">
        余额尚未刷新。 <Button variant="outline" type="button" @click="refreshWallet">刷新余额</Button>
      </p>
    </CardHeader>
    <CardContent class="grid gap-4">
      <p v-if="order" role="status">订单 #{{ order.id }} 成交，到账 {{ flowAmount(order.delivered_bytes) }}，实扣 {{ money(order.debited_minor) }}。</p>
      <p v-if="error" class="text-sm text-destructive" role="alert">{{ error }}</p>
      <p v-if="loading" role="status">正在加载报价…</p>
      <div v-else-if="loadError">
        <p role="alert">无法获取套餐报价。</p>
        <Button variant="outline" type="button" @click="loadCatalog">重新加载</Button>
      </div>
      <fieldset v-else-if="packages.length" class="grid gap-2" :disabled="submitting || !!pending">
        <legend class="mb-2 text-sm font-medium">选择一份套餐</legend>
        <label v-for="pkg in packages" :key="pkg.id" class="flex items-center gap-3 rounded-md border p-3 text-sm">
          <input v-model="selection" type="radio" name="traffic-package" :value="pkg.id" @change="error = ''; order = undefined" />
          <span>{{ flowAmount(pkg.capacity_bytes) }}</span>
          <strong class="ml-auto">{{ money(pkg.price_minor) }}</strong>
        </label>
      </fieldset>
      <p v-else>暂未开放购买。</p>
      <p v-if="!loading && !loadError && selection && !selected && !pending" class="text-sm">所选套餐目前不可购买。</p>
      <div v-if="confirmation && (pending || (!loading && !loadError))" class="grid gap-3 rounded-md border p-3">
        <p>购买一份 {{ flowAmount(confirmation.capacity_bytes) }} 流量套餐，合计 {{ money(confirmation.price_minor) }}。流量长期有效，到账后先抵扣欠用流量。</p>
        <Button class="justify-self-start" type="button" :disabled="submitting" @click="submit">
          {{ submitting ? '正在结算…' : pending ? '重试原提交' : '确认并扣款' }}
        </Button>
      </div>
    </CardContent>
  </Card>
</template>
