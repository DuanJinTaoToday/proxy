<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { listProducts, productNames } from './api/product'
import type { Product } from './api/product'
import StaticCheckout from './StaticCheckout.vue'
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
      <nav aria-label="官网导航" class="flex flex-wrap gap-4">
        <RouterLink to="/products">产品</RouterLink>
        <RouterLink to="/products#socks5">SOCKS5 接入说明</RouterLink>
        <RouterLink to="/">控制台</RouterLink>
      </nav>
    </div>
  </header>
  <main class="shell wallet-console">
    <h1>住宅代理产品</h1>
    <section id="socks5" class="panel">
      <h2>动态住宅代理 SOCKS5 接入说明</h2>
      <p>在客户端选择 SOCKS5 和用户名／密码认证。以下为示例配置。</p>
      <dl class="grid gap-2 sm:grid-cols-[max-content_1fr]">
        <dt>网关地址</dt><dd><code>proxy.example.com</code></dd>
        <dt>端口</dt><dd><code>1080</code></dd>
        <dt>用户名</dt><dd><code>demo01.browser</code>（格式为 <code>主账号名.子账号名</code>）</dd>
        <dt>密码</dt><dd><code>example-password</code></dd>
      </dl>
      <p>登录控制台后，在「动态住宅代理 → 代理子账号」选择已启用的账号，查看并复制实际接入配置。</p>
      <p>成功建立的连接按实际上传和下载字节量扣减流量，计量可能包含 SOCKS5 协议字节。失败连接不计量，1 GB = 1,000,000,000 字节。</p>
    </section>
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
      <StaticCheckout v-else />
    </section>
  </main>
</template>

<style scoped>
section, .panel { padding: 16px; }
</style>
