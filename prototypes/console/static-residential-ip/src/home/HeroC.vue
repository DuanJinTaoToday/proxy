<script setup lang="ts">
// 方案 C · 实时控制台：右侧仿终端循环演示接入，左侧是标题与按钮。
import HeroCanvas from './HeroCanvas.vue'
import LiveTerminal from './LiveTerminal.vue'
import { formatStat, heroStats } from './home-demo'
import { primaryHref } from './home-products'

defineProps<{ hero: 'map' | 'globe' }>()
</script>

<template>
  <section class="hero hero-c">
    <HeroCanvas :mode="hero" :globe-x="0.66" :globe-scale="0.5" :opacity="0.6" />
    <div class="hero-shade" />
    <div class="c-grid">
      <div class="c-copy">
        <p class="eyebrow"><span class="live-dot" />SOCKS5 · 真人住宅出口</p>
        <h1>一行配置<br><em>接入全球住宅网络</em></h1>
        <p class="lead">换一个会话标识，就换一个国家的出口。动态流量按字节如实计量、长期有效；静态住宅 IP 独享固定，续费保留原号。</p>
        <div class="hero-ctas">
          <a class="cta primary lg" :href="primaryHref">查看价格</a>
          <a class="cta outline lg" href="#integration">查看接入示例</a>
        </div>
        <dl class="c-stats">
          <div v-for="stat in heroStats" :key="stat.label"><dd>{{ formatStat(stat, stat.value) }}</dd><dt>{{ stat.label }}</dt></div>
        </dl>
      </div>
      <LiveTerminal class="c-term" />
    </div>
  </section>
</template>

<style scoped>
.hero-c { min-height: max(700px, 100vh); }
.hero-shade { position: absolute; inset: 0; pointer-events: none; background: linear-gradient(90deg, rgba(6, 18, 16, .9) 0%, rgba(6, 18, 16, .6) 40%, rgba(6, 18, 16, .15) 75%); }
.c-grid { position: relative; max-width: 1240px; margin: 0 auto; padding: 150px 24px 70px; display: grid; grid-template-columns: 1fr 1.05fr; gap: 56px; align-items: center; pointer-events: none; }
.c-grid a, .c-term { pointer-events: auto; }
h1 { font-size: clamp(38px, 4.6vw, 62px); line-height: 1.12; font-weight: 800; margin: 18px 0 22px; }
h1 em { font-style: normal; color: var(--mint); }
.lead { font-size: 17px; line-height: 1.75; color: #b7cdc6; }
.hero-ctas { display: flex; gap: 14px; margin-top: 32px; }
.c-stats { display: grid; grid-template-columns: repeat(4, auto); gap: 28px; margin: 56px 0 0; }
.c-stats dd { margin: 0; font-size: 22px; font-weight: 800; color: #fff; }
.c-stats dt { font-size: 12px; color: #8fb0a6; margin-top: 4px; }
@media (max-width: 960px) { .c-grid { grid-template-columns: 1fr; padding-top: 110px; } .c-stats { grid-template-columns: repeat(2, 1fr); } }
</style>
