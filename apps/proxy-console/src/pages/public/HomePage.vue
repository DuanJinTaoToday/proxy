<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElButton } from 'element-plus'

// 官网首页骨架。商品区、报价和教程由官网切片交付。
const { t } = useI18n()
</script>

<template>
  <section class="home-hero">
    <h1 class="home-hero__title">{{ t('home.title') }}</h1>
    <p class="home-hero__subtitle">{{ t('home.subtitle') }}</p>
    <RouterLink to="/console" custom v-slot="{ navigate, href }">
      <ElButton type="primary" size="large" tag="a" :href="href" @click="navigate">{{ t('home.console') }}</ElButton>
    </RouterLink>
  </section>
</template>

<style scoped>
.home-hero { display: flex; flex-direction: column; align-items: flex-start; gap: var(--space-4); max-width: 720px; }
.home-hero__title { font-size: calc(var(--font-size-title) * 1.5); font-weight: var(--font-weight-bold); line-height: var(--line-height-tight); }
.home-hero__subtitle { color: var(--color-text-secondary); font-size: var(--font-size-lg); }
</style>
