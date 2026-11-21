<script setup lang="ts">
import { computed, ref } from 'vue'
import { dayFromNumber, dayNumber } from '@/shared/lib/businessDates'

interface UsagePoint {
  date: string
  bytes: bigint
}

const props = defineProps<{
  days: UsagePoint[]
  range?: { start: string, end: string }
  label: string
  formatBytes: (bytes: bigint) => string
}>()
const activeDate = ref<string>()
const activeDay = computed(() => props.days.find(day => day.date === activeDate.value))
const start = computed(() => dayNumber(props.range?.start ?? props.days[0]?.date ?? '1970-01-01'))
const end = computed(() => dayNumber(props.range?.end ?? props.days.at(-1)?.date ?? '1970-01-01'))
const dayWidth = computed(() => 100 / (end.value - start.value + 1))
const xAt = (day: number) => (day - start.value + 0.5) * dayWidth.value
const peak = computed(() => props.days.reduce((max, day) => day.bytes > max ? day.bytes : max, 1n))
const gridMaximum = computed(() => {
  const magnitude = 10n ** BigInt(peak.value.toString().length - 1)
  const step = peak.value > 4n * magnitude ? 2n * magnitude : magnitude
  return ((peak.value + 4n * step - 1n) / (4n * step)) * 4n * step
})
const ticks = computed(() => [4n, 3n, 2n, 1n, 0n].map(step => gridMaximum.value * step / 4n))
const samples = computed(() => {
  const values = new Map(props.days.map(day => [dayNumber(day.date), day.bytes]))
  // Missing days have zero usage. Add only the points needed to draw their baseline.
  for (const day of props.days) {
    const ordinal = dayNumber(day.date)
    if (ordinal > start.value && !values.has(ordinal - 1)) values.set(ordinal - 1, 0n)
    if (ordinal < end.value && !values.has(ordinal + 1)) values.set(ordinal + 1, 0n)
  }
  return [...values.entries()].sort(([a], [b]) => a - b)
})
const line = computed(() => {
  const points = samples.value.map(([day, bytes]) => ({
    x: xAt(day),
    y: 100 - 100 * Number(bytes) / Number(gridMaximum.value),
  }))
  const first = points[0]
  const last = points.at(-1)
  if (!first || !last) return ''
  return ['M0,' + first.y, ...points.map(({ x, y }) => `L${x},${y}`), 'L100,' + last.y].join(' ')
})
const area = computed(() => line.value ? line.value + ' L100,100 L0,100 Z' : '')
const axisDays = computed(() => {
  const offset = end.value - start.value
  const count = Math.min(offset, 6)
  return Array.from({ length: count + 1 }, (_, index) => {
    const day = start.value + (count ? Math.round(index * offset / count) : 0)
    const date = dayFromNumber(day)
    return { date, label: offset >= 365 ? date : date.slice(5), x: xAt(day) }
  })
})
</script>

<template>
  <div class="usage-chart tabular">
    <div class="usage-chart__axes" aria-hidden="true">
      <span v-for="tick in ticks" :key="tick.toString()">{{ formatBytes(tick) }}</span>
    </div>
    <div class="usage-chart__body">
      <div class="usage-chart__plot" @pointerleave="activeDate = undefined">
        <svg role="img" :aria-label="label" viewBox="0 0 100 100" preserveAspectRatio="none">
          <line v-for="tick in [0, 25, 50, 75, 100]" :key="tick" x1="0" x2="100" :y1="tick" :y2="tick" />
          <path :d="area" class="usage-chart__fill" />
          <path :d="line" class="usage-chart__line" />
        </svg>
        <div v-for="day in days" :key="day.date" aria-hidden="true" class="usage-chart__column"
          :class="{ 'usage-chart__active': day.date === activeDate }"
          :style="{ left: (xAt(dayNumber(day.date)) - dayWidth / 2) + '%', width: dayWidth + '%' }"
          :data-date="day.date" @pointerenter="activeDate = day.date" @pointerleave="activeDate = undefined" />
        <div v-if="activeDay" class="usage-chart__tooltip" role="status"
          :class="{ 'usage-chart__tooltip--left': xAt(dayNumber(activeDay.date)) >= 50 }">
          <time :datetime="activeDay.date">{{ activeDay.date }}</time>
          <strong>{{ formatBytes(activeDay.bytes) }}</strong>
        </div>
      </div>
      <div class="usage-chart__dates" aria-hidden="true">
        <span v-for="day in axisDays" :key="day.date" :style="{ left: day.x + '%' }">{{ day.label }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.usage-chart { display: grid; grid-template-columns: minmax(0, auto) minmax(0, 1fr); gap: var(--space-2); }
.usage-chart__axes {
  display: grid;
  grid-template-rows: repeat(4, 1fr) 0;
  align-items: start;
  block-size: var(--usage-chart-height);
  text-align: end;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  max-inline-size: var(--usage-chart-axis-width);
  overflow-wrap: anywhere;
}
.usage-chart__axes span { transform: translateY(-50%); }
.usage-chart__plot { position: relative; block-size: var(--usage-chart-height); }
.usage-chart__plot svg { width: 100%; height: 100%; }
.usage-chart__plot line, .usage-chart__line { vector-effect: non-scaling-stroke; stroke-width: var(--border-width); stroke: var(--color-border); }
.usage-chart__line { stroke: var(--color-primary); stroke-width: var(--focus-ring-width); fill: none; }
.usage-chart__fill { fill: var(--color-accent); }
.usage-chart__column { position: absolute; inset-block: 0; }
.usage-chart__active { background: var(--color-primary-soft); opacity: .4; }
.usage-chart__tooltip {
  position: absolute;
  inset-block-start: var(--space-2);
  inset-inline-end: var(--space-2);
  display: grid;
  padding: var(--space-2) var(--space-3);
  background: var(--color-surface);
  color: var(--color-text);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-control);
  box-shadow: var(--shadow-overlay);
  font-size: var(--font-size-xs);
  pointer-events: none;
}
.usage-chart__tooltip--left { inset-inline: var(--space-2) auto; }
.usage-chart__dates {
  position: relative;
  min-block-size: calc(var(--font-size-xs) * var(--line-height-normal));
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  margin-block-start: var(--space-1);
}
.usage-chart__dates span { position: absolute; transform: translateX(-50%); white-space: nowrap; }
.usage-chart__dates span:first-child:not(:only-child) { transform: none; }
.usage-chart__dates span:last-child:not(:only-child) { transform: translateX(-100%); }
@media (max-width: 700px) {
  .usage-chart__dates span:nth-child(even):not(:last-child) { visibility: hidden; }
}
</style>
