<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElButton, ElDropdown, ElDropdownItem, ElDropdownMenu, ElMessageBox } from 'element-plus'
import { ArrowDown, User } from '@element-plus/icons-vue'
import { maskPhone } from './rules'
import { accountLoadFailed, currentAccount, loadAccount, sessionPhone, signOut } from './session'

// 已登录时顶栏的用户入口：显示主账号名和手机号，提供账户信息、订单记录和退出登录。
const router = useRouter()
const { t, locale } = useI18n()

const accountName = computed(() => currentAccount.value?.primaryAccountName
  ?? t(accountLoadFailed.value ? 'auth.accountFailed' : 'auth.loadingAccount'))

onMounted(loadAccount)

async function confirmSignOut() {
  const confirmed = await ElMessageBox.confirm(t('auth.logoutBody'), t('auth.logoutTitle'), {
    confirmButtonText: t('auth.logout'),
    cancelButtonText: t('auth.cancel'),
    type: 'warning',
  }).then(() => true, () => false)
  if (confirmed) signOut(`/${locale.value}/login`)
}
</script>

<template>
  <ElDropdown trigger="click" placement="bottom-end">
    <ElButton text :aria-label="t('auth.accountMenu')">
      <User class="account-menu__icon" />
      <span class="account-menu__name">{{ accountName }}</span>
      <ArrowDown class="account-menu__chevron" />
    </ElButton>
    <template #dropdown>
      <div class="account-menu__head">
        <strong>{{ accountName }}</strong>
        <span class="tabular">{{ maskPhone(sessionPhone) }}</span>
        <ElButton v-if="accountLoadFailed" link type="primary" size="small" @click="loadAccount">{{ t('auth.retry') }}</ElButton>
      </div>
      <ElDropdownMenu>
        <ElDropdownItem @click="router.push({ name: 'console-account-profile' })">{{ t('auth.profile') }}</ElDropdownItem>
        <ElDropdownItem @click="router.push({ name: 'console-account-orders' })">{{ t('auth.orders') }}</ElDropdownItem>
        <ElDropdownItem divided @click="confirmSignOut">{{ t('auth.logout') }}</ElDropdownItem>
      </ElDropdownMenu>
    </template>
  </ElDropdown>
</template>

<style scoped>
.account-menu__icon { width: var(--icon-size); height: var(--icon-size); margin-right: var(--space-1); }
.account-menu__name { max-width: var(--account-name-width); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.account-menu__chevron { width: 1em; margin-left: var(--space-1); }
.account-menu__head {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-1);
  max-width: var(--auth-card-width);
  padding: var(--space-3) var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
  overflow-wrap: anywhere;
}
.account-menu__head span { color: var(--color-text-secondary); font-size: var(--font-size-xs); }

@media (max-width: 767px) {
  .account-menu__name, .account-menu__chevron { display: none; }
}
</style>
