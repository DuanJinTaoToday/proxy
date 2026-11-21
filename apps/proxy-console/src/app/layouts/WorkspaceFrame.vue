<script setup lang="ts">
import { ref } from 'vue'
import { ElDrawer } from 'element-plus'
import AppHeader from './AppHeader.vue'
import AppSidebar from './AppSidebar.vue'
import type { NavSection } from '../navigation'

// 用户控制台和运营控制台共用的工作区框架：顶栏、侧栏和内容区。窄屏时侧栏收进抽屉。
defineProps<{ sections: NavSection[], navLabel: string, ops?: boolean }>()
const drawerOpen = ref(false)
</script>

<template>
  <AppHeader menu-button :ops="ops" @open-menu="drawerOpen = true">
    <template #brand-extra><slot name="brand-extra" /></template>
  </AppHeader>
  <div class="workspace">
    <AppSidebar class="workspace__sidebar" :sections="sections" :label="navLabel" />
    <main class="workspace__main">
      <RouterView />
    </main>
  </div>
  <ElDrawer v-model="drawerOpen" direction="ltr" size="min(var(--sidebar-width), 80vw)" :with-header="false" style="--el-drawer-padding-primary: 0">
    <AppSidebar :sections="sections" :label="navLabel" @navigate="drawerOpen = false" />
  </ElDrawer>
</template>

<style scoped>
.workspace {
  display: grid;
  grid-template-columns: var(--sidebar-width) minmax(0, 1fr);
  min-height: calc(100dvh - var(--header-height));
}
.workspace__main {
  min-width: 0;
  width: 100%;
  max-width: var(--content-max-width);
  padding: var(--page-padding);
}

@media (max-width: 767px) {
  .workspace { grid-template-columns: minmax(0, 1fr); }
  .workspace__sidebar { display: none; }
  .workspace__main { padding: var(--space-4) var(--space-3); }
}
</style>
