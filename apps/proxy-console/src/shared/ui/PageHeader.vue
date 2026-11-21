<script setup lang="ts">
// 页面标题区：标题在左，页面级操作在右，窄屏时操作换行。
defineProps<{ title: string }>()
</script>

<template>
  <div class="page-header">
    <h1 class="page-header__title">{{ title }}</h1>
    <div v-if="$slots.actions" class="page-header__actions"><slot name="actions" /></div>
  </div>
</template>

<style scoped>
.page-header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}
.page-header__title {
  font-size: var(--font-size-title);
  font-weight: var(--font-weight-bold);
  line-height: var(--line-height-tight);
  overflow-wrap: anywhere;
}
.page-header__actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
</style>
