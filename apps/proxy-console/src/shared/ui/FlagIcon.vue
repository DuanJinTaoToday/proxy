<script setup lang="ts">
import { useId } from 'vue'
import type { FlagCode } from '@/shared/lib/regionFlags'

export type { FlagCode }

withDefaults(defineProps<{ code: FlagCode, size?: number }>(), { size: 28 })
const clipId = `flag-clip-${useId()}`
</script>

<template>
  <span class="flag-icon" :style="{ width: `${size}px`, height: `${size}px` }" aria-hidden="true">
    <svg v-if="code === 'other'" viewBox="0 0 32 32" fill="none">
      <circle cx="16" cy="16" r="16" class="flag-icon__globe-bg" />
      <circle cx="16" cy="16" r="9.5" class="flag-icon__globe-stroke" stroke-width="1.6" />
      <ellipse cx="16" cy="16" rx="4.6" ry="9.5" class="flag-icon__globe-stroke" stroke-width="1.4" />
      <path d="M6.8 16h18.4M8.4 11.8h15.2M8.4 20.2h15.2" class="flag-icon__globe-stroke" stroke-width="1.3" />
    </svg>
    <svg v-else viewBox="0 0 32 32">
      <defs><clipPath :id="clipId"><circle cx="16" cy="16" r="16" /></clipPath></defs>
      <g :clip-path="`url(#${clipId})`">
        <template v-if="code === 'us'">
          <rect width="32" height="32" fill="#ffffff" />
          <path d="M0 0h32v2.5H0zm0 5h32v2.5H0zm0 5h32v2.5H0zm0 5h32v2.5H0zm0 5h32v2.5H0zm0 5h32v2.5H0zm0 5h32v2.5H0z" fill="#d32f2f" />
          <rect width="16" height="17" fill="#1e3a8a" />
          <g fill="#ffffff">
            <circle cx="3.2" cy="3.4" r=".9" /><circle cx="6.8" cy="3.4" r=".9" /><circle cx="10.4" cy="3.4" r=".9" /><circle cx="13.6" cy="3.4" r=".9" />
            <circle cx="5" cy="6.6" r=".9" /><circle cx="8.6" cy="6.6" r=".9" /><circle cx="12.2" cy="6.6" r=".9" />
            <circle cx="3.2" cy="9.8" r=".9" /><circle cx="6.8" cy="9.8" r=".9" /><circle cx="10.4" cy="9.8" r=".9" /><circle cx="13.6" cy="9.8" r=".9" />
            <circle cx="5" cy="13" r=".9" /><circle cx="8.6" cy="13" r=".9" /><circle cx="12.2" cy="13" r=".9" />
          </g>
        </template>
        <template v-else-if="code === 'gb'">
          <rect width="32" height="32" fill="#1e3a8a" />
          <path d="M0 0l32 32M32 0L0 32" stroke="#ffffff" stroke-width="6.5" />
          <path d="M0 0l32 32M32 0L0 32" stroke="#d32f2f" stroke-width="2.6" />
          <path d="M16 0v32M0 16h32" stroke="#ffffff" stroke-width="9" />
          <path d="M16 0v32M0 16h32" stroke="#d32f2f" stroke-width="5.2" />
        </template>
        <template v-else-if="code === 'jp'">
          <rect width="32" height="32" fill="#ffffff" />
          <circle cx="16" cy="16" r="7.2" fill="#dc2626" />
        </template>
        <template v-else-if="code === 'de'">
          <rect width="32" height="10.7" fill="#111827" />
          <rect y="10.7" width="32" height="10.6" fill="#dc2626" />
          <rect y="21.3" width="32" height="10.7" fill="#facc15" />
        </template>
        <template v-else-if="code === 'fr'">
          <rect width="10.7" height="32" fill="#1d4ed8" />
          <rect x="10.7" width="10.6" height="32" fill="#ffffff" />
          <rect x="21.3" width="10.7" height="32" fill="#dc2626" />
        </template>
        <template v-else-if="code === 'ca'">
          <rect width="32" height="32" fill="#ffffff" />
          <rect width="8.5" height="32" fill="#dc2626" />
          <rect x="23.5" width="8.5" height="32" fill="#dc2626" />
          <path d="M16 8.5l1.5 3.6 2.8-1-.9 3.1 2.6 1.2-1.8 1.5.5 2-3.7-.6V22h-2v-3.7l-3.7.6.5-2-1.8-1.5 2.6-1.2-.9-3.1 2.8 1z" fill="#dc2626" />
        </template>
        <template v-else-if="code === 'it'">
          <rect width="10.7" height="32" fill="#15803d" />
          <rect x="10.7" width="10.6" height="32" fill="#ffffff" />
          <rect x="21.3" width="10.7" height="32" fill="#dc2626" />
        </template>
        <template v-else-if="code === 'es'">
          <rect width="32" height="8" fill="#dc2626" />
          <rect y="8" width="32" height="16" fill="#facc15" />
          <rect y="24" width="32" height="8" fill="#dc2626" />
          <rect x="7" y="12.5" width="5" height="7" rx="1.5" fill="#b91c1c" />
        </template>
        <template v-else-if="code === 'sg'">
          <rect width="32" height="16" fill="#dc2626" />
          <rect y="16" width="32" height="16" fill="#ffffff" />
          <circle cx="11.5" cy="9.2" r="4.2" fill="#ffffff" />
          <circle cx="13.1" cy="9.2" r="3.4" fill="#dc2626" />
          <g fill="#ffffff">
            <circle cx="15.8" cy="7.2" r=".8" /><circle cx="17.8" cy="8.5" r=".8" /><circle cx="17" cy="10.8" r=".8" /><circle cx="14.6" cy="10.8" r=".8" /><circle cx="13.8" cy="8.5" r=".8" />
          </g>
        </template>
        <template v-else-if="code === 'br'">
          <rect width="32" height="32" fill="#16a34a" />
          <polygon points="16,5 28,16 16,27 4,16" fill="#facc15" />
          <circle cx="16" cy="16" r="5.5" fill="#1d4ed8" />
          <path d="M11 15.2c3.2-1 6.8-.6 10 1.4" stroke="#ffffff" stroke-width="1.3" fill="none" />
        </template>
        <template v-else-if="code === 'kr'">
          <rect width="32" height="32" fill="#ffffff" />
          <circle cx="16" cy="16" r="6.2" fill="#1d4ed8" />
          <path d="M16 9.8a6.2 6.2 0 0 1 0 12.4 3.1 3.1 0 0 1 0-6.2 3.1 3.1 0 0 0 0-6.2z" fill="#dc2626" />
          <path d="M6.5 9.5l3-3M22.5 25.5l3-3M22.5 6.5l3 3M6.5 22.5l3 3" stroke="#111827" stroke-width="1.8" />
        </template>
        <template v-else-if="code === 'my'">
          <rect width="32" height="32" fill="#ffffff" />
          <path d="M0 0h32v3.2H0zm0 6.4h32v3.2H0zm0 6.4h32v3.2H0zm0 6.4h32v3.2H0zm0 6.4h32v3.2H0z" fill="#dc2626" />
          <rect width="17" height="16" fill="#1e3a8a" />
          <circle cx="8" cy="8.2" r="4" fill="#facc15" />
          <circle cx="9.4" cy="8.2" r="3.2" fill="#1e3a8a" />
          <circle cx="12.5" cy="8.2" r="2" fill="#facc15" />
        </template>
        <template v-else-if="code === 'th'">
          <rect width="32" height="5.5" fill="#dc2626" />
          <rect y="5.5" width="32" height="5" fill="#ffffff" />
          <rect y="10.5" width="32" height="11" fill="#1e3a8a" />
          <rect y="21.5" width="32" height="5" fill="#ffffff" />
          <rect y="26.5" width="32" height="5.5" fill="#dc2626" />
        </template>
        <template v-else-if="code === 'vn'">
          <rect width="32" height="32" fill="#dc2626" />
          <polygon points="16,8.2 18.2,14.2 24.5,14.2 19.4,17.8 21.3,23.8 16,20.2 10.7,23.8 12.6,17.8 7.5,14.2 13.8,14.2" fill="#facc15" />
        </template>
        <template v-else-if="code === 'id'">
          <rect width="32" height="16" fill="#dc2626" />
          <rect y="16" width="32" height="16" fill="#ffffff" />
        </template>
        <template v-else-if="code === 'au'">
          <rect width="32" height="32" fill="#1e3a8a" />
          <path d="M0 8h16M8 0v16" stroke="#ffffff" stroke-width="3.6" />
          <path d="M0 8h16M8 0v16" stroke="#dc2626" stroke-width="2" />
          <g fill="#ffffff">
            <circle cx="23.5" cy="10" r="1.4" /><circle cx="26.2" cy="16" r="1.4" /><circle cx="22.5" cy="22.5" r="1.6" /><circle cx="9.5" cy="23.5" r="2.2" />
          </g>
        </template>
        <template v-else-if="code === 'hk'">
          <rect width="32" height="32" fill="#dc2626" />
          <g fill="#ffffff">
            <circle cx="16" cy="11.2" r="2.5" /><circle cx="20.6" cy="14.6" r="2.5" /><circle cx="18.8" cy="20" r="2.5" /><circle cx="13.2" cy="20" r="2.5" /><circle cx="11.4" cy="14.6" r="2.5" />
          </g>
        </template>
        <template v-else-if="code === 'tw'">
          <rect width="32" height="32" fill="#dc2626" />
          <rect width="17" height="17" fill="#1e3a8a" />
          <circle cx="9.5" cy="9.5" r="4" fill="#ffffff" />
        </template>
        <template v-else-if="code === 'ae'">
          <rect x="9" width="23" height="10.7" fill="#15803d" />
          <rect x="9" y="10.7" width="23" height="10.6" fill="#ffffff" />
          <rect x="9" y="21.3" width="23" height="10.7" fill="#111827" />
          <rect width="9.5" height="32" fill="#dc2626" />
        </template>
        <template v-else-if="code === 'ph'">
          <rect width="32" height="16" fill="#1d4ed8" />
          <rect y="16" width="32" height="16" fill="#dc2626" />
          <polygon points="0,0 15.5,16 0,32" fill="#ffffff" />
          <circle cx="6.2" cy="16" r="3" fill="#facc15" />
        </template>
      </g>
      <circle cx="16" cy="16" r="15.5" fill="none" class="flag-icon__ring" />
    </svg>
  </span>
</template>

<style scoped>
.flag-icon { display: inline-flex; flex: none; border-radius: 50%; overflow: hidden; }
.flag-icon svg { width: 100%; height: 100%; display: block; }
.flag-icon__globe-bg { fill: var(--home-brand-soft, var(--color-surface-muted)); }
.flag-icon__globe-stroke { stroke: var(--home-brand, var(--color-primary)); }
.flag-icon__ring { stroke: var(--home-flag-ring, var(--color-border)); }
</style>
