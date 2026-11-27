<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElInput } from 'element-plus'
import { formatYuan } from '@/shared/lib/money'
import { useCommissionTransfer } from './useCommissionTransfer'

const props = defineProps<{
  tenantId: string
  availableMinor?: bigint
}>()

const emit = defineEmits<{
  refresh: []
}>()

const { t, locale } = useI18n()
const amountInputId = useId()
const fieldErrorId = useId()
const balanceUnavailableId = useId()

const { amount, busy, pendingAmount, notice, submit, retry } = useCommissionTransfer(props, () => emit('refresh'))

const fieldErrorMessage = computed(() => {
  if (notice.value?.kind === 'invalid') return t('users.commissions.transfer.invalid')
  if (notice.value?.kind === 'exceeds') return t('users.commissions.transfer.exceeds')
  return undefined
})

const amountDescribedBy = computed(() => {
  const ids: string[] = []
  if (fieldErrorMessage.value) ids.push(fieldErrorId)
  if (props.availableMinor === undefined) ids.push(balanceUnavailableId)
  return ids.length > 0 ? ids.join(' ') : undefined
})

const alertNotice = computed<
  { type: 'success' | 'warning' | 'error', title: string, description: string, role: 'status' | 'alert' } | undefined
>(() => {
  const current = notice.value
  if (!current || current.kind === 'invalid' || current.kind === 'exceeds') return undefined
  switch (current.kind) {
    case 'success':
      return {
        type: 'success',
        title: t('users.commissions.transfer.success'),
        description: t('users.commissions.transfer.successDetail', {
          amount: formatYuan(current.amountMinor, locale.value),
        }),
        role: 'status',
      }
    case 'uncertain':
      return {
        type: 'warning',
        title: t('users.commissions.transfer.uncertainTitle'),
        description: t('users.commissions.transfer.uncertainDetail'),
        role: 'alert',
      }
    case 'storage':
      return {
        type: 'warning',
        title: t('users.commissions.transfer.storageTitle'),
        description: t('users.commissions.transfer.storageDetail'),
        role: 'alert',
      }
    case 'insufficient':
      return {
        type: 'error',
        title: t('users.commissions.transfer.insufficientTitle'),
        description: t('users.commissions.transfer.insufficientDetail'),
        role: 'alert',
      }
    case 'rejected':
      return {
        type: 'error',
        title: t('users.commissions.transfer.rejectedTitle'),
        description: t('users.commissions.transfer.rejectedDetail'),
        role: 'alert',
      }
  }
})
</script>

<template>
  <div class="commission-transfer">
    <div v-if="pendingAmount !== undefined" class="commission-transfer__pending">
      <div class="commission-transfer__pending-info">
        <span class="commission-transfer__label">{{ t('users.commissions.transfer.pendingLabel') }}</span>
        <strong class="commission-transfer__pending-amount tabular">{{ formatYuan(pendingAmount, locale) }}</strong>
      </div>
      <div class="commission-transfer__actions">
        <ElButton
          class="commission-transfer__btn"
          native-type="button"
          type="primary"
          :loading="busy"
          :disabled="busy"
          @click="retry"
        >
          {{ t('users.commissions.transfer.retry') }}
        </ElButton>
      </div>
      <p class="commission-transfer__hint">
        {{ t('users.commissions.transfer.pendingHint') }}
      </p>
    </div>

    <form v-else class="commission-transfer__form" novalidate @submit.prevent="submit">
      <div class="commission-transfer__field">
        <label :for="amountInputId" class="commission-transfer__label">
          {{ t('users.commissions.transfer.amountLabel') }}
        </label>
        <div class="commission-transfer__controls">
          <ElInput
            :id="amountInputId"
            v-model="amount"
            class="commission-transfer__input tabular"
            :placeholder="t('users.commissions.transfer.amountPlaceholder')"
            :disabled="busy"
            :aria-invalid="fieldErrorMessage ? 'true' : undefined"
            :aria-describedby="amountDescribedBy"
            inputmode="decimal"
            autocomplete="off"
            spellcheck="false"
          />
          <div class="commission-transfer__actions">
            <ElButton
              class="commission-transfer__btn"
              native-type="submit"
              type="primary"
              :loading="busy"
              :disabled="busy || props.availableMinor === undefined"
            >
              {{ t('users.commissions.transfer.submit') }}
            </ElButton>
          </div>
        </div>
        <p v-if="fieldErrorMessage" :id="fieldErrorId" class="commission-transfer__error" role="alert">
          {{ fieldErrorMessage }}
        </p>
        <p v-if="props.availableMinor === undefined" :id="balanceUnavailableId" class="commission-transfer__hint">
          {{ t('users.commissions.transfer.balanceUnavailable') }}
        </p>
      </div>
    </form>

    <ElAlert
      v-if="alertNotice"
      :type="alertNotice.type"
      :title="alertNotice.title"
      :description="alertNotice.description"
      :role="alertNotice.role"
      :closable="false"
      show-icon
    />
  </div>
</template>

<style scoped>
.commission-transfer {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-3);
  padding: var(--space-4);
  border: var(--border-width) solid var(--el-border-color-lighter);
  border-radius: var(--el-border-radius-base);
  background: var(--el-fill-color-lighter);
  min-width: 0;
}

.commission-transfer__form,
.commission-transfer__field {
  display: grid;
  gap: var(--space-2);
  min-width: 0;
}

.commission-transfer__label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.commission-transfer__controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}

.commission-transfer__input {
  flex: 1 1 12rem;
  min-width: 0;
  max-width: 16rem;
}

.commission-transfer__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  flex-shrink: 0;
}

.commission-transfer__btn {
  margin-left: 0;
}

.commission-transfer__pending {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2) var(--space-4);
  min-width: 0;
}

.commission-transfer__pending-info {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: var(--space-2);
  min-width: 0;
}

.commission-transfer__pending-amount {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--color-text);
}

.commission-transfer__hint {
  flex-basis: 100%;
  margin: 0;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.commission-transfer__error {
  margin: 0;
  color: var(--color-danger-text);
  font-size: var(--font-size-xs);
}

@media (max-width: 600px) {
  .commission-transfer {
    padding: var(--space-3);
  }

  .commission-transfer__input {
    max-width: 100%;
  }
}
</style>
