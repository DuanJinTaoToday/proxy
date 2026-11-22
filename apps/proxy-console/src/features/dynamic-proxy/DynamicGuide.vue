<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElAlert, ElCard } from 'element-plus'
import LinkButton from '@/shared/ui/LinkButton.vue'

const { t } = useI18n()
const steps = ['purchase', 'account', 'credentials', 'session', 'connect', 'usage']
const questions = ['authentication', 'quota', 'rotation']
const python = [
  'import requests',
  '',
  'proxy = "socks5h://USERNAME:PASSWORD@GATEWAY_HOST:PORT"',
  'resp = requests.get("https://api.ipify.org", proxies={"http": proxy, "https": proxy}, timeout=30)',
  'print(resp.text)',
].join('\n')
</script>

<template>
  <ElCard shadow="never" class="guide">
    <h2>{{ t('dynamicGuide.heading') }}</h2>
    <ElAlert type="warning" :closable="false" show-icon :title="t('dynamicGuide.region')" />
    <ol>
      <li v-for="step in steps" :key="step">
        <strong>{{ t('dynamicGuide.' + step + '.title') }}</strong>
        <i18n-t v-if="step === 'credentials'" keypath="dynamicGuide.credentials.body" tag="p" scope="global">
          <template #username><code>{{ t('dynamicGuide.usernameFormat') }}</code></template>
        </i18n-t>
        <i18n-t v-else-if="step === 'session'" keypath="dynamicGuide.session.body" tag="p" scope="global">
          <template #session><code>{{ t('dynamicGuide.sessionFormat') }}</code></template>
        </i18n-t>
        <p v-else>{{ t('dynamicGuide.' + step + '.body') }}</p>
        <div v-if="step === 'connect'" class="guide-example">
          <div>Python</div>
          <pre><code>{{ python }}</code></pre>
        </div>
      </li>
    </ol>
    <LinkButton :to="{ name: 'console-dynamic-setup' }">{{ t('dynamicGuide.openSetup') }}</LinkButton>
    <section class="guide-faq">
      <h3>{{ t('dynamicGuide.faq') }}</h3>
      <dl>
        <div v-for="question in questions" :key="question">
          <dt>{{ t('dynamicGuide.' + question + '.title') }}</dt>
          <dd>{{ t('dynamicGuide.' + question + '.body') }}</dd>
        </div>
      </dl>
    </section>
  </ElCard>
</template>

<style scoped>
.guide { --el-card-padding: var(--space-5); border-radius: var(--radius-card); }
h2 { font-size: var(--font-size-xl); margin-block-end: var(--space-3); }
h3 { font-size: var(--font-size-lg); }
ol { padding-inline-start: var(--space-5); margin-block: var(--space-5); }
li + li, dl > div + div { margin-block-start: var(--space-4); }
p, dd { margin: var(--space-1) 0 0; color: var(--color-text-secondary); }
dt { font-weight: var(--font-weight-semibold); }
code { font-family: var(--font-mono); font-size: var(--font-size-xs); overflow-wrap: anywhere; }
p > code { padding-inline: var(--space-1); background: var(--color-surface-muted); border-radius: var(--radius-control); }
.guide-example { margin-block-start: var(--space-2); background: var(--color-surface-muted); border: var(--border-width) solid var(--color-border); border-radius: var(--radius-card); }
.guide-example > div { padding: var(--space-1) var(--space-3); border-block-end: var(--border-width) solid var(--color-border); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
pre { white-space: pre-wrap; margin: 0; padding: var(--space-2) var(--space-3); }
.guide-faq { border-block-start: var(--border-width) solid var(--color-border); padding-block-start: var(--space-4); margin-block-start: var(--space-5); }
</style>
