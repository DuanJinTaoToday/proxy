<script setup lang="ts">
// 首屏动效：点阵世界地图或可拖拽的点阵地球，城市节点之间循环飞出连接弧线。纯 canvas，无额外依赖。
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { cities } from './home-demo'
import { gridDots, sphereDots } from './world'

const props = withDefaults(defineProps<{
  mode: 'map' | 'globe'
  theme?: 'dark' | 'light'
  /** 地球中心的横向位置（0–1），地图忽略。 */
  globeX?: number
  /** 地球半径相对画布短边的比例。 */
  globeScale?: number
  /** 整体不透明度，用作背景时调低。 */
  opacity?: number
}>(), { theme: 'dark', globeX: 0.7, globeScale: 0.42, opacity: 1 })

const canvas = ref<HTMLCanvasElement>()
const deg = Math.PI / 180
const mapDots = gridDots(2)
const globeDots = sphereDots(2.2)
const palette = () => props.theme === 'dark'
  ? { dot: '179, 240, 215', arc: '120, 255, 200', node: '179, 240, 215', sphere: ['rgba(20, 60, 50, 0.55)', 'rgba(8, 20, 18, 0.2)'], rim: 'rgba(120, 255, 200, 0.28)' }
  : { dot: '8, 105, 80', arc: '8, 140, 105', node: '8, 105, 80', sphere: ['rgba(231, 246, 239, 0.9)', 'rgba(231, 246, 239, 0.3)'], rim: 'rgba(8, 105, 80, 0.22)' }

interface Arc { from: number, to: number, t: number, speed: number }
let arcs: Arc[] = []
let width = 0, height = 0, dpr = 1
let frame = 0, last = 0
let spin = -100 * deg, tilt = 22 * deg
let dragging: { x: number, y: number, spin: number, tilt: number } | null = null
let idleUntil = 0
let mapLayer: HTMLCanvasElement | null = null
const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches

function spawnArc() {
  const from = Math.floor(Math.random() * cities.length)
  let to = Math.floor(Math.random() * cities.length)
  if (to === from) to = (to + 3) % cities.length
  arcs.push({ from, to, t: 0, speed: 0.35 + Math.random() * 0.25 })
}

// ── 平面地图 ───────────────────────────────────────
const latTop = 80, latBottom = -56
function mapFit() {
  const s = Math.min(width / 360, height / (latTop - latBottom)) * 0.98
  const ox = (width - 360 * s) / 2
  const oy = (height - (latTop - latBottom) * s) / 2
  return { s, ox, oy }
}
function mapPoint(lon: number, lat: number) {
  const { s, ox, oy } = mapFit()
  return { x: ox + (lon + 180) * s, y: oy + (latTop - lat) * s }
}
function renderMapLayer() {
  mapLayer = document.createElement('canvas')
  mapLayer.width = width * dpr
  mapLayer.height = height * dpr
  const ctx = mapLayer.getContext('2d')!
  ctx.scale(dpr, dpr)
  const { s } = mapFit()
  const r = Math.max(s * 0.55, 1)
  const c = palette()
  ctx.fillStyle = `rgba(${c.dot}, ${props.theme === 'dark' ? 0.32 : 0.28})`
  for (const [lon, lat] of mapDots) {
    const p = mapPoint(lon, lat)
    ctx.beginPath()
    ctx.arc(p.x, p.y, r, 0, Math.PI * 2)
    ctx.fill()
  }
}
function drawMap(ctx: CanvasRenderingContext2D, time: number) {
  if (mapLayer && mapLayer.width && mapLayer.height) ctx.drawImage(mapLayer, 0, 0, width, height)
  const c = palette()
  for (const arc of arcs) {
    const a = cities[arc.from], b = cities[arc.to]
    const p0 = mapPoint(a.lon, a.lat), p1 = mapPoint(b.lon, b.lat)
    const dist = Math.hypot(p1.x - p0.x, p1.y - p0.y)
    const cx = (p0.x + p1.x) / 2, cy = (p0.y + p1.y) / 2 - dist * 0.32
    const at = (u: number) => ({
      x: (1 - u) ** 2 * p0.x + 2 * (1 - u) * u * cx + u * u * p1.x,
      y: (1 - u) ** 2 * p0.y + 2 * (1 - u) * u * cy + u * u * p1.y,
    })
    drawTrail(ctx, arc, at, c.arc)
  }
  drawNodes(ctx, time, city => ({ ...mapPoint(city.lon, city.lat), visible: 1 }))
}

// ── 地球 ─────────────────────────────────────────
function globeGeom() {
  const R = Math.min(width, height) * props.globeScale
  return { R, cx: width * props.globeX, cy: height / 2 }
}
function project(x: number, y: number, z: number) {
  const x1 = x * Math.cos(spin) - z * Math.sin(spin)
  const z1 = x * Math.sin(spin) + z * Math.cos(spin)
  const y2 = y * Math.cos(tilt) - z1 * Math.sin(tilt)
  const z2 = y * Math.sin(tilt) + z1 * Math.cos(tilt)
  return { x: x1, y: y2, z: z2 }
}
function unit(lon: number, lat: number): [number, number, number] {
  return [Math.cos(lat * deg) * Math.sin(lon * deg), Math.sin(lat * deg), Math.cos(lat * deg) * Math.cos(lon * deg)]
}
function toScreen(v: { x: number, y: number }) {
  const { R, cx, cy } = globeGeom()
  return { x: cx + v.x * R, y: cy - v.y * R }
}
function drawGlobe(ctx: CanvasRenderingContext2D, time: number) {
  const { R, cx, cy } = globeGeom()
  const c = palette()
  const glow = ctx.createRadialGradient(cx, cy, R * 0.9, cx, cy, R * 1.35)
  glow.addColorStop(0, c.rim)
  glow.addColorStop(1, 'rgba(0,0,0,0)')
  ctx.fillStyle = glow
  ctx.beginPath(); ctx.arc(cx, cy, R * 1.35, 0, Math.PI * 2); ctx.fill()
  const body = ctx.createRadialGradient(cx - R * 0.35, cy - R * 0.4, R * 0.1, cx, cy, R)
  body.addColorStop(0, c.sphere[0])
  body.addColorStop(1, c.sphere[1])
  ctx.fillStyle = body
  ctx.beginPath(); ctx.arc(cx, cy, R, 0, Math.PI * 2); ctx.fill()

  const size = Math.max(R / 260, 0.8)
  for (const [lon, lat] of globeDots) {
    const p = project(...unit(lon, lat))
    if (p.z <= 0) continue
    const s = toScreen(p)
    ctx.fillStyle = `rgba(${c.dot}, ${0.18 + p.z * 0.62})`
    ctx.beginPath(); ctx.arc(s.x, s.y, size * (0.7 + p.z * 0.9), 0, Math.PI * 2); ctx.fill()
  }

  for (const arc of arcs) {
    const a = unit(cities[arc.from].lon, cities[arc.from].lat)
    const b = unit(cities[arc.to].lon, cities[arc.to].lat)
    const omega = Math.acos(Math.min(1, a[0] * b[0] + a[1] * b[1] + a[2] * b[2]))
    const lift = 0.12 + omega * 0.12
    const at = (u: number) => {
      const k0 = Math.sin((1 - u) * omega) / Math.sin(omega), k1 = Math.sin(u * omega) / Math.sin(omega)
      const h = 1 + lift * Math.sin(Math.PI * u)
      const p = project((a[0] * k0 + b[0] * k1) * h, (a[1] * k0 + b[1] * k1) * h, (a[2] * k0 + b[2] * k1) * h)
      const hidden = p.z < 0 && p.x * p.x + p.y * p.y < 1
      return { ...toScreen(p), hidden }
    }
    drawTrail(ctx, arc, at, c.arc)
  }
  drawNodes(ctx, time, city => {
    const p = project(...unit(city.lon, city.lat))
    return { ...toScreen(p), visible: p.z > 0 ? Math.min(1, p.z * 3) : 0 }
  })
}

// ── 共用：弧线拖尾与节点 ───────────────────────────────
function drawTrail(ctx: CanvasRenderingContext2D, arc: Arc, at: (u: number) => { x: number, y: number, hidden?: boolean }, rgb: string) {
  const head = Math.min(arc.t, 1)
  const tail = Math.max(0, arc.t - 0.45)
  const fade = arc.t > 1 ? Math.max(0, 1 - (arc.t - 1) / 0.45) : 1
  const steps = 28
  ctx.lineCap = 'round'
  // 淡色全程轨迹
  ctx.strokeStyle = `rgba(${rgb}, ${0.12 * fade})`
  ctx.lineWidth = 1
  ctx.beginPath()
  let pen = false
  for (let i = 0; i <= steps; i++) {
    const p = at((i / steps) * head)
    if (p.hidden) { pen = false; continue }
    if (pen) ctx.lineTo(p.x, p.y); else ctx.moveTo(p.x, p.y)
    pen = true
  }
  ctx.stroke()
  // 亮色拖尾
  let prev = at(tail)
  for (let i = 1; i <= steps; i++) {
    const u = tail + ((head - tail) * i) / steps
    const p = at(u)
    if (!p.hidden && !prev.hidden) {
      ctx.strokeStyle = `rgba(${rgb}, ${(i / steps) * 0.95 * fade})`
      ctx.lineWidth = 1 + (i / steps) * 1.6
      ctx.beginPath(); ctx.moveTo(prev.x, prev.y); ctx.lineTo(p.x, p.y); ctx.stroke()
    }
    prev = p
  }
  const tip = at(head)
  if (!tip.hidden && arc.t <= 1) {
    const g = ctx.createRadialGradient(tip.x, tip.y, 0, tip.x, tip.y, 10)
    g.addColorStop(0, `rgba(${rgb}, 0.9)`)
    g.addColorStop(1, `rgba(${rgb}, 0)`)
    ctx.fillStyle = g
    ctx.beginPath(); ctx.arc(tip.x, tip.y, 10, 0, Math.PI * 2); ctx.fill()
  }
}
function drawNodes(ctx: CanvasRenderingContext2D, time: number, place: (city: typeof cities[number]) => { x: number, y: number, visible: number }) {
  const c = palette()
  cities.forEach((city, i) => {
    const p = place(city)
    if (p.visible <= 0) return
    const pulse = ((time / 1800 + i * 0.37) % 1)
    ctx.strokeStyle = `rgba(${c.node}, ${(1 - pulse) * 0.6 * p.visible})`
    ctx.lineWidth = 1.2
    ctx.beginPath(); ctx.arc(p.x, p.y, 3 + pulse * 12, 0, Math.PI * 2); ctx.stroke()
    ctx.fillStyle = `rgba(${c.node}, ${p.visible})`
    ctx.beginPath(); ctx.arc(p.x, p.y, 2.6, 0, Math.PI * 2); ctx.fill()
  })
}

// ── 循环 ─────────────────────────────────────────
function resize() {
  const el = canvas.value!
  const box = el.getBoundingClientRect()
  width = box.width; height = box.height
  dpr = Math.min(window.devicePixelRatio || 1, 2)
  el.width = width * dpr
  el.height = height * dpr
  renderMapLayer()
}
function draw(time: number) {
  const dt = last ? Math.min((time - last) / 1000, 0.05) : 0
  last = time
  if (!reduced) {
    for (const arc of arcs) arc.t += dt * arc.speed
    arcs = arcs.filter(arc => arc.t < 1.45)
    // 首帧错开各条弧线的进度，之后每补一条都从起点飞出。
    const seeding = arcs.length === 0
    while (arcs.length < 7) { spawnArc(); if (seeding) arcs.at(-1)!.t = Math.random() * 1.2 }
    if (props.mode === 'globe' && !dragging && time > idleUntil) spin += dt * 0.12
  }
  else if (!arcs.length) { for (let i = 0; i < 6; i++) { spawnArc(); arcs[i].t = 0.5 + i * 0.08 } }
  const ctx = canvas.value!.getContext('2d')!
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, width, height)
  ctx.globalAlpha = props.opacity
  if (props.mode === 'map') drawMap(ctx, time); else drawGlobe(ctx, time)
  ctx.globalAlpha = 1
  if (!reduced || dragging) frame = requestAnimationFrame(draw)
}

function onDown(e: PointerEvent) {
  if (props.mode !== 'globe') return
  const { R, cx, cy } = globeGeom()
  const box = canvas.value!.getBoundingClientRect()
  if (Math.hypot(e.clientX - box.left - cx, e.clientY - box.top - cy) > R * 1.05) return
  dragging = { x: e.clientX, y: e.clientY, spin, tilt }
  canvas.value!.setPointerCapture(e.pointerId)
}
function onMove(e: PointerEvent) {
  const box = canvas.value!.getBoundingClientRect()
  if (props.mode === 'globe' && !dragging) {
    const { R, cx, cy } = globeGeom()
    canvas.value!.style.cursor = Math.hypot(e.clientX - box.left - cx, e.clientY - box.top - cy) <= R ? 'grab' : ''
  }
  if (!dragging) return
  canvas.value!.style.cursor = 'grabbing'
  const { R } = globeGeom()
  spin = dragging.spin - (e.clientX - dragging.x) / R
  tilt = Math.max(-0.3, Math.min(1.1, dragging.tilt + (e.clientY - dragging.y) / R))
}
function onUp() {
  dragging = null
  idleUntil = performance.now() + 1500
  if (canvas.value) canvas.value.style.cursor = ''
}

let observer: ResizeObserver
onMounted(() => {
  observer = new ResizeObserver(resize)
  observer.observe(canvas.value!)
  resize()
  frame = requestAnimationFrame(draw)
})
onBeforeUnmount(() => { cancelAnimationFrame(frame); observer.disconnect() })
watch(() => [props.mode, props.theme], () => { renderMapLayer(); if (reduced) frame = requestAnimationFrame(draw) })
</script>

<template>
  <canvas ref="canvas" class="hero-canvas" :class="{ interactive: mode === 'globe' }" aria-hidden="true"
    @pointerdown="onDown" @pointermove="onMove" @pointerup="onUp" @pointercancel="onUp" />
</template>

<style scoped>
.hero-canvas { position: absolute; inset: 0; width: 100%; height: 100%; display: block; touch-action: pan-y; }
</style>
