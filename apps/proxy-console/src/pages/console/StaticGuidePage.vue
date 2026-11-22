<script setup lang="ts">
import { ElCard } from 'element-plus'
import { useI18n } from 'vue-i18n'
import LinkButton from '@/shared/ui/LinkButton.vue'

const { t } = useI18n()
</script>

<template>
  <ElCard class="static-guide" shadow="never" :body-style="{ padding: 'var(--space-5)' }">
    <h2 class="static-guide__title">{{ t('staticGuide.title') }}</h2>
    <ol class="static-guide__steps">
      <li>
        <h3>{{ t('staticGuide.connectionTitle') }}</h3>
        <p>{{ t('staticGuide.connectionDescription') }}</p>
      </li>
      <li>
        <h3>{{ t('staticGuide.clientTitle') }}</h3>
        <i18n-t keypath="staticGuide.clientDescription" tag="p" scope="global">
          <template #format><code class="mono">{{ t('staticGuide.credentialFormat') }}</code></template>
        </i18n-t>
      </li>
      <li>
        <h3>{{ t('staticGuide.renewalTitle') }}</h3>
        <p>{{ t('staticGuide.renewalDescription') }}</p>
      </li>
    </ol>
    <LinkButton :to="{ name: 'console-static-ips' }">{{ t('staticGuide.backToIps') }}</LinkButton>
  </ElCard>
</template>

<style scoped>
.static-guide { --el-card-border-radius: var(--radius-card); }

.static-guide__title {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-semibold);
}

.static-guide__steps {
  display: grid;
  gap: var(--space-4);
  margin: var(--space-5) 0;
  padding-inline-start: var(--space-5);
}

.static-guide__steps h3 {
  font-size: inherit;
  font-weight: var(--font-weight-semibold);
}

.static-guide__steps p {
  margin-top: var(--space-2);
  color: var(--color-text-secondary);
  overflow-wrap: anywhere;
}

.static-guide code {
  padding-inline: var(--space-1);
  border-radius: var(--space-1);
  background: var(--color-surface-muted);
  font-size: var(--font-size-xs);
}
</style>
