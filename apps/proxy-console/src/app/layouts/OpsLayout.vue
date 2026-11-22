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
