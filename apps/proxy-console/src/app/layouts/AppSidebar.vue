<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMenu, ElMenuItem, ElMenuItemGroup } from 'element-plus'
import { activeNavPath, type NavSection } from '../navigation'

const props = defineProps<{ sections: NavSection[], label: string, tenant?: boolean }>()
defineEmits<{ navigate: [] }>()

const route = useRoute()
const { t } = useI18n()
const active = computed(() => activeNavPath(props.sections, route.path))
</script>

<template>
  <nav class="app-sidebar" :class="{ 'app-sidebar--tenant': tenant }" :aria-label="label">
    <ElMenu router :default-active="active" class="app-sidebar__menu" @select="$emit('navigate')">
      <template v-for="(section, index) in sections" :key="index">
        <ElMenuItemGroup v-if="section.labelKey" class="app-sidebar__group">
          <template #title><span class="app-sidebar__label">{{ t(section.labelKey) }}</span></template>
          <ElMenuItem v-for="item in section.items" :key="item.path" :index="item.path" class="app-sidebar__item" :class="{ 'app-sidebar__current': item.path === active }">
            <component :is="item.icon" class="app-sidebar__icon" /><span>{{ t(item.labelKey) }}</span>
          </ElMenuItem>
        </ElMenuItemGroup>
        <template v-else>
          <ElMenuItem v-for="item in section.items" :key="item.path" :index="item.path" class="app-sidebar__item" :class="{ 'app-sidebar__current': item.path === active }">
            <component :is="item.icon" class="app-sidebar__icon" /><span>{{ t(item.labelKey) }}</span>
          </ElMenuItem>
        </template>
      </template>
    </ElMenu>
  </nav>
</template>

<style scoped>
.app-sidebar { height: 100%; padding: var(--space-4) var(--space-3); background: var(--color-shell); }
.app-sidebar__menu {
  --el-menu-bg-color: var(--color-shell);
  --el-menu-text-color: var(--color-shell-text);
  --el-menu-hover-bg-color: var(--color-shell-hover);
  --el-menu-active-color: var(--color-on-primary);
  --el-menu-item-height: var(--nav-item-height);
  --el-menu-base-level-padding: var(--space-3);
  --el-text-color-secondary: var(--color-shell-muted);
  border-right: 0;
}
.app-sidebar__item { height: auto; min-height: var(--nav-item-height); padding-block: var(--space-2); line-height: var(--line-height-tight); white-space: normal; }
.app-sidebar__current { border-radius: var(--radius-control); background: var(--color-shell-active); }
.app-sidebar__icon { width: var(--icon-size); height: var(--icon-size); margin-right: var(--space-2); flex: none; }

.app-sidebar--tenant { padding: 22px var(--space-4); }
.app-sidebar--tenant .app-sidebar__group { margin-top: 18px; }
.app-sidebar--tenant .app-sidebar__label { position: relative; inset-block-start: -1px; }
.app-sidebar--tenant .app-sidebar__item {
  column-gap: 10px;
  border-radius: 7px;
  font-weight: var(--font-weight-medium);
}
.app-sidebar--tenant .app-sidebar__item + .app-sidebar__item { margin-top: var(--space-1); }
.app-sidebar--tenant .app-sidebar__icon { width: 17px; height: 17px; margin-right: 0; }

@media (max-width: 950px) {
  .app-sidebar--tenant { padding-inline: var(--space-3); }
}
</style>
