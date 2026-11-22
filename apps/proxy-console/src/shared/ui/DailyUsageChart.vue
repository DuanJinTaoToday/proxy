<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId, useTemplateRef } from 'vue'
import { dayFromNumber, dayNumber } from '@/shared/lib/businessDates'
import { bytesPerGB } from '@/shared/lib/trafficUnits'

const props = defineProps<{
  days: { date: string, bytes: bigint }[]
  range?: { start: string, end: string }
  label: string
  formatBytes: (bytes: bigint) => string
  formatTick: (bytes: bigint) => string
  amountLabel: string
}>()

interface PlotPoint {
  x: number
  y: number
}

const root = useTemplateRef<HTMLDivElement>('root')
const gradientId = useId()
const width = ref(720)
const height = 240
const top = 16
const right = 16
const baseline = height - 28
const hovered = ref<number>()
const peak = computed(() => props.days.reduce((max, day) => day.bytes > max ? day.bytes : max, 1n))
const ceiling = computed(() => Number((peak.value + bytesPerGB - 1n) / bytesPerGB))
const tickStep = computed(() => Math.ceil(ceiling.value / 5))
const tickCount = computed(() => Math.ceil(ceiling.value / tickStep.value))
const maxGB = computed(() => tickCount.value * tickStep.value)
const ticks = computed(() => Array.from({ length: tickCount.value + 1 }, (_, index) => BigInt(index * tickStep.value) * bytesPerGB))
const left = computed(() => Math.max(44, props.formatTick(ticks.value.at(-1) ?? 0n).length * 6 + 10))
const plotWidth = computed(() => Math.max(1, width.value - left.value - right))
const start = computed(() => dayNumber(props.range?.start ?? props.days[0]?.date ?? '1970-01-01'))
const end = computed(() => dayNumber(props.range?.end ?? props.days.at(-1)?.date ?? '1970-01-01'))
const duration = computed(() => Math.max(0, end.value - start.value))
const pointStep = computed(() => plotWidth.value / Math.max(1, duration.value))
const xAt = (offset: number) => left.value + pointStep.value * (duration.value === 0 ? 0.5 : offset)
const yAt = (bytes: bigint) => baseline - Number(bytes) / Number(bytesPerGB) / maxGB.value * (baseline - top)
const recorded = computed(() => new Map(props.days.map(day => [dayNumber(day.date), day.bytes])))
const points = computed(() => {
  if (!props.days.length) return []
  const samplesWithZeroGapEdges = new Map(recorded.value)
  for (const day of recorded.value.keys()) {
    if (day > start.value && !samplesWithZeroGapEdges.has(day - 1)) samplesWithZeroGapEdges.set(day - 1, 0n)
    if (day < end.value && !samplesWithZeroGapEdges.has(day + 1)) samplesWithZeroGapEdges.set(day + 1, 0n)
  }
  if (!samplesWithZeroGapEdges.has(start.value)) samplesWithZeroGapEdges.set(start.value, 0n)
  if (!samplesWithZeroGapEdges.has(end.value)) samplesWithZeroGapEdges.set(end.value, 0n)
  return [...samplesWithZeroGapEdges].sort(([a], [b]) => a - b).map(([day, bytes]) => ({
    day, x: xAt(day - start.value), y: yAt(bytes),
  }))
})
const activePoint = computed(() => {
  if (hovered.value === undefined || hovered.value > duration.value) return
  const day = start.value + hovered.value
  const bytes = recorded.value.get(day) ?? 0n
  return { date: dayFromNumber(day), bytes, x: xAt(hovered.value), y: yAt(bytes) }
})
const axisDates = computed(() => {
  const count = duration.value + 1
  const step = Math.max(1, Math.ceil(count / Math.max(2, Math.floor(plotWidth.value / 64))))
  const offsets = Array.from({ length: Math.ceil(count / step) }, (_, index) => index * step)
    .filter(offset => offset <= duration.value - step)
  offsets.push(duration.value)
  return offsets.map(offset => {
    const date = dayFromNumber(start.value + offset)
    const shortDate = Number(date.slice(5, 7)) + '/' + Number(date.slice(8, 10))
    return { offset, date, label: duration.value >= 365 ? date : shortDate, x: xAt(offset) }
  })
})
function hitBounds(date: string) {
  const offset = dayNumber(date) - start.value
  const x = xAt(offset)
  const leftEdge = offset === 0 ? 0 : x - pointStep.value / 2
  const rightEdge = offset === duration.value ? width.value : x + pointStep.value / 2
  return { x: leftEdge, width: rightEdge - leftEdge }
}

let observer: ResizeObserver | undefined
onMounted(() => {
  if (!root.value) return
  observer = new ResizeObserver(([entry]) => { width.value = Math.max(1, entry.contentRect.width) })
  observer.observe(root.value)
})
onUnmounted(() => observer?.disconnect())

function monotonePath(pts: readonly PlotPoint[]) {
  const first = pts[0]
  if (!first) return ''
  let path = `M${first.x},${first.y}`
  if (pts.length === 1) return path

  const widths = pts.slice(1).map((point, index) => point.x - pts[index].x)
  const slopes = pts.slice(1).map((point, index) => (point.y - pts[index].y) / widths[index])
  const monotoneTangents = pts.map((_, index) => {
    if (index === 0) return slopes[0]
    if (index === pts.length - 1) return slopes[index - 1]
    const before = slopes[index - 1]
    const after = slopes[index]
    if (before * after <= 0) return 0
    const left = widths[index - 1]
    const right = widths[index]
    return 3 * (left + right) / ((2 * right + left) / before + (right + 2 * left) / after)
  })
  for (const [index, point] of pts.slice(1).entries()) {
    const previous = pts[index]
    const handle = widths[index] / 3
    path += `C${previous.x + handle},${previous.y + handle * monotoneTangents[index]} ${point.x - handle},${point.y - handle * monotoneTangents[index + 1]} ${point.x},${point.y}`
  }
  return path
}

const line = computed(() => monotonePath(points.value))
const area = computed(() => {
  const last = points.value.at(-1)
  return last ? `${line.value}L${last.x},${baseline}L${points.value[0].x},${baseline}Z` : ''
})

function hover(event: PointerEvent) {
  if (!(event.currentTarget instanceof SVGSVGElement) || !props.days.length) return
  const px = event.clientX - event.currentTarget.getBoundingClientRect().left
  hovered.value = Math.min(duration.value, Math.max(0, Math.round((px - left.value) / pointStep.value)))
}
</script>

<template>
  <div ref="root" class="usage-chart tabular">
    <svg :viewBox="`0 0 ${width} ${height}`" :height="height"
      role="img" :aria-label="label" @pointermove="hover" @pointerleave="hovered = undefined">
      <defs>
        <linearGradient :id="gradientId" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-opacity="0.28" />
          <stop offset="100%" stop-opacity="0.02" />
        </linearGradient>
      </defs>
      <g class="usage-chart__grid">
        <template v-for="tick in ticks" :key="tick.toString()">
          <line :x1="left" :x2="width - right" :y1="yAt(tick)" :y2="yAt(tick)" />
          <text :x="left - 10" :y="yAt(tick)" dy="0.32em" text-anchor="end">{{ formatTick(tick) }}</text>
        </template>
      </g>
      <path :d="area" :fill="`url(#${gradientId})`" />
      <path :d="line" class="usage-chart__line" />
      <rect v-for="day in days" :key="day.date" :data-date="day.date" v-bind="hitBounds(day.date)" :y="top"
        :height="baseline - top" fill="transparent" aria-hidden="true" />
      <text v-for="tick in axisDates" :key="tick.date" class="usage-chart__axis" :x="tick.x" :y="height - 6"
        :text-anchor="duration === 0 ? 'middle' : tick.offset === 0 ? 'start' : tick.offset === duration ? 'end' : 'middle'">{{ tick.label }}</text>
      <g v-if="activePoint">
        <line class="usage-chart__crosshair" :x1="activePoint.x" :x2="activePoint.x" :y1="top" :y2="baseline" />
        <circle class="usage-chart__dot" :cx="activePoint.x" :cy="activePoint.y" r="5" />
      </g>
    </svg>
    <div v-if="activePoint" class="usage-chart__tooltip" role="status"
      :class="{ 'usage-chart__tooltip--left': activePoint.x > width - 140 }"
      :style="{ left: `${activePoint.x}px`, top: `${activePoint.y}px` }">
      <time :datetime="activePoint.date">{{ activePoint.date.replaceAll('-', '/') }}</time>
      <strong><i class="usage-chart__swatch" />{{ amountLabel }} {{ formatBytes(activePoint.bytes) }}</strong>
    </div>
  </div>
</template>

<style scoped>
.usage-chart { position: relative; min-inline-size: 0; }
.usage-chart svg { display: block; inline-size: 100%; overflow: visible; }
.usage-chart stop { stop-color: var(--color-usage-series); }
.usage-chart__grid line { stroke: var(--color-border); stroke-width: var(--border-width); shape-rendering: crispEdges; }
.usage-chart__grid text, .usage-chart__axis { fill: var(--color-text-secondary); font-size: var(--font-size-usage-axis); }
.usage-chart__line { fill: none; stroke: var(--color-usage-series); stroke-width: 2; stroke-linejoin: round; stroke-linecap: round; }
.usage-chart__crosshair { stroke: var(--color-text-secondary); stroke-width: var(--border-width); stroke-opacity: .4; }
.usage-chart__dot { fill: var(--color-usage-series); stroke: var(--color-surface); stroke-width: 2; }
.usage-chart__swatch {
  display: inline-block;
  inline-size: var(--space-2);
  block-size: var(--space-2);
  margin-inline-end: 6px;
  border-radius: 2px;
  background: var(--color-usage-series);
}
.usage-chart__tooltip {
  position: absolute;
  transform: translate(-50%, calc(-100% - 10px));
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 10px;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-usage-tooltip);
  background: var(--color-surface);
  box-shadow: var(--shadow-usage-tooltip);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  line-height: normal;
  white-space: nowrap;
  pointer-events: none;
}
.usage-chart__tooltip--left { transform: translate(calc(-100% - var(--space-3)), -50%); }
.usage-chart__tooltip strong { color: var(--color-text); font-size: var(--font-size-sm); }
</style>
