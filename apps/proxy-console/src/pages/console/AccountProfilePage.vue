<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElSkeleton } from 'element-plus'
import { accountEmail, accountLoadFailed, currentAccount, loadAccount } from '@/features/auth'
import { AccessLocationCard, CommissionRecords, ReferralShareCard } from '@/features/users'

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
          <dt>{{ t('users.email') }}</dt>
          <dd>{{ accountEmail }}</dd>
        </div>
      </dl>
      <div v-else-if="accountLoadFailed" class="account-error">
        <ElAlert type="error" :title="t('auth.accountFailed')" :closable="false" show-icon />
        <ElButton @click="loadAccount">{{ t('auth.retry') }}</ElButton>
      </div>
      <ElSkeleton v-else :rows="2" :aria-label="t('auth.loadingAccount')" animated />
    </ElCard>
    <AccessLocationCard />
    <ReferralShareCard class="account-profile__referral" />
    <CommissionRecords mode="tenant" class="account-profile__commissions" />
  </div>
</template>

<style scoped>
.account-profile {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  align-items: stretch;
  gap: var(--space-4);
  line-height: normal;
}
.account-profile__referral,
.account-profile__commissions { grid-column: 1 / -1; min-width: 0; }
.profile-card { --el-card-padding: var(--space-5); border-radius: var(--dashboard-card-radius); }
.profile-card h2 { margin-block-end: var(--space-4); font-size: var(--font-size-xl); line-height: var(--tenant-profile-heading-height); }
dl { display: grid; gap: var(--space-3); margin: 0; }
dl > div { display: flex; justify-content: space-between; gap: var(--space-4); }
dl > div + div { padding-block-start: var(--space-3); border-block-start: var(--border-width) solid var(--color-border); }
dt { color: var(--color-text-secondary); }
dd { margin: 0; font-weight: var(--font-weight-semibold); overflow-wrap: anywhere; text-align: end; }
.account-error { display: grid; justify-items: start; gap: var(--space-3); }
@media (max-width: 950px) { .account-profile { grid-template-columns: minmax(0, 1fr); } }
</style>
