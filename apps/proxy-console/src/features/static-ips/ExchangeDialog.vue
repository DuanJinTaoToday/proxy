<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElDialog, ElTag } from 'element-plus'
import { ApiError } from '@/api/client'
import type { StaticIp } from '@/api/staticIps'
import {
  getStaticIpExchange,
  submitStaticIpExchange,
  type StaticIpExchangeQuota,
  type StaticIpExchangeRecord,
  type StaticIpExchangeStatus,
  type StaticIpExchangeSubmission,
} from '@/api/staticIpExchanges'
import { endpoint, useStaticIpPresentation } from './presentation'
import type { ExchangeIntent } from './exchangeIntent'

const props = defineProps<{
  intent: ExchangeIntent
  currentAllocation?: StaticIp
  quota?: StaticIpExchangeQuota
  quotaLoading?: boolean
  quotaError?: unknown
}>()

const emit = defineEmits<{
  closed: []
  submission: [payload: StaticIpExchangeSubmission, unresolved: boolean]
  changed: [allocationId?: string]
  retryQuota: []
}>()

const { t, locale } = useI18n()
const { formatDate, errorMessage } = useStaticIpPresentation()
const open = ref(true)
const controller = new AbortController()
const record = shallowRef<StaticIpExchangeRecord>()
const submission = shallowRef<StaticIpExchangeSubmission | undefined>(
  props.intent.kind === 'resume' ? props.intent.submission : undefined,
)
const busy = ref<'submitting' | 'checking'>()
const failure = shallowRef<unknown>()
const notFoundOnServer = ref(false)
const completedResume = ref(false)
const pollAttempts = ref(0)
const pollExhausted = ref(false)
const polling = ref(false)

const maxPollAttempts = 10
const pollIntervalMs = 3_000
let pollTimer: number | undefined

const statusTagType: Record<StaticIpExchangeStatus, 'info' | 'warning' | 'danger' | 'success'> = {
  pending: 'info',
  unknown: 'warning',
  failed: 'danger',
  activating: 'info',
  ready: 'success',
  support_required: 'warning',
}

const confirmedStatuses: readonly StaticIpExchangeStatus[] = ['activating', 'ready', 'support_required']

function isConfirmedStatus(status: StaticIpExchangeStatus | undefined): boolean {
  return status !== undefined && confirmedStatuses.includes(status)
}

const targetIp = computed(() => props.currentAllocation ?? props.intent.ip)
const isConfirmedRecord = computed(() => isConfirmedStatus(record.value?.status))
const confirmedAllocation = computed(() => {
  const current = props.currentAllocation
  const rec = record.value
  if (!current || !rec || !isConfirmedStatus(rec.status)) return undefined
  if (current.id !== rec.allocationId) return undefined
  if (current.exchangeIdempotencyKey !== rec.idempotencyKey) return undefined
  if (!isConfirmedStatus(current.exchangeStatus)) return undefined
  return current
})
const previousAddress = computed(() => {
  const original = props.intent.ip
  if (!original) return undefined
  const originalEndpoint = endpoint(original)
  if (confirmedAllocation.value && endpoint(confirmedAllocation.value) === originalEndpoint) return undefined
  return originalEndpoint
})
const supportHref = computed(() => (locale.value === 'en' ? '/en#contact' : '/zh-CN#contact'))
const activeKey = computed(() =>
  record.value?.idempotencyKey
  ?? submission.value?.idempotency_key
  ?? (props.intent.kind === 'inspect' ? props.intent.idempotencyKey : undefined),
)
const canConfirmNew = computed(() =>
  props.intent.kind === 'exchange'
  && !props.quotaLoading
  && !props.quotaError
  && props.quota !== undefined
  && props.quota.remaining > 0,
)
const dialogTitle = computed(() =>
  record.value || props.intent.kind !== 'exchange'
    ? t('staticIps.exchange.resultTitle')
    : t('staticIps.exchange.title'),
)

function clearPoll() {
  polling.value = false
  if (pollTimer !== undefined) {
    window.clearTimeout(pollTimer)
    pollTimer = undefined
  }
}

function shouldPoll(status: StaticIpExchangeStatus) {
  return status === 'pending' || status === 'activating'
}

function isDefiniteRejection(error: unknown): error is ApiError {
  return error instanceof ApiError
    && error.status >= 400
    && error.status < 500
    && error.status !== 401
    && error.status !== 408
    && error.status !== 429
}

function publishSubmission(payload: StaticIpExchangeSubmission, unresolved: boolean) {
  submission.value = unresolved ? payload : undefined
  emit('submission', payload, unresolved)
}

async function perform(operation: NonNullable<typeof busy.value>, task: () => Promise<void>) {
  if (busy.value) return
  busy.value = operation
  failure.value = undefined
  try {
    await task()
  } catch (error) {
    if (!controller.signal.aborted) failure.value = error
  } finally {
    busy.value = undefined
  }
}

function schedulePoll(key: string) {
  clearPoll()
  if (controller.signal.aborted) return
  if (pollAttempts.value >= maxPollAttempts) {
    pollExhausted.value = true
    return
  }
  polling.value = true
  pollTimer = window.setTimeout(() => {
    pollTimer = undefined
    void pollOnce(key)
  }, pollIntervalMs)
}

async function pollOnce(key: string) {
  if (busy.value || controller.signal.aborted) {
    polling.value = false
    return
  }
  pollAttempts.value += 1
  try {
    const next = await getStaticIpExchange(key, controller.signal)
    if (controller.signal.aborted) return
    const previousStatus = record.value?.status
    record.value = next
    if (next.status !== previousStatus || (isConfirmedStatus(next.status) && !confirmedAllocation.value)) {
      emit('changed', next.allocationId)
    }
    if (shouldPoll(next.status)) schedulePoll(key)
    else clearPoll()
  } catch (error) {
    clearPoll()
    if (!controller.signal.aborted) failure.value = error
  }
}

function submit() {
  const existing = submission.value
  if (!existing && !canConfirmNew.value) return
  clearPoll()
  notFoundOnServer.value = false
  pollAttempts.value = 0
  pollExhausted.value = false

  return perform('submitting', async () => {
    const payload: StaticIpExchangeSubmission = existing ?? (
      props.intent.kind === 'exchange'
        ? {
            allocation_id: props.intent.ip.id,
            node_id: props.intent.ip.nodeId,
            idempotency_key: crypto.randomUUID(),
          }
        : (() => { throw new Error('Missing exchange target') })()
    )
    publishSubmission(payload, true)
    try {
      const next = await submitStaticIpExchange(payload, controller.signal)
      if (controller.signal.aborted) return
      publishSubmission(payload, false)
      record.value = next
      emit('changed', next.allocationId)
      if (shouldPoll(next.status)) schedulePoll(next.idempotencyKey)
    } catch (error) {
      if (!controller.signal.aborted && isDefiniteRejection(error)) {
        publishSubmission(payload, false)
        completedResume.value = props.intent.kind === 'resume'
        emit('changed')
      }
      throw error
    }
  })
}

function lookupOriginal() {
  const key = activeKey.value
  if (!key) return
  clearPoll()
  notFoundOnServer.value = false
  pollAttempts.value = 0
  pollExhausted.value = false

  return perform('checking', async () => {
    try {
      const next = await getStaticIpExchange(key, controller.signal)
      if (controller.signal.aborted) return
      if (submission.value) publishSubmission(submission.value, false)
      record.value = next
      emit('changed', next.allocationId)
      if (shouldPoll(next.status)) schedulePoll(next.idempotencyKey)
    } catch (error) {
      if (!controller.signal.aborted && error instanceof ApiError && error.status === 404) {
        notFoundOnServer.value = true
        return
      }
      throw error
    }
  })
}

onMounted(() => {
  if (props.intent.kind === 'inspect') void lookupOriginal()
})

onUnmounted(() => {
  clearPoll()
  controller.abort()
})
</script>

<template>
  <ElDialog
    v-model="open"
    :title="dialogTitle"
    class="exchange-dialog"
    width="min(calc(var(--order-drawer-width) + var(--space-6) + var(--space-2)), calc(100vw - 2 * var(--space-4)))"
    align-center
    :show-close="busy !== 'submitting'"
    :close-on-click-modal="busy !== 'submitting'"
    :close-on-press-escape="busy !== 'submitting'"
    @closed="emit('closed')"
  >
    <div class="exchange-body">
      <ElAlert v-if="failure" :title="errorMessage(failure)" type="error" show-icon :closable="false" role="alert" />

      <template v-if="record">
        <div class="exchange-summary">
          <div class="summary-row">
            <span class="summary-label">{{ t('staticIps.state') }}</span>
            <ElTag :type="statusTagType[record.status]">
              {{ t(`staticIps.exchange.status.${record.status}`) }}
            </ElTag>
          </div>
          <template v-if="isConfirmedRecord">
            <div class="summary-row">
              <span class="summary-label">{{ t('staticIps.exchange.currentAddress') }}</span>
              <span v-if="confirmedAllocation" class="summary-value tabular">{{ endpoint(confirmedAllocation) }}</span>
              <span v-else class="summary-pending" role="status">{{ t('staticIps.exchange.waitingNewAddress') }}</span>
            </div>
            <div v-if="previousAddress" class="summary-row">
              <span class="summary-label">{{ t('staticIps.exchange.previousAddress') }}</span>
              <span class="summary-secondary tabular">{{ previousAddress }}</span>
            </div>
          </template>
          <div v-else-if="targetIp" class="summary-row">
            <span class="summary-label">{{ t('staticIps.address') }}</span>
            <span class="summary-value tabular">{{ endpoint(targetIp) }}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">{{ t('staticIps.exchange.requestKey') }}</span>
            <span class="summary-value mono">{{ record.idempotencyKey }}</span>
          </div>
          <div v-if="record.updatedAt" class="summary-row">
            <span class="summary-label">{{ t('staticIps.exchange.updatedAt') }}</span>
            <time class="summary-value tabular" :datetime="record.updatedAt.toISOString()">
              {{ formatDate(record.updatedAt) }}
            </time>
          </div>
        </div>

        <ElAlert
          :type="record.status === 'failed' ? 'error' : record.status === 'ready' ? 'success' : record.status === 'unknown' || record.status === 'support_required' ? 'warning' : 'info'"
          show-icon
          :closable="false"
          :title="t(`staticIps.exchange.statusDetail.${record.status}`)"
        >
          <a v-if="record.status === 'support_required'" :href="supportHref" class="support-link">
            {{ t('staticIps.exchange.contactSupport') }}
          </a>
        </ElAlert>

        <p v-if="polling" class="exchange-note" role="status">{{ t('staticIps.exchange.polling') }}</p>
        <p v-else-if="pollExhausted && (record.status === 'pending' || record.status === 'activating')" class="exchange-note" role="status">
          {{ t('staticIps.exchange.pollExhausted') }}
        </p>
      </template>

      <template v-else-if="submission">
        <ElAlert
          v-if="notFoundOnServer"
          type="warning"
          show-icon
          :closable="false"
          :title="t('staticIps.exchange.notFound')"
        />
        <ElAlert
          v-else-if="busy !== 'submitting'"
          type="warning"
          show-icon
          :closable="false"
          :title="t('staticIps.exchange.unverifiedSubmission')"
        />
        <p v-if="busy === 'submitting'" role="status">{{ t('staticIps.exchange.submitting') }}</p>
        <p v-else-if="busy === 'checking'" role="status">{{ t('staticIps.exchange.checking') }}</p>

        <div class="exchange-summary">
          <div v-if="targetIp" class="summary-row">
            <span class="summary-label">{{ t('staticIps.address') }}</span>
            <span class="summary-value tabular">{{ endpoint(targetIp) }}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">{{ t('staticIps.exchange.requestKey') }}</span>
            <span class="summary-value mono">{{ submission.idempotency_key }}</span>
          </div>
        </div>
      </template>

      <template v-else-if="intent.kind === 'exchange'">
        <p class="exchange-intro">{{ t('staticIps.exchange.intro') }}</p>

        <div class="exchange-summary">
          <div class="summary-row">
            <span class="summary-label">{{ t('staticIps.address') }}</span>
            <span class="summary-value tabular">{{ endpoint(intent.ip) }}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">{{ t('staticIps.region') }}</span>
            <span class="summary-value">{{ intent.ip.region || t('staticIps.unknownRegion') }}</span>
          </div>
          <div class="summary-row">
            <span class="summary-label">{{ t('staticIps.exchange.quotaTitle') }}</span>
            <span class="summary-value tabular">
              <template v-if="quotaLoading">—</template>
              <template v-else-if="quotaError || !quota">{{ t('staticIps.exchange.quotaLoadFailed') }}</template>
              <template v-else>{{ t('staticIps.exchange.quotaCount', { count: quota.remaining }) }}</template>
            </span>
          </div>
        </div>

        <ul class="exchange-rules">
          <li>{{ t('staticIps.exchange.ruleSameRegion') }}</li>
          <li>{{ t('staticIps.exchange.ruleFree') }}</li>
          <li>{{ t('staticIps.exchange.ruleQuotaCost') }}</li>
        </ul>

        <ElAlert v-if="quotaError" type="error" show-icon :closable="false" :title="t('staticIps.exchange.quotaLoadFailed')" role="alert">
          <p>{{ errorMessage(quotaError) }}</p>
          <ElButton link type="primary" @click="emit('retryQuota')">{{ t('staticIps.retry') }}</ElButton>
        </ElAlert>
        <ElAlert
          v-else-if="quota && quota.remaining <= 0"
          type="warning"
          show-icon
          :closable="false"
          :title="t('staticIps.exchange.disabled.noQuota')"
        />
      </template>

      <template v-else>
        <p v-if="busy === 'checking'" role="status">{{ t('staticIps.exchange.checking') }}</p>
        <ElAlert
          v-else-if="notFoundOnServer"
          type="warning"
          show-icon
          :closable="false"
          :title="t('staticIps.exchange.inspectNotFound')"
        />
      </template>
    </div>

    <template #footer>
      <div class="dialog-actions">
        <ElButton
          :type="(record && record.status !== 'pending' && record.status !== 'activating') || completedResume ? 'primary' : 'default'"
          :disabled="busy === 'submitting'"
          @click="open = false"
        >
          {{ t(record || completedResume ? 'staticIps.exchange.done' : 'staticIps.exchange.cancel') }}
        </ElButton>

        <template v-if="record">
          <ElButton
            v-if="record.status === 'pending' || record.status === 'unknown' || record.status === 'activating' || record.status === 'support_required'"
            type="primary"
            :loading="busy === 'checking'"
            @click="lookupOriginal"
          >
            {{ t('staticIps.exchange.queryOriginal') }}
          </ElButton>
        </template>

        <template v-else-if="submission">
          <ElButton :loading="busy === 'checking'" :disabled="busy === 'submitting'" @click="lookupOriginal">
            {{ t('staticIps.exchange.queryOriginal') }}
          </ElButton>
          <ElButton type="primary" :loading="busy === 'submitting'" :disabled="busy === 'checking'" @click="submit">
            {{ t('staticIps.exchange.retryOriginal') }}
          </ElButton>
        </template>

        <template v-else-if="intent.kind === 'exchange'">
          <ElButton type="primary" :loading="busy === 'submitting'" :disabled="!canConfirmNew" @click="submit">
            {{ t('staticIps.exchange.confirm') }}
          </ElButton>
        </template>

        <template v-else-if="intent.kind === 'inspect'">
          <ElButton type="primary" :loading="busy === 'checking'" @click="lookupOriginal">
            {{ t('staticIps.exchange.queryOriginal') }}
          </ElButton>
        </template>
      </div>
    </template>
  </ElDialog>
</template>

<style scoped>
.exchange-body { display: grid; gap: var(--space-4); min-width: 0; }
.exchange-intro, .exchange-note { color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.exchange-summary {
  display: grid;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface-muted);
}
.summary-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: var(--space-3);
  flex-wrap: wrap;
}
.summary-label { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.summary-value {
  color: var(--color-text);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
  overflow-wrap: anywhere;
}
.summary-secondary, .summary-pending {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
  overflow-wrap: anywhere;
}
.exchange-rules {
  margin: 0;
  padding-inline-start: var(--space-4);
  display: grid;
  gap: var(--space-1);
  color: var(--color-text);
  font-size: var(--font-size-sm);
}
.support-link {
  display: inline-block;
  margin-top: var(--space-1);
  color: var(--color-primary);
  font-weight: var(--font-weight-semibold);
  text-decoration: underline;
}
.dialog-actions { display: flex; flex-wrap: wrap; gap: var(--space-2); justify-content: flex-end; }
.dialog-actions button { margin: 0; }
</style>

<style>
.exchange-dialog {
  --el-dialog-border-radius: var(--radius-card);
  --el-dialog-padding-primary: var(--space-5);
  --el-dialog-box-shadow: var(--shadow-overlay);
}
</style>
