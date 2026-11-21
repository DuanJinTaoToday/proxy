<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LinkButton from '@/shared/ui/LinkButton.vue'

// 官网首页骨架。商品区、报价和教程由官网切片交付。
const { t } = useI18n()
</script>

<template>
  <section class="home-hero">
    <h1 class="home-hero__title">{{ t('home.title') }}</h1>
    <p class="home-hero__subtitle">{{ t('home.subtitle') }}</p>
    <LinkButton to="/console" type="primary" size="large">{{ t('home.console') }}</LinkButton>
  </section>
</template>

<style scoped>
.home-hero { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-4); max-width: var(--content-narrow-width); }
.home-hero__title { font-size: var(--font-size-display); font-weight: var(--font-weight-bold); line-height: var(--line-height-tight); }
.home-hero__subtitle { color: var(--color-text-secondary); font-size: var(--font-size-lg); }
</style>
