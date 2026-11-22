<script setup lang="ts">
// 双商品分屏（来自方案 B）：两个商品左右并排，悬停的一侧展开、提亮。用在深色背景上。
import { ref } from 'vue'
import { CircleCheck } from '@element-plus/icons-vue'
import { products } from './home-products'

const focus = ref<'dynamic' | 'static' | null>(null)
const rotating = ['73.41.208.16 · 美国 洛杉矶', '86.12.77.204 · 英国 伦敦', '189.6.33.150 · 巴西 圣保罗', '126.80.4.91 · 日本 东京', '91.64.120.38 · 德国 法兰克福', '49.181.20.7 · 澳大利亚 悉尼']
</script>

<template>
  <div class="split" :class="focus ? `focus-${focus}` : ''" @mouseleave="focus = null">
    <article v-for="p in products" :key="p.key" class="half" :class="[p.key, { closed: !p.onSale }]" @mouseenter="focus = p.key">
      <div class="half-art" aria-hidden="true">
        <div v-if="p.key === 'dynamic'" class="ticker"><span v-for="(ip, i) in [...rotating, ...rotating]" :key="i">{{ ip }}</span></div>
        <div v-else class="fixed-grid"><span v-for="i in 48" :key="i" :class="{ lit: i % 7 === 3 }" /></div>
      </div>
      <div class="half-body">
        <div class="half-top">
          <span class="product-unit">{{ p.unit }}</span>
          <span v-if="!p.onSale" class="closed-tag">暂未开放购买</span>
        </div>
        <h2>{{ p.name }}</h2>
        <p class="half-tagline">{{ p.tagline }}</p>
        <div v-if="p.onSale && p.price" class="half-price"><strong>{{ p.price }}</strong><span>{{ p.priceUnit }}</span></div>
        <div v-else class="half-price soon">开售后公布报价</div>
        <ul>
          <li v-for="b in p.bullets" :key="b"><CircleCheck />{{ b }}</li>
        </ul>
        <a v-if="p.onSale" class="cta primary lg" :href="p.href">查看价格</a>
        <a v-else class="cta outline lg" href="#faq">了解产品</a>
      </div>
    </article>
  </div>
</template>

<style scoped>
.split { max-width: 1240px; margin: 0 auto; display: flex; gap: 18px; min-height: 520px; color: #fff; }
.half { position: relative; flex: 1; overflow: hidden; border-radius: 26px; border: 1px solid rgba(179, 240, 215, .14); transition: flex .45s cubic-bezier(.2, .8, .2, 1), filter .35s, border-color .35s; }
.half.dynamic { background: radial-gradient(120% 90% at 0% 0%, #1d5a49 0%, #0d2a24 55%, #081613 100%); }
.half.static { background: radial-gradient(120% 90% at 100% 0%, #2a4f3a 0%, #10291f 55%, #081613 100%); }
.focus-dynamic .dynamic, .focus-static .static { flex: 1.45; border-color: rgba(179, 240, 215, .45); }
.focus-dynamic .static, .focus-static .dynamic { filter: saturate(.6) brightness(.72); }
.half-art { position: absolute; right: 24px; bottom: 28px; top: 46%; width: 170px; pointer-events: none; overflow: hidden; transition: opacity .3s; -webkit-mask-image: linear-gradient(180deg, transparent, #000 25%, #000 75%, transparent); mask-image: linear-gradient(180deg, transparent, #000 25%, #000 75%, transparent); }
.ticker { position: absolute; right: 0; top: 0; display: flex; flex-direction: column; gap: 14px; font: 13px/1 ui-monospace, Consolas, monospace; color: rgba(179, 240, 215, .45); animation: tick 18s linear infinite; text-align: right; }
.ticker span { white-space: nowrap; }
@keyframes tick { from { transform: translateY(0) } to { transform: translateY(-50%) } }
.fixed-grid { position: absolute; right: 0; top: 50%; transform: translateY(-50%); display: grid; grid-template-columns: repeat(8, 14px); gap: 16px; }
.fixed-grid span { width: 8px; height: 8px; border-radius: 2px; background: rgba(179, 240, 215, .16); }
.fixed-grid .lit { background: var(--mint); box-shadow: 0 0 12px rgba(179, 240, 215, .8); animation: glow 2.4s ease-in-out infinite; }
@keyframes glow { 50% { opacity: .45 } }
.half-body { position: relative; height: 100%; padding: 40px; display: flex; flex-direction: column; gap: 16px; max-width: 400px; }
.focus-dynamic .static .half-art, .focus-static .dynamic .half-art { opacity: 0; }
.half-top { display: flex; gap: 10px; }
.product-unit { font-size: 12px; font-weight: 650; color: var(--mint); background: rgba(179, 240, 215, .1); padding: 4px 10px; border-radius: 99px; }
.closed-tag { font-size: 12px; font-weight: 650; color: var(--amber); background: var(--amber-soft); padding: 4px 10px; border-radius: 99px; }
h2 { font-size: 36px; font-weight: 800; }
.half-tagline { color: #a9c1ba; line-height: 1.7; }
.half-price strong { font-size: 52px; font-weight: 800; color: #fff; letter-spacing: -.02em; }
.half-price span { color: #a9c1ba; margin-left: 6px; font-size: 16px; }
.half-price.soon { font-size: 22px; color: #a9c1ba; padding: 18px 0; }
ul { list-style: none; padding: 0; margin: 0 0 12px; display: grid; gap: 10px; color: #d9e6e2; }
li { display: flex; align-items: center; gap: 8px; }
li svg { color: var(--mint); }
.half-body .cta { align-self: flex-start; margin-top: auto; }
.closed .half-price strong { color: #a9c1ba; }

@media (max-width: 1100px) { .half-art { display: none; } }
@media (max-width: 860px) { .split { flex-direction: column; } .focus-dynamic .static, .focus-static .dynamic { filter: none; } }
</style>
