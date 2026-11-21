<script setup lang="ts">
import { ElButton, type ButtonProps } from 'element-plus'
import type { RouteLocationRaw } from 'vue-router'

// 站内跳转按钮：渲染为真实链接，保留新标签页打开等浏览器行为，点击时走前端路由。
defineProps<{ to: RouteLocationRaw, type?: ButtonProps['type'], text?: boolean, size?: ButtonProps['size'] }>()
</script>

<template>
  <RouterLink :to="to" custom v-slot="{ href, navigate }">
    <ElButton tag="a" :href="href" :type="type" :text="text" :size="size" @click="navigate"><slot /></ElButton>
  </RouterLink>
</template>
