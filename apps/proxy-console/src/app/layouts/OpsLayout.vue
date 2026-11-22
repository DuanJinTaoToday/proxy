<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { opsNav } from '../navigation'

const { t } = useI18n()
</script>

<template>
  <div class="density-compact">
    <WorkspaceFrame ops :sections="opsNav" :nav-label="t('nav.opsNav')">
      <template #brand-extra><span class="ops-badge">{{ t('app.opsBadge') }}</span></template>
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
  margin-inline-start: var(--ops-badge-offset);
  padding: var(--ops-badge-offset) var(--space-2);
  border-radius: var(--radius-control);
  background: var(--color-shell);
  color: var(--color-accent);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  line-height: var(--ops-badge-line-height);
  white-space: nowrap;
}
</style>
