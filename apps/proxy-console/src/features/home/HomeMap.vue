<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { gridDots } from './world'

const canvas = ref<HTMLCanvasElement>()
const mapDots = gridDots(2.2)

const cities: [number, number][] = [
  [-118.2, 34.1], [-74, 40.7], [-79.4, 43.7], [-99.1, 19.4], [-46.6, -23.5],
  [-0.1, 51.5], [8.7, 50.1], [2.35, 48.9], [55.3, 25.2], [28, -26.2],
  [72.9, 19.1], [103.8, 1.35], [114.2, 22.3], [139.7, 35.7], [127, 37.6], [151.2, -33.9],
]
type Arc = { from: number, to: number, progress: number, speed: number }

let cleanup: (() => void) | undefined

function readToken(el: HTMLElement, name: string) {
  return getComputedStyle(el).getPropertyValue(name).trim()
}

onMounted(() => {
  const element = canvas.value
  const ctx = element?.getContext('2d')
  const layer = document.createElement('canvas')
  const layerCtx = layer.getContext('2d')
  if (!element || !ctx || !layerCtx) return

  const motion = matchMedia('(prefers-reduced-motion: reduce)')
  let width = 0, height = 0, scale = 0, offsetX = 0, offsetY = 0
  let frame = 0, last = 0
  let arcs: Arc[] = []

  const readPalette = () => ({
    dotFill: readToken(element, '--home-canvas-hero-dot'),
    arcHeadRgb: readToken(element, '--home-canvas-arc-head-rgb'),
    arcTailRgb: readToken(element, '--home-canvas-arc-tail-rgb'),
    cityBrightRgb: readToken(element, '--home-canvas-city-bright-rgb'),
    citySoftRgb: readToken(element, '--home-canvas-city-soft-rgb'),
    cityBrightCore: readToken(element, '--home-canvas-city-bright-core'),
    citySoftCore: readToken(element, '--home-sky-300'),
  })

  let palette = readPalette()

  const point = (lon: number, lat: number) => ({
    x: offsetX + (lon + 180) * scale,
    y: offsetY + (78 - lat) * scale,
  })

  function makeArc(progress: number): Arc {
    const from = Math.floor(Math.random() * cities.length)
    const to = (from + 1 + Math.floor(Math.random() * (cities.length - 1))) % cities.length
    return { from, to, progress, speed: 0.34 + Math.random() * 0.24 }
  }

  function render(time: number) {
    if (!ctx || !width || !height) return
    const dt = last ? Math.min((time - last) / 1000, 0.05) : 0
    last = time
    if (!motion.matches) {
      for (const route of arcs) route.progress += dt * route.speed
      arcs = arcs.filter(route => route.progress < 1.45)
      while (arcs.length < 7) arcs.push(makeArc(0))
    }

    ctx.clearRect(0, 0, width, height)
    ctx.drawImage(layer, 0, 0, width, height)

    for (const route of arcs) {
      const start = point(...cities[route.from])
      const end = point(...cities[route.to])
      const apex = {
        x: (start.x + end.x) / 2,
        y: (start.y + end.y) / 2 - Math.hypot(end.x - start.x, end.y - start.y) * 0.3,
      }
      const head = Math.min(route.progress, 1)
      const tail = Math.max(0, route.progress - 0.45)
      const fade = Math.max(0, (1.45 - Math.max(route.progress, 1)) / 0.45)
      const at = (t: number) => ({
        x: (1 - t) ** 2 * start.x + 2 * (1 - t) * t * apex.x + t * t * end.x,
        y: (1 - t) ** 2 * start.y + 2 * (1 - t) * t * apex.y + t * t * end.y,
      })

      ctx.lineCap = 'round'
      for (let i = 1; i <= 26; i++) {
        const p1 = at(tail + (head - tail) * (i - 1) / 26)
        const p2 = at(tail + (head - tail) * i / 26)
        const ratio = i / 26
        ctx.strokeStyle = ratio > 0.72
          ? `rgba(${palette.arcHeadRgb}, ${ratio * 0.92 * fade})`
          : `rgba(${palette.arcTailRgb}, ${ratio * 0.78 * fade})`
        ctx.lineWidth = 1 + ratio * 1.5
        ctx.beginPath()
        ctx.moveTo(p1.x, p1.y)
        ctx.lineTo(p2.x, p2.y)
        ctx.stroke()
      }
    }

    cities.forEach((city, index) => {
      const { x, y } = point(...city)
      const bright = index % 3 === 0
      const rgb = bright ? palette.cityBrightRgb : palette.citySoftRgb
      const radius = bright ? 16 : 12
      const gradient = ctx.createRadialGradient(x, y, 0, x, y, radius)
      gradient.addColorStop(0, `rgba(${rgb}, 0.56)`)
      gradient.addColorStop(1, `rgba(${rgb}, 0)`)
      ctx.fillStyle = gradient
      ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2)
      ctx.fillStyle = bright ? palette.cityBrightCore : palette.citySoftCore
      ctx.beginPath()
      ctx.arc(x, y, bright ? 2.8 : 2.3, 0, Math.PI * 2)
      ctx.fill()
    })

    if (!motion.matches) frame = requestAnimationFrame(render)
  }

  function resize() {
    if (!element || !ctx || !layerCtx) return
    cancelAnimationFrame(frame)
    const bounds = element.getBoundingClientRect()
    width = bounds.width
    height = bounds.height
    if (!width || !height) return
    palette = readPalette()
    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    element.width = layer.width = Math.round(width * dpr)
    element.height = layer.height = Math.round(height * dpr)
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    layerCtx.setTransform(dpr, 0, 0, dpr, 0, 0)

    scale = Math.min(width / 352, height / 136) * 0.98
    offsetX = (width - 360 * scale) / 2 + Math.min(width * 0.06, 80)
    offsetY = (height - 136 * scale) / 2 + 12

    layerCtx.fillStyle = palette.dotFill
    const dotRadius = Math.max(scale * 0.52, 0.95)
    for (const dot of mapDots) {
      const { x, y } = point(...dot)
      layerCtx.beginPath()
      layerCtx.arc(x, y, dotRadius, 0, Math.PI * 2)
      layerCtx.fill()
    }

    if (!arcs.length) arcs = cities.slice(0, 7).map((_, index) => makeArc(0.42 + index * 0.09))
    last = 0
    frame = requestAnimationFrame(render)
  }

  const observer = new ResizeObserver(resize)
  observer.observe(element)
  motion.addEventListener('change', resize)
  cleanup = () => {
    cancelAnimationFrame(frame)
    observer.disconnect()
    motion.removeEventListener('change', resize)
  }
})

onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <canvas ref="canvas" class="home-map" aria-hidden="true" />
</template>

<style scoped>
.home-map {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}
</style>
