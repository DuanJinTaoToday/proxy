<script setup lang="ts">
// 方案 B · 双商品分屏：首屏一分为二，两个商品直接做主角。
import ProductSplit from './ProductSplit.vue'
import { formatStat, heroStats } from './home-demo'
</script>

<template>
  <section class="hero hero-b">
    <div class="b-head">
      <p class="eyebrow"><span class="live-dot" />全球住宅代理网络</p>
      <h1>两种住宅代理，<em>一个账户统一管理</em></h1>
    </div>
    <ProductSplit />
    <dl class="b-stats">
      <div v-for="stat in heroStats" :key="stat.label"><dd>{{ formatStat(stat, stat.value) }}</dd><dt>{{ stat.label }}</dt></div>
    </dl>
  </section>
</template>

<style scoped>
.hero-b { padding: 128px 24px 40px; }
.b-head { max-width: 1240px; margin: 0 auto 36px; text-align: center; }
.b-head .eyebrow { justify-content: center; }
h1 { font-size: clamp(34px, 4vw, 54px); font-weight: 800; margin-top: 14px; }
h1 em { font-style: normal; color: var(--mint); }
.b-stats { max-width: 1240px; margin: 36px auto 0; display: grid; grid-template-columns: repeat(4, 1fr); border-top: 1px solid rgba(179, 240, 215, .12); }
.b-stats div { padding: 22px 0 0; text-align: center; }
.b-stats dd { margin: 0; font-size: 26px; font-weight: 800; color: #fff; }
.b-stats dt { color: #8fb0a6; font-size: 13px; margin-top: 4px; }
@media (max-width: 860px) { .b-stats { grid-template-columns: repeat(2, 1fr); } }
</style>
