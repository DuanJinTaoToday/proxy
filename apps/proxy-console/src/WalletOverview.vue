<script setup lang="ts">
import { computed } from 'vue'
import { ArrowDownLeft, ArrowUpRight, RotateCcw, Wallet } from 'lucide-vue-next'
import type { Entry } from './api/wallet'
import { dateTime, money } from './wallet-format'
import { Badge } from './ui/badge'
import Card from './ui/card/Card.vue'

const props = defineProps<{ balance?: string; entries: Entry[] | null }>()
const entryDelta = (entry: Entry) => BigInt(entry.after_minor ?? '0') - BigInt(entry.before_minor ?? '0')
const cards = computed(() => [
  { label: '可用余额', value: money(props.balance), note: '人民币（CNY）', icon: Wallet },
  ...([
    { label: '近期充值', kind: 'recharge', icon: ArrowDownLeft },
    { label: '近期扣款', kind: 'debit', icon: ArrowUpRight },
    { label: '近期退款', kind: 'refund', icon: RotateCcw },
  ] as const).map(item => {
    const entries = props.entries?.filter(entry => entry.kind === item.kind)
    const total = entries?.reduce((sum, entry) => sum + entryDelta(entry), 0n)
    return {
      ...item,
      value: total === undefined ? '—' : money(item.kind === 'debit' ? -total : total),
      note: entries ? `${entries.length} 笔 · 最近一页流水` : '依据最近一页流水',
    }
  }),
])

const chart = computed(() => {
  const entries = [...(props.entries ?? [])].reverse()
  const balances = entries.map(entry => BigInt(entry.after_minor ?? '0'))
  let min = 0n
  let max = 0n
  for (const balance of balances) {
    if (balance < min) min = balance
    if (balance > max) max = balance
  }
  if (min === max) max = min + 100n
  const points = balances.map((balance, index) => ({
    x: entries.length === 1 ? 400 : 12 + index * 776 / (entries.length - 1),
    y: 122 - Number((balance - min) * 11200n / (max - min)) / 100,
  }))
  const line = points.map(({ x, y }, index) => `${index ? 'L' : 'M'}${x},${y}`).join(' ')
  return { entries, points, min, max, line, area: points.length > 1 ? `${line} L788,122 L12,122 Z` : '' }
})

const shortDate = new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric', timeZone: 'Asia/Shanghai' })
</script>

<template>
  <div class="summary-grid">
    <Card v-for="card in cards" :key="card.label" class="dashboard-card summary-card block">
      <div class="summary-heading"><h2>{{ card.label }}</h2><component :is="card.icon" :size="16" /></div>
      <p class="summary-number">{{ card.value }}</p>
      <p class="summary-note">{{ card.note }}</p>
    </Card>
  </div>

  <Card role="region" class="dashboard-card chart-card block" aria-labelledby="wallet-chart-title">
    <div class="chart-heading">
      <div><h2 id="wallet-chart-title">余额走势</h2><p>最近一页钱包流水 · 按交易顺序</p></div>
      <Badge variant="outline" class="dashboard-badge"><span class="chart-key"></span>钱包余额</Badge>
    </div>
    <p v-if="!chart.points.length" class="chart-empty">{{ entries === null ? '流水加载后显示余额走势' : '暂无钱包流水' }}</p>
    <div v-else class="balance-chart">
      <div class="chart-axis" aria-hidden="true"><span>{{ money(chart.max) }}</span><span>{{ money((chart.max + chart.min) / 2n) }}</span><span>{{ money(chart.min) }}</span></div>
      <div class="chart-plot">
        <svg viewBox="0 0 800 132" preserveAspectRatio="none" role="img" aria-labelledby="wallet-chart-title">
          <defs><linearGradient id="wallet-balance-area" x1="0" x2="0" y1="0" y2="1"><stop stop-color="#18181b" stop-opacity=".19" /><stop offset="1" stop-color="#18181b" stop-opacity=".02" /></linearGradient></defs>
          <path d="M0 10H800 M0 66H800 M0 122H800" stroke="#e4e4e7" fill="none" vector-effect="non-scaling-stroke" />
          <path v-if="chart.area" :d="chart.area" fill="url(#wallet-balance-area)" />
          <path :d="chart.line" fill="none" stroke="#27272a" stroke-width="2" vector-effect="non-scaling-stroke" />
          <line v-for="(point, index) in chart.points" :key="chart.entries[index].id"
            :x1="point.x" :x2="point.x" :y1="point.y" :y2="point.y + .01"
            stroke="#27272a" stroke-width="5" stroke-linecap="round" vector-effect="non-scaling-stroke">
            <title>{{ dateTime(chart.entries[index].created_at) }} · {{ money(chart.entries[index].after_minor) }}</title>
          </line>
        </svg>
        <div class="chart-dates" :class="{ 'single-date': chart.entries.length === 1 }" aria-hidden="true">
          <span>{{ shortDate.format(new Date(chart.entries[0].created_at)) }}</span>
          <span v-if="chart.entries.length > 1">{{ shortDate.format(new Date(chart.entries[chart.entries.length - 1].created_at)) }}</span>
        </div>
      </div>
    </div>
  </Card>
</template>
