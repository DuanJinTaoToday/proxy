<script setup lang="ts">
import { ref, useId } from 'vue'
import { ChevronRight, Layers3, PanelLeft } from 'lucide-vue-next'
import { Button } from './ui/button'
import './dashboard.css'

defineProps<{ brand: string; area: string; title: string; accountName?: string }>()
const emit = defineEmits<{ logout: [] }>()
const sidebarOpen = ref(true)
const mobileSidebarOpen = ref(false)
const mainId = useId()
const sidebarId = useId()

function closeSidebar() {
  mobileSidebarOpen.value = false
}

function toggleSidebar() {
  if (window.matchMedia('(max-width: 760px)').matches) mobileSidebarOpen.value = !mobileSidebarOpen.value
  else sidebarOpen.value = !sidebarOpen.value
}
</script>

<template>
  <div class="console-dashboard" :class="{ 'sidebar-collapsed': !sidebarOpen, 'mobile-sidebar-open': mobileSidebarOpen }">
    <a class="skip-link" :href="'#' + mainId">跳到主要内容</a>
    <aside :id="sidebarId" class="dashboard-sidebar">
      <div class="dashboard-brand">
        <span class="brand-icon"><Layers3 :size="17" /></span><strong>{{ brand }}</strong>
      </div>
      <slot name="navigation" :close-sidebar="closeSidebar" />
      <div v-if="accountName" class="dashboard-account">
        <span class="tenant-avatar" aria-hidden="true">{{ accountName.slice(0, 1) }}</span>
        <div><strong :title="accountName">{{ accountName }}</strong><small>{{ area }}</small></div>
      </div>
    </aside>

    <div class="dashboard-inset">
      <header class="dashboard-header">
        <Button variant="ghost" size="icon-sm" class="sidebar-toggle" type="button" aria-label="切换侧栏" :aria-controls="sidebarId" @click="toggleSidebar">
          <PanelLeft :size="17" />
        </Button>
        <span class="header-divider"></span>
        <span class="header-parent">{{ area }}</span><ChevronRight class="header-parent" :size="13" />
        <h1>{{ title }}</h1>
        <Button variant="outline" class="dashboard-logout" type="button" @click="emit('logout')">退出登录</Button>
      </header>

      <main :id="mainId" class="dashboard-main" tabindex="-1"><slot /></main>
    </div>
  </div>
</template>
