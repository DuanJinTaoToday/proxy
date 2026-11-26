<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElConfigProvider } from 'element-plus'
import { formatDocumentTitle } from './documentTitle'
import { elementLocaleOf } from './i18n'

const route = useRoute()
const { t, locale } = useI18n()
const elLocale = computed(() => elementLocaleOf(locale.value))

// 标题随路由、入口和语言变化，控制台切换语言不改 URL 也能更新。
watchEffect(() => {
  document.title = formatDocumentTitle(route, t)
})
</script>

<template>
  <ElConfigProvider :locale="elLocale">
    <RouterView />
  </ElConfigProvider>
</template>
