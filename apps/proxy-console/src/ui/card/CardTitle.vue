<script setup lang="ts">
import type { HTMLAttributes } from "vue"
import { cn } from "../utils"

const props = defineProps<{
  as?: 'h1' | 'h2' | 'h3'
  class?: HTMLAttributes["class"]
}>()
</script>

<template>
  <component
    :is="as ?? 'h3'"
    data-slot="card-title"
    :class="cn('leading-none font-semibold', props.class)"
  >
    <slot />
  </component>
</template>
