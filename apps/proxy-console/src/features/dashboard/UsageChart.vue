<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatGB } from '@/features/dynamic-proxy'

const props = defineProps<{ days: { date: string, bytes: bigint }[] }>()
const { t, locale } = useI18n()
const activeDate = ref<string>()
const activeIndex = computed(() => props.days.findIndex(day => day.date === activeDate.value))
const activeDay = computed(() => props.days[activeIndex.value])
const peak = computed(() => props.days.reduce((max, day) => day.bytes > max ? day.bytes : max, 1n))
const gridMaximum = computed(() => {
  const magnitude = 10n ** BigInt(peak.value.toString().length - 1)
  const step = peak.value > 4n * magnitude ? 2n * magnitude : magnitude
  return ((peak.value + 4n * step - 1n) / (4n * step)) * 4n * step
})
const ticks = computed(() => [4n, 3n, 2n, 1n, 0n].map(step => gridMaximum.value * step / 4n))
const points = computed(() => props.days.map((day, index) => ({
  x: (index + 0.5) * 100 / props.days.length,
  y: 100 - 100 * Number(day.bytes) / Number(gridMaximum.value),
})))
const line = computed(() => points.value.map(({ x, y }, index) => `${index === 0 ? 'M' : 'L'}${x},${y}`).join(' '))
const area = computed(() => {
  const first = points.value[0]
  const last = points.value.at(-1)
  return first && last ? `${line.value} L${last.x},100 L${first.x},100 Z` : ''
})
</script>

<template>
  <div class="usage-chart tabular">
    <div class="usage-chart__axes" aria-hidden="true">
      <span v-for="tick in ticks" :key="tick.toString()">{{ t('dashboard.gb', { value: formatGB(tick, locale) }) }}</span>
    </div>
    <div class="usage-chart__body">
      <div class="usage-chart__plot" @pointerleave="activeDate = undefined">
        <svg role="img" :aria-label="t('dashboard.chartLabel')" viewBox="0 0 100 100" preserveAspectRatio="none">
          <line v-for="tick in [0, 25, 50, 75, 100]" :key="tick" x1="0" x2="100" :y1="tick" :y2="tick" />
          <path :d="area" class="usage-chart__fill" />
          <path :d="line" class="usage-chart__line" />
        </svg>
        <div class="usage-chart__columns" aria-hidden="true">
          <div v-for="day in days" :key="day.date" :class="{ 'usage-chart__active': day.date === activeDate }"
            :data-date="day.date" @pointerenter="activeDate = day.date" />
        </div>
        <div v-if="activeDay" class="usage-chart__tooltip" role="status"
          :class="{ 'usage-chart__tooltip--left': activeIndex >= Math.floor(days.length / 2) }">
          <time :datetime="activeDay.date">{{ activeDay.date }}</time>
          <strong>{{ t('dashboard.gb', { value: formatGB(activeDay.bytes, locale) }) }}</strong>
        </div>
      </div>
      <div class="usage-chart__dates" aria-hidden="true">
        <span v-for="day in days" :key="day.date">{{ day.date.slice(5) }}</span>
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
  block-size: var(--dashboard-chart-height);
  text-align: end;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  max-inline-size: var(--dashboard-chart-axis-width);
  overflow-wrap: anywhere;
}
.usage-chart__axes span { transform: translateY(-50%); }
.usage-chart__plot { position: relative; block-size: var(--dashboard-chart-height); }
.usage-chart__plot svg { width: 100%; height: 100%; }
.usage-chart__plot line, .usage-chart__line { vector-effect: non-scaling-stroke; stroke-width: var(--border-width); stroke: var(--color-border); }
.usage-chart__line { stroke: var(--color-primary); stroke-width: var(--focus-ring-width); fill: none; }
.usage-chart__fill { fill: var(--color-accent); }
.usage-chart__columns { position: absolute; inset: 0; display: flex; }
.usage-chart__columns > div { flex: 1; }
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
  display: flex;
  justify-content: space-around;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  margin-block-start: var(--space-1);
}
@media (max-width: 479px) {
  .usage-chart__dates span:nth-child(even) { visibility: hidden; }
}
</style>
