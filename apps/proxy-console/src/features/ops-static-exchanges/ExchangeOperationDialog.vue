<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  ElAlert,
  ElButton,
  ElForm,
  ElFormItem,
  ElInput,
  ElMessage,
  ElRadio,
  ElRadioGroup,
  ElTag,
} from 'element-plus'
import OpsDialog from '@/shared/ui/OpsDialog.vue'
import { ApiError } from '@/api/client'
import {
  adjustOpsStaticIpExchangeQuota,
  adjustStaticIpExchangeQuotaPath,
  resolveOpsStaticIpExchange,
  resolveStaticIpExchangePath,
  type OperatorStaticIpExchange,
  type OperatorStaticIpExchangeAuditEntry,
  type OperatorStaticIpExchangeQuota,
  type StaticIpExchangeResolutionOutcome,
} from '@/api/opsStaticExchanges'
import { currentOperator, hasOperatorPermission } from '@/features/auth'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import {
  clearPendingIntent,
  generateRequestKey,
  isDefinitiveClientRejection,
  loadPendingIntent,
  savePendingIntent,
  toIntentErrorSnapshot,
  type ExchangeIntentAction,
  type PendingAdjustQuotaIntent,
  type PendingExchangeIntent,
  type PendingResolveExchangeIntent,
} from './exchangeIntent'
import { canCorrectExchange, canResolveExchange, exchangeStatusBadge } from './useOpsStaticExchanges'

export type ExchangeDialogMode = 'adjust' | 'resolve' | 'correct'

const INT32_MIN = -2_147_483_648
const INT32_MAX = 2_147_483_647

const props = defineProps<{
  mode: ExchangeDialogMode
  tenantId: string
  tenantName?: string
  quota?: OperatorStaticIpExchangeQuota
  exchange?: OperatorStaticIpExchange
  initialIntent?: PendingExchangeIntent
}>()

const emit = defineEmits<{
  completed: []
  intentsChanged: []
}>()

const open = defineModel<boolean>({ required: true })

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const canWrite = computed(() => hasOperatorPermission('manage_billing'))
const operator = computed(() => currentOperator.value?.username ?? '')

const intentAction = computed<ExchangeIntentAction>(() => {
  if (props.mode === 'adjust') return 'adjust_quota'
  return props.mode === 'correct' ? 'correct_exchange' : 'resolve_exchange'
})

const targetIdempotencyKey = computed(() => {
  if (props.exchange?.idempotencyKey) return props.exchange.idempotencyKey
  if (props.initialIntent && props.initialIntent.action !== 'adjust_quota') {
    return props.initialIntent.idempotencyKey
  }
  return ''
})

const deltaInput = ref('')
const outcome = ref<StaticIpExchangeResolutionOutcome | ''>('')
const expectedIp = ref('')
const evidence = ref('')
const submitting = ref(false)
const failure = shallowRef<unknown>()
const pendingIntent = shallowRef<PendingExchangeIntent>()
const adjustAuditResult = shallowRef<OperatorStaticIpExchangeAuditEntry>()

const isLockedReplay = computed(() => Boolean(pendingIntent.value))

const dialogTitle = computed(() => {
  if (props.mode === 'adjust') return t('opsExchange.dialog.adjustTitle')
  if (props.mode === 'correct') return t('opsExchange.dialog.correctTitle')
  return t('opsExchange.dialog.resolveTitle')
})

const parsedDelta = computed(() => {
  const raw = deltaInput.value.trim()
  if (!/^[+-]?\d+$/.test(raw)) return undefined
  const num = Number(raw)
  if (!Number.isSafeInteger(num)) return undefined
  return num
})

const eligibilityError = computed(() => {
  if (props.mode === 'adjust' || isLockedReplay.value) return ''
  if (!props.exchange) return 'opsExchange.validation.notEligible'
  if (props.mode === 'resolve' && !canResolveExchange(props.exchange)) {
    return 'opsExchange.validation.notEligible'
  }
  if (props.mode === 'correct' && !canCorrectExchange(props.exchange)) {
    return 'opsExchange.validation.notEligible'
  }
  return ''
})

const validationKey = computed(() => {
  if (!canWrite.value) return 'opsExchange.validation.permission'
  if (isLockedReplay.value) return ''
  if (eligibilityError.value) return eligibilityError.value

  if (props.mode === 'adjust') {
    const value = parsedDelta.value
    if (value === undefined || value === 0) return 'opsExchange.validation.deltaRequired'
    if (value < INT32_MIN || value > INT32_MAX) return 'opsExchange.validation.deltaRange'
    if (props.quota && value < -props.quota.remaining) {
      return 'opsExchange.validation.deltaExceedsRemaining'
    }
    if (!evidence.value.trim()) return 'opsExchange.validation.evidenceRequired'
    return ''
  }

  if (outcome.value !== 'executed' && outcome.value !== 'not_executed') {
    return 'opsExchange.validation.outcomeRequired'
  }
  if (outcome.value === 'executed' && !expectedIp.value.trim()) {
    return 'opsExchange.validation.expectedIpRequired'
  }
  if (!evidence.value.trim()) return 'opsExchange.validation.evidenceRequired'
  return ''
})

const projectedQuota = computed(() => {
  if (props.mode !== 'adjust' || !props.quota) return undefined
  const delta = parsedDelta.value
  if (delta === undefined || delta === 0 || delta < -props.quota.remaining) return undefined
  return {
    total: props.quota.total + delta,
    remaining: props.quota.remaining + delta,
  }
})

const statusBadge = computed(() => props.exchange ? exchangeStatusBadge(props.exchange) : undefined)

function formatDate(date?: Date) {
  return date ? shanghaiDateTime(date, locale.value, 'short') : t('opsExchange.common.none')
}

function formatQuotaSnapshot(snapshot?: OperatorStaticIpExchangeQuota) {
  if (!snapshot) return t('opsExchange.common.none')
  return t('opsExchange.dialog.quotaSnapshot', {
    total: snapshot.total,
    remaining: snapshot.remaining,
    reserved: snapshot.reserved,
    used: snapshot.used,
  })
}

function hydrateDialog() {
  failure.value = undefined
  adjustAuditResult.value = undefined

  const existing = (
    props.initialIntent
    && props.initialIntent.operator === operator.value
    && props.initialIntent.tenantId === props.tenantId
    && props.initialIntent.action === intentAction.value
      ? props.initialIntent
      : loadPendingIntent(
          operator.value,
          intentAction.value,
          props.tenantId,
          intentAction.value === 'adjust_quota' ? undefined : targetIdempotencyKey.value,
        )
  )

  pendingIntent.value = existing

  if (existing) {
    if (existing.action === 'adjust_quota') {
      deltaInput.value = String(existing.payload.delta)
      evidence.value = existing.payload.evidence
      outcome.value = ''
      expectedIp.value = ''
    } else {
      deltaInput.value = ''
      outcome.value = existing.payload.outcome
      expectedIp.value = existing.payload.expected_ip ?? ''
      evidence.value = existing.payload.evidence
    }
    if (existing.lastError) {
      failure.value = new ApiError(
        existing.lastError.status,
        existing.lastError.reason,
        existing.lastError.message,
      )
    }
    return
  }

  deltaInput.value = ''
  outcome.value = ''
  expectedIp.value = ''
  evidence.value = ''
}

watch(
  [open, () => props.mode, () => props.tenantId, targetIdempotencyKey, () => props.initialIntent],
  ([isOpen]) => {
    if (isOpen) hydrateDialog()
  },
  { immediate: true },
)

async function executeIntent(intent: PendingExchangeIntent) {
  if (!canWrite.value || submitting.value) return
  if (intent.operator !== operator.value || intent.tenantId !== props.tenantId) return
  submitting.value = true
  failure.value = undefined
  try {
    if (intent.action === 'adjust_quota') {
      const audit = await adjustOpsStaticIpExchangeQuota(intent.path, intent.payload)
      clearPendingIntent(intent.operator, intent.action, intent.tenantId)
      pendingIntent.value = undefined
      adjustAuditResult.value = audit
      emit('intentsChanged')
      emit('completed')
    } else {
      await resolveOpsStaticIpExchange(intent.path, intent.payload)
      clearPendingIntent(intent.operator, intent.action, intent.tenantId, intent.idempotencyKey)
      pendingIntent.value = undefined
      emit('intentsChanged')
      emit('completed')
      ElMessage.success(t(
        intent.action === 'correct_exchange'
          ? 'opsExchange.dialog.correctSuccess'
          : 'opsExchange.dialog.resolveSuccess',
      ))
      open.value = false
    }
  } catch (error) {
    failure.value = error
    if (isDefinitiveClientRejection(error)) {
      clearPendingIntent(
        intent.operator,
        intent.action,
        intent.tenantId,
        intent.action === 'adjust_quota' ? undefined : intent.idempotencyKey,
      )
      pendingIntent.value = undefined
      emit('intentsChanged')
    } else {
      const updated: PendingExchangeIntent = {
        ...intent,
        lastError: toIntentErrorSnapshot(error),
      }
      savePendingIntent(updated)
      pendingIntent.value = updated
      emit('intentsChanged')
    }
  } finally {
    submitting.value = false
  }
}

async function submitNewRequest() {
  if (!canWrite.value || submitting.value || pendingIntent.value || validationKey.value) return
  if (!operator.value || !props.tenantId) return

  const requestKey = generateRequestKey()
  const trimmedEvidence = evidence.value.trim()

  if (props.mode === 'adjust') {
    const delta = parsedDelta.value
    if (delta === undefined || delta === 0) return
    const intent: PendingAdjustQuotaIntent = {
      operator: operator.value,
      tenantId: props.tenantId,
      action: 'adjust_quota',
      path: adjustStaticIpExchangeQuotaPath(props.tenantId),
      requestKey,
      createdAt: new Date().toISOString(),
      payload: {
        delta,
        request_key: requestKey,
        evidence: trimmedEvidence,
      },
    }
    savePendingIntent(intent)
    pendingIntent.value = intent
    emit('intentsChanged')
    await executeIntent(intent)
    return
  }

  const key = targetIdempotencyKey.value
  if (!key || (outcome.value !== 'executed' && outcome.value !== 'not_executed')) return

  const payload: PendingResolveExchangeIntent['payload'] = {
    request_key: requestKey,
    outcome: outcome.value,
    evidence: trimmedEvidence,
  }
  if (outcome.value === 'executed') {
    payload.expected_ip = expectedIp.value.trim()
  }
  if (props.mode === 'correct') {
    payload.correction = true
  }

  const intent: PendingResolveExchangeIntent = {
    operator: operator.value,
    tenantId: props.tenantId,
    action: props.mode === 'correct' ? 'correct_exchange' : 'resolve_exchange',
    idempotencyKey: key,
    allocationId: props.exchange?.allocationId,
    path: resolveStaticIpExchangePath(props.tenantId, key),
    requestKey,
    createdAt: new Date().toISOString(),
    payload,
  }
  savePendingIntent(intent)
  pendingIntent.value = intent
  emit('intentsChanged')
  await executeIntent(intent)
}

async function retryOriginalRequest() {
  if (!pendingIntent.value) return
  await executeIntent(pendingIntent.value)
}
</script>

<template>
  <OpsDialog
    v-model="open"
    :title="dialogTitle"
    width="min(560px, calc(100vw - var(--space-6)))"
    :close-on-click-modal="!submitting"
    :close-on-press-escape="!submitting"
    :show-close="!submitting"
  >
    <dl class="op-summary">
      <div>
        <dt>{{ t('opsExchange.search.selected') }}</dt>
        <dd>
          <span v-if="tenantName">{{ tenantName }} · </span>
          <span class="mono">{{ tenantId }}</span>
        </dd>
      </div>

      <template v-if="mode === 'adjust' && quota">
        <div>
          <dt>{{ t('opsExchange.quota.title') }}</dt>
          <dd class="tabular">{{ formatQuotaSnapshot(quota) }}</dd>
        </div>
      </template>

      <template v-else-if="exchange">
        <div>
          <dt>{{ t('opsExchange.columns.status') }}</dt>
          <dd>
            <ElTag v-if="statusBadge" :type="statusBadge.type" size="small" disable-transitions>
              {{ statusBadge.labelKey ? t(statusBadge.labelKey) : statusBadge.raw }}
            </ElTag>
          </dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.labels.allocationId') }}</dt>
          <dd class="mono">{{ exchange.allocationId || t('opsExchange.common.none') }}</dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.labels.idempotencyKey') }}</dt>
          <dd class="mono">{{ exchange.idempotencyKey }}</dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.columns.ips') }}</dt>
          <dd class="mono">
            {{ exchange.oldIp || t('opsExchange.common.none') }} → {{ exchange.newIp || t('opsExchange.common.none') }}
          </dd>
        </div>
        <div v-if="exchange.supplierOrderId || exchange.supplierItemId">
          <dt>{{ t('opsExchange.columns.supplier') }}</dt>
          <dd class="mono">
            {{ exchange.supplierOrderId || t('opsExchange.common.none') }}
            <template v-if="exchange.supplierItemId"> / {{ exchange.supplierItemId }}</template>
          </dd>
        </div>
        <div v-if="exchange.resolution">
          <dt>{{ t('opsExchange.dialog.outcomeLabel') }}</dt>
          <dd>
            {{ exchange.resolution === 'executed' ? t('opsExchange.resolution.executed') : exchange.resolution === 'not_executed' ? t('opsExchange.resolution.notExecuted') : exchange.resolution }}
          </dd>
        </div>
      </template>
    </dl>

    <template v-if="adjustAuditResult">
      <ElAlert
        type="success"
        :title="t('opsExchange.dialog.adjustSuccessTitle')"
        :closable="false"
        show-icon
        class="op-alert"
      />
      <dl class="op-summary op-audit-result">
        <div>
          <dt>{{ t('opsExchange.labels.before') }}</dt>
          <dd class="tabular">{{ formatQuotaSnapshot(adjustAuditResult.before) }}</dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.labels.after') }}</dt>
          <dd class="tabular">{{ formatQuotaSnapshot(adjustAuditResult.after) }}</dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.columns.actor') }}</dt>
          <dd class="mono">{{ adjustAuditResult.actorId || operator || t('opsExchange.common.none') }}</dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.columns.evidence') }}</dt>
          <dd>{{ adjustAuditResult.evidence || t('opsExchange.common.none') }}</dd>
        </div>
        <div>
          <dt>{{ t('opsExchange.labels.requestKey') }}</dt>
          <dd class="mono">{{ adjustAuditResult.requestKey || t('opsExchange.common.none') }}</dd>
        </div>
        <div v-if="adjustAuditResult.createdAt">
          <dt>{{ t('opsExchange.columns.time') }}</dt>
          <dd>{{ formatDate(adjustAuditResult.createdAt) }}</dd>
        </div>
      </dl>
    </template>

    <template v-else>
      <ElAlert
        v-if="pendingIntent"
        type="warning"
        :title="t('opsExchange.dialog.pendingIntentTitle')"
        :closable="false"
        show-icon
        class="op-alert"
      >
        <p class="op-alert-text">{{ t('opsExchange.dialog.pendingIntentBody') }}</p>
        <dl class="op-locked-meta">
          <div>
            <dt>{{ t('opsExchange.dialog.lockedRequestKey') }}</dt>
            <dd class="mono">{{ pendingIntent.requestKey }}</dd>
          </div>
          <div>
            <dt>{{ t('opsExchange.dialog.lockedPath') }}</dt>
            <dd class="mono">{{ pendingIntent.path }}</dd>
          </div>
        </dl>
      </ElAlert>

      <ElAlert
        v-if="mode !== 'adjust'"
        type="warning"
        :title="t('opsExchange.dialog.verificationWarningTitle')"
        :closable="false"
        show-icon
        class="op-alert"
      >
        {{ t('opsExchange.dialog.verificationWarningBody') }}
      </ElAlert>

      <ElAlert
        v-if="mode === 'correct'"
        type="info"
        :title="t('opsExchange.dialog.correctionWarningTitle')"
        :closable="false"
        show-icon
        class="op-alert"
      >
        {{ t('opsExchange.dialog.correctionWarningBody') }}
      </ElAlert>

      <ElForm
        label-position="top"
        class="op-form"
        :disabled="submitting || !canWrite || isLockedReplay"
        @submit.prevent="isLockedReplay ? retryOriginalRequest() : submitNewRequest()"
      >
        <template v-if="mode === 'adjust'">
          <ElFormItem :label="t('opsExchange.dialog.deltaLabel')" for="ops-exchange-delta" required>
            <ElInput
              id="ops-exchange-delta"
              v-model="deltaInput"
              inputmode="numeric"
              :placeholder="t('opsExchange.dialog.deltaPlaceholder')"
              :aria-label="t('opsExchange.dialog.deltaLabel')"
            />
            <p class="op-hint">
              {{ t('opsExchange.dialog.deltaHint', { maxDecrease: quota?.remaining ?? 0 }) }}
            </p>
            <p v-if="parsedDelta && parsedDelta > 0" class="op-delta-preview op-delta-preview--add">
              {{ t('opsExchange.dialog.deltaIncrease', { value: parsedDelta }) }}
            </p>
            <p v-else-if="parsedDelta && parsedDelta < 0" class="op-delta-preview op-delta-preview--deduct">
              {{ t('opsExchange.dialog.deltaDecrease', { value: parsedDelta }) }}
            </p>
            <p v-if="projectedQuota" class="op-hint">
              {{ t('opsExchange.dialog.projected', projectedQuota) }}
            </p>
          </ElFormItem>
        </template>

        <template v-else>
          <ElFormItem :label="t('opsExchange.dialog.outcomeLabel')" required>
            <ElRadioGroup
              v-model="outcome"
              :aria-label="t('opsExchange.dialog.outcomeLabel')"
              class="op-radio-group"
            >
              <ElRadio value="executed" class="op-radio">
                {{ t('opsExchange.dialog.outcomeExecuted') }}
              </ElRadio>
              <ElRadio value="not_executed" class="op-radio">
                {{ t('opsExchange.dialog.outcomeNotExecuted') }}
              </ElRadio>
            </ElRadioGroup>
          </ElFormItem>

          <ElFormItem
            v-if="outcome === 'executed'"
            :label="t('opsExchange.dialog.expectedIpLabel')"
            for="ops-exchange-expected-ip"
            required
          >
            <ElInput
              id="ops-exchange-expected-ip"
              v-model="expectedIp"
              :placeholder="t('opsExchange.dialog.expectedIpPlaceholder')"
              :aria-label="t('opsExchange.dialog.expectedIpLabel')"
            />
          </ElFormItem>
        </template>

        <ElFormItem :label="t('opsExchange.dialog.evidenceLabel')" for="ops-exchange-evidence" required>
          <ElInput
            id="ops-exchange-evidence"
            v-model="evidence"
            type="textarea"
            :rows="3"
            maxlength="500"
            show-word-limit
            :placeholder="mode === 'adjust' ? t('opsExchange.dialog.adjustEvidencePlaceholder') : t('opsExchange.dialog.resolveEvidencePlaceholder')"
            :aria-label="t('opsExchange.dialog.evidenceLabel')"
          />
        </ElFormItem>
      </ElForm>

      <ElAlert
        v-if="failure"
        type="error"
        :title="errorMessage(failure)"
        :closable="false"
        show-icon
        class="op-alert"
      />
    </template>

    <template #footer>
      <div class="op-footer">
        <span v-if="!adjustAuditResult && validationKey" class="op-error">{{ t(validationKey) }}</span>
        <template v-if="adjustAuditResult">
          <ElButton class="ops-dialog-action" type="primary" @click="open = false">
            {{ t('opsExchange.common.close') }}
          </ElButton>
        </template>
        <template v-else-if="isLockedReplay">
          <ElButton class="ops-dialog-action" :disabled="submitting" @click="open = false">
            {{ t('opsExchange.common.close') }}
          </ElButton>
          <ElButton
            class="ops-dialog-action"
            type="warning"
            :disabled="!canWrite"
            :loading="submitting"
            @click="retryOriginalRequest"
          >
            {{ t('opsExchange.actions.retryOriginal') }}
          </ElButton>
        </template>
        <template v-else>
          <ElButton class="ops-dialog-action" :disabled="submitting" @click="open = false">
            {{ t('opsExchange.common.cancel') }}
          </ElButton>
          <ElButton
            class="ops-dialog-action"
            type="primary"
            :disabled="Boolean(validationKey)"
            :loading="submitting"
            @click="submitNewRequest"
          >
            {{ mode === 'adjust' ? t('opsExchange.dialog.confirmAdjust') : t('opsExchange.dialog.confirmResolve') }}
          </ElButton>
        </template>
      </div>
    </template>
  </OpsDialog>
</template>

<style scoped>
.op-summary {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0 0 var(--space-4);
  font-size: var(--font-size-sm);
  line-height: normal;
}
.op-summary div {
  display: flex;
  gap: var(--space-3);
  justify-content: space-between;
}
.op-summary dt {
  color: var(--color-text-secondary);
  flex: none;
}
.op-summary dd {
  margin: 0;
  font-weight: var(--font-weight-semibold);
  text-align: end;
  overflow-wrap: anywhere;
}
.op-audit-result {
  padding: var(--space-3);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface-muted);
}
.op-alert {
  margin-block-end: var(--space-3);
}
.op-alert-text {
  margin: 0 0 var(--space-2);
}
.op-locked-meta {
  display: flex;
  flex-direction: column;
  gap: var(--space-1);
  margin: 0;
  font-size: var(--font-size-xs);
}
.op-locked-meta div {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.op-locked-meta dt {
  color: var(--color-text-secondary);
}
.op-locked-meta dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.op-form {
  margin-block-start: var(--space-2);
}
.op-hint {
  margin: var(--space-1) 0 0;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  line-height: var(--line-height-normal);
}
.op-delta-preview {
  margin: var(--space-1) 0 0;
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
}
.op-delta-preview--add {
  color: var(--color-success-text, var(--el-color-success));
}
.op-delta-preview--deduct {
  color: var(--color-warning-text, var(--el-color-warning));
}
.op-radio-group {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-1);
}
.op-radio {
  margin: 0;
  white-space: normal;
  height: auto;
  min-height: var(--space-6);
}
.op-footer {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: flex-end;
  width: 100%;
  gap: var(--space-2);
}
.op-error {
  margin-inline-end: auto;
  color: var(--color-danger-text);
  font-size: var(--font-size-xs);
}
</style>
