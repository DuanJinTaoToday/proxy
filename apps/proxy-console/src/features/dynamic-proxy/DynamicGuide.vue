<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElAlert, ElCard } from 'element-plus'
import LinkButton from '@/shared/ui/LinkButton.vue'

const { t } = useI18n()
const steps = ['purchase', 'account', 'credentials', 'session', 'connect', 'usage']
const questions = ['authentication', 'quota', 'rotation']
</script>

<template>
  <ElCard shadow="never" class="guide">
    <template #header><h2>{{ t('dynamicGuide.heading') }}</h2></template>
    <ElAlert type="warning" :closable="false" show-icon :title="t('dynamicGuide.region')" />
    <ol>
      <li v-for="step in steps" :key="step">
        <strong>{{ t('dynamicGuide.' + step + '.title') }}</strong>
        <p>{{ t('dynamicGuide.' + step + '.body') }}</p>
        <code v-if="step === 'credentials'">{{ t('dynamicGuide.usernameFormat') }}</code>
        <code v-if="step === 'session'">{{ t('dynamicGuide.sessionFormat') }}</code>
        <pre v-if="step === 'connect'"><code>{{ t('dynamicGuide.command') }}</code></pre>
      </li>
    </ol>
    <h3>{{ t('dynamicGuide.faq') }}</h3>
    <dl>
      <div v-for="question in questions" :key="question">
        <dt>{{ t('dynamicGuide.' + question + '.title') }}</dt>
        <dd>{{ t('dynamicGuide.' + question + '.body') }}</dd>
      </div>
    </dl>
    <LinkButton :to="{ name: 'console-dynamic-setup' }" type="primary">{{ t('dynamicGuide.openSetup') }}</LinkButton>
  </ElCard>
</template>

<style scoped>
.guide { max-inline-size: var(--content-narrow-width); }
h2, h3 { margin: 0; font-size: var(--font-size-lg); }
ol { padding-inline-start: var(--space-5); }
li, dl > div { margin-block: var(--space-4); }
p, dd { margin: var(--space-1) 0 0; color: var(--color-text-secondary); }
dt { font-weight: var(--font-weight-semibold); }
code { font-family: var(--font-mono); overflow-wrap: anywhere; }
pre { white-space: pre-wrap; padding: var(--space-3); background: var(--color-surface-muted); border-radius: var(--radius-control); }
</style>
