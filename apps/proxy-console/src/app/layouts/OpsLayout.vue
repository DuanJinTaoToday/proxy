<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElTag } from 'element-plus'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { opsNav } from '../navigation'

const { t } = useI18n()
</script>

<template>
  <div class="density-compact">
    <WorkspaceFrame :sections="opsNav" :nav-label="t('nav.opsNav')">
      <template #brand-extra><ElTag effect="dark" size="small" disable-transitions>{{ t('app.opsBadge') }}</ElTag></template>
    </WorkspaceFrame>
  </div>
</template>
