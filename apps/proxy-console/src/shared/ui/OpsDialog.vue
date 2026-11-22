<script setup lang="ts">
import { ElDialog } from 'element-plus'

defineProps<{ title?: string }>()
const open = defineModel<boolean>({ required: true })
</script>

<template>
  <ElDialog v-model="open" class="ops-dialog" body-class="ops-dialog-body" footer-class="ops-dialog-footer" align-center>
    <template #header="{ titleId }">
      <span :id="titleId" class="ops-dialog-title" role="heading" aria-level="2">{{ title }}</span>
    </template>
    <slot />
    <template #footer><slot name="footer" /></template>
  </ElDialog>
</template>

<style>
:root .ops-dialog {
  --el-dialog-padding-primary: var(--space-5);
  --el-dialog-border-radius: var(--ops-card-radius);
  --el-border-radius-base: var(--ops-control-radius);
  --el-component-size: var(--control-height);
}
.ops-dialog-title {
  color: var(--color-text);
  font-size: var(--font-size-brand);
  font-weight: var(--font-weight-bold);
  line-height: var(--el-dialog-font-line-height);
}
.ops-dialog .ops-dialog-body { color: var(--color-text); }
.ops-dialog .ops-dialog-footer {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--space-2);
  padding-block-start: var(--ops-dialog-footer-padding);
}
.ops-dialog-footer .ops-dialog-action {
  margin-inline-start: 0;
  padding-inline: var(--space-3);
  font-size: var(--font-size-sm);
}
</style>
