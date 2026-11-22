<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElButton, ElMessage } from 'element-plus'
import { Check, CopyDocument } from '@element-plus/icons-vue'
import { hasSession } from '@/features/auth'
import LinkButton from '@/shared/ui/LinkButton.vue'
import HomeMap from './HomeMap.vue'
import { exits, heroPhrases, quickCommand } from './demo'

const { t, locale } = useI18n()
const index = ref(0)
const copied = ref(false)
let rotation: number | undefined
let resetCopy: number | undefined

onMounted(() => {
  if (!matchMedia('(prefers-reduced-motion: reduce)').matches) {
    rotation = window.setInterval(() => { index.value = (index.value + 1) % heroPhrases.length }, 2800)
  }
})
onUnmounted(() => {
  clearInterval(rotation)
  clearTimeout(resetCopy)
})

async function copyCommand() {
  try {
    await navigator.clipboard.writeText(quickCommand)
    copied.value = true
    clearTimeout(resetCopy)
    resetCopy = window.setTimeout(() => { copied.value = false }, 2200)
  } catch {
    ElMessage.error(t('marketing.copyFailed'))
  }
}
</script>

<template>
  <section class="home-hero">
    <HomeMap />
    <div class="hero-ambient" aria-hidden="true" />
    <div class="hero-shade" aria-hidden="true" />
    <div class="d-inner">
      <div class="hero-copy">
        <h1>
          <span class="line">{{ t('marketing.title') }}</span>
          <span class="line rotator">
            <Transition name="swap"><em :key="index">{{ t(`marketing.${heroPhrases[index]}`) }}</em></Transition>
          </span>
        </h1>
        <p class="lead">{{ t('marketing.heroLead') }}<br>{{ t('marketing.heroLeadMore') }}</p>
      </div>
      <div class="hero-foot">
        <div class="d-ctas">
          <LinkButton v-if="hasSession()" class="pill primary" to="/console">{{ t('home.console') }}</LinkButton>
          <LinkButton v-else class="pill primary" :to="{ name: 'register', params: { locale } }">{{ t('marketing.freeRegister') }}</LinkButton>
          <a class="pill ghost" href="#products">{{ t('marketing.learn') }} ↓</a>
        </div>
        <ElButton class="agent-pill" :class="{ copied }" @click="copyCommand">
          <span class="agent-dots" aria-hidden="true"><i /><i /><i /></span>
          <span aria-live="polite">{{ t(copied ? 'marketing.copiedCommand' : 'marketing.copyCommand') }}</span>
          <component :is="copied ? Check : CopyDocument" class="agent-icon" aria-hidden="true" />
        </ElButton>
      </div>
    </div>
    <div class="exit-strip">
      <span class="exit-label">{{ t('marketing.exampleExits') }}</span>
      <div class="exit-rail">
        <div class="exit-track">
          <span v-for="(exit, i) in [...exits, ...exits]" :key="i" :aria-hidden="i >= exits.length"><i />{{ t(`marketing.cities.${exit.city}`) }}</span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped src="./hero.css"></style>
