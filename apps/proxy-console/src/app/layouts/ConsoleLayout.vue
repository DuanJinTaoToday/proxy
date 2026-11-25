<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { sessionEmail } from '@/features/auth'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { consoleNav } from '../navigation'

const { t } = useI18n()
</script>

<template>
  <WorkspaceFrame tenant :key="sessionEmail" :sections="consoleNav" :nav-label="t('nav.consoleNav')" />
</template>
