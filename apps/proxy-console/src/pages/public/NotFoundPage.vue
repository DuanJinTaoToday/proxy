<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElResult } from 'element-plus'
import LinkButton from '@/shared/ui/LinkButton.vue'

const { t, locale } = useI18n()
</script>

<template>
  <main class="not-found">
    <ElResult icon="warning" :title="t('notFound.title')" :sub-title="t('notFound.description')">
      <template #extra>
        <LinkButton :to="`/${locale}/`" type="primary">{{ t('notFound.home') }}</LinkButton>
      </template>
    </ElResult>
  </main>
</template>

<style scoped>
.not-found { display: grid; place-items: center; min-height: 100dvh; padding: var(--space-4); }
</style>
