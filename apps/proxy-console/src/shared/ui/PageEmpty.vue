<script setup lang="ts">
import { ElEmpty } from 'element-plus'

// 页面级空状态：在卡片里展示说明，可通过默认插槽放置下一步操作。
defineProps<{ description: string, hint?: string }>()
</script>

<template>
  <section class="page-empty">
    <ElEmpty :description="description" :image-size="88">
      <p v-if="hint" class="page-empty__hint">{{ hint }}</p>
      <slot />
    </ElEmpty>
  </section>
</template>

<style scoped>
.page-empty {
  padding: var(--space-6) var(--space-4);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--shadow-card);
}
.page-empty__hint { color: var(--color-text-secondary); font-size: var(--font-size-sm); }
</style>
