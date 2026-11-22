<script setup lang="ts">
// 方案 D 首屏：高对比电影感地图/地球、无间隙轮换大标题、左下纯白胶囊主按钮 + 右下快捷复制 SOCKS5 胶囊。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { Check, CopyDocument } from '@element-plus/icons-vue'
import HeroCanvas from './HeroCanvas.vue'
import { sample } from './home-demo'

defineProps<{ hero: 'map' | 'globe' }>()

const phrases = ['跑通跨境店铺', '养稳社媒矩阵', '采集公开数据', '核对海外广告', '观察本地市场']
const index = ref(0)
const copied = ref(false)
let timer = 0
let copyTimer = 0
onMounted(() => { timer = window.setInterval(() => { index.value = (index.value + 1) % phrases.length }, 2800) })
onBeforeUnmount(() => { clearInterval(timer); clearTimeout(copyTimer) })

const quickCmd = `curl -x socks5h://${sample.username}-sid-a7k2:${sample.password}@${sample.host}:${sample.port} https://ipinfo.io`
async function copyQuick() {
  try {
    await navigator.clipboard.writeText(quickCmd)
    copied.value = true
    clearTimeout(copyTimer)
    copyTimer = window.setTimeout(() => { copied.value = false }, 2200)
  } catch { /* ignore */ }
}

// 示例出口，仅作氛围展示。
const exits = ['美国 洛杉矶', '英国 伦敦', '日本 东京', '德国 法兰克福', '巴西 圣保罗', '新加坡', '澳大利亚 悉尼', '加拿大 多伦多', '阿联酋 迪拜']
</script>

<template>
  <section class="hero hero-d">
    <HeroCanvas :mode="hero" :globe-x="0.7" :globe-scale="0.44" :opacity="1" cinematic />
    <div class="hero-ambient" aria-hidden="true" />
    <div class="hero-shade" aria-hidden="true" />
    <div class="d-inner">
      <div class="hero-copy">
        <h1>
          <span class="line">用真实住宅 IP</span>
          <span class="line rotator" aria-live="polite">
            <Transition name="swap"><em :key="index">{{ phrases[index] }}</em></Transition>
          </span>
        </h1>
        <p class="lead">动态住宅按流量计费，静态住宅 IP 按个租用。<br>一个账户，两种都能用。</p>
      </div>
      <div class="hero-foot">
        <div class="d-ctas">
          <a class="pill primary" href="/">免费注册</a>
          <a class="pill ghost" href="#products">了解产品 ↓</a>
        </div>
        <button type="button" class="agent-pill" :class="{ copied }" @click="copyQuick">
          <span class="agent-dots" aria-hidden="true"><i /><i /><i /></span>
          <span class="agent-label">{{ copied ? '已复制 SOCKS5 示例命令' : '复制 SOCKS5 接入命令' }}</span>
          <Check v-if="copied" class="agent-icon" />
          <CopyDocument v-else class="agent-icon" />
        </button>
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
.hero-d { min-height: 100svh; display: flex; flex-direction: column; background: #020808; }
.hero-d::after { display: none; }
.hero-ambient {
  position: absolute; inset: 0; pointer-events: none;
  background:
    radial-gradient(58% 64% at 72% 40%, rgba(22, 118, 92, .34) 0%, rgba(7, 36, 32, .16) 46%, transparent 74%),
    radial-gradient(36% 40% at 82% 24%, rgba(255, 206, 120, .09) 0%, transparent 65%);
}
.hero-shade {
  position: absolute; inset: 0; pointer-events: none;
  background:
    linear-gradient(90deg, rgba(2, 8, 8, .92) 0%, rgba(2, 8, 8, .62) 38%, rgba(2, 8, 8, .08) 70%),
    linear-gradient(0deg, #020808 0%, rgba(2, 8, 8, 0) 24%);
}
.d-inner { position: relative; flex: 1; width: 100%; max-width: 1240px; margin: 0 auto; padding: 140px 24px 44px; display: flex; flex-direction: column; justify-content: flex-end; gap: 40px; pointer-events: none; }
.d-inner a, .d-inner button { pointer-events: auto; }
.hero-copy { max-width: 780px; }
h1 { display: grid; font-size: clamp(50px, 6.2vw, 96px); line-height: 1.1; font-weight: 450; letter-spacing: -.032em; margin: 0 0 26px; color: #fff; }
.line { display: block; white-space: nowrap; }
/* 无间隙轮换：新旧词同时在位过渡，不留空白帧 */
.rotator { position: relative; display: block; height: 1.16em; overflow: hidden; }
.rotator em { position: absolute; left: 0; top: 0; font-style: normal; background: linear-gradient(90deg, #ffffff 0%, #d4fae9 56%, #82ebc0 100%); -webkit-background-clip: text; background-clip: text; color: transparent; }
.swap-enter-active, .swap-leave-active { transition: transform .52s cubic-bezier(.22, 1, .36, 1), opacity .42s ease; }
.swap-enter-from { transform: translateY(85%); opacity: 0; }
.swap-leave-to { transform: translateY(-85%); opacity: 0; }
.lead { max-width: 28em; font-size: clamp(17px, 1.2vw, 21px); line-height: 1.65; font-weight: 400; color: rgba(255, 255, 255, .74); }
.hero-foot { display: flex; align-items: center; justify-content: space-between; gap: 20px; flex-wrap: wrap; padding-bottom: 12px; }
.d-ctas { display: flex; align-items: center; gap: 14px; flex-wrap: wrap; }
.pill { display: inline-flex; align-items: center; justify-content: center; gap: 8px; height: 54px; padding: 0 32px; border-radius: 9999px; font-size: 16px; text-decoration: none; white-space: nowrap; transition: background .2s, border-color .2s, transform .15s, color .2s; }
.pill:hover { transform: translateY(-1px); }
.pill.primary { background: #fff; color: #050909; font-weight: 600; }
.pill.primary:hover { background: #e6eae9; }
.pill.ghost { border: 1px solid rgba(255, 255, 255, .26); background: rgba(255, 255, 255, .04); color: #fff; font-weight: 500; backdrop-filter: blur(10px); }
.pill.ghost:hover { border-color: rgba(255, 255, 255, .64); background: rgba(255, 255, 255, .09); }

/* 右下角互动胶囊：对标 Shopify 首屏右下角，一键复制示例命令 */
.agent-pill {
  display: inline-flex; align-items: center; gap: 12px; height: 54px; padding: 0 22px 0 14px;
  border-radius: 9999px; border: 1px solid rgba(255, 255, 255, .24);
  background: rgba(6, 14, 14, .66); color: #fff; backdrop-filter: blur(14px);
  cursor: pointer; font-size: 14.5px; font-weight: 500;
  transition: border-color .2s, background .2s, transform .15s;
}
.agent-pill:hover { border-color: rgba(255, 255, 255, .58); background: rgba(14, 26, 24, .82); transform: translateY(-1px); }
.agent-pill.copied { border-color: rgba(125, 232, 190, .65); color: #bdf7de; }
.agent-dots { display: inline-flex; align-items: center; padding: 6px 10px; border-radius: 999px; background: rgba(255, 255, 255, .92); gap: 4px; }
.agent-dots i { width: 6px; height: 6px; border-radius: 50%; background: #050909; }
.agent-dots i:nth-child(2) { background: #14966e; }
.agent-dots i:nth-child(3) { background: #3b82f6; }
.agent-icon { width: 15px; height: 15px; opacity: .78; }

.exit-strip { position: relative; width: 100%; max-width: 1240px; margin: 0 auto; padding: 18px 24px 26px; border-top: 1px solid rgba(255, 255, 255, .07); display: flex; align-items: center; gap: 18px; }
.exit-label { flex: none; font-size: 12px; color: rgba(255, 255, 255, .42); letter-spacing: .08em; }
.exit-rail { flex: 1; min-width: 0; overflow: hidden; -webkit-mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent); mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent); }
.exit-track { display: flex; gap: 36px; width: max-content; animation: marquee 50s linear infinite; font-size: 12.5px; color: rgba(255, 255, 255, .52); }
.exit-track span { display: inline-flex; align-items: center; gap: 8px; white-space: nowrap; }
.exit-track i { width: 4px; height: 4px; border-radius: 50%; background: rgba(130, 235, 192, .85); }
@keyframes marquee { to { transform: translateX(-50%) } }
@media (prefers-reduced-motion: reduce) { .exit-track { animation: none; } }
@media (min-width: 1680px) { .pill, .agent-pill { height: 58px; font-size: 17px; } }
@media (max-width: 860px) { .hero-d { min-height: 92svh; } .d-inner { padding: 112px 18px 32px; } h1 { font-size: 40px; } .agent-pill { width: 100%; justify-content: space-between; } .exit-strip { padding: 14px 18px 20px; } }
</style>
