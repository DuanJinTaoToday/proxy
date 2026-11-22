<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElButton, ElDropdown, ElDropdownItem, ElDropdownMenu, ElMessageBox } from 'element-plus'
import { ArrowDown } from '@element-plus/icons-vue'
import { currentOperator, signOutOperator } from './operatorSession'

const { t } = useI18n()

async function confirmLogout() {
  const confirmed = await ElMessageBox.confirm(t('auth.opsLogoutHint'), t('auth.logoutTitle'), {
    type: 'warning',
    confirmButtonText: t('auth.logout'),
    cancelButtonText: t('auth.cancel'),
  }).then(() => true, () => false)
  if (confirmed) signOutOperator()
}
</script>

<template>
  <ElDropdown v-if="currentOperator" trigger="click" placement="bottom-end" @command="confirmLogout">
    <ElButton text class="operator-menu">
      <span class="operator-menu__name">{{ currentOperator.username }}</span><ArrowDown class="operator-menu__arrow" aria-hidden="true" />
    </ElButton>
    <template #dropdown>
      <div class="operator-menu__account">
        <strong>{{ currentOperator.username }}</strong>
        <span v-for="permission in currentOperator.permissions" :key="permission">{{ t(`auth.operatorPermission.${permission}`) }}</span>
      </div>
      <ElDropdownMenu><ElDropdownItem command="logout">{{ t('auth.logout') }}</ElDropdownItem></ElDropdownMenu>
    </template>
  </ElDropdown>
</template>

<style scoped>
.operator-menu {
  height: var(--ops-top-control-height);
  margin-inline-start: var(--space-1);
  padding-inline: var(--ops-top-control-inline-padding);
  border-radius: var(--ops-control-radius);
  font-weight: var(--font-weight-semibold);
}
.operator-menu__name { max-width: min(40vw, var(--account-name-width)); overflow: hidden; text-overflow: ellipsis; }
.operator-menu__arrow { width: var(--font-size-xs); margin-inline-start: var(--space-2); color: var(--color-text-secondary); }
.operator-menu__account {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  inline-size: min(var(--catalog-sale-text-width), 75vw);
  padding: var(--space-3) var(--space-4);
  border-block-end: var(--border-width) solid var(--color-border);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  overflow-wrap: anywhere;
}
.operator-menu__account strong { color: var(--color-text); font-size: var(--font-size-md); }
</style>
