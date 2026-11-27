<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElAlert, ElDialog } from 'element-plus'
import { hasSupportContact } from './support'
import SupportChannels from './SupportChannels.vue'
import SupportNotice from './SupportNotice.vue'

const { t } = useI18n()
const open = defineModel<boolean>({ required: true })
</script>

<template>
  <ElDialog v-model="open" :title="t('marketing.support.title')" width="min(440px, calc(100vw - var(--space-6)))" append-to-body>
    <SupportChannels v-if="hasSupportContact" variant="panel" />
    <ElAlert v-else :title="t('marketing.support.soonAll')" type="info" :closable="false" />
    <SupportNotice class="console-support__notice" />
  </ElDialog>
</template>

<style scoped>
.console-support__notice { margin-block-start: var(--space-4); color: var(--color-text-secondary); }
</style>
