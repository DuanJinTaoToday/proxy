<script setup lang="ts">
import { onScopeDispose, shallowRef, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElMessage, ElSkeleton, ElTag } from 'element-plus'
import { CopyDocument } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { getTenantInvite, type TenantInvite } from '@/api/referrals'
import { accountLoadFailed, currentAccount } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

type ReferralState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'unverified' }
  | { kind: 'failed', error: unknown }
  | { kind: 'ready', data: TenantInvite }

const { t } = useI18n()
const errorMessage = useErrorMessage()
const headingId = useId()
const state = shallowRef<ReferralState>({ kind: 'loading' })
let activeRequest: AbortController | undefined

onScopeDispose(() => activeRequest?.abort())

async function load(targetTenantId = currentAccount.value?.tenantId) {
  activeRequest?.abort()
  activeRequest = undefined
  if (!targetTenantId) {
    state.value = accountLoadFailed.value ? { kind: 'idle' } : { kind: 'loading' }
    return
  }
  const controller = new AbortController()
  activeRequest = controller
  state.value = { kind: 'loading' }
  try {
    const data = await getTenantInvite(controller.signal)
    if (controller.signal.aborted || currentAccount.value?.tenantId !== targetTenantId) return
    state.value = { kind: 'ready', data }
  } catch (error) {
    if (controller.signal.aborted || currentAccount.value?.tenantId !== targetTenantId) return
    if (error instanceof ApiError && error.reason === 'EMAIL_NOT_VERIFIED') {
      state.value = { kind: 'unverified' }
      return
    }
    state.value = { kind: 'failed', error }
  }
}

watch(
  [() => currentAccount.value?.tenantId, accountLoadFailed],
  ([tenantId]) => {
    void load(tenantId)
  },
  { immediate: true },
)

async function copyText(text: string, kind: 'code' | 'url') {
  try {
    if (!navigator.clipboard?.writeText) throw new Error('clipboard unavailable')
    await navigator.clipboard.writeText(text)
    ElMessage.success(t(kind === 'code' ? 'users.referral.codeCopied' : 'users.referral.urlCopied'))
  } catch {
    ElMessage.warning(t('users.referral.copyFailed'))
  }
}
</script>

<template>
  <ElCard class="referral-card" role="region" shadow="never" :aria-labelledby="headingId">
    <div class="referral-card__body">
      <div class="referral-card__header">
        <h2 :id="headingId" class="referral-card__heading">{{ t('users.referral.title') }}</h2>
        <ElTag
          v-if="state.kind === 'ready'"
          :type="state.data.promotionStatus === 'active' ? 'success' : 'warning'"
          effect="light"
          size="small"
        >
          {{ t(state.data.promotionStatus === 'active' ? 'users.referral.statusActive' : 'users.referral.statusPaused') }}
        </ElTag>
      </div>

      <div v-if="state.kind === 'loading'" role="status" :aria-label="t('users.referral.loading')">
        <ElSkeleton animated :rows="3" />
      </div>

      <ElAlert
        v-else-if="state.kind === 'unverified'"
        type="warning"
        :title="t('users.referral.emailNotVerified')"
        :description="t('users.referral.emailNotVerifiedDetail')"
        :closable="false"
        show-icon
      />

      <div v-else-if="state.kind === 'failed'" class="referral-card__error">
        <ElAlert
          type="error"
          :title="t('users.referral.loadFailed')"
          :description="errorMessage(state.error)"
          :closable="false"
          show-icon
        />
        <ElButton size="small" @click="load()">{{ t('auth.retry') }}</ElButton>
      </div>

      <template v-else-if="state.kind === 'ready'">
        <ElAlert
          v-if="state.data.promotionStatus === 'paused'"
          type="warning"
          :title="t('users.referral.pausedNotice')"
          :closable="false"
          show-icon
        />

        <dl class="referral-card__fields">
          <div class="referral-card__row">
            <dt>{{ t('users.referral.status') }}</dt>
            <dd>{{ t(state.data.promotionStatus === 'active' ? 'users.referral.statusActive' : 'users.referral.statusPaused') }}</dd>
          </div>
          <div class="referral-card__row referral-card__row--copy">
            <div class="referral-card__value-group">
              <dt>{{ t('users.referral.inviteCode') }}</dt>
              <dd class="referral-card__selectable mono">{{ state.data.inviteCode }}</dd>
            </div>
            <ElButton class="referral-card__copy-btn" :icon="CopyDocument" @click="copyText(state.data.inviteCode, 'code')">
              {{ t('users.referral.copyCode') }}
            </ElButton>
          </div>
          <div class="referral-card__row referral-card__row--copy">
            <div class="referral-card__value-group">
              <dt>{{ t('users.referral.inviteUrl') }}</dt>
              <dd class="referral-card__selectable referral-card__url mono">{{ state.data.inviteUrl }}</dd>
            </div>
            <ElButton class="referral-card__copy-btn" :icon="CopyDocument" @click="copyText(state.data.inviteUrl, 'url')">
              {{ t('users.referral.copyUrl') }}
            </ElButton>
          </div>
        </dl>
      </template>
    </div>
  </ElCard>
</template>

<style scoped>
.referral-card {
  --el-card-padding: var(--space-5);
  border-radius: var(--dashboard-card-radius);
}

.referral-card__body {
  display: grid;
  gap: var(--space-4);
}

.referral-card__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}

.referral-card__heading {
  margin: 0;
  font-size: var(--font-size-xl);
  line-height: var(--tenant-profile-heading-height);
  font-weight: var(--font-weight-semibold);
}

.referral-card__error {
  display: grid;
  justify-items: start;
  gap: var(--space-3);
}

.referral-card__fields {
  display: grid;
  gap: var(--space-3);
  margin: 0;
}

.referral-card__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2) var(--space-4);
}

.referral-card__row + .referral-card__row {
  padding-block-start: var(--space-3);
  border-block-start: var(--border-width) solid var(--color-border);
}

.referral-card__value-group {
  display: grid;
  gap: var(--space-1);
  flex: 1 1 14rem;
  min-width: 0;
}

.referral-card__fields dt {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
}

.referral-card__fields dd {
  margin: 0;
  font-weight: var(--font-weight-semibold);
  overflow-wrap: anywhere;
}

.referral-card__selectable {
  user-select: all;
}

.referral-card__url {
  word-break: break-all;
}

.referral-card__copy-btn {
  margin-left: 0;
  flex-shrink: 0;
}
</style>
