<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElAlert, ElButton, ElDialog, ElEmpty, ElInput, ElInputNumber, ElOption, ElRadioButton, ElRadioGroup, ElSelect } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { supplierNodes, supplierProjects, type ProjectKey, type SupplierNode } from './catalog'
import { deliverIps, formatMoney, priceCents, productLabels, regionName, stockOf, termsOf, type Product } from './demo'
import { addIps, balanceCents, recordStaticPurchase } from './state'

const emit = defineEmits<{ purchased: [count: number] }>()

const productHints: Record<Product, string> = {
  exclusive: '一个 IP 只分配给你一人使用，价格按地区不同。',
  shared: '同一 IP 可能与其他用户共用，各地区统一价格，支持按天、按周购买。',
}

const product = ref<Product>('exclusive')
const project = ref<ProjectKey>('none')
const keyword = ref('')
const continent = ref('all')
const line = ref('all')
const termCode = ref('yp1')
const cart = ref(new Map<number, number>())
const sold = ref(new Map<string, number>())
const confirmOpen = ref(false)

const terms = computed(() => termsOf(supplierNodes[0][product.value].prices))
const term = computed(() => terms.value.find(t => t.code === termCode.value) ?? terms.value[0])
const continents = computed(() => [...new Set(supplierNodes.map(node => node.continent))])
const lines = computed(() => [...new Set(supplierNodes.map(node => node.line))])

function soldKey(node: SupplierNode) { return `${product.value}:${product.value === 'shared' ? project.value : ''}:${node.nodeId}` }
function stock(node: SupplierNode) { return stockOf(node, product.value, project.value) - (sold.value.get(soldKey(node)) ?? 0) }
function unitCents(node: SupplierNode) { return priceCents(node[product.value].prices, term.value.code) ?? 0 }

const groups = computed(() => {
  const term = keyword.value.trim()
  const nodes = supplierNodes
    .filter(node => (continent.value === 'all' || node.continent === continent.value)
      && (line.value === 'all' || node.line === line.value)
      && (!term || regionName(node).includes(term) || node.isp.includes(term)))
    .sort((a, b) => Number(stock(b) > 0) - Number(stock(a) > 0))
  return continents.value
    .map(name => ({ name, nodes: nodes.filter(node => node.continent === name) }))
    .filter(group => group.nodes.length)
})

const cartLines = computed(() => supplierNodes
  .filter(node => cart.value.has(node.nodeId))
  .map(node => ({ node, quantity: cart.value.get(node.nodeId)!, unitCents: unitCents(node) })))
const quantity = computed(() => cartLines.value.reduce((sum, l) => sum + l.quantity, 0))
const totalCents = computed(() => cartLines.value.reduce((sum, l) => sum + l.quantity * l.unitCents, 0))
const shortfallCents = computed(() => Math.max(0, totalCents.value - balanceCents.value))

watch(product, () => {
  cart.value = new Map()
  if (!terms.value.some(t => t.code === termCode.value)) termCode.value = 'yp1'
})

watch(project, () => {
  const next = new Map<number, number>()
  for (const [id, qty] of cart.value) {
    const available = stock(supplierNodes.find(node => node.nodeId === id)!)
    if (available > 0) next.set(id, Math.min(qty, available))
  }
  cart.value = next
})

function setQuantity(node: SupplierNode, value: number | undefined) {
  const next = new Map(cart.value)
  if (!value) next.delete(node.nodeId)
  else next.set(node.nodeId, Math.min(value, stock(node)))
  cart.value = next
}

function submit() {
  const delivered = cartLines.value.flatMap(l => deliverIps(l.node, product.value, l.quantity, term.value.days))
  const nextSold = new Map(sold.value)
  for (const l of cartLines.value) nextSold.set(soldKey(l.node), (nextSold.get(soldKey(l.node)) ?? 0) + l.quantity)
  sold.value = nextSold
  recordStaticPurchase(cartLines.value.map(l => ({
    label: `${productLabels[product.value]} · ${regionName(l.node)} · ${l.node.isp}`,
    quantity: l.quantity, unitCents: l.unitCents, delivered: l.quantity,
  })), term.value)
  addIps(delivered)
  cart.value = new Map()
  confirmOpen.value = false
  emit('purchased', delivered.length)
}
</script>

<template>
  <div class="pricing">
    <section class="pricing-card">
      <div class="product-switch">
        <ElRadioGroup v-model="product" size="large" aria-label="选择产品">
          <ElRadioButton v-for="(label, value) in productLabels" :key="value" :value="value">{{ label }}静态 IP</ElRadioButton>
        </ElRadioGroup>
        <p>{{ productHints[product] }}</p>
      </div>

      <div class="pricing-head">
        <h2>选择国家/地区</h2>
        <ElInput v-model="keyword" class="region-search" :prefix-icon="Search" clearable placeholder="搜索国家、城市或运营商" />
      </div>
      <div v-if="product === 'shared'" class="pricing-filter">
        <span id="project-label">用途</span>
        <ElSelect v-model="project" class="project-select" aria-labelledby="project-label">
          <ElOption v-for="p in supplierProjects" :key="p.pid" :value="p.pid" :label="p.label" />
        </ElSelect>
        <span class="filter-hint">不同用途的可购数量不同</span>
      </div>
      <div v-if="lines.length > 1" class="pricing-filter">
        <span id="line-label">线路</span>
        <ElRadioGroup v-model="line" aria-labelledby="line-label">
          <ElRadioButton value="all">全部</ElRadioButton>
          <ElRadioButton v-for="l in lines" :key="l" :value="l">{{ l }}</ElRadioButton>
        </ElRadioGroup>
      </div>
      <div class="pricing-filter">
        <span id="continent-label">大洲</span>
        <ElRadioGroup v-model="continent" aria-labelledby="continent-label">
          <ElRadioButton value="all">全部</ElRadioButton>
          <ElRadioButton v-for="name in continents" :key="name" :value="name">{{ name }}</ElRadioButton>
        </ElRadioGroup>
      </div>

      <section v-for="group in groups" :key="group.name" class="region-group">
        <h3>{{ group.name }}</h3>
        <div class="region-grid">
          <div v-for="node in group.nodes" :key="node.nodeId" class="region-card"
            :class="{ selected: cart.has(node.nodeId), soldout: stock(node) <= 0 }">
            <span class="type-tag">{{ node.line }}</span>
            <button v-if="!cart.has(node.nodeId)" type="button" class="region-pick" :disabled="stock(node) <= 0"
              :aria-label="`选择 ${regionName(node)} ${node.isp}`" @click="setQuantity(node, 1)">
              <span class="region-code">{{ node.code }}</span>
              <span class="region-text">
                <strong>{{ regionName(node) }}<span class="isp">{{ node.isp }}</span></strong>
                <span class="region-meta">
                  <template v-if="stock(node) > 0">可购 {{ stock(node) }}</template><template v-else>暂时缺货</template>
                  <span class="region-price">{{ formatMoney(unitCents(node)) }}/IP</span>
                </span>
              </span>
            </button>
            <div v-else class="region-pick">
              <span class="region-code">{{ node.code }}</span>
              <span class="region-text">
                <strong>{{ regionName(node) }}<span class="isp">{{ node.isp }}</span></strong>
                <ElInputNumber :model-value="cart.get(node.nodeId)" :min="0" :max="stock(node)" size="small"
                  :aria-label="`${regionName(node)} ${node.isp} 数量`" @update:model-value="setQuantity(node, $event ?? 0)" />
              </span>
            </div>
          </div>
        </div>
      </section>
      <ElEmpty v-if="!groups.length" description="没有符合条件的地区" :image-size="72" />
    </section>

    <aside class="summary-card" aria-label="订单摘要">
      <div class="summary-head">
        <h2>订单摘要 · {{ productLabels[product] }}</h2>
        <ElButton link :disabled="!cartLines.length" @click="cart = new Map()">清空</ElButton>
      </div>
      <div class="summary-terms">
        <span id="term-label">租期</span>
        <ElRadioGroup v-model="termCode" aria-labelledby="term-label">
          <ElRadioButton v-for="t in terms" :key="t.code" :value="t.code">{{ t.name }}</ElRadioButton>
        </ElRadioGroup>
      </div>

      <div class="summary-lines">
        <div v-for="l in cartLines" :key="l.node.nodeId" class="summary-line">
          <div><strong>{{ regionName(l.node) }} · {{ l.node.isp }}</strong><span>{{ formatMoney(l.unitCents) }}/IP</span></div>
          <ElInputNumber :model-value="l.quantity" :min="0" :max="stock(l.node)" size="small"
            :aria-label="`${regionName(l.node)} ${l.node.isp} 数量`" @update:model-value="setQuantity(l.node, $event ?? 0)" />
        </div>
        <ElEmpty v-if="!cartLines.length" description="在左侧选择地区和数量" :image-size="56" />
      </div>

      <dl class="summary-total">
        <div><dt>IP 数量</dt><dd>{{ quantity }} 个</dd></div>
        <div><dt>租期</dt><dd>{{ term.name }} · {{ term.days }} 天</dd></div>
        <div v-if="product === 'shared'"><dt>用途</dt><dd>{{ supplierProjects.find(p => p.pid === project)?.label }}</dd></div>
        <div><dt>钱包余额</dt><dd>{{ formatMoney(balanceCents) }}</dd></div>
        <div class="grand"><dt>总价</dt><dd>{{ formatMoney(totalCents) }}</dd></div>
      </dl>
      <ElAlert v-if="shortfallCents" type="warning" :closable="false" show-icon
        :title="`余额不足，还差 ${formatMoney(shortfallCents)}`" description="请联系运营为钱包充值后再购买。" />
      <ElButton type="primary" size="large" class="summary-submit" :disabled="!quantity || shortfallCents > 0"
        @click="confirmOpen = true">立即购买</ElButton>
    </aside>

    <ElDialog v-model="confirmOpen" title="确认购买" width="min(560px, calc(100vw - 32px))" align-center>
      <p class="dialog-intro">提交时按最新价格和库存复核并锁价。已分配的 IP 立即可用，未能交付的部分退回钱包。</p>
      <div class="quote">
        <div class="quote-heading"><span>{{ productLabels[product] }} · 地区</span><span>数量</span><span>小计</span></div>
        <div v-for="l in cartLines" :key="l.node.nodeId" class="quote-item">
          <strong>{{ regionName(l.node) }} · {{ l.node.isp }}</strong>
          <span>{{ l.quantity }} × {{ formatMoney(l.unitCents) }}</span>
          <span>{{ formatMoney(l.quantity * l.unitCents) }}</span>
        </div>
      </div>
      <dl class="summary-total compact">
        <div><dt>租期</dt><dd>{{ term.name }} · {{ term.days }} 天</dd></div>
        <div><dt>支付后余额</dt><dd>{{ formatMoney(balanceCents - totalCents) }}</dd></div>
        <div class="grand"><dt>应付</dt><dd>{{ formatMoney(totalCents) }}</dd></div>
      </dl>
      <template #footer>
        <ElButton @click="confirmOpen = false">返回修改</ElButton>
        <ElButton type="primary" @click="submit">确认支付</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
