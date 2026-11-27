<script setup lang="ts">
import { ref } from 'vue'
import { ElDropdownItem } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { sessionEmail } from '@/features/auth'
import { ConsoleSupport } from '@/features/home'
import WorkspaceFrame from './WorkspaceFrame.vue'
import { consoleNav } from '../navigation'

const { t } = useI18n()
const supportOpen = ref(false)
</script>

<template>
  <WorkspaceFrame tenant :key="sessionEmail" :sections="consoleNav" :nav-label="t('nav.consoleNav')">
    <template #account-actions>
      <ElDropdownItem @click="supportOpen = true">{{ t('marketing.support.title') }}</ElDropdownItem>
    </template>
  </WorkspaceFrame>
  <ConsoleSupport v-model="supportOpen" />
</template>
