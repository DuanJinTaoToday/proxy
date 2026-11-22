<script setup lang="ts">
// 方案 A · 数据冲击：全屏深色背景铺满动效，大号数字逐个滚动计数。
import { onMounted, ref } from 'vue'
import HeroCanvas from './HeroCanvas.vue'
import { formatStat, heroStats } from './home-demo'
import { primaryHref } from './home-products'

defineProps<{ hero: 'map' | 'globe' }>()

const shown = ref(heroStats.map(() => 0))
onMounted(() => {
  const start = performance.now()
  const run = (now: number) => {
    let done = true
    heroStats.forEach((stat, i) => {
      const local = Math.min(Math.max((now - start - i * 180) / 1600, 0), 1)
      if (local < 1) done = false
      shown.value[i] = stat.value * (1 - (1 - local) ** 3)
    })
    if (!done) requestAnimationFrame(run)
  }
  requestAnimationFrame(run)
})
</script>

<template>
  <section class="hero hero-a">
    <HeroCanvas :mode="hero" :globe-x="0.72" :globe-scale="0.34" :opacity="hero === 'map' ? 0.9 : 1" />
    <div class="hero-shade" />
    <div class="hero-inner" :class="hero">
      <p class="eyebrow"><span class="live-dot" />全球住宅代理网络 · 实时在线</p>
      <h1>全球真实住宅 IP<br><em>让出海业务稳定在线</em></h1>
      <p class="lead">9000 万+ 真人住宅 IP，覆盖 195+ 国家和地区。动态住宅按 GB 计费、流量长期有效；静态住宅 IP 独享固定，到期前续费保留原号。</p>
      <div class="hero-ctas">
        <a class="cta primary lg" :href="primaryHref">查看价格</a>
        <a class="cta outline lg" href="/">免费注册</a>
      </div>
      <dl class="stat-row">
        <div v-for="(stat, i) in heroStats" :key="stat.label">
          <dt>{{ stat.label }}</dt>
          <dd>{{ formatStat(stat, shown[i]) }}</dd>
        </div>
      </dl>
    </div>
    <p v-if="hero === 'globe'" class="drag-hint">拖动地球旋转</p>
  </section>
</template>

<style scoped>
.hero-a { min-height: max(720px, 100vh); }
.hero-shade { position: absolute; inset: 0; pointer-events: none; background: linear-gradient(90deg, rgba(6, 18, 16, .92) 0%, rgba(6, 18, 16, .55) 42%, rgba(6, 18, 16, 0) 70%), linear-gradient(0deg, rgba(6, 18, 16, .9), rgba(6, 18, 16, 0) 30%); }
.hero-inner { position: relative; max-width: 1240px; margin: 0 auto; padding: 170px 24px 80px; pointer-events: none; }
.hero-inner > * { max-width: 640px; }
.hero-inner a { pointer-events: auto; }
h1 { font-size: clamp(40px, 5.2vw, 68px); line-height: 1.12; font-weight: 800; letter-spacing: -.01em; margin: 18px 0 22px; }
h1 em { font-style: normal; background: linear-gradient(90deg, #b3f0d7, #5fe3b0 60%, #d8fff0); -webkit-background-clip: text; background-clip: text; color: transparent; }
.lead { font-size: 18px; line-height: 1.75; color: #b7cdc6; }
.hero-ctas { display: flex; gap: 14px; margin-top: 36px; }
.stat-row { display: grid; grid-template-columns: repeat(4, auto); gap: 44px; margin: 72px 0 0; max-width: none !important; }
.stat-row dt { font-size: 13px; color: #8fb0a6; order: 2; }
.stat-row div { display: flex; flex-direction: column-reverse; gap: 6px; }
.stat-row dd { margin: 0; font-size: 34px; font-weight: 800; color: #fff; font-variant-numeric: tabular-nums; }
.drag-hint { position: absolute; right: 12%; bottom: 64px; color: #7fa398; font-size: 12px; letter-spacing: .12em; pointer-events: none; }
@media (max-width: 860px) { .stat-row { grid-template-columns: repeat(2, 1fr); gap: 24px; } .hero-inner { padding-top: 120px; } .drag-hint { display: none; } }
</style>
