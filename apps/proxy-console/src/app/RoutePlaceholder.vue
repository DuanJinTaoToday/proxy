<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/shared/ui/PageHeader.vue'
import PageEmpty from '@/shared/ui/PageEmpty.vue'
import { tabsParent } from './router'

// 尚未交付的页面占位。作为页签子页时，标题由外层页签容器显示。
const route = useRoute()
const { t } = useI18n()
</script>

<template>
  <PageHeader v-if="!tabsParent(route) && route.meta.titleKey" :title="t(route.meta.titleKey)" />
  <PageEmpty :description="t('page.pending')" :hint="t('page.pendingHint')" />
</template>
