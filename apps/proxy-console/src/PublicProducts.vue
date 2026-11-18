<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { ArrowRight, ChartColumn, Network, Terminal } from 'lucide-vue-next'
import { listProducts } from './api/product'
import type { Product, ProductKind } from './api/product'
import brandMark from './assets/kunxing-mark.png'
import StaticCheckout from './StaticCheckout.vue'
import { flowAmount, money } from './wallet-format'

const offers = {
  dynamic: { name: '动态住宅代理', unit: '按 GB 购买流量', description: '通过 SOCKS5 接入，按需补充流量，在控制台查看每日用量和余额。' },
  static: { name: '静态住宅 IP', unit: '按 IP 数量和租期购买', description: '租用固定的住宅 IP，集中查看到期时间，并在到期前办理保号续费。' },
} satisfies Record<ProductKind, { name: string; unit: string; description: string }>

const preview = ref<ProductKind>('dynamic')
const menu = ref<HTMLDetailsElement>()
const menuToggle = ref<HTMLElement>()

function closeMenuOutside(event: PointerEvent) {
  if (menu.value && !event.composedPath().includes(menu.value)) menu.value.open = false
}

function selectProduct(kind: ProductKind) {
  if (menu.value) menu.value.open = false
  document.getElementById(kind)?.focus({ preventScroll: true })
}

function dismissMenu() {
  if (!menu.value?.open) return
  menu.value.open = false
  menuToggle.value?.focus()
}

const products = ref<Product[]>([])
const productKinds: readonly ProductKind[] = ['dynamic', 'static']
const catalogOffers = computed(() => {
  const productsByKind = new Map(products.value.map(product => [product.kind, product]))
  return productKinds.map(kind => ({
    kind,
    ...offers[kind],
    product: productsByKind.get(kind),
  }))
})
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

onMounted(() => {
  document.addEventListener('pointerdown', closeMenuOutside)
  void load()
})
onUnmounted(() => {
  document.removeEventListener('pointerdown', closeMenuOutside)
  controller.abort()
})
</script>

<template>
  <div class="product-site">
    <a class="skip-link" href="#product-main">跳到主要内容</a>
    <header class="product-header">
      <div class="top product-wrap">
        <RouterLink class="brand" to="/"><img :src="brandMark" alt="" />鲲星代理</RouterLink>
        <nav aria-label="官网导航">
          <a class="nav-home" href="#product-main">首页</a>
          <details ref="menu" @keydown.esc="dismissMenu">
            <summary ref="menuToggle">代理产品</summary>
            <div class="menu">
              <a v-for="(offer, kind) in offers" :key="kind" :href="'#' + kind" @click="selectProduct(kind)">{{ offer.name }}</a>
            </div>
          </details>
        </nav>
        <div class="access">
          <RouterLink class="signin" to="/console">登录</RouterLink>
          <RouterLink class="btn primary small" :to="{ name: 'console', query: { register: '1' } }">注册</RouterLink>
        </div>
      </div>
    </header>
    <main id="product-main" tabindex="-1">
      <section class="product-hero" aria-labelledby="headline">
        <div class="product-wrap">
          <h1 id="headline">换个出口，<wbr>看见更多，<wbr><span>发现机会</span></h1>
          <p class="lead">拥有1亿+来自200多个国家/地区的住宅IP，专为大规模数据抓取设计。完美适配AI、BI等场景，有效提升您的业务成功率。</p>
          <div class="actions">
            <RouterLink class="btn primary" :to="{ name: 'console', query: { register: '1' } }">开始使用 <span aria-hidden="true">&nbsp;→</span></RouterLink>
            <a class="btn secondary" href="#plans">查看产品</a>
          </div>
          <div class="product-preview">
            <section class="preview-console" aria-label="控制台示意">
              <div class="console-top"><span>鲲星代理控制台</span><span class="sample">界面示意</span></div>
              <div class="console-body">
                <div class="switches" role="group" aria-label="切换产品示意">
                  <button
                    v-for="(offer, kind) in offers"
                    :key="kind"
                    type="button"
                    :aria-pressed="preview === kind"
                    :aria-controls="kind + '-panel'"
                    @click="preview = kind"
                  >{{ offer.name }}</button>
                </div>
                <div class="demo" id="dynamic-panel" :hidden="preview !== 'dynamic'">
                  <div class="demo-heading">
                    <h2>动态住宅代理</h2>
                    <span class="preview-balance">剩余流量 <strong>24.8 GB</strong></span>
                  </div>
                  <div class="demo-grid">
                    <section class="metric">
                      <h3>近 7 天用量</h3>
                      <div class="chart" role="img" aria-label="近 7 天用量示意，约 1 至 3 GB 每天">
                        <div class="chart-axis"><span>3 GB</span><span>2 GB</span><span>1 GB</span><span>0</span></div>
                        <div class="bars">
                          <span v-for="(height, day) in [32, 49, 40, 74, 57, 84, 69]" :key="day" :style="{ '--height': height + '%' }"></span>
                        </div>
                        <div class="chart-days"><span>周一</span><span>周三</span><span>周五</span><span>周日</span></div>
                      </div>
                    </section>
                    <section class="metric">
                      <h3>代理配置</h3>
                      <dl class="config">
                        <div><dt>协议</dt><dd>SOCKS5</dd></div>
                        <div><dt>网关地址</dt><dd><code>gateway.example.com</code></dd></div>
                        <div><dt>端口</dt><dd><code>1080</code></dd></div>
                        <div><dt>用户名</dt><dd><code>demo.team1</code></dd></div>
                      </dl>
                    </section>
                  </div>
                </div>
                <div class="demo" id="static-panel" :hidden="preview !== 'static'">
                  <div class="demo-heading">
                    <h2>静态住宅 IP</h2>
                    <span class="preview-balance">租用中 <strong>3 个 IP</strong></span>
                  </div>
                  <div class="demo-grid">
                    <section class="metric">
                      <h3>租用概览</h3>
                      <dl class="config">
                        <div><dt>当前状态</dt><dd>租用中</dd></div>
                        <div><dt>最近到期</dt><dd>剩余 12 天</dd></div>
                        <div><dt>续费方式</dt><dd>到期前手动保号续费</dd></div>
                      </dl>
                    </section>
                    <section class="metric">
                      <h3>代理配置</h3>
                      <dl class="config">
                        <div><dt>IP 地址</dt><dd><code>203.0.113.24</code></dd></div>
                        <div><dt>端口</dt><dd><code>1080</code></dd></div>
                        <div><dt>用户名</dt><dd><code>demo-user</code></dd></div>
                      </dl>
                    </section>
                  </div>
                </div>
              </div>
            </section>
          </div>
        </div>
      </section>

      <section id="plans" class="products" aria-labelledby="product-title">
        <div class="product-wrap">
          <h2 id="product-title">代理产品</h2>
          <p v-if="loading" class="product-state" role="status">正在加载商品…</p>
          <div v-else-if="loadError" class="product-state">
            <p role="alert">暂时无法查询商品状态。</p>
            <button class="btn secondary" type="button" @click="load">重新加载</button>
          </div>
          <p v-else-if="products.length === 0" class="product-state">暂无商品。</p>
          <div class="cards">
            <article v-for="offer in catalogOffers" :id="offer.kind" :key="offer.kind" class="card" tabindex="-1">
              <h3>{{ offer.name }}</h3>
              <p class="unit">{{ offer.unit }}</p>
              <p class="description">{{ offer.product?.description || offer.description }}</p>
              <RouterLink class="product-link" to="/console">前往控制台 <span aria-hidden="true">→</span></RouterLink>
              <p v-if="offer.product && !offer.product.can_purchase" class="product-status">暂未开放购买</p>
              <ul v-else-if="offer.kind === 'dynamic' && offer.product?.can_purchase" class="package-list">
                <li v-for="pkg in offer.product.packages" :key="pkg.id">
                  <strong>{{ flowAmount(pkg.capacity_bytes) }}</strong>
                  <span>{{ money(pkg.price_minor) }}</span>
                  <RouterLink class="btn secondary" :to="{ name: 'console', query: { view: 'packages', package: pkg.id } }">购买套餐</RouterLink>
                </li>
              </ul>
              <div v-else-if="offer.kind === 'static' && offer.product?.can_purchase" class="public-checkout">
                <StaticCheckout />
              </div>
            </article>
          </div>
        </div>
      </section>

      <section id="socks5" class="product-section connection-guide">
        <div class="product-wrap">
          <h2>动态住宅代理接入</h2>
          <ol>
            <li><Network :size="25" aria-hidden="true" /><h3>选择代理子账号</h3><p>在控制台打开「动态住宅代理 → 代理子账号」，选择已启用的账号并复制实际接入配置。</p></li>
            <li><Terminal :size="25" aria-hidden="true" /><h3>配置 SOCKS5 客户端</h3><p>选择 SOCKS5 和用户名／密码认证，填入复制的网关地址、端口、用户名和密码。</p></li>
            <li><ChartColumn :size="25" aria-hidden="true" /><h3>核对日用量</h3><p>成功连接按实际上传和下载字节扣量，可能包含 SOCKS5 协议字节。失败连接不计量。1 GB = 1,000,000,000 字节。</p></li>
          </ol>
          <dl class="connection-example">
            <div><dt>协议</dt><dd><code>SOCKS5</code></dd></div>
            <div><dt>网关地址示例</dt><dd><code>proxy.example.com</code></dd></div>
            <div><dt>端口示例</dt><dd><code>1080</code></dd></div>
            <div><dt>用户名格式</dt><dd><code>主账号名.子账号名</code></dd></div>
          </dl>
          <RouterLink class="btn primary" :to="{ name: 'console', query: { view: 'subaccounts' } }">打开代理子账号 <ArrowRight :size="18" /></RouterLink>
        </div>
      </section>
    </main>
    <footer class="product-footer">
      <div class="bottom product-wrap">
        <RouterLink to="/">鲲星代理</RouterLink>
        <div class="footer-links">
          <a href="#socks5">接入说明</a>
          <RouterLink to="/console">租户控制台</RouterLink>
        </div>
      </div>
    </footer>
  </div>
</template>

<style scoped src="./products.css"></style>
