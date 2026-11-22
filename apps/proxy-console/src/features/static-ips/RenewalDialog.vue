<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElDialog, ElRadioButton, ElRadioGroup, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { ApiError } from '@/api/client'
import {
  getStaticRenewalOrder, listRenewalTerms, quoteStaticRenewal, recheckStaticRenewal, submitStaticRenewal,
  type RenewalOrder, type RenewalOutcome, type RenewalSubmission,
} from '@/api/staticIps'
import { formatYuan } from '@/shared/lib/money'
import { endpoint, useStaticIpPresentation } from './presentation'
import type { RenewalIntent } from './renewalIntent'

const props = defineProps<{ intent: RenewalIntent }>()
const emit = defineEmits<{
  closed: []
  submission: [payload: RenewalSubmission, unresolved: boolean]
  changed: []
}>()

const { t, locale } = useI18n()
const { formatDate, errorMessage } = useStaticIpPresentation()
const open = ref(true)
const controller = new AbortController()
type RenewalQuote = Awaited<ReturnType<typeof quoteStaticRenewal>>
const terms = shallowRef<Awaited<ReturnType<typeof listRenewalTerms>>>()
const term = ref('')
const quote = shallowRef<RenewalQuote>()
const order = shallowRef<RenewalOrder>()
const submission = shallowRef<RenewalSubmission | undefined>(props.intent.kind === 'resume' ? props.intent.submission : undefined)
const completedResume = ref(false)
const busy = ref<'loadingTerms' | 'loadingOrder' | 'submitting' | 'checking' | 'quote'>()
const failure = shallowRef<unknown>()
const targets = props.intent.kind === 'renew' ? props.intent.ips.map(ip => ({ ...ip, address: endpoint(ip) })) : []
const ids = targets.map(ip => ip.id)
const quoteLines = computed(() => quote.value?.lines.map(line => {
  const ip = targets.find(target => target.id === line.id)
  return { ...line, address: ip?.address ?? line.ip, ipType: ip?.ipType }
}) ?? [])
const expectedTotal = computed(() => quote.value?.lines.reduce((sum, line) => sum + line.unitMinor, 0n) ?? 0n)
const outcomeType = { pending: 'warning', succeeded: 'success', failed: 'danger' } as const

function isRenewalOutcome(value: unknown): value is RenewalOutcome {
  return value === 'pending' || value === 'succeeded' || value === 'failed'
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

function loadTerms() {
  return perform('loadingTerms', async () => {
    const options = await listRenewalTerms(ids, controller.signal)
    terms.value = options
    term.value = options.find(option => option.code === term.value)?.code ?? options[0]?.code ?? ''
  })
}

function loadOrder() {
  if (props.intent.kind !== 'order') return
  const { id } = props.intent
  return perform('loadingOrder', async () => { order.value = await getStaticRenewalOrder(id, controller.signal) })
}

function refreshQuote() {
  const selected = term.value
  if (!selected) return
  quote.value = undefined
  return perform('quote', async () => { quote.value = await quoteStaticRenewal(ids, selected, controller.signal) })
}

function publishSubmission(payload: RenewalSubmission, unresolved: boolean) {
  submission.value = unresolved ? payload : undefined
  emit('submission', payload, unresolved)
}

function hasDefiniteFailure(error: unknown): error is ApiError {
  return error instanceof ApiError && [
    'STATIC_QUOTE_EXPIRED', 'STATIC_RENEWAL_EXPIRED', 'STATIC_RENEWAL_PRICE_UNAVAILABLE', 'STATIC_RENEWAL_PENDING',
    'WALLET_INSUFFICIENT_FUNDS', 'WALLET_RECORD_NOT_FOUND', 'WALLET_REQUEST_CONFLICT',
  ].includes(error.reason)
}

function submit() {
  if (!submission.value && !quote.value) return
  return perform('submitting', async () => {
    const payload = submission.value ?? {
      supplier_item_ids: ids,
      term: term.value,
      request_key: crypto.randomUUID(),
      confirmation_token: quote.value?.token ?? '',
    }
    publishSubmission(payload, true)
    try {
      order.value = await submitStaticRenewal(payload, controller.signal)
      if (controller.signal.aborted) return
      publishSubmission(payload, false)
      emit('changed')
    } catch (error) {
      if (hasDefiniteFailure(error) && !controller.signal.aborted) {
        publishSubmission(payload, false)
        completedResume.value = props.intent.kind === 'resume'
        if (error.reason !== 'WALLET_INSUFFICIENT_FUNDS') quote.value = undefined
        if (error.reason === 'STATIC_RENEWAL_PRICE_UNAVAILABLE') terms.value = undefined
        emit('changed')
      }
      throw error
    }
  })
}

function recheck() {
  const id = order.value?.id
  if (!id) return
  return perform('checking', async () => {
    try {
      order.value = await recheckStaticRenewal(id, controller.signal)
    } catch (error) {
      if (error instanceof ApiError && error.reason === 'STATIC_SUPPLIER_UNAVAILABLE') {
        order.value = await getStaticRenewalOrder(id, controller.signal).catch(() => order.value)
      }
      throw error
    } finally {
      if (!controller.signal.aborted) emit('changed')
    }
  })
}

function changeTerm() {
  quote.value = undefined
  failure.value = undefined
}

onMounted(() => {
  if (props.intent.kind === 'renew') void loadTerms()
  if (props.intent.kind === 'order') void loadOrder()
})
onUnmounted(() => controller.abort())
</script>

<template>
  <ElDialog
    v-model="open" :title="t(order || intent.kind === 'order' ? 'staticIps.renewal.result' : 'staticIps.renewal.title')"
    class="renewal-dialog" width="min(calc(var(--order-drawer-width) + var(--space-6) + var(--space-2)), calc(100vw - 2 * var(--space-4)))" align-center
    :show-close="!busy" :close-on-click-modal="!busy" :close-on-press-escape="!busy" @closed="emit('closed')"
  >
    <div class="renewal-body">
      <ElAlert v-if="failure" :title="errorMessage(failure)" type="error" show-icon :closable="false" role="alert" />

      <template v-if="order">
        <p class="tabular">{{ t('staticIps.renewal.order', { id: order.id }) }}</p>
        <ElTable :data="order.items" row-key="id" class="renewal-quote">
          <ElTableColumn :label="t('staticIps.address')" prop="ip" min-width="128" class-name="mono" />
          <ElTableColumn :label="t('staticIps.state')" min-width="128">
            <template #default="{ row }">
              <ElTag v-if="isRenewalOutcome(row.outcome)" :type="outcomeType[row.outcome]">
                {{ t(`staticIps.renewal.outcome.${row.outcome}`) }}
              </ElTag>
            </template>
          </ElTableColumn>
        </ElTable>
      </template>
      <template v-else-if="submission">
        <ElAlert v-if="!busy" :title="t('staticIps.renewal.unknown')" type="warning" show-icon :closable="false" />
        <p v-else role="status">{{ t('staticIps.renewal.submitting') }}</p>
      </template>
      <template v-else-if="intent.kind === 'renew'">
        <p class="renewal-intro">{{ t('staticIps.renewal.intro') }}</p>
        <p v-if="busy === 'loadingTerms'" role="status">{{ t('staticIps.renewal.loadingTerms') }}</p>
        <ElAlert v-if="terms && !terms.length" :title="t('staticIps.renewal.noTerms')" type="warning" :closable="false" show-icon />
        <div v-if="terms?.length" class="renewal-terms">
          <span>{{ t('staticIps.renewal.term') }}</span>
          <ElRadioGroup v-model="term" class="term-options" :aria-label="t('staticIps.renewal.term')" :disabled="Boolean(busy)" @change="changeTerm">
            <ElRadioButton v-for="option in terms" :key="option.code" :value="option.code">
              {{ t('staticIps.renewal.days', { days: option.days }) }}
            </ElRadioButton>
          </ElRadioGroup>
        </div>
        <ElTable v-if="quote" :data="quoteLines" row-key="id" class="renewal-quote">
          <ElTableColumn :label="t('staticIps.renewal.targets', { count: ids.length })" min-width="145">
            <template #default="{ row }">
              <span class="quote-endpoint tabular">{{ row.address }}</span>
              <span v-if="row.ipType" class="quote-product">{{ row.ipType }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="t('staticIps.renewal.price')" min-width="54">
            <template #default="{ row }">
              <span class="tabular">{{ formatYuan(row.unitMinor, locale) }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="t('staticIps.renewal.newExpiry')" min-width="110" align="right">
            <template #default="{ row }">
              <time class="tabular" :datetime="row.newExpiry.toISOString()">{{ formatDate(row.newExpiry) }}</time>
            </template>
          </ElTableColumn>
        </ElTable>
        <ElTable v-else :data="targets" row-key="id" class="renewal-quote">
          <ElTableColumn :label="t('staticIps.renewal.targets', { count: ids.length })" min-width="145">
            <template #default="{ row }">
              <span class="quote-endpoint tabular">{{ row.address }}</span>
              <span v-if="row.ipType" class="quote-product">{{ row.ipType }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn :label="t('staticIps.expiresAt')" min-width="110" align="right">
            <template #default="{ row }">
              <span class="tabular">{{ formatDate(row.expiry) }}</span>
            </template>
          </ElTableColumn>
        </ElTable>
        <p v-if="quote" class="quote-total"><span>{{ t('staticIps.renewal.total') }}</span><strong class="tabular">{{ formatYuan(expectedTotal, locale) }}</strong></p>
      </template>
      <p v-else-if="busy === 'loadingOrder'" role="status">{{ t('staticIps.renewal.loadingOrder') }}</p>
      <p v-if="busy === 'checking'" role="status">{{ t('staticIps.renewal.checking') }}</p>
    </div>

    <template #footer>
      <div class="dialog-actions">
        <ElButton :type="order && order.status !== 'pending' ? 'primary' : 'default'"
          :disabled="Boolean(busy)" @click="open = false">
          {{ t(order || completedResume ? 'staticIps.renewal.done' : 'staticIps.renewal.cancel') }}
        </ElButton>
        <ElButton v-if="order?.status === 'pending'" type="primary" :loading="busy === 'checking'" @click="recheck">
          {{ t('staticIps.check') }}
        </ElButton>
        <ElButton v-else-if="submission" type="primary" :loading="busy === 'submitting'" @click="submit">
          {{ t('staticIps.retryOriginal') }}
        </ElButton>
        <ElButton v-else-if="!order && intent.kind === 'order' && failure" type="primary" @click="loadOrder">
          {{ t('staticIps.retry') }}
        </ElButton>
        <ElButton v-else-if="!order && intent.kind === 'renew' && terms === undefined && failure" type="primary" @click="loadTerms">
          {{ t('staticIps.retry') }}
        </ElButton>
        <ElButton v-else-if="!order && intent.kind === 'renew' && quote" type="primary" @click="submit">
          {{ t('staticIps.renewal.confirm') }}
        </ElButton>
        <ElButton v-else-if="!order && intent.kind === 'renew' && term" type="primary" :loading="busy === 'quote'" @click="refreshQuote">
          {{ t('staticIps.renewal.getQuote') }}
        </ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<style scoped>
.renewal-body { display: grid; gap: var(--space-4); min-width: 0; }
.renewal-intro { color: var(--color-text-secondary); }
.renewal-terms {
  display: flex;
  gap: var(--space-3);
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  font-weight: var(--font-weight-semibold);
}
.term-options { gap: var(--space-2) 0; }
.renewal-quote {
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  font-size: var(--font-size-xs);
}
.quote-endpoint { color: var(--color-text); font-size: var(--font-size-md); font-weight: var(--font-weight-semibold); }
.quote-product {
  display: inline-block;
  margin-inline-start: var(--space-1);
  padding-inline: var(--space-1);
  border-radius: var(--space-1);
  background: var(--color-success-bg);
  color: var(--color-success-text);
  font-size: var(--font-size-xs);
}
.quote-total {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: var(--space-3);
  font-weight: var(--font-weight-semibold);
}
.quote-total strong { color: var(--color-text); font-size: var(--font-size-title); }
.dialog-actions { display: flex; flex-wrap: wrap; gap: var(--space-2); justify-content: flex-end; }
.dialog-actions button { margin: 0; }
</style>

<style>
.renewal-dialog {
  --el-dialog-border-radius: var(--radius-card);
  --el-dialog-padding-primary: var(--space-5);
  --el-dialog-box-shadow: var(--shadow-overlay);
}
</style>
