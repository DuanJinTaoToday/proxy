<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ApiError } from './api/control'
import { listProducts, listTrafficPackages, productNames, setProductSale, updateTrafficPackage } from './api/product'
import type { Product, ProductKind, TrafficPackage } from './api/product'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ expired: [] }>()
const products = ref<Product[]>([])
const packages = ref<TrafficPackage[]>([])
const evidence = ref<Record<ProductKind, string>>({ dynamic: '', static: '' })
const quoteReady = ref<Record<ProductKind, boolean>>({ dynamic: false, static: false })
const busy = ref(false)
const error = ref('')
const loaded = ref(false)
const controller = new AbortController()

async function refresh() {
  const [sales, tiers] = await Promise.all([
    listProducts(controller.signal), listTrafficPackages(props.token, controller.signal),
  ])
  products.value = sales
  packages.value = tiers
  loaded.value = true
}

async function run(action: () => Promise<unknown>) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    await action()
  } catch (err) {
    if (controller.signal.aborted) return
    if (err instanceof ApiError && err.status === 401) emit('expired')
    else if (err instanceof ApiError && err.reason === 'PACKAGE_CHANGED') error.value = '套餐已被更新，请重新加载后再修改。'
    else if (err instanceof ApiError && err.reason === 'PRODUCT_QUOTE_NOT_READY') error.value = '请先给所有上架套餐定价，并保留至少一档。'
    else if (err instanceof ApiError && err.status === 400) error.value = '请检查配置、报价确认和验收依据。'
    else error.value = '商品操作失败，请重新加载后再试。'
  } finally {
    busy.value = false
  }
}

function save(pkg: TrafficPackage) {
  return run(async () => {
    await updateTrafficPackage(props.token, pkg)
    await refresh()
  })
}

function toggle(product: Product) {
  return run(async () => {
    await setProductSale(props.token, product.kind, !product.open, quoteReady.value[product.kind], evidence.value[product.kind])
    quoteReady.value[product.kind] = false
    evidence.value[product.kind] = ''
    await refresh()
  })
}

onMounted(() => run(refresh))
onUnmounted(() => controller.abort())
</script>

<template>
  <section class="panel product-settings" aria-labelledby="sale-settings-title">
    <h2 id="sale-settings-title">商品开售状态</h2>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <button type="button" class="secondary" :disabled="busy" @click="run(refresh)">重新加载商品</button>
    <p v-if="busy" role="status">正在处理商品配置…</p>
    <template v-if="loaded">
      <form v-for="product in products" :key="product.kind" class="login-form" @submit.prevent="toggle(product)">
        <h3>{{ productNames[product.kind] }} · {{ product.open ? '已开售' : '未开售' }}</h3>
        <template v-if="!product.open">
          <label>验收依据<input v-model="evidence[product.kind]" maxlength="1000" required></label>
          <label><input v-model="quoteReady[product.kind]" type="checkbox" required> 已确认报价就绪</label>
        </template>
        <button type="submit" :disabled="busy">{{ product.open ? '关闭购买' : '确认开售' }}</button>
      </form>
      <h3>流量套餐</h3>
      <p>首次设置 5 GB 基准价时，系统按默认折扣为尚未修改的 50 GB 和 200 GB 档定价。</p>
      <form v-for="pkg in packages" :key="pkg.id" class="login-form" @submit.prevent="save(pkg)">
        <h4>{{ pkg.id }}</h4>
        <label>容量（字节）<input v-model="pkg.capacity_bytes" inputmode="numeric" pattern="[1-9][0-9]*" required></label>
        <label>总价（分，0 表示未定价）<input v-model="pkg.price_minor" inputmode="numeric" pattern="[0-9]+" placeholder="0"></label>
        <label><input v-model="pkg.listed" type="checkbox"> 上架</label>
        <button type="submit" :disabled="busy">保存套餐</button>
      </form>
    </template>
  </section>
</template>

<style scoped>
.product-settings { padding: 16px; }
.product-settings h2 { margin-bottom: 12px; }
form { margin-top: 16px; }
form h3, form h4 { margin: 0; }
form button { justify-self: start; }
input[type="checkbox"] { min-height: auto; }
</style>
