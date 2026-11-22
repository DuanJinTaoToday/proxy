<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCheckbox, ElDialog, ElForm, ElFormItem, ElInput, ElInputNumber, ElMessage, ElRadio, ElRadioGroup, ElTag } from 'element-plus'
import { ApiError } from '@/api/client'
import { resolveOpsStaticPurchase, type OpsPendingOrder } from '@/api/opsOrders'
import { hasOperatorPermission } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { useOrderFormatting } from './format'

const props = defineProps<{ record: OpsPendingOrder, waited: string }>()
const emit = defineEmits<{ changed: [] }>()
const open = defineModel<boolean>({ required: true })
const { t } = useI18n()
const { money, dateTime, itemDescription } = useOrderFormatting()
const errorMessage = useErrorMessage()
const mode = ref<'sync' | 'close'>('sync')
const confirmedFailure = ref(false)
const supplierOrderId = ref('')
const evidence = ref('')
const pending = ref(false)
const failure = shallowRef<unknown>()
const canBill = computed(() => hasOperatorPermission('manage_billing'))
const closeRemaining = computed(() => mode.value === 'close')
const needsConfirmation = computed(() => closeRemaining.value && !props.record.canClose)
const encoder = new TextEncoder()

watch(open, value => {
  if (!value) return
  mode.value = 'sync'
  confirmedFailure.value = false
  supplierOrderId.value = props.record.supplierOrderId
  evidence.value = ''
  failure.value = undefined
}, { immediate: true })

const validation = computed(() => {
  if (!canBill.value) return 'orders.ops.permission'
  const evidenceText = evidence.value.trim()
  if (!evidenceText) return 'orders.ops.missingEvidence'
  if (encoder.encode(evidenceText).length > 500 || encoder.encode(supplierOrderId.value.trim()).length > 128) return 'orders.ops.inputTooLong'
  if (needsConfirmation.value && !confirmedFailure.value) return 'orders.ops.closeBlocked'
  if (!closeRemaining.value && !supplierOrderId.value.trim() && !props.record.supplierOrderId) return 'orders.ops.missingSupplierOrder'
  return ''
})

function resolutionError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.reason === 'WALLET_ORDER_STATE') return t('orders.ops.orderState')
    if (error.reason === 'STATIC_SUPPLIER_UNAVAILABLE') return t('orders.ops.supplierUnavailable')
  }
  return errorMessage(error)
}

async function submit() {
  if (pending.value || validation.value) return
  pending.value = true
  failure.value = undefined
  try {
    const result = await resolveOpsStaticPurchase(props.record, {
      supplierOrderId: supplierOrderId.value.trim(),
      evidence: evidence.value.trim(),
      closeRemaining: closeRemaining.value,
      confirmedUndeliverable: needsConfirmation.value && confirmedFailure.value,
    })
    const released = result.refundedMinor - props.record.order.refundedMinor
    if (result.status === 'pending') ElMessage.success(t('orders.ops.synced'))
    else if (released > 0n) ElMessage.success(t('orders.ops.closedRefund', { amount: money(released) }))
    else ElMessage.success(t('orders.ops.completed'))
    open.value = false
    emit('changed')
  } catch (error) {
    failure.value = error
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <ElDialog v-model="open" :title="t('orders.ops.verifyTitle')" class="ops-dialog"
    width="min(var(--ops-resolve-width), calc(100vw - var(--space-6)))" align-center
    :close-on-click-modal="!pending" :close-on-press-escape="!pending" :show-close="!pending">
    <dl class="resolve-summary">
      <div><dt>{{ t('orders.ops.order') }}</dt><dd><span class="mono">{{ record.order.id }}</span> · {{ t(`orders.kinds.${record.order.kind}`) }}</dd></div>
      <div><dt>{{ t('orders.ops.tenant') }}</dt><dd>{{ record.tenant.name }} · {{ record.tenant.account }}</dd></div>
      <div><dt>{{ t('orders.placedAt') }}</dt><dd>{{ dateTime(record.order.createdAt) }} · {{ t('orders.ops.waitedValue', { duration: waited }) }}</dd></div>
      <div><dt>{{ t('orders.ops.stage') }}</dt><dd><ElTag :type="record.canClose ? 'danger' : 'warning'" size="small" disable-transitions>{{ t(record.canClose ? 'orders.ops.closable' : 'orders.ops.review') }}</ElTag></dd></div>
    </dl>
    <ElForm label-position="top" class="resolve-form" :disabled="pending || !canBill" @submit.prevent="submit">
      <ElFormItem :label="t('orders.ops.supplierOrder')" for="supplier-order">
        <ElInput id="supplier-order" v-model="supplierOrderId" :placeholder="t('orders.ops.supplierOrderPlaceholder')" maxlength="128" />
      </ElFormItem>
      <ElFormItem :label="t('orders.ops.delivered')">
        <table class="resolve-lines">
          <tbody>
            <tr v-for="item in record.order.items" :key="item.position">
              <td>{{ itemDescription(item) }}<span class="resolve-hint">{{ t('orders.ops.unitPrice', { amount: money(item.unitMinor) }) }}</span></td>
              <td class="resolve-count">
                <ElInputNumber :model-value="item.deliveredCount" :aria-label="itemDescription(item)" size="small" controls-position="right" disabled />
                <span>/ {{ item.quantity }}</span>
              </td>
            </tr>
          </tbody>
        </table>
        <p class="resolve-hint">{{ t('orders.ops.deliveredHint') }}</p>
      </ElFormItem>
      <ElFormItem :label="t('orders.ops.mode')">
        <ElRadioGroup v-model="mode" class="resolve-mode">
          <ElRadio value="sync">{{ t('orders.ops.sync') }}</ElRadio>
          <ElRadio value="close">{{ t('orders.ops.close') }}</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElAlert v-if="needsConfirmation" type="warning" :closable="false" show-icon class="resolve-alert" :title="t('orders.ops.earlyTitle')">
        {{ t('orders.ops.earlyDescription') }}
        <ElCheckbox v-model="confirmedFailure">{{ t('orders.ops.confirmFailure') }}</ElCheckbox>
      </ElAlert>
      <ElFormItem :label="t('orders.ops.evidence')" for="resolution-evidence" required>
        <ElInput id="resolution-evidence" v-model="evidence" type="textarea" :rows="3" maxlength="500" show-word-limit
          :placeholder="t('orders.ops.evidencePlaceholder')" />
      </ElFormItem>
    </ElForm>
    <dl v-if="closeRemaining" class="resolve-summary resolve-refund">
      <div><dt>{{ t('orders.ops.refund') }}</dt><dd>{{ money(record.order.unsettledMinor) }}</dd></div>
    </dl>
    <ElAlert v-if="failure" type="error" :title="resolutionError(failure)" :closable="false" show-icon class="resolve-alert" />
    <template #footer>
      <div class="resolve-actions">
        <span v-if="validation" class="resolve-error">{{ t(validation) }}</span>
        <ElButton :disabled="pending" @click="open = false">{{ t('auth.cancel') }}</ElButton>
        <ElButton type="primary" :disabled="Boolean(validation)" :loading="pending" @click="submit">{{ t(closeRemaining ? 'orders.ops.confirmClose' : 'orders.ops.save') }}</ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<style scoped src="./opsResolutionDialog.css"></style>
