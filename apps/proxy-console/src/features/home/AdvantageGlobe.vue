<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { isLand } from './world'

// 3D 正交点阵半球模型（核心优势卡片底部视觉），全部绘制色通过 tokens.css 语义变量读取。
const canvas = ref<HTMLCanvasElement>()
let cleanup: (() => void) | undefined

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

    const brand = readToken(el, '--home-brand')
    const white = readToken(el, '--home-light')
    const domeStart = readToken(el, '--home-globe-dome-start')
    const domeMid = readToken(el, '--home-globe-dome-mid')
    const domeEnd = readToken(el, '--home-globe-dome-end')
    const gridStroke = readToken(el, '--home-globe-grid')
    const landRgb = readToken(el, '--home-globe-land-rgb')
    const oceanRgb = readToken(el, '--home-globe-ocean-rgb')
    const brandRgb = readToken(el, '--home-brand-rgb')
    const skyRgb = readToken(el, '--home-canvas-city-soft-rgb')

    const cx = w * 0.52
    const r = Math.min(w * 0.48, h * 1.48)
    const cy = h + r * 0.36
    const lon0 = 55 * Math.PI / 180
    const lat0 = 30 * Math.PI / 180
    const sinLat0 = Math.sin(lat0)
    const cosLat0 = Math.cos(lat0)

    const domeGrad = ctx.createRadialGradient(cx, cy - r * 0.72, r * 0.08, cx, cy, r)
    domeGrad.addColorStop(0, domeStart)
    domeGrad.addColorStop(0.65, domeMid)
    domeGrad.addColorStop(1, domeEnd)
    ctx.fillStyle = domeGrad
    ctx.beginPath()
    ctx.arc(cx, cy, r, 0, Math.PI * 2)
    ctx.fill()

    ctx.strokeStyle = gridStroke
    ctx.lineWidth = 0.8
    for (const ring of [0.98, 0.82, 0.64]) {
      ctx.beginPath()
      ctx.ellipse(cx, cy, r * ring, r * ring * 0.96, 0, Math.PI, Math.PI * 2)
      ctx.stroke()
    }

    const project = (lonDeg: number, latDeg: number) => {
      const lon = lonDeg * Math.PI / 180
      const lat = latDeg * Math.PI / 180
      const cosLat = Math.cos(lat)
      const sinLat = Math.sin(lat)
      const dLon = lon - lon0
      const cosC = sinLat0 * sinLat + cosLat0 * cosLat * Math.cos(dLon)
      if (cosC <= 0.05) return null
      const x = cx + r * cosLat * Math.sin(dLon)
      const y = cy - r * (cosLat0 * sinLat - sinLat0 * cosLat * Math.cos(dLon))
      return { x, y, depth: cosC }
    }

    for (let lat = -10; lat <= 78; lat += 3.2) {
      const lonStep = Math.max(3.2, 3.2 / Math.max(Math.cos(lat * Math.PI / 180), 0.28))
      for (let lon = -180; lon < 180; lon += lonStep) {
        const pt = project(lon, lat)
        if (!pt || pt.y > h + 4 || pt.y < 6) continue
        const land = isLand(lon, lat)
        const alpha = (land ? 0.56 : 0.17) * Math.min(1, pt.depth * 1.25)
        ctx.fillStyle = land
          ? `rgba(${landRgb}, ${alpha.toFixed(2)})`
          : `rgba(${oceanRgb}, ${alpha.toFixed(2)})`
        ctx.beginPath()
        ctx.arc(pt.x, pt.y, land ? 1.25 : 0.78, 0, Math.PI * 2)
        ctx.fill()
      }
    }

    const hubs = [
      project(10, 48),
      project(55, 25),
      project(116, 36),
      project(104, 10),
      project(-4, 53),
    ].filter((p): p is NonNullable<typeof p> => Boolean(p && p.y < h))

    const target = hubs[2] ?? hubs[0]
    if (target) {
      for (const src of hubs) {
        if (src === target) continue
        const midX = (src.x + target.x) / 2
        const midY = Math.min(src.y, target.y) - Math.hypot(target.x - src.x, target.y - src.y) * 0.26
        const arcGrad = ctx.createLinearGradient(src.x, src.y, target.x, target.y)
        arcGrad.addColorStop(0, `rgba(${skyRgb}, 0.14)`)
        arcGrad.addColorStop(0.55, `rgba(${brandRgb}, 0.55)`)
        arcGrad.addColorStop(1, `rgba(${brandRgb}, 0.85)`)
        ctx.strokeStyle = arcGrad
        ctx.lineWidth = 1.15
        ctx.beginPath()
        ctx.moveTo(src.x, src.y)
        ctx.quadraticCurveTo(midX, midY, target.x, target.y)
        ctx.stroke()

        ctx.fillStyle = `rgba(${brandRgb}, 0.55)`
        ctx.beginPath()
        ctx.arc(src.x, src.y, 2, 0, Math.PI * 2)
        ctx.fill()
      }

      const halo = ctx.createRadialGradient(target.x, target.y, 0, target.x, target.y, 14)
      halo.addColorStop(0, `rgba(${brandRgb}, 0.32)`)
      halo.addColorStop(1, `rgba(${brandRgb}, 0)`)
      ctx.fillStyle = halo
      ctx.beginPath()
      ctx.arc(target.x, target.y, 14, 0, Math.PI * 2)
      ctx.fill()

      ctx.fillStyle = brand
      ctx.beginPath()
      ctx.arc(target.x, target.y, 3.6, 0, Math.PI * 2)
      ctx.fill()
      ctx.fillStyle = white
      ctx.beginPath()
      ctx.arc(target.x, target.y, 1.3, 0, Math.PI * 2)
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
  <canvas ref="canvas" class="adv-globe" aria-hidden="true" />
</template>

<style scoped>
.adv-globe {
  position: absolute;
  inset-inline: 0;
  inset-block-end: 0;
  width: 100%;
  height: var(--home-globe-height);
  pointer-events: none;
  -webkit-mask-image: linear-gradient(180deg, transparent 0%, black 22%, black 100%);
  mask-image: linear-gradient(180deg, transparent 0%, black 22%, black 100%);
}
</style>
