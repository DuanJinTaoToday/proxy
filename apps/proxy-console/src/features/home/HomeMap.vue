<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { gridDots } from './world'

const canvas = ref<HTMLCanvasElement>()
const mapDots = gridDots(2)
const cities: [number, number][] = [
  [-118.2, 34.1], [-74, 40.7], [-79.4, 43.7], [-99.1, 19.4], [-46.6, -23.5],
  [-0.1, 51.5], [8.7, 50.1], [2.35, 48.9], [55.3, 25.2], [28, -26.2],
  [72.9, 19.1], [103.8, 1.35], [114.2, 22.3], [139.7, 35.7], [127, 37.6], [151.2, -33.9],
]
type Arc = { from: number, to: number, progress: number, speed: number }

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
  const point = (lon: number, lat: number) => ({ x: offsetX + (lon + 180) * scale, y: offsetY + (80 - lat) * scale })

  function arc(progress: number): Arc {
    const from = Math.floor(Math.random() * cities.length)
    const to = (from + 1 + Math.floor(Math.random() * (cities.length - 1))) % cities.length
    return { from, to, progress, speed: 0.35 + Math.random() * 0.25 }
  }

  function render(time: number) {
    if (!ctx || !width || !height) return
    const dt = last ? Math.min((time - last) / 1000, 0.05) : 0
    last = time
    if (!motion.matches) {
      for (const route of arcs) route.progress += dt * route.speed
      arcs = arcs.filter(route => route.progress < 1.45)
      while (arcs.length < 7) arcs.push(arc(0))
    }
    ctx.clearRect(0, 0, width, height)
    ctx.drawImage(layer, 0, 0, width, height)
    for (const route of arcs) {
      const start = point(...cities[route.from])
      const end = point(...cities[route.to])
      const apex = { x: (start.x + end.x) / 2, y: (start.y + end.y) / 2 - Math.hypot(end.x - start.x, end.y - start.y) * 0.32 }
      const head = Math.min(route.progress, 1)
      const tail = Math.max(0, route.progress - 0.45)
      const fade = Math.max(0, (1.45 - Math.max(route.progress, 1)) / 0.45)
      const at = (t: number) => ({
        x: (1 - t) ** 2 * start.x + 2 * (1 - t) * t * apex.x + t * t * end.x,
        y: (1 - t) ** 2 * start.y + 2 * (1 - t) * t * apex.y + t * t * end.y,
      })
      ctx.lineCap = 'round'
      for (let i = 1; i <= 28; i++) {
        const start = at(tail + (head - tail) * (i - 1) / 28)
        const end = at(tail + (head - tail) * i / 28)
        ctx.strokeStyle = `rgba(190,255,232,${i / 28 * 0.95 * fade})`
        ctx.lineWidth = 1 + i / 28 * 1.6
        ctx.beginPath()
        ctx.moveTo(start.x, start.y)
        ctx.lineTo(end.x, end.y)
        ctx.stroke()
      }
    }
    cities.forEach((city, index) => {
      const { x, y } = point(...city)
      const warm = index % 3 === 0
      const rgb = warm ? '255,214,138' : '125,242,198'
      const radius = warm ? 18 : 13
      const gradient = ctx.createRadialGradient(x, y, 0, x, y, radius)
      gradient.addColorStop(0, `rgba(${rgb},0.55)`)
      gradient.addColorStop(1, `rgba(${rgb},0)`)
      ctx.fillStyle = gradient
      ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2)
      ctx.fillStyle = `rgb(${rgb})`
      ctx.beginPath()
      ctx.arc(x, y, 3, 0, Math.PI * 2)
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
    const dpr = Math.min(window.devicePixelRatio || 1, 2)
    element.width = layer.width = width * dpr
    element.height = layer.height = height * dpr
    ctx.scale(dpr, dpr)
    layerCtx.scale(dpr, dpr)
    scale = Math.min(width / 360, height / 136) * 0.98
    offsetX = (width - 360 * scale) / 2
    offsetY = (height - 136 * scale) / 2
    layerCtx.fillStyle = 'rgba(148,232,206,0.44)'
    for (const dot of mapDots) {
      const { x, y } = point(...dot)
      layerCtx.beginPath()
      layerCtx.arc(x, y, Math.max(scale * 0.62, 1), 0, Math.PI * 2)
      layerCtx.fill()
    }
    if (!arcs.length) arcs = cities.slice(0, 7).map((_, index) => arc(0.48 + index * 0.08))
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

let cleanup: (() => void) | undefined
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <canvas ref="canvas" class="home-map" aria-hidden="true" />
</template>

<style scoped>
.home-map { position: absolute; inset: 0; width: 100%; height: 100%; pointer-events: none; }
</style>
