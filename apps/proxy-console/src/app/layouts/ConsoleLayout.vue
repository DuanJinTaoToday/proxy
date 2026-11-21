<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { consoleNav } from '../navigation'

const { t } = useI18n()
</script>

<template>
  <WorkspaceFrame :sections="consoleNav" :nav-label="t('nav.consoleNav')" />
</template>
