<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import PageHeader from '@/shared/ui/PageHeader.vue'
import PageEmpty from '@/shared/ui/PageEmpty.vue'

// 尚未实现的页面。作为页签子页时，标题由外层页面显示。
const route = useRoute()
const { t } = useI18n()
const inTabs = computed(() => Boolean(route.matched.at(-2)?.meta.tabs))
</script>

<template>
  <PageHeader v-if="!inTabs && route.meta.titleKey" :title="t(route.meta.titleKey)" />
  <PageEmpty :description="t('page.pending')" :hint="t('page.pendingHint')" />
</template>
