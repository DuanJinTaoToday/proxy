<script setup lang="ts">
// 原型切换条：方案、动效与开售状态。不属于被评审的设计，生产构建中不显示。
import { onBeforeUnmount, onMounted } from 'vue'
import { ArrowLeft, ArrowRight } from '@element-plus/icons-vue'
import { saleScenario, type SaleScenario } from './home-products'

const props = defineProps<{ variants: { key: string, name: string }[], variant: string, hero: 'map' | 'globe' }>()
const emit = defineEmits<{ 'update:variant': [string], 'update:hero': ['map' | 'globe'] }>()

const show = import.meta.env.DEV
const index = () => props.variants.findIndex(v => v.key === props.variant)
const step = (d: number) => emit('update:variant', props.variants[(index() + d + props.variants.length) % props.variants.length].key)
const scenarios: { key: SaleScenario, label: string }[] = [
  { key: 'both', label: '两个都开售' },
  { key: 'dynamic', label: '只开动态' },
  { key: 'static', label: '只开静态' },
]

function onKey(e: KeyboardEvent) {
  const t = e.target as HTMLElement
  if (t.closest('input, textarea, [contenteditable]')) return
  if (e.key === 'ArrowLeft') step(-1)
  if (e.key === 'ArrowRight') step(1)
}
onMounted(() => window.addEventListener('keydown', onKey))
onBeforeUnmount(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <div v-if="show" class="proto-bar" role="toolbar" aria-label="原型切换">
    <span class="proto-tag">原型</span>
    <button type="button" aria-label="上一个方案" @click="step(-1)"><ArrowLeft /></button>
    <strong class="proto-label">{{ variant }} · {{ variants[index()]?.name }}</strong>
    <button type="button" aria-label="下一个方案" @click="step(1)"><ArrowRight /></button>
    <span class="sep" />
    <span class="proto-group" :class="{ off: variant === 'B' }" :title="variant === 'B' ? '方案 B 首屏没有动效背景' : ''">
      动效
      <button v-for="h in (['map', 'globe'] as const)" :key="h" type="button" :class="{ on: hero === h }" @click="emit('update:hero', h)">{{ h === 'map' ? '地图' : '地球' }}</button>
    </span>
    <span class="sep" />
    <span class="proto-group">
      开售
      <button v-for="s in scenarios" :key="s.key" type="button" :class="{ on: saleScenario === s.key }" @click="saleScenario = s.key">{{ s.label }}</button>
    </span>
  </div>
</template>

<style scoped>
.proto-bar { position: fixed; left: 50%; bottom: 20px; transform: translateX(-50%); z-index: 100; display: flex; align-items: center; gap: 8px; padding: 8px 12px; border-radius: 999px; background: #ffde59; color: #1a1a1a; box-shadow: 0 12px 30px rgba(0, 0, 0, .35); font-size: 13px; white-space: nowrap; font-family: system-ui, sans-serif; }
.proto-tag { font-weight: 800; font-size: 11px; letter-spacing: .08em; background: #1a1a1a; color: #ffde59; padding: 3px 8px; border-radius: 99px; }
button { border: 0; background: rgba(0, 0, 0, .08); color: inherit; border-radius: 99px; padding: 5px 10px; cursor: pointer; display: inline-flex; align-items: center; font-size: 12px; }
button:hover { background: rgba(0, 0, 0, .16); }
button.on { background: #1a1a1a; color: #ffde59; }
.proto-label { min-width: 120px; text-align: center; }
.sep { width: 1px; height: 18px; background: rgba(0, 0, 0, .2); }
.proto-group { display: inline-flex; align-items: center; gap: 4px; font-weight: 650; }
.proto-group.off { opacity: .45; }
</style>
