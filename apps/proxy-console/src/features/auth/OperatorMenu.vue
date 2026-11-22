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
      {{ currentOperator.username }}<ArrowDown class="operator-menu__arrow" />
    </ElButton>
    <template #dropdown>
      <ElDropdownMenu><ElDropdownItem command="logout">{{ t('auth.logout') }}</ElDropdownItem></ElDropdownMenu>
    </template>
  </ElDropdown>
</template>

<style scoped>
.operator-menu { font-weight: var(--font-weight-semibold); }
.operator-menu__arrow { width: 1em; margin-inline-start: var(--space-2); color: var(--color-text-secondary); }
</style>
