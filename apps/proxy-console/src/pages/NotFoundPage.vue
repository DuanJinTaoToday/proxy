<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElButton, ElResult } from 'element-plus'

const { t, locale } = useI18n()
</script>

<template>
  <main class="not-found">
    <ElResult icon="warning" :title="t('notFound.title')" :sub-title="t('notFound.description')">
      <template #extra>
        <RouterLink :to="`/${locale}/`" custom v-slot="{ navigate, href }">
          <ElButton type="primary" tag="a" :href="href" @click="navigate">{{ t('notFound.home') }}</ElButton>
        </RouterLink>
      </template>
    </ElResult>
  </main>
</template>

<style scoped>
.not-found { display: grid; place-items: center; min-height: 100dvh; padding: var(--space-4); }
</style>
