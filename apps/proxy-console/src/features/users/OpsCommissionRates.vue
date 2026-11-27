<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElInput, ElMessage, ElSkeleton, ElTag } from 'element-plus'
import { getOpsCommissionRates, updateOpsCommissionRates, type CommissionRates } from '@/api/commissions'
import { hasOperatorPermission } from '@/features/auth'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { formatBpsInput, formatBpsPercent, parsePercentToBps, type RateParseError } from './commissionFormat'

type RatesState =
  | { kind: 'denied' }
  | { kind: 'loading' }
  | { kind: 'error', error: unknown }
  | { kind: 'ready', data: CommissionRates }

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const headingId = useId()
const firstRateId = useId()
const repeatRateId = useId()
const firstErrorId = useId()
const repeatErrorId = useId()

const canRead = computed(() => hasOperatorPermission('read_operations'))
const canEdit = computed(() => canRead.value && hasOperatorPermission('manage_billing'))

const state = shallowRef<RatesState>({ kind: 'loading' })
const firstDraft = ref('')
const repeatDraft = ref('')
const firstError = ref<RateParseError | undefined>(undefined)
const repeatError = ref<RateParseError | undefined>(undefined)
const saving = ref(false)
const saveError = shallowRef<unknown>(undefined)

let activeLoad: AbortController | undefined
let activeSave: AbortController | undefined
let loadSeq = 0

onScopeDispose(() => {
  activeLoad?.abort()
  activeSave?.abort()
})

function syncDrafts(data: CommissionRates) {
  firstDraft.value = formatBpsInput(data.firstPurchaseBps)
  repeatDraft.value = formatBpsInput(data.repeatPurchaseBps)
  firstError.value = undefined
  repeatError.value = undefined
}

async function load() {
  activeLoad?.abort()
  activeLoad = undefined
  const seq = ++loadSeq
  if (!canRead.value) {
    state.value = { kind: 'denied' }
    return
  }

  const controller = new AbortController()
  activeLoad = controller
  state.value = { kind: 'loading' }
  saveError.value = undefined
  try {
    const data = await getOpsCommissionRates(controller.signal)
    if (controller.signal.aborted || seq !== loadSeq || !canRead.value) return
    state.value = { kind: 'ready', data }
    syncDrafts(data)
  } catch (error) {
    if (controller.signal.aborted || seq !== loadSeq || !canRead.value) return
    state.value = { kind: 'error', error }
  }
}

watch(
  canRead,
  readable => {
    if (!readable) {
      activeLoad?.abort()
      activeSave?.abort()
      activeLoad = undefined
      activeSave = undefined
      saving.value = false
      saveError.value = undefined
      firstError.value = undefined
      repeatError.value = undefined
      state.value = { kind: 'denied' }
      return
    }
    void load()
  },
  { immediate: true },
)

watch(canEdit, editable => {
  if (!editable) {
    activeSave?.abort()
    activeSave = undefined
    saving.value = false
    saveError.value = undefined
    firstError.value = undefined
    repeatError.value = undefined
    if (state.value.kind === 'ready') {
      syncDrafts(state.value.data)
    }
  }
})

function onFirstInput(value: string) {
  firstDraft.value = value
  if (firstError.value) firstError.value = undefined
  if (saveError.value) saveError.value = undefined
}

function onRepeatInput(value: string) {
  repeatDraft.value = value
  if (repeatError.value) repeatError.value = undefined
  if (saveError.value) saveError.value = undefined
}

async function save() {
  if (saving.value || !canEdit.value || state.value.kind !== 'ready') return

  const first = parsePercentToBps(firstDraft.value)
  const repeat = parsePercentToBps(repeatDraft.value)
  firstError.value = first.ok ? undefined : first.error
  repeatError.value = repeat.ok ? undefined : repeat.error
  saveError.value = undefined
  if (!first.ok || !repeat.ok) return

  saving.value = true
  const controller = new AbortController()
  activeSave = controller
  try {
    const updated = await updateOpsCommissionRates(
      { firstPurchaseBps: first.bps, repeatPurchaseBps: repeat.bps },
      controller.signal,
    )
    if (controller.signal.aborted || !canRead.value) return
    state.value = { kind: 'ready', data: updated }
    syncDrafts(updated)
    ElMessage.success(t('users.commissions.rates.saved'))
  } catch (error) {
    if (controller.signal.aborted) return
    saveError.value = error
  } finally {
    if (activeSave === controller) {
      activeSave = undefined
      saving.value = false
    }
  }
}
</script>

<template>
  <ElCard class="ops-commission-rates" role="region" shadow="never" :aria-labelledby="headingId">
    <div class="ops-commission-rates__body">
      <div class="ops-commission-rates__header">
        <h2 :id="headingId" class="ops-commission-rates__heading">{{ t('users.commissions.rates.title') }}</h2>
        <div v-if="state.kind === 'ready'" class="ops-commission-rates__meta">
          <ElTag type="info" effect="light" size="small">
            {{ t('users.commissions.rateVersionTag', { version: state.data.version }) }}
          </ElTag>
          <span class="ops-commission-rates__updated">
            {{ t('users.commissions.rates.updatedAt', { date: shanghaiDateTime(state.data.updatedAt, locale, 'short') }) }}
          </span>
        </div>
      </div>

      <ElAlert
        v-if="state.kind === 'denied'"
        type="warning"
        :title="t('errors.ACCESS_DENIED')"
        :closable="false"
        show-icon
      />

      <div v-else-if="state.kind === 'loading'" role="status" :aria-label="t('users.commissions.rates.loading')">
        <ElSkeleton animated :rows="2" />
      </div>

      <div v-else-if="state.kind === 'error'" class="ops-commission-rates__error">
        <ElAlert
          type="error"
          :title="t('users.commissions.rates.loadFailed')"
          :description="errorMessage(state.error)"
          :closable="false"
          show-icon
        />
        <ElButton size="small" @click="load">{{ t('auth.retry') }}</ElButton>
      </div>

      <template v-else-if="state.kind === 'ready'">
        <ElAlert
          type="info"
          :title="t('users.commissions.rates.scopeNotice')"
          :closable="false"
          show-icon
        />

        <form v-if="canEdit" class="ops-commission-rates__form" novalidate @submit.prevent="save">
          <div class="ops-commission-rates__inputs">
            <div class="ops-commission-rates__field">
              <label :for="firstRateId" class="ops-commission-rates__label">
                {{ t('users.commissions.rates.firstPurchase') }}
              </label>
              <ElInput
                :id="firstRateId"
                :model-value="firstDraft"
                class="tabular"
                inputmode="decimal"
                autocomplete="off"
                :disabled="saving"
                :aria-invalid="firstError ? 'true' : undefined"
                :aria-describedby="firstError ? firstErrorId : undefined"
                @update:model-value="onFirstInput"
              >
                <template #append>%</template>
              </ElInput>
              <p v-if="firstError" :id="firstErrorId" class="ops-commission-rates__field-error" role="alert">
                {{ t(`users.commissions.rates.validation.${firstError}`) }}
              </p>
            </div>

            <div class="ops-commission-rates__field">
              <label :for="repeatRateId" class="ops-commission-rates__label">
                {{ t('users.commissions.rates.repeatPurchase') }}
              </label>
              <ElInput
                :id="repeatRateId"
                :model-value="repeatDraft"
                class="tabular"
                inputmode="decimal"
                autocomplete="off"
                :disabled="saving"
                :aria-invalid="repeatError ? 'true' : undefined"
                :aria-describedby="repeatError ? repeatErrorId : undefined"
                @update:model-value="onRepeatInput"
              >
                <template #append>%</template>
              </ElInput>
              <p v-if="repeatError" :id="repeatErrorId" class="ops-commission-rates__field-error" role="alert">
                {{ t(`users.commissions.rates.validation.${repeatError}`) }}
              </p>
            </div>
          </div>

          <ElAlert
            v-if="saveError"
            type="error"
            :title="t('users.commissions.rates.saveFailed')"
            :description="errorMessage(saveError)"
            :closable="false"
            show-icon
          />

          <div class="ops-commission-rates__actions">
            <ElButton native-type="submit" type="primary" :loading="saving" :disabled="saving">
              {{ t('users.commissions.rates.save') }}
            </ElButton>
            <span class="ops-commission-rates__hint">{{ t('users.commissions.rates.precisionHint') }}</span>
          </div>
        </form>

        <template v-else>
          <dl class="ops-commission-rates__readonly">
            <div class="ops-commission-rates__readonly-item">
              <dt>{{ t('users.commissions.rates.firstPurchase') }}</dt>
              <dd class="tabular">{{ formatBpsPercent(state.data.firstPurchaseBps) }}</dd>
            </div>
            <div class="ops-commission-rates__readonly-item">
              <dt>{{ t('users.commissions.rates.repeatPurchase') }}</dt>
              <dd class="tabular">{{ formatBpsPercent(state.data.repeatPurchaseBps) }}</dd>
            </div>
          </dl>
          <p class="ops-commission-rates__permission">{{ t('users.commissions.rates.billingPermissionOnly') }}</p>
        </template>
      </template>
    </div>
  </ElCard>
</template>

<style scoped>
.ops-commission-rates {
  --el-card-padding: var(--space-5);
  border-radius: var(--dashboard-card-radius);
  min-width: 0;
  max-width: 100%;
}

.ops-commission-rates__body {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-4);
  min-width: 0;
}

.ops-commission-rates__header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2) var(--space-4);
}

.ops-commission-rates__heading {
  margin: 0;
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
}

.ops-commission-rates__meta {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
}

.ops-commission-rates__updated {
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.ops-commission-rates__error {
  display: grid;
  justify-items: start;
  gap: var(--space-3);
}

.ops-commission-rates__form {
  display: grid;
  gap: var(--space-4);
}

.ops-commission-rates__inputs {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr));
  gap: var(--space-4);
}

.ops-commission-rates__field {
  display: grid;
  gap: var(--space-2);
  align-content: start;
  min-width: 0;
}

.ops-commission-rates__label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.ops-commission-rates__field-error {
  margin: 0;
  color: var(--color-danger-text);
  font-size: var(--font-size-xs);
}

.ops-commission-rates__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-3);
}

.ops-commission-rates__hint,
.ops-commission-rates__permission {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.ops-commission-rates__readonly {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr));
  gap: var(--space-4);
  margin: 0;
}

.ops-commission-rates__readonly-item {
  display: grid;
  gap: var(--space-1);
}

.ops-commission-rates__readonly dt {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
}

.ops-commission-rates__readonly dd {
  margin: 0;
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
}
</style>
