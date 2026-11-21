<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElTabPane, ElTabs } from 'element-plus'
import PageHeader from '@/shared/ui/PageHeader.vue'

// 带页签的页面。每个页签是独立的子路由，支持直接访问、刷新和浏览器后退。
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const parent = computed(() => route.matched.find(record => record.meta.tabs))
const tabs = computed(() => parent.value?.meta.tabs ?? [])
const active = computed({
  get: () => String(route.name ?? ''),
  set: name => { router.push({ name, query: {} }) },
})
</script>

<template>
  <PageHeader v-if="parent?.meta.titleKey" :title="t(parent.meta.titleKey)" />
  <ElTabs v-model="active" class="tabbed-page__tabs">
    <ElTabPane v-for="tab in tabs" :key="tab.name" :name="tab.name" :label="t(tab.labelKey)" />
  </ElTabs>
  <RouterView />
</template>

<style scoped>
.tabbed-page__tabs { --el-tabs-header-height: 42px; margin-bottom: var(--space-1); }
</style>
