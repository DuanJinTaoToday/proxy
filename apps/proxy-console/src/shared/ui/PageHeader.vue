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
  margin-bottom: var(--page-heading-gap, var(--space-4));
}
.page-header__title {
  font-size: var(--page-heading-size, var(--font-size-title));
  font-weight: var(--page-heading-weight, var(--font-weight-bold));
  line-height: var(--page-heading-height, var(--line-height-tight));
  letter-spacing: var(--page-heading-tracking, normal);
  overflow-wrap: anywhere;
}
.page-header__actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
</style>
