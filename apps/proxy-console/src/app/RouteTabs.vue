<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElButton, ElTabPane, ElTabs } from 'element-plus'
import PageHeader from '@/shared/ui/PageHeader.vue'
import { tabsParent } from './router'

// 页签容器。每个页签是独立的子路由，支持直接访问、刷新和浏览器后退。
const route = useRoute()
const router = useRouter()
const { t } = useI18n()

const parent = computed(() => tabsParent(route))
const action = computed(() => parent.value?.meta.tabAction)
const active = computed({
  get: () => String(route.name ?? ''),
  set: name => { router.push({ name }) },
})
</script>

<template>
  <PageHeader v-if="parent?.meta.titleKey" :title="t(parent.meta.titleKey)">
    <template v-if="action" #actions>
      <ElButton class="route-action" type="primary" @click="router.push({ name: action.name })">{{ t(action.labelKey) }}</ElButton>
    </template>
  </PageHeader>
  <ElTabs v-model="active" class="route-tabs">
    <ElTabPane v-for="tab in parent?.meta.tabs" :key="tab.name" :name="tab.name" :label="t(tab.labelKey)">
      <template #label><slot name="label" :tab="tab">{{ t(tab.labelKey) }}</slot></template>
    </ElTabPane>
  </ElTabs>
  <slot><RouterView /></slot>
</template>

<style scoped>
.route-action { padding-inline: var(--space-3); font-size: var(--font-size-sm); }
.route-tabs { --el-tabs-header-height: var(--tabs-height); margin-bottom: var(--route-tabs-gap, var(--space-1)); }
</style>
