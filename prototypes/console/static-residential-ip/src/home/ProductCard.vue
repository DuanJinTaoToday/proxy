<script setup lang="ts">
import { CircleCheck } from '@element-plus/icons-vue'
import type { HomeProduct } from './home-products'

defineProps<{ product: HomeProduct, tone?: 'light' | 'dark' }>()
</script>

<template>
  <article class="product-card" :class="[tone ?? 'light', { closed: !product.onSale }]">
    <div class="product-top">
      <span class="product-unit">{{ product.unit }}</span>
      <span v-if="!product.onSale" class="closed-tag">暂未开放购买</span>
    </div>
    <h3>{{ product.name }}</h3>
    <p class="product-tagline">{{ product.tagline }}</p>
    <ul>
      <li v-for="b in product.bullets" :key="b"><CircleCheck />{{ b }}</li>
    </ul>
    <div class="product-foot">
      <div v-if="product.onSale && product.price" class="product-price"><strong>{{ product.price }}</strong><span>{{ product.priceUnit }}</span></div>
      <div v-else class="product-price muted">开售后公布报价</div>
      <a v-if="product.onSale" class="cta primary" :href="product.href">查看价格</a>
      <a v-else class="cta ghost" href="#faq">了解产品</a>
    </div>
  </article>
</template>

<style scoped>
.product-card { position: relative; display: flex; flex-direction: column; gap: 14px; padding: 28px; border-radius: 20px; background: #fff; border: 1px solid var(--line); box-shadow: 0 18px 40px -28px rgba(20, 37, 34, .45); transition: transform .25s, box-shadow .25s; }
.product-card:hover { transform: translateY(-4px); box-shadow: 0 26px 50px -28px rgba(20, 37, 34, .55); }
.product-card.dark { background: rgba(255, 255, 255, .04); border-color: rgba(179, 240, 215, .16); color: #e8f3ef; box-shadow: none; backdrop-filter: blur(6px); }
.product-top { display: flex; justify-content: space-between; align-items: center; }
.product-unit { font-size: 12px; font-weight: 650; letter-spacing: .04em; color: var(--teal); background: var(--teal-soft); padding: 4px 10px; border-radius: 99px; }
.dark .product-unit { color: var(--mint); background: rgba(179, 240, 215, .1); }
.closed-tag { font-size: 12px; font-weight: 650; color: var(--amber); background: var(--amber-soft); padding: 4px 10px; border-radius: 99px; }
h3 { font-size: 24px; font-weight: 750; }
.product-tagline { color: var(--muted); line-height: 1.6; }
.dark .product-tagline { color: #a9c1ba; }
ul { list-style: none; padding: 0; margin: 4px 0 0; display: grid; gap: 10px; }
li { display: flex; align-items: center; gap: 8px; }
li svg { color: var(--teal); }
.dark li svg { color: var(--mint); }
.product-foot { margin-top: auto; padding-top: 18px; border-top: 1px dashed var(--line); display: flex; align-items: center; justify-content: space-between; gap: 12px; }
.dark .product-foot { border-color: rgba(179, 240, 215, .18); }
.product-price strong { font-size: 30px; font-weight: 800; color: var(--teal); }
.dark .product-price strong { color: var(--mint); }
.product-price span { color: var(--muted); margin-left: 4px; }
.product-price.muted { color: var(--muted); }
.closed { opacity: .92; }
</style>
