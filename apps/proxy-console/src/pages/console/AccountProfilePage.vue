<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElSkeleton } from 'element-plus'
import { accountLoadFailed, currentAccount, loadAccount, maskPhone, sessionPhone } from '@/features/auth'
import { AccessLocationCard } from '@/features/users'

const { t } = useI18n()
</script>

<template>
  <div class="account-profile">
    <ElCard role="region" shadow="never" class="profile-card" aria-labelledby="profile-account">
      <h2 id="profile-account">{{ t('users.account') }}</h2>
      <dl v-if="currentAccount">
        <div>
          <dt>{{ t('users.primaryAccount') }}</dt>
          <dd>{{ currentAccount.primaryAccountName }}</dd>
        </div>
        <div>
          <dt>{{ t('users.phone') }}</dt>
          <dd class="tabular">{{ maskPhone(sessionPhone) }}</dd>
        </div>
      </dl>
      <div v-else-if="accountLoadFailed" class="account-error">
        <ElAlert type="error" :title="t('auth.accountFailed')" :closable="false" show-icon />
        <ElButton @click="loadAccount">{{ t('auth.retry') }}</ElButton>
      </div>
      <ElSkeleton v-else :rows="2" :aria-label="t('auth.loadingAccount')" animated />
    </ElCard>
    <AccessLocationCard />
  </div>
</template>

<style scoped>
.account-profile {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--account-profile-card-min-width)), 1fr));
  align-items: start;
  gap: var(--space-4);
}
.profile-card { --el-card-padding: var(--space-5); }
h2 { margin: 0 0 var(--space-4); font-size: var(--font-size-xl); }
dl { display: grid; gap: var(--space-3); margin: 0; }
dl > div { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 2fr); gap: var(--space-4); }
dt { color: var(--color-text-secondary); }
dd { margin: 0; overflow-wrap: anywhere; }
.account-error { display: grid; justify-items: start; gap: var(--space-3); }
</style>
