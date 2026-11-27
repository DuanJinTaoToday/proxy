<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElMessage, ElSkeleton, ElTag, ElTooltip } from 'element-plus'
import {
  getOpsTenantPromotion,
  pauseOpsTenantPromotion,
  resumeOpsTenantPromotion,
  type PromotionStatus,
} from '@/api/referrals'
import { hasOperatorPermission } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import OpsDialog from '@/shared/ui/OpsDialog.vue'

type PromotionState =
  | { kind: 'loading' }
  | { kind: 'error', error: unknown }
  | { kind: 'ready', status: PromotionStatus }

const props = defineProps<{ tenantId: string }>()

const { t } = useI18n()
const errorMessage = useErrorMessage()
const headingId = useId()

const canRead = computed(() => hasOperatorPermission('read_operations'))
const canManage = computed(() => hasOperatorPermission('manage_tenant_accounts'))

const state = shallowRef<PromotionState>({ kind: 'loading' })
const confirmOpen = ref(false)
const pendingAction = ref<'pause' | 'resume' | null>(null)
const submitting = ref(false)
const actionError = shallowRef<unknown>()

let activeRequest: AbortController | undefined
onScopeDispose(() => activeRequest?.abort())

async function load(targetTenantId = props.tenantId) {
  if (!canRead.value) return
  activeRequest?.abort()
  const controller = new AbortController()
  activeRequest = controller
  state.value = { kind: 'loading' }
  try {
    const result = await getOpsTenantPromotion(targetTenantId, controller.signal)
    if (controller.signal.aborted || props.tenantId !== targetTenantId) return
    state.value = { kind: 'ready', status: result.promotionStatus }
  } catch (error) {
    if (controller.signal.aborted || props.tenantId !== targetTenantId) return
    state.value = { kind: 'error', error }
  }
}

watch(
  () => props.tenantId,
  (tenantId) => {
    confirmOpen.value = false
    pendingAction.value = null
    actionError.value = undefined
    submitting.value = false
    void load(tenantId)
  },
  { immediate: true },
)

function openConfirm(action: 'pause' | 'resume') {
  if (!canManage.value || submitting.value) return
  actionError.value = undefined
  pendingAction.value = action
  confirmOpen.value = true
}

async function confirmAction() {
  const action = pendingAction.value
  if (!canManage.value || submitting.value || !action) return
  const targetTenantId = props.tenantId
  submitting.value = true
  actionError.value = undefined
  try {
    const updated = action === 'pause'
      ? await pauseOpsTenantPromotion(targetTenantId)
      : await resumeOpsTenantPromotion(targetTenantId)
    if (props.tenantId !== targetTenantId) return
    state.value = { kind: 'ready', status: updated.promotionStatus }
    confirmOpen.value = false
    pendingAction.value = null
    ElMessage.success(t(action === 'pause' ? 'users.promotion.pausedSuccess' : 'users.promotion.resumedSuccess'))

    activeRequest?.abort()
    const controller = new AbortController()
    activeRequest = controller
    try {
      const latest = await getOpsTenantPromotion(targetTenantId, controller.signal)
      if (!controller.signal.aborted && props.tenantId === targetTenantId) {
        state.value = { kind: 'ready', status: latest.promotionStatus }
      }
    } catch {
      // Retain the confirmed status returned by the mutation if the follow-up refresh fails.
    }
  } catch (error) {
    if (props.tenantId !== targetTenantId) return
    actionError.value = error
  } finally {
    if (props.tenantId === targetTenantId) submitting.value = false
  }
}
</script>

<template>
  <ElCard class="ops-promotion-card" role="region" shadow="never" :aria-labelledby="headingId">
    <div class="ops-promotion">
      <div class="ops-promotion__info">
        <div class="ops-promotion__title-row">
          <h2 :id="headingId" class="ops-promotion__heading">{{ t('users.promotion.title') }}</h2>
          <ElTag
            v-if="state.kind === 'ready'"
            :type="state.status === 'active' ? 'success' : 'warning'"
            effect="light"
            size="small"
          >
            {{ t(state.status === 'active' ? 'users.promotion.statusActive' : 'users.promotion.statusPaused') }}
          </ElTag>
        </div>

        <div v-if="state.kind === 'loading'" role="status" :aria-label="t('users.promotion.loading')">
          <ElSkeleton animated :rows="1" />
        </div>

        <div v-else-if="state.kind === 'error'" class="ops-promotion__error">
          <ElAlert
            type="error"
            :title="t('users.promotion.loadFailed')"
            :description="errorMessage(state.error)"
            :closable="false"
            show-icon
          />
          <ElButton size="small" @click="load()">{{ t('auth.retry') }}</ElButton>
        </div>

        <p v-else class="ops-promotion__hint">
          {{ t(state.status === 'active' ? 'users.promotion.activeHint' : 'users.promotion.pausedHint') }}
        </p>
      </div>

      <div v-if="state.kind === 'ready'" class="ops-promotion__actions">
        <ElTooltip :disabled="canManage" :content="t('users.promotion.permission')" placement="bottom-end">
          <span>
            <ElButton
              v-if="state.status === 'active'"
              type="warning"
              plain
              :disabled="!canManage || submitting"
              @click="openConfirm('pause')"
            >
              {{ t('users.promotion.pause') }}
            </ElButton>
            <ElButton
              v-else
              type="primary"
              :disabled="!canManage || submitting"
              @click="openConfirm('resume')"
            >
              {{ t('users.promotion.resume') }}
            </ElButton>
          </span>
        </ElTooltip>
      </div>
    </div>

    <OpsDialog
      v-model="confirmOpen"
      :title="t(pendingAction === 'pause' ? 'users.promotion.pauseTitle' : 'users.promotion.resumeTitle')"
      width="min(var(--ops-dialog-width), calc(100vw - var(--space-6)))"
      :close-on-click-modal="!submitting"
      :close-on-press-escape="!submitting"
      :show-close="!submitting"
    >
      <ElAlert
        v-if="actionError"
        type="error"
        :title="errorMessage(actionError)"
        :closable="false"
        show-icon
        class="ops-promotion__dialog-alert"
      />
      <p class="ops-promotion__dialog-body">
        {{ t(pendingAction === 'pause' ? 'users.promotion.pauseDescription' : 'users.promotion.resumeDescription') }}
      </p>
      <template #footer>
        <ElButton class="ops-dialog-action" :disabled="submitting" @click="confirmOpen = false">
          {{ t('auth.cancel') }}
        </ElButton>
        <ElButton
          class="ops-dialog-action"
          :type="pendingAction === 'pause' ? 'danger' : 'primary'"
          :loading="submitting"
          @click="confirmAction"
        >
          {{ t(pendingAction === 'pause' ? 'users.promotion.pauseConfirm' : 'users.promotion.resumeConfirm') }}
        </ElButton>
      </template>
    </OpsDialog>
  </ElCard>
</template>

<style scoped>
.ops-promotion-card {
  --el-card-padding: var(--ops-detail-card-block-padding) var(--ops-detail-card-inline-padding);
  border-radius: var(--ops-card-radius);
}

.ops-promotion {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3) var(--space-4);
}

.ops-promotion__info {
  display: grid;
  gap: var(--space-2);
  flex: 1 1 16rem;
  min-width: 0;
}

.ops-promotion__title-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}

.ops-promotion__heading {
  margin: 0;
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
}

.ops-promotion__hint {
  margin: 0;
  font-size: var(--font-size-xs);
  line-height: normal;
  color: var(--color-text-secondary);
  overflow-wrap: anywhere;
}

.ops-promotion__error {
  display: grid;
  justify-items: start;
  gap: var(--space-2);
}

.ops-promotion__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  flex-shrink: 0;
}

.ops-promotion__dialog-alert {
  margin-block-end: var(--space-4);
}

.ops-promotion__dialog-body {
  margin: 0;
  line-height: 1.6;
  color: var(--color-text-secondary);
}
</style>
