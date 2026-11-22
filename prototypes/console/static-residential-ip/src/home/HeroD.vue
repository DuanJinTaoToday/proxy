<script setup lang="ts">
// 方案 D 首屏：地图动效、轮换标题和唯一主按钮「免费注册」。
// 高度收在视口以内，常见桌面尺寸下能看到下一段的开头；轮换词占固定高度的一行，切换时正文和按钮不移动。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import HeroCanvas from './HeroCanvas.vue'

defineProps<{ hero: 'map' | 'globe' }>()

const phrases = ['跑通跨境店铺', '养稳社媒矩阵', '采集公开数据', '核对海外广告', '观察本地市场']
const index = ref(0)
let timer = 0
onMounted(() => { timer = window.setInterval(() => { index.value = (index.value + 1) % phrases.length }, 2800) })
onBeforeUnmount(() => clearInterval(timer))

// 示例出口，仅作氛围展示。
const exits = ['美国 洛杉矶', '英国 伦敦', '日本 东京', '德国 法兰克福', '巴西 圣保罗', '新加坡', '澳大利亚 悉尼', '加拿大 多伦多', '阿联酋 迪拜']
</script>

<template>
  <section class="hero hero-d">
    <HeroCanvas :mode="hero" :globe-x="0.72" :globe-scale="0.36" :opacity="hero === 'map' ? 0.85 : 1" />
    <div class="hero-shade" />
    <div class="d-inner">
      <p class="eyebrow"><span class="live-dot" />全球住宅代理网络</p>
      <h1>
        <span class="line">用真实住宅 IP</span>
        <span class="line rotator" aria-live="polite">
          <Transition name="swap"><em :key="index">{{ phrases[index] }}</em></Transition>
        </span>
      </h1>
      <p class="lead">动态住宅按流量计费，静态住宅 IP 按个租用。注册一个账户，两种都能用。</p>
      <div class="d-ctas">
        <a class="cta primary lg" href="/">免费注册</a>
        <a class="text-link" href="#products">先看看怎么选 ↓</a>
      </div>
    </div>
    <div class="exit-strip" aria-hidden="true">
      <span class="exit-label">示例出口</span>
      <div class="exit-rail">
        <div class="exit-track"><span v-for="(e, i) in [...exits, ...exits]" :key="i"><i />{{ e }}</span></div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero-d { min-height: clamp(560px, calc(100vh - 120px), 860px); display: flex; flex-direction: column; background: radial-gradient(80% 70% at 75% 40%, #12382f 0%, #0a1d19 55%, #050e0c 100%); }
.hero-d::after { display: none; }
.hero-shade { position: absolute; inset: 0; pointer-events: none; background: linear-gradient(90deg, rgba(5, 14, 12, .94) 0%, rgba(5, 14, 12, .6) 40%, rgba(5, 14, 12, 0) 70%), linear-gradient(0deg, #050e0c 0%, rgba(5, 14, 12, 0) 22%); }
.d-inner { position: relative; flex: 1; width: 100%; max-width: 1240px; margin: 0 auto; padding: 120px 24px 48px; display: flex; flex-direction: column; justify-content: center; align-items: flex-start; pointer-events: none; }
.d-inner a { pointer-events: auto; }
h1 { display: grid; font-size: clamp(42px, 5vw, 96px); line-height: 1.12; font-weight: 850; letter-spacing: -.02em; margin: 22px 0 24px; }
.line { display: block; white-space: nowrap; }
/* 固定一行高度并让新旧两个词叠在同一格里，切换不改变布局。 */
.rotator { display: grid; height: 1.12em; overflow: hidden; }
.rotator em { grid-area: 1 / 1; font-style: normal; background: linear-gradient(90deg, #b3f0d7, #5fe3b0 60%, #d8fff0); -webkit-background-clip: text; background-clip: text; color: transparent; }
.swap-enter-active { transition: transform .5s cubic-bezier(.2, .8, .2, 1) .12s, opacity .4s .12s; }
.swap-leave-active { transition: transform .3s ease-in, opacity .25s; }
.swap-enter-from { transform: translateY(70%); opacity: 0; }
.swap-leave-to { transform: translateY(-70%); opacity: 0; }
.lead { max-width: 30em; font-size: clamp(17px, 1.1vw, 21px); line-height: 1.75; color: #b7cdc6; }
.d-ctas { display: flex; align-items: center; gap: 28px; margin-top: 34px; }
.text-link { color: #e8f3ef; font-weight: 650; text-decoration: none; border-bottom: 1px solid rgba(232, 243, 239, .4); padding-bottom: 2px; }
.text-link:hover { color: var(--mint); border-color: var(--mint); }
/* 弱化的出口条：无底色、无边框、低对比，只在首屏底部安静地滚动。 */
.exit-strip { position: relative; width: 100%; max-width: 1240px; margin: 0 auto; padding: 0 24px 22px; display: flex; align-items: center; gap: 16px; opacity: .55; }
.exit-label { flex: none; font-size: 12px; color: #6f8d84; letter-spacing: .08em; }
.exit-rail { flex: 1; min-width: 0; overflow: hidden; -webkit-mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent); mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent); }
.exit-track { display: flex; gap: 36px; width: max-content; animation: marquee 50s linear infinite; font-size: 12.5px; color: #8fb0a6; }
.exit-track span { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }
.exit-track i { width: 5px; height: 5px; border-radius: 50%; background: #5fe3b0; }
@keyframes marquee { to { transform: translateX(-50%) } }
@media (prefers-reduced-motion: reduce) { .exit-track { animation: none; } }
@media (min-width: 1680px) { .d-ctas .cta.lg { height: 58px; padding: 0 38px; font-size: 18px; } .text-link { font-size: 17px; } .eyebrow { font-size: 15px; } }
@media (max-width: 860px) { .hero-d { min-height: 0; } .d-inner { padding: 112px 18px 40px; justify-content: flex-start; } h1 { font-size: 40px; } .exit-strip { padding: 0 18px 18px; } }
</style>
