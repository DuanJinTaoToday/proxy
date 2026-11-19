<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { useMediaQuery } from '@vueuse/core'
import { ChevronDown, ChevronRight, Layers3, LogOut, PanelLeft } from 'lucide-vue-next'
import { DropdownMenuContent, DropdownMenuItem, DropdownMenuRoot, DropdownMenuSeparator, DropdownMenuTrigger } from 'reka-ui'
import { Button } from './ui/button'
import './dashboard.css'

defineProps<{ brand: string; brandLogo?: string; area: string; title: string; accountName?: string }>()
const emit = defineEmits<{ logout: [] }>()
const sidebarOpen = ref(true)
const mobileSidebarOpen = ref(false)
const compact = useMediaQuery('(max-width: 760px)')
const expanded = computed(() => compact.value ? mobileSidebarOpen.value : sidebarOpen.value)
const mainId = useId()
const sidebarId = useId()

function closeSidebar() {
  mobileSidebarOpen.value = false
}

function revealSidebar() {
  if (compact.value) mobileSidebarOpen.value = true
  else sidebarOpen.value = true
}

function toggleSidebar() {
  if (compact.value) mobileSidebarOpen.value = !mobileSidebarOpen.value
  else sidebarOpen.value = !sidebarOpen.value
}
</script>

<template>
  <div class="console-dashboard" :class="{ 'sidebar-collapsed': !sidebarOpen, 'mobile-sidebar-open': mobileSidebarOpen }">
    <a class="skip-link" :href="'#' + mainId">跳到主要内容</a>
    <aside :id="sidebarId" class="dashboard-sidebar">
      <div class="dashboard-brand">
        <RouterLink to="/" :aria-label="brand + '首页'">
          <img v-if="brandLogo" :src="brandLogo" alt="" width="23" height="23" class="shrink-0 object-contain" />
          <Layers3 v-else :size="23" aria-hidden="true" />
          <strong>{{ brand }}</strong>
        </RouterLink>
      </div>
      <slot name="navigation" :close-sidebar="closeSidebar" :reveal-sidebar="revealSidebar" :expanded="expanded" />
    </aside>

    <div class="dashboard-inset">
      <header class="dashboard-header">
        <Button variant="ghost" size="icon-sm" class="sidebar-toggle" type="button" aria-label="切换侧栏" :aria-controls="sidebarId" :aria-expanded="expanded" @click="toggleSidebar">
          <PanelLeft :size="17" />
        </Button>
        <span class="header-divider"></span>
        <span class="header-parent">{{ area }}</span><ChevronRight class="header-parent" :size="13" />
        <span class="header-current">{{ title }}</span>
        <DropdownMenuRoot :modal="false">
          <DropdownMenuTrigger as-child>
            <Button variant="ghost" class="account-trigger" type="button">
              <span>{{ accountName || '我的账号' }}</span><ChevronDown :size="16" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent class="account-menu" align="end" :side-offset="8" :collision-padding="12">
            <slot name="account-links" :item="DropdownMenuItem" />
            <DropdownMenuSeparator v-if="$slots['account-links']" class="account-menu-divider" />
            <DropdownMenuItem class="account-menu-item" @select="emit('logout')"><LogOut :size="17" />退出登录</DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenuRoot>
      </header>

      <main :id="mainId" class="dashboard-main" tabindex="-1">
        <h1 class="dashboard-title">{{ title }}</h1>
        <slot />
      </main>
    </div>
  </div>
</template>
