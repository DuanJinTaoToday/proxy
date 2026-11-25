<script setup lang="ts">
// 认证页面共用的卡片：标题、说明和表单内容。
defineProps<{ title: string, subtitle?: string }>()
</script>

<template>
  <section class="auth-card">
    <h1 class="auth-card__title">{{ title }}</h1>
    <p v-if="subtitle" class="auth-card__subtitle">{{ subtitle }}</p>
    <slot />
  </section>
</template>

<style scoped>
.auth-card {
  width: 100%;
  max-width: var(--auth-card-width);
  margin-inline: auto;
  padding: var(--space-6);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--shadow-card);
}
.auth-card__title { font-size: var(--font-size-title); font-weight: var(--font-weight-bold); line-height: var(--line-height-tight); overflow-wrap: anywhere; }
.auth-card__subtitle { margin: var(--space-2) 0 var(--space-5); color: var(--color-text-secondary); }
.auth-card :deep(.auth-card__alert) { margin-bottom: var(--space-4); }
.auth-card :deep(.auth-card__hint) { margin-left: var(--space-2); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.auth-card :deep(.auth-card__submit) { width: 100%; margin-top: var(--space-2); }
.auth-card :deep(.auth-card__footer) { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-2); margin-top: var(--space-4); color: var(--color-text-secondary); }

@media (max-width: 767px) {
  .auth-card { padding: var(--space-5) var(--space-4); }
}
</style>
