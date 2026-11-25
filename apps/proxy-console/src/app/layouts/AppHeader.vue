<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElButton, ElDropdown, ElDropdownItem, ElDropdownMenu } from 'element-plus'
import { ArrowDown, Check, DataLine, Menu } from '@element-plus/icons-vue'
import BrandMark from '@/shared/ui/BrandMark.vue'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { AccountMenu, hasSession, OperatorMenu, sessionPhone } from '@/features/auth'
import { NotificationBell } from '@/features/notifications'
import { applyLocale, isLocale, locales, rememberLocale, type Locale } from '../i18n'

// 三个入口共用的顶栏。右侧顺序固定为「通知铃铛 → 语言切换 → 用户入口」。
defineProps<{ menuButton?: boolean, ops?: boolean, tenant?: boolean, marketing?: boolean }>()
defineEmits<{ openMenu: [] }>()

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const homePath = computed(() => `/${locale.value}/`)

// 官网切换到对应语言的同一路由并保留查询参数；控制台保持当前 URL，只更新偏好。
function switchLocale(next: Locale) {
  rememberLocale(next)
  if (isLocale(route.params.locale)) {
    router.replace(route.fullPath.replace(/^\/[^/?#]+/, `/${next}`))
  } else {
    applyLocale(next)
  }
}
</script>

<template>
  <header class="app-header" :class="{ 'app-header--ops': ops, 'app-header--tenant': tenant, 'app-header--marketing': marketing }">
    <div class="app-header__start">
      <ElButton v-if="menuButton" class="app-header__menu" text :icon="Menu" :aria-label="t('nav.openMenu')" @click="$emit('openMenu')" />
      <RouterLink :to="ops ? '/ops' : homePath" class="app-header__brand" :aria-label="t('app.name')">
        <BrandMark />
      </RouterLink>
      <slot name="brand-extra" />
    </div>

    <nav v-if="marketing" class="app-header__marketing-menu" :aria-label="t('marketing.resources')">
      <a href="#products">{{ t('marketing.products') }}</a>
      <a href="#integration">{{ t('marketing.guide') }}</a>
      <a href="#manage">{{ t('marketing.manage') }}</a>
      <a href="#faq">{{ t('marketing.faq') }}</a>
    </nav>
    <div class="app-header__tools">
      <RouterLink v-if="ops" to="/console" target="_blank" rel="noopener" class="app-header__customer-link">
        <DataLine aria-hidden="true" /><span>{{ t('header.customerConsole') }}</span>
      </RouterLink>
      <NotificationBell v-else :key="sessionPhone" :account-key="sessionPhone" />

      <ElDropdown trigger="click" placement="bottom-end" @command="switchLocale">
        <ElButton class="app-header__language" text :aria-label="t('header.language', { name: t(`locale.${locale}`) })">
          <svg class="app-header__globe" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z" /></svg><span class="app-header__locale-name">{{ t(`locale.${locale}`) }}</span><ArrowDown class="app-header__chevron" />
        </ElButton>
        <template #dropdown>
          <ElDropdownMenu>
            <ElDropdownItem v-for="item in locales" :key="item" :command="item" :lang="item">
              <span class="app-header__check"><Check v-if="item === locale" /></span>{{ t(`locale.${item}`) }}
            </ElDropdownItem>
          </ElDropdownMenu>
        </template>
      </ElDropdown>

      <OperatorMenu v-if="ops" />
      <AccountMenu v-else-if="hasSession()" :tenant="tenant" />
      <template v-else>
        <LinkButton :to="`/${locale}/login`" text>{{ t('header.login') }}</LinkButton>
        <LinkButton :to="`/${locale}/register`" type="primary" :class="{ 'home-header-register': marketing }">{{ t('header.register') }}</LinkButton>
      </template>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  position: sticky;
  top: 0;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  height: var(--header-height);
  padding-inline: var(--page-padding);
  border-bottom: var(--border-width) solid var(--color-border);
  background: var(--color-surface);
}
.app-header__start, .app-header__tools { display: flex; align-items: center; gap: var(--space-2); min-width: 0; }
.app-header__brand {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--font-size-brand);
  font-weight: var(--font-weight-bold);
  text-decoration: none;
  white-space: nowrap;
}
.app-header__chevron { margin-left: var(--space-1); width: 1em; }
.app-header__globe { width: var(--icon-size); height: var(--icon-size); margin-right: var(--space-1); }
.app-header__check { display: inline-flex; width: var(--space-4); margin-right: var(--space-2); color: var(--color-primary); }
.app-header__menu { display: none; }
.app-header--ops .app-header__brand { gap: var(--ops-brand-gap); font-size: var(--ops-brand-size); font-weight: var(--font-weight-heavy); }
.app-header--ops .app-header__start { gap: var(--ops-brand-gap); }
.app-header--ops .app-header__language {
  height: var(--ops-top-control-height);
  padding-inline: var(--ops-top-control-inline-padding);
  color: var(--color-text-secondary);
  border: var(--border-width) dashed var(--color-border);
  border-radius: var(--ops-control-radius);
  font-weight: var(--font-weight-medium);
}
.app-header--ops .app-header__globe { display: none; }
.app-header--ops .app-header__chevron { width: var(--font-size-xs); margin-inline-start: var(--space-2); }
.app-header__customer-link {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  height: var(--ops-top-control-height);
  padding-inline: var(--ops-top-control-inline-padding);
  font-weight: var(--font-weight-semibold);
  text-decoration: none;
}
.app-header__customer-link svg { width: var(--icon-size); color: var(--color-text-secondary); }
.app-header__customer-link:hover { background: var(--color-surface-muted); border-radius: var(--ops-control-radius); }

.app-header--tenant {
  --tenant-header-item-height: 36px;
  --tenant-header-item-padding: 10px;
  --tenant-header-item-gap: var(--space-2);
}
.app-header--tenant .app-header__brand {
  gap: var(--workspace-brand-gap);
  font-size: var(--workspace-brand-size);
  font-weight: var(--font-weight-heavy);
  line-height: normal;
}
.app-header--tenant .app-header__language {
  height: var(--tenant-header-item-height);
  padding-inline: var(--tenant-header-item-padding);
  border-radius: var(--space-2);
  font-weight: var(--font-weight-semibold);
}
.app-header--tenant .app-header__language[aria-expanded="true"] { background: var(--color-surface-muted); }
.app-header--tenant .app-header__chevron,
.app-header--tenant .app-header__globe { color: var(--color-text-secondary); }
.app-header--tenant .app-header__chevron { width: var(--font-size-xs); margin-left: var(--tenant-header-item-gap); }
.app-header--tenant .app-header__globe { margin-right: var(--tenant-header-item-gap); }

@media (max-width: 767px) {
  .app-header { padding-inline: var(--space-2); }
  .app-header__menu { display: inline-flex; }
  .app-header__start, .app-header--ops .app-header__start { gap: var(--space-1); }
  .app-header--ops .app-header__globe { display: inline-block; margin-inline-end: 0; }
  .app-header--ops .app-header__language { border-color: transparent; }
  .app-header__tools { gap: 0; }
  .app-header__brand { --brand-wordmark-display: none; }
  .app-header__customer-link, .app-header__locale-name, .app-header__chevron { display: none; }

  .app-header--tenant {
    --tenant-header-item-padding: 7px;
    --tenant-header-item-gap: var(--space-1);
    gap: var(--space-1);
    padding-inline: var(--space-3);
  }
  .app-header--tenant .app-header__brand { gap: var(--space-2); font-size: var(--font-size-xl); }
  .app-header--tenant .app-header__menu { order: 1; width: var(--space-5); padding-inline: 0; }
  .app-header--tenant .app-header__tools { gap: var(--space-1); }
}

@media (390px <= width <= 767px) {
  .app-header--tenant .app-header__brand { --brand-wordmark-display: inline-block; }
  .app-header--tenant .app-header__locale-name,
  .app-header--tenant .app-header__chevron { display: inline-block; }
}
</style>
