<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import FlagIcon, { type FlagCode } from '@/shared/ui/FlagIcon.vue'
import { coverageHubs, coverageRoutes, gridDots } from './world'

const { t, locale } = useI18n()
const canvas = ref<HTMLCanvasElement>()
const mapDots = gridDots(2.5)
let cleanup: (() => void) | undefined

// 按 #100 / #113 决策：首页先静态展示 10 个常见国家与地区（6 列网格满格无落单），开售前按实际目录调整。
const countries: { code: FlagCode, wide?: boolean }[] = [
  { code: 'us', wide: true },
  { code: 'gb', wide: true },
  { code: 'jp' },
  { code: 'de' },
  { code: 'fr' },
  { code: 'ca' },
  { code: 'it' },
  { code: 'es' },
  { code: 'sg', wide: true },
  { code: 'other', wide: true },
]

const lonRange = 350
const latTop = 78
const latRange = 136
const lonLeft = -172

const hubPercent = (lon: number, lat: number) => ({
  left: `${((lon - lonLeft) / lonRange) * 100}%`,
  top: `${((latTop - lat) / latRange) * 100}%`,
})

function readToken(el: HTMLElement, name: string) {
  return getComputedStyle(el).getPropertyValue(name).trim()
}

onMounted(() => {
  const el = canvas.value
  const ctx = el?.getContext('2d')
  if (!el || !ctx) return

  function draw() {
    if (!el || !ctx) return
    const bounds = el.getBoundingClientRect()
    const w = bounds.width
    const h = bounds.height
    if (!w || !h) return

    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    el.width = Math.round(w * dpr)
    el.height = Math.round(h * dpr)
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    ctx.clearRect(0, 0, w, h)

    const dotFill = readToken(el, '--home-map-dot')
    const brand = readToken(el, '--home-brand')
    const white = readToken(el, '--home-light')
    const brandRgb = readToken(el, '--home-brand-rgb')
    const cyanRgb = readToken(el, '--home-canvas-city-bright-rgb')

    const toXY = (lon: number, lat: number) => ({
      x: ((lon - lonLeft) / lonRange) * w,
      y: ((latTop - lat) / latRange) * h,
    })

    const r = Math.max(Math.min(w / 460, 1.5), 1.05)
    ctx.fillStyle = dotFill
    for (const [lon, lat] of mapDots) {
      const { x, y } = toXY(lon, lat)
      ctx.beginPath()
      ctx.arc(x, y, r, 0, Math.PI * 2)
      ctx.fill()
    }

    for (const [fromIdx, toIdx] of coverageRoutes) {
      const a = toXY(coverageHubs[fromIdx].lon, coverageHubs[fromIdx].lat)
      const b = toXY(coverageHubs[toIdx].lon, coverageHubs[toIdx].lat)
      const dist = Math.hypot(b.x - a.x, b.y - a.y)
      const cx = (a.x + b.x) / 2
      const cy = (a.y + b.y) / 2 - dist * 0.24

      const grad = ctx.createLinearGradient(a.x, a.y, b.x, b.y)
      grad.addColorStop(0, `rgba(${brandRgb}, 0.22)`)
      grad.addColorStop(0.5, `rgba(${brandRgb}, 0.72)`)
      grad.addColorStop(1, `rgba(${cyanRgb}, 0.32)`)
      ctx.strokeStyle = grad
      ctx.lineWidth = 1.25
      ctx.beginPath()
      ctx.moveTo(a.x, a.y)
      ctx.quadraticCurveTo(cx, cy, b.x, b.y)
      ctx.stroke()
    }

    for (const hub of coverageHubs) {
      const { x, y } = toXY(hub.lon, hub.lat)
      const halo = ctx.createRadialGradient(x, y, 0, x, y, 12)
      halo.addColorStop(0, `rgba(${brandRgb}, 0.28)`)
      halo.addColorStop(1, `rgba(${brandRgb}, 0)`)
      ctx.fillStyle = halo
      ctx.beginPath()
      ctx.arc(x, y, 12, 0, Math.PI * 2)
      ctx.fill()

      ctx.fillStyle = brand
      ctx.beginPath()
      ctx.arc(x, y, 3.2, 0, Math.PI * 2)
      ctx.fill()
      ctx.fillStyle = white
      ctx.beginPath()
      ctx.arc(x, y, 1.15, 0, Math.PI * 2)
      ctx.fill()
    }
  }

  const observer = new ResizeObserver(draw)
  observer.observe(el)
  draw()
  cleanup = () => observer.disconnect()
})

onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <section id="coverage" class="sec sec-light-alt">
    <div class="wrap">
      <header class="sec-head">
        <h2>{{ t('marketing.coverage.title') }}</h2>
        <p class="sec-sub">{{ t('marketing.coverage.sub') }}</p>
      </header>

      <div class="coverage">
        <div class="map-box" aria-hidden="true">
          <canvas ref="canvas" class="map-canvas" />
          <span
            v-for="hub in coverageHubs"
            :key="`pill-${hub.code}`"
            class="map-pill"
            :class="`map-pill--${hub.pillSide}`"
            :style="hubPercent(hub.lon, hub.lat)"
          >
            <FlagIcon :code="hub.code" :size="18" />
            <span>{{ t(`marketing.coverage.countries.${hub.labelKey}`) }}</span>
          </span>
        </div>

        <ul class="country-grid" role="list">
          <li v-for="item in countries" :key="item.code" :class="{ 'is-wide': item.wide }">
            <RouterLink class="country-card" :to="{ path: `/${locale}/pricing`, query: { category: 'exclusive' } }">
              <FlagIcon :code="item.code" :size="28" />
              <span class="country-card__name">{{ t(`marketing.coverage.countries.${item.code}`) }}</span>
            </RouterLink>
          </li>
        </ul>
      </div>
    </div>
  </section>
</template>

<style scoped>
.coverage {
  display: grid;
  grid-template-columns: minmax(0, 1.25fr) minmax(0, 1fr);
  gap: var(--home-coverage-gap);
  align-items: center;
  margin-top: var(--home-block-gap);
}
.map-box {
  position: relative;
  aspect-ratio: 350 / 154;
  width: 100%;
}
.map-canvas {
  width: 100%;
  height: 100%;
  display: block;
}

.map-pill {
  position: absolute;
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) var(--space-2) var(--space-1) var(--space-1);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-pill);
  background: var(--home-light);
  box-shadow: var(--home-card-shadow);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-medium);
  color: var(--home-text-light);
  white-space: nowrap;
  transform: translate(var(--home-map-pill-offset), -50%);
}
.map-pill--left { transform: translate(calc(-100% - var(--home-map-pill-offset)), -50%); }

.country-grid {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: var(--space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}
.country-grid li { grid-column: span 2; }
.country-grid li.is-wide { grid-column: span 3; }

.country-card {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  min-height: var(--home-country-card-height);
  padding: var(--space-3) var(--space-4);
  border: var(--border-width) solid var(--home-border-subtle);
  border-radius: var(--home-radius-md);
  background: var(--home-light);
  box-shadow: var(--home-card-shadow);
  color: var(--home-text-light);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-medium);
  transition: border-color .2s, transform .25s var(--home-ease), box-shadow .25s var(--home-ease);
}
.country-card:hover {
  border-color: var(--home-sky-300);
  box-shadow: var(--home-card-shadow-hover);
  transform: translateY(-2px);
}
.country-card__name { overflow-wrap: anywhere; }

@media (max-width: 960px) {
  .coverage { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 560px) {
  .map-pill { display: none; }
  .country-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .country-grid li,
  .country-grid li.is-wide { grid-column: span 1; }
}
</style>
