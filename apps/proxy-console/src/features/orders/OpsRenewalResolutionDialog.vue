<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCheckbox, ElForm, ElFormItem, ElInput, ElMessage, ElRadio, ElRadioGroup, ElTag } from 'element-plus'
import OpsDialog from '@/shared/ui/OpsDialog.vue'
import { ApiError } from '@/api/client'
import type { OpsPendingOrder } from '@/api/opsOrders'
import { resolveOpsStaticRenewal } from '@/api/opsStaticRenewals'
import type { OrderItem } from '@/api/orders'
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
const selectedId = ref('')
const evidence = ref('')
const pending = ref(false)
const failure = shallowRef<unknown>()
const canBill = computed(() => hasOperatorPermission('manage_billing'))
const closing = computed(() => mode.value === 'close')
const needsConfirmation = computed(() => closing.value && !props.record.canClose)
const encoder = new TextEncoder()

function isPending(item: OrderItem) {
  return item.outcome === 'pending' && item.supplierItemId !== undefined && props.record.pendingRenewalItems.includes(item.supplierItemId)
}

const selected = computed(() => props.record.order.items.find(item => item.supplierItemId === selectedId.value && isPending(item)))

watch(open, value => {
  if (!value) return
  mode.value = 'sync'
  confirmedFailure.value = false
  selectedId.value = ''
  evidence.value = ''
  failure.value = undefined
}, { immediate: true })

const validation = computed(() => {
  if (!canBill.value) return 'orders.ops.permission'
  if (!evidence.value.trim()) return 'orders.ops.missingEvidence'
  if (encoder.encode(evidence.value.trim()).length > 500) return 'orders.renewalOps.evidenceTooLong'
  if (!selected.value) return 'orders.renewalOps.selectItem'
  if (needsConfirmation.value && !confirmedFailure.value) return 'orders.renewalOps.closeBlocked'
  return ''
})

function resolutionError(error: unknown) {
  if (error instanceof ApiError) {
    if (error.reason === 'WALLET_ORDER_STATE' || error.reason === 'WALLET_RECORD_NOT_FOUND') return t('orders.ops.orderState')
    if (error.reason === 'STATIC_SUPPLIER_UNAVAILABLE') return t('orders.ops.supplierUnavailable')
  }
  return errorMessage(error)
}

async function submit() {
  const supplierItemId = selected.value?.supplierItemId
  if (!supplierItemId || pending.value || validation.value) return
  pending.value = true
  failure.value = undefined
  const action = !closing.value ? 'sync' : needsConfirmation.value ? 'confirm_failure' : 'close_unknown'
  try {
    const result = await resolveOpsStaticRenewal(props.record, { supplierItemId, action, evidence: evidence.value.trim() })
    const outcome = result.items.find(item => item.supplierItemId === supplierItemId)?.outcome
    if (outcome === 'succeeded') ElMessage.success(t('orders.renewalOps.succeeded'))
    else if (outcome === 'failed') ElMessage.success(t('orders.renewalOps.refunded'))
    else ElMessage.info(t('orders.renewalOps.stillPending'))
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
  <OpsDialog v-model="open" :title="t('orders.ops.verifyTitle')"
    width="min(var(--ops-resolve-width), calc(100vw - var(--space-6)))"
    :close-on-click-modal="!pending" :close-on-press-escape="!pending" :show-close="!pending">
    <dl class="resolve-summary resolve-heading">
      <div><dt>{{ t('orders.ops.order') }}</dt><dd><span class="mono">{{ record.order.id }}</span> · {{ t('orders.kinds.static_renewal') }}</dd></div>
      <div><dt>{{ t('orders.ops.tenant') }}</dt><dd>{{ record.tenant.name }} · {{ record.tenant.account }}</dd></div>
      <div><dt>{{ t('orders.placedAt') }}</dt><dd>{{ dateTime(record.order.createdAt, 'short') }} · {{ t('orders.ops.waitedValue', { duration: waited }) }}</dd></div>
      <div><dt>{{ t('orders.ops.stage') }}</dt><dd><ElTag :type="record.canClose ? 'danger' : 'warning'" size="small" disable-transitions>{{ t(record.canClose ? 'orders.ops.closable' : 'orders.ops.review') }}</ElTag></dd></div>
    </dl>
    <ElForm label-position="top" class="resolve-form" :disabled="pending || !canBill" @submit.prevent="submit">
      <ElFormItem :label="t('orders.renewalOps.items')">
        <ElRadioGroup v-model="selectedId" :aria-label="t('orders.renewalOps.items')" class="renewal-items">
          <table class="resolve-lines">
            <tbody>
              <tr v-for="item in record.order.items" :key="item.position">
                <td>
                  <span class="mono">{{ itemDescription(item) }}</span>
                  <span class="resolve-hint">{{ t('orders.renewalOps.itemDetails', { id: item.supplierItemId, amount: money(item.unitMinor) }) }}</span>
                </td>
                <td class="renewal-status">
                  <ElRadio v-if="isPending(item)" :value="item.supplierItemId" :aria-label="itemDescription(item)">{{ t('orders.status.pending') }}</ElRadio>
                  <ElTag v-else size="small" disable-transitions :type="item.outcome === 'succeeded' ? 'success' : item.outcome === 'failed' ? 'danger' : 'info'">
                    {{ t(item.outcome === 'succeeded' ? 'orders.renewalOps.successStatus' : item.outcome === 'failed' ? 'orders.renewalOps.refundedStatus' : 'orders.status.pending') }}
                  </ElTag>
                </td>
              </tr>
            </tbody>
          </table>
        </ElRadioGroup>
        <p class="resolve-hint resolve-delivery-hint">{{ t('orders.renewalOps.itemHint') }}</p>
      </ElFormItem>
      <ElFormItem :label="t('orders.ops.mode')">
        <ElRadioGroup v-model="mode" class="resolve-mode" :aria-label="t('orders.ops.mode')">
          <ElRadio value="sync">{{ t('orders.renewalOps.sync') }}</ElRadio>
          <ElRadio value="close">{{ t('orders.renewalOps.close') }}</ElRadio>
        </ElRadioGroup>
      </ElFormItem>
      <ElAlert v-if="needsConfirmation" type="warning" :closable="false" show-icon class="resolve-alert" :title="t('orders.ops.earlyTitle')">
        {{ t('orders.renewalOps.earlyDescription') }}
        <ElCheckbox v-model="confirmedFailure" :disabled="pending || !canBill">{{ t('orders.renewalOps.confirmFailure') }}</ElCheckbox>
      </ElAlert>
      <ElFormItem :label="t('orders.ops.evidence')" for="renewal-evidence" required>
        <ElInput id="renewal-evidence" v-model="evidence" type="textarea" :rows="3" maxlength="500" show-word-limit
          :placeholder="t('orders.ops.evidencePlaceholder')" />
      </ElFormItem>
    </ElForm>
    <dl v-if="closing && selected" class="resolve-summary resolve-refund">
      <div><dt>{{ t('orders.ops.refund') }}</dt><dd>{{ money(selected.unitMinor) }}</dd></div>
    </dl>
    <ElAlert v-if="failure" type="error" :title="resolutionError(failure)" :closable="false" show-icon class="resolve-alert" />
    <template #footer>
      <div class="resolve-actions">
        <span v-if="validation" class="resolve-error">{{ t(validation) }}</span>
        <ElButton class="ops-dialog-action" :disabled="pending" @click="open = false">{{ t('auth.cancel') }}</ElButton>
        <ElButton class="ops-dialog-action" type="primary" :disabled="Boolean(validation)" :loading="pending" @click="submit">{{ t(closing ? 'orders.ops.confirmClose' : 'orders.ops.save') }}</ElButton>
      </div>
    </template>
  </OpsDialog>
</template>

<style scoped src="./opsResolutionDialog.css"></style>
<style scoped>
.renewal-items { display: block; width: 100%; }
.renewal-status { width: 1%; padding-inline-start: var(--space-4); text-align: end; white-space: nowrap; }
.renewal-status .el-radio { margin: 0; }
</style>
