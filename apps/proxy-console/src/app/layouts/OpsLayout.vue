<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElTag } from 'element-plus'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { opsNav } from '../navigation'

const { t } = useI18n()
</script>

<template>
  <div class="density-compact">
    <WorkspaceFrame ops :sections="opsNav" :nav-label="t('nav.opsNav')">
      <template #brand-extra><ElTag effect="dark" size="small" disable-transitions class="ops-badge">{{ t('app.opsBadge') }}</ElTag></template>
    </WorkspaceFrame>
  </div>
</template>

<style>
.density-compact {
  --el-border-radius-base: var(--ops-control-radius);
}
.density-compact .el-dialog, .el-dialog.ops-dialog {
  --el-dialog-padding-primary: var(--space-5);
  --el-dialog-border-radius: var(--ops-card-radius);
  --el-border-radius-base: var(--ops-control-radius);
  --el-component-size: var(--control-height);
}
.density-compact .el-dialog__title, .ops-dialog .el-dialog__title {
  color: var(--color-text);
  font-size: var(--font-size-brand);
  font-weight: var(--font-weight-bold);
}
.density-compact .el-dialog__body, .ops-dialog .el-dialog__body {
  color: var(--color-text);
}
.density-compact .el-dialog__footer, .ops-dialog .el-dialog__footer {
  display: flex;
  justify-content: flex-end;
  gap: var(--space-2);
  padding-block-start: var(--ops-dialog-footer-padding);
}
.density-compact .el-dialog__footer .el-button, .ops-dialog .el-dialog__footer .el-button {
  margin-inline-start: 0;
  padding-inline: var(--space-3);
  font-size: var(--font-size-sm);
}
.ops-page-title {
  margin-block-end: var(--ops-page-heading-gap);
  font-size: var(--ops-display-size);
  line-height: var(--ops-heading-line-height);
  letter-spacing: var(--ops-display-tracking);
  font-weight: var(--font-weight-heavy);
}
</style>

<style scoped>
.ops-badge {
  --el-tag-bg-color: var(--color-shell);
  --el-tag-border-color: var(--color-shell);
  --el-tag-text-color: var(--color-accent);
  font-weight: var(--font-weight-semibold);
}
</style>
