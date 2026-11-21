<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

const props = defineProps<{ data: { day: string, gb: number }[] }>()

const root = ref<HTMLElement>()
const width = ref(720)
const height = 240
const pad = { top: 16, right: 16, bottom: 28, left: 44 }
const plotH = height - pad.top - pad.bottom
const plotW = computed(() => width.value - pad.left - pad.right)

let observer: ResizeObserver | undefined
onMounted(() => {
  observer = new ResizeObserver(([entry]) => { width.value = Math.max(320, entry.contentRect.width) })
  observer.observe(root.value!)
})
onBeforeUnmount(() => observer?.disconnect())

const hovered = ref<number | null>(null)
const ticks = computed(() => {
  const top = Math.max(1, Math.ceil(Math.max(...props.data.map(d => d.gb))))
  const step = top <= 5 ? 1 : Math.ceil(top / 5)
  return Array.from({ length: Math.floor(top / step) + 1 }, (_, i) => i * step)
})
const yMax = computed(() => ticks.value.at(-1)!)
const x = (i: number) => pad.left + (props.data.length === 1 ? plotW.value / 2 : plotW.value * i / (props.data.length - 1))
const y = (gb: number) => pad.top + plotH - gb / yMax.value * plotH
const points = computed(() => props.data.map((d, i) => [x(i), y(d.gb)] as const))

// 单调三次插值（Fritsch–Carlson）：曲线平滑，且不会越过相邻数据点，避免画出低于 0 的波谷。
function monotonePath(pts: readonly (readonly [number, number])[]) {
  const n = pts.length
  if (n < 2) return ''
  const dx = pts.slice(1).map((p, i) => p[0] - pts[i][0])
  const slope = pts.slice(1).map((p, i) => (p[1] - pts[i][1]) / dx[i])
  const t = pts.map((_, i) => {
    if (i === 0) return slope[0]
    if (i === n - 1) return slope[n - 2]
    const a = slope[i - 1]
    const b = slope[i]
    return a * b <= 0 ? 0 : 3 * (dx[i - 1] + dx[i]) / ((2 * dx[i] + dx[i - 1]) / a + (dx[i] + 2 * dx[i - 1]) / b)
  })
  let d = `M${pts[0][0]},${pts[0][1]}`
  for (let i = 0; i < n - 1; i++) {
    const h = dx[i] / 3
    d += `C${pts[i][0] + h},${pts[i][1] + h * t[i]} ${pts[i + 1][0] - h},${pts[i + 1][1] - h * t[i + 1]} ${pts[i + 1][0]},${pts[i + 1][1]}`
  }
  return d
}

const line = computed(() => monotonePath(points.value))
const baseline = pad.top + plotH
const area = computed(() => `${line.value}L${points.value.at(-1)![0]},${baseline}L${points.value[0][0]},${baseline}Z`)
const label = (day: string) => `${Number(day.slice(5, 7))}/${Number(day.slice(8, 10))}`
// 标签之间至少留约 64px，点多时隔几个显示一个，最后一天始终显示。
const labelEvery = computed(() => Math.max(1, Math.ceil(props.data.length / Math.max(2, Math.floor(plotW.value / 64)))))
const fmt = (gb: number) => `${gb.toFixed(2)} GB`

function onMove(event: MouseEvent) {
  const box = (event.currentTarget as SVGElement).getBoundingClientRect()
  const px = event.clientX - box.left
  const step = plotW.value / Math.max(1, props.data.length - 1)
  hovered.value = Math.min(props.data.length - 1, Math.max(0, Math.round((px - pad.left) / step)))
}
</script>

<template>
  <div ref="root" class="usage-chart">
    <svg :style="{ width: `${width}px`, height: `${height}px` }" :viewBox="`0 0 ${width} ${height}`"
      role="img" :aria-label="`动态代理近 ${data.length} 天每日扣减流量折线图`" @mousemove="onMove" @mouseleave="hovered = null">
      <defs>
        <linearGradient id="usage-fill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#0f8a64" stop-opacity="0.28" />
          <stop offset="100%" stop-color="#0f8a64" stop-opacity="0.02" />
        </linearGradient>
      </defs>
      <g class="chart-grid">
        <template v-for="tick in ticks" :key="tick">
          <line :x1="pad.left" :x2="width - pad.right" :y1="y(tick)" :y2="y(tick)" />
          <text :x="pad.left - 10" :y="y(tick)" dy="0.32em" text-anchor="end">{{ tick }} GB</text>
        </template>
      </g>
      <path :d="area" fill="url(#usage-fill)" />
      <path :d="line" class="chart-line" />
      <template v-for="(d, i) in data" :key="d.day">
        <text v-if="(i % labelEvery === 0 && data.length - 1 - i >= labelEvery) || i === data.length - 1" class="chart-axis" :x="x(i)" :y="height - 6"
          :text-anchor="i === 0 ? 'start' : i === data.length - 1 ? 'end' : 'middle'">{{ label(d.day) }}</text>
      </template>
      <g v-if="hovered !== null">
        <line class="chart-crosshair" :x1="x(hovered)" :x2="x(hovered)" :y1="pad.top" :y2="baseline" />
        <circle class="chart-dot" :cx="x(hovered)" :cy="y(data[hovered].gb)" r="5" />
      </g>
    </svg>
    <div v-if="hovered !== null" class="chart-tooltip" role="status"
      :class="{ 'flip-left': x(hovered) > width - 140 }"
      :style="{ left: `${x(hovered)}px`, top: `${y(data[hovered].gb)}px` }">
      <span>{{ data[hovered].day.replaceAll('-', '/') }}</span>
      <strong><i class="chart-swatch" />扣减 {{ fmt(data[hovered].gb) }}</strong>
    </div>
  </div>
</template>
