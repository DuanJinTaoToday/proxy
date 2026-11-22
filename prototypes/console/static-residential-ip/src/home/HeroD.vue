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
    <HeroCanvas :mode="hero" :globe-x="0.72" :globe-scale="0.38" :opacity="hero === 'map' ? 0.95 : 1" />
    <div class="hero-shade" />
    <div class="d-inner">
      <h1>
        <span class="line">用真实住宅 IP</span>
        <span class="line rotator" aria-live="polite">
          <Transition name="swap"><em :key="index">{{ phrases[index] }}</em></Transition>
        </span>
      </h1>
      <p class="lead">动态住宅按流量计费，静态住宅 IP 按个租用。注册一个账户，两种都能用。</p>
      <div class="d-ctas">
        <a class="pill primary" href="/">免费注册</a>
        <a class="pill ghost" href="#products">先看看怎么选 ↓</a>
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
.hero-d { min-height: 100svh; display: flex; flex-direction: column; background: radial-gradient(68% 68% at 74% 42%, #0e332a 0%, #051210 52%, #020808 100%); }
.hero-d::after { display: none; }
.hero-shade { position: absolute; inset: 0; pointer-events: none; background: linear-gradient(90deg, rgba(2, 8, 8, .92) 0%, rgba(2, 8, 8, .58) 42%, rgba(2, 8, 8, .08) 72%), linear-gradient(0deg, #020808 0%, rgba(2, 8, 8, 0) 24%); }
.d-inner { position: relative; flex: 1; width: 100%; max-width: 1240px; margin: 0 auto; padding: 132px 24px 56px; display: flex; flex-direction: column; justify-content: center; align-items: flex-start; pointer-events: none; }
.d-inner a { pointer-events: auto; }
h1 { display: grid; font-size: clamp(46px, 5.4vw, 84px); line-height: 1.12; font-weight: 500; letter-spacing: -.028em; margin: 0 0 26px; color: #fff; }
.line { display: block; white-space: nowrap; }
/* 固定一行高度并让新旧两个词叠在同一格里，切换不改变布局。 */
.rotator { display: grid; height: 1.14em; overflow: hidden; }
.rotator em { grid-area: 1 / 1; font-style: normal; background: linear-gradient(90deg, #ffffff 0%, #cef7e6 55%, #7de8be 100%); -webkit-background-clip: text; background-clip: text; color: transparent; }
.swap-enter-active { transition: transform .5s cubic-bezier(.2, .8, .2, 1) .12s, opacity .4s .12s; }
.swap-leave-active { transition: transform .3s ease-in, opacity .25s; }
.swap-enter-from { transform: translateY(70%); opacity: 0; }
.swap-leave-to { transform: translateY(-70%); opacity: 0; }
.lead { max-width: 28em; font-size: clamp(17px, 1.18vw, 20px); line-height: 1.7; font-weight: 400; color: rgba(255, 255, 255, .72); }
.d-ctas { display: flex; align-items: center; gap: 16px; margin-top: 38px; flex-wrap: wrap; }
.pill { display: inline-flex; align-items: center; justify-content: center; gap: 8px; height: 52px; padding: 0 30px; border-radius: 9999px; font-size: 15.5px; text-decoration: none; white-space: nowrap; transition: background .2s, border-color .2s, transform .15s, color .2s; }
.pill:hover { transform: translateY(-1px); }
.pill.primary { background: #fff; color: #050909; font-weight: 600; }
.pill.primary:hover { background: #e4e8e7; }
.pill.ghost { border: 1px solid rgba(255, 255, 255, .28); background: rgba(255, 255, 255, .04); color: #fff; font-weight: 500; backdrop-filter: blur(8px); }
.pill.ghost:hover { border-color: rgba(255, 255, 255, .65); background: rgba(255, 255, 255, .09); }
/* 弱化的出口条：安静压在完整首屏底部。 */
.exit-strip { position: relative; width: 100%; max-width: 1240px; margin: 0 auto; padding: 18px 24px 26px; border-top: 1px solid rgba(255, 255, 255, .07); display: flex; align-items: center; gap: 18px; }
.exit-label { flex: none; font-size: 12px; color: rgba(255, 255, 255, .42); letter-spacing: .08em; }
.exit-rail { flex: 1; min-width: 0; overflow: hidden; -webkit-mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent); mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent); }
.exit-track { display: flex; gap: 36px; width: max-content; animation: marquee 50s linear infinite; font-size: 12.5px; color: rgba(255, 255, 255, .52); }
.exit-track span { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }
.exit-track i { width: 4px; height: 4px; border-radius: 50%; background: rgba(125, 232, 190, .8); }
@keyframes marquee { to { transform: translateX(-50%) } }
@media (prefers-reduced-motion: reduce) { .exit-track { animation: none; } }
@media (min-width: 1680px) { .pill { height: 56px; padding: 0 36px; font-size: 17px; } }
@media (max-width: 860px) { .hero-d { min-height: 92svh; } .d-inner { padding: 112px 18px 40px; justify-content: flex-start; } h1 { font-size: 40px; } .exit-strip { padding: 14px 18px 20px; } }
</style>
