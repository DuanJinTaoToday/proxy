<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElInputNumber, ElResult } from 'element-plus'
import { ApiError } from '@/api/client'
import { quoteWalletRecharge, rechargeWallet, type OpsTenant } from '@/api/opsTenants'
import { currentOperator, hasOperatorPermission } from '@/features/auth'
import { formatYuan } from '@/shared/lib/money'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const props = defineProps<{ tenant: OpsTenant }>()
const emit = defineEmits<{ recharged: [balance: bigint] }>()
const open = defineModel<boolean>({ required: true })
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
type Quote = Awaited<ReturnType<typeof quoteWalletRecharge>>
type Receipt = Awaited<ReturnType<typeof rechargeWallet>>
type Step = { kind: 'form' } | { kind: 'confirm', quote: Quote } | { kind: 'done', receipt: Receipt }
const step = shallowRef<Step>({ kind: 'form' })
const amount = ref<number>()
const reason = ref('')
const pending = ref(false)
const uncertain = ref(false)
const failure = shallowRef<unknown>()
const cents = computed(() => Math.round((amount.value ?? 0) * 100))
const validAmount = computed(() => Number.isSafeInteger(cents.value) && cents.value >= 1 && cents.value <= 10_000_000)
const validForm = computed(() => validAmount.value && Boolean(reason.value.trim()))
const format = (minor: bigint) => formatYuan(minor, locale.value)
const mayClose = computed(() => !pending.value && !uncertain.value)

watch(open, value => {
  if (value && !uncertain.value) {
    step.value = { kind: 'form' }
    amount.value = undefined
    reason.value = ''
    failure.value = undefined
  }
})

async function prepare() {
  if (!validForm.value || pending.value || !hasOperatorPermission('manage_billing')) return
  pending.value = true
  failure.value = undefined
  try {
    const quote = await quoteWalletRecharge(props.tenant.id, {
      key: crypto.randomUUID(), reason: reason.value.trim(), amountMinor: BigInt(cents.value),
    })
    step.value = { kind: 'confirm', quote }
  } catch (error) {
    failure.value = error
  } finally {
    pending.value = false
  }
}

function edit() {
  failure.value = undefined
  step.value = { kind: 'form' }
}

async function submit() {
  if (step.value.kind !== 'confirm' || pending.value || !hasOperatorPermission('manage_billing')) return
  const { quote } = step.value
  pending.value = true
  failure.value = undefined
  try {
    const receipt = await rechargeWallet(quote.tenant.id, quote)
    uncertain.value = false
    step.value = { kind: 'done', receipt }
    emit('recharged', receipt.afterMinor)
  } catch (error) {
    failure.value = error
    // The service replays a committed request before checking quote expiry or balance.
    // Keep the confirmation token and request key if the response may have been lost.
    uncertain.value = !(error instanceof ApiError && error.status >= 400 && error.status < 500)
    if (error instanceof ApiError && ['WALLET_BALANCE_CHANGED', 'WALLET_QUOTE_EXPIRED'].includes(error.reason)) {
      step.value = { kind: 'form' }
    }
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <ElDialog v-model="open" :title="step.kind === 'confirm' ? t('wallet.opsReviewTitle') : t('wallet.opsRecharge')"
    width="min(var(--ops-dialog-width), calc(100vw - var(--space-6)))" align-center
    class="recharge-dialog" :close-on-click-modal="mayClose"
    :close-on-press-escape="mayClose" :show-close="mayClose">
    <ElAlert v-if="failure" type="error" :title="uncertain ? t('wallet.opsUncertain') : errorMessage(failure)" :closable="false" show-icon class="recharge-error" />
    <ElForm v-if="step.kind === 'form'" label-position="top" @submit.prevent="prepare">
      <ElFormItem :label="t('wallet.opsTenant')"><span>{{ tenant.name }} · <span class="mono">{{ tenant.account }}</span></span></ElFormItem>
      <ElFormItem :label="t('wallet.opsAmount')" for="recharge-amount" required>
        <ElInputNumber id="recharge-amount" v-model="amount" :min="0.01" :max="100000" :precision="2" :controls="false" placeholder="0.00" class="recharge-amount" />
      </ElFormItem>
      <ElFormItem :label="t('wallet.opsReason')" for="recharge-reason" required>
        <ElInput id="recharge-reason" v-model="reason" type="textarea" :rows="2" maxlength="200" show-word-limit :placeholder="t('wallet.opsReasonExample')" />
      </ElFormItem>
      <p class="recharge-hint">{{ t('wallet.opsReasonHint') }}</p>
    </ElForm>

    <template v-else-if="step.kind === 'confirm'">
      <dl class="recharge-summary">
        <div><dt>{{ t('wallet.opsTenantName') }}</dt><dd>{{ step.quote.tenant.name }}</dd></div>
        <div><dt>{{ t('wallet.opsPrimaryAccount') }}</dt><dd class="mono">{{ step.quote.tenant.account }}</dd></div>
        <div><dt>{{ t('wallet.opsReason') }}</dt><dd>{{ step.quote.reason }}</dd></div>
        <div><dt>{{ t('wallet.opsCurrent') }}</dt><dd>{{ format(step.quote.tenant.availableMinor) }}</dd></div>
        <div><dt>{{ t('wallet.opsAfter') }}</dt><dd>{{ format(step.quote.expectedAfterMinor) }}</dd></div>
        <div class="recharge-summary__amount"><dt>{{ t('wallet.opsAmountSummary') }}</dt><dd>{{ format(step.quote.amountMinor) }}</dd></div>
      </dl>
      <p class="recharge-hint">{{ t('wallet.opsReviewHint', { operator: currentOperator?.username }) }}</p>
    </template>

    <ElResult v-else icon="success" :title="t('wallet.opsDone')"
      :sub-title="t('wallet.opsDoneBalance', { tenant: tenant.name, amount: format(step.receipt.afterMinor) })">
      <template #extra>
        <dl class="recharge-summary recharge-summary--receipt">
          <div><dt>{{ t('wallet.opsEntry') }}</dt><dd class="mono">{{ step.receipt.id }}</dd></div>
          <div><dt>{{ t('wallet.opsOperator') }}</dt><dd>{{ step.receipt.operatorName }}</dd></div>
          <div><dt>{{ t('wallet.opsEntryTime') }}</dt><dd>{{ shanghaiDateTime(step.receipt.createdAt, locale) }}</dd></div>
          <div><dt>{{ t('wallet.opsChange') }}</dt><dd>{{ format(step.receipt.afterMinor - step.receipt.amountMinor) }} → {{ format(step.receipt.afterMinor) }}</dd></div>
        </dl>
      </template>
    </ElResult>

    <template #footer>
      <div class="recharge-actions">
        <template v-if="step.kind === 'form'">
          <span v-if="amount !== undefined && !validForm" class="recharge-actions__error">{{ t(validAmount ? 'wallet.opsMissingReason' : 'wallet.opsMissingAmount') }}</span>
          <ElButton :disabled="pending" @click="open = false">{{ t('auth.cancel') }}</ElButton>
          <ElButton type="primary" :disabled="!validForm" :loading="pending" @click="prepare">{{ t('wallet.opsNext') }}</ElButton>
        </template>
        <template v-else-if="step.kind === 'confirm'">
          <ElButton v-if="!uncertain" :disabled="pending" @click="edit">{{ t('wallet.opsEdit') }}</ElButton>
          <ElButton type="primary" :loading="pending" @click="submit">
            {{ uncertain ? t('wallet.opsRetry') : t('wallet.opsConfirm', { amount: format(step.quote.amountMinor) }) }}
          </ElButton>
        </template>
        <ElButton v-else type="primary" @click="open = false">{{ t('wallet.opsFinish') }}</ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<style scoped>
.recharge-error { margin-block-end: var(--space-4); }
.recharge-dialog {
  --el-dialog-padding-primary: var(--space-5);
  --el-dialog-border-radius: var(--ops-card-radius);
  --el-border-radius-base: var(--ops-control-radius);
}
.recharge-amount { width: 100%; }
.recharge-hint { margin-block-start: var(--space-1); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.recharge-summary {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  margin: 0;
  padding-block-start: var(--space-3);
  border-top: var(--border-width) solid var(--color-border);
  font-size: var(--font-size-sm);
}
.recharge-summary div { display: flex; justify-content: space-between; gap: var(--space-3); }
.recharge-summary dt { color: var(--color-text-secondary); flex: none; }
.recharge-summary dd { margin: 0; overflow-wrap: anywhere; font-weight: var(--font-weight-semibold); font-variant-numeric: tabular-nums; }
.recharge-summary__amount dt { align-self: flex-end; font-weight: var(--font-weight-semibold); color: var(--color-text); }
.recharge-summary__amount dd { font-size: var(--font-size-title); }
.recharge-summary--receipt { margin-block-start: var(--space-4); }
.recharge-actions { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; column-gap: var(--space-2); row-gap: var(--space-2); }
.recharge-actions .el-button { margin: 0; }
.recharge-actions__error { color: var(--color-danger-text); font-size: var(--font-size-sm); }
</style>
