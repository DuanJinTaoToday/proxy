<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { listProducts, productNames } from './api/product'
import type { Product } from './api/product'
import { flowAmount, money } from './wallet-format'

const products = ref<Product[]>([])
const loading = ref(true)
const loadError = ref(false)
const controller = new AbortController()

async function load() {
  loading.value = true
  loadError.value = false
  try {
    products.value = await listProducts(controller.signal)
  } catch {
    if (!controller.signal.aborted) loadError.value = true
  } finally {
    loading.value = false
  }
}

onMounted(load)
onUnmounted(() => controller.abort())
</script>

<template>
  <header class="topbar">
    <div class="shell topbar-inner">
      <strong>住宅代理</strong>
      <RouterLink to="/">控制台</RouterLink>
    </div>
  </header>
  <main class="shell wallet-console">
    <h1>住宅代理产品</h1>
    <p v-if="loading" role="status">正在加载商品…</p>
    <div v-else-if="loadError" class="panel">
      <p role="alert">暂时无法查询商品状态。</p>
      <button class="secondary" type="button" @click="load">重新加载</button>
    </div>
    <section v-for="product in products" v-else :key="product.kind" class="panel">
      <h2>{{ productNames[product.kind] }}</h2>
      <p>{{ product.description }}</p>
      <p v-if="!product.can_purchase">暂未开放购买</p>
      <ul v-else-if="product.kind === 'dynamic'">
        <li v-for="pkg in product.packages" :key="pkg.id">
          {{ flowAmount(pkg.capacity_bytes) }} · {{ money(pkg.price_minor) }}
          <RouterLink :to="{ path: '/', query: { view: 'packages', package: pkg.id } }">购买</RouterLink>
        </li>
      </ul>
    </section>
  </main>
</template>

<style scoped>
section, .panel { padding: 16px; }
</style>
