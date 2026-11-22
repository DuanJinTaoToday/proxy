<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { sessionPhone } from '@/features/auth'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { consoleNav } from '../navigation'

const { t } = useI18n()
</script>

<template>
  <WorkspaceFrame tenant :key="sessionPhone" :sections="consoleNav" :nav-label="t('nav.consoleNav')" />
</template>
