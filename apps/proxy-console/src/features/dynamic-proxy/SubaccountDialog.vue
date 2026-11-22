<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElSwitch, type FormInstance, type FormRules } from 'element-plus'
import { Minus, Plus, Refresh, WarningFilled } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { createSubaccount, updateSubaccount, type Subaccount } from '@/api/subaccounts'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { bytesPerGB, formatGB, parseLimitGB } from '@/shared/lib/trafficUnits'

export type SubaccountAction = { kind: 'create' } | { kind: 'edit' | 'password' | 'status', account: Subaccount }

const props = defineProps<{ action: SubaccountAction, primaryAccount: string }>()
const emit = defineEmits<{ close: [], saved: [account: Subaccount], quotaReached: [] }>()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const account = props.action.kind === 'create' ? undefined : props.action.account
const form = ref<FormInstance>()
const model = reactive({
  name: '',
  password: '',
  remark: account?.remark ?? '',
  active: account?.active ?? true,
  limited: account?.limitBytes !== undefined,
  limitGB: account?.limitBytes === undefined ? '10' : formatGB(account.limitBytes, locale.value, false),
})
const submitting = ref(false)
const failure = ref<unknown>()
const fieldErrors = reactive<Partial<Record<'name' | 'password' | 'remark' | 'limitGB', string>>>({})
const errorFields: Record<string, keyof typeof fieldErrors> = {
  INVALID_SUBACCOUNT_NAME: 'name',
  SUBACCOUNT_NAME_EXISTS: 'name',
  INVALID_PROXY_PASSWORD: 'password',
  INVALID_SUBACCOUNT_REMARK: 'remark',
  INVALID_SUBACCOUNT_LIMIT: 'limitGB',
  SUBACCOUNT_LIMIT_BELOW_USAGE: 'limitGB',
}

const title = computed(() => {
  switch (props.action.kind) {
    case 'create': return t('subaccounts.createTitle')
    case 'edit': return t('subaccounts.editTitle')
    case 'password': return t('subaccounts.changePassword')
    case 'status': return t('subaccounts.disableTitle', { name: account?.name })
  }
})
const remarkByteLimit = 512
const encoder = new TextEncoder()
const remarkBytes = (value: string) => encoder.encode(value).length
const limitBytes = computed(() => model.limited ? parseLimitGB(model.limitGB) : undefined)
const limitStep = 5n * bytesPerGB
const minimumLimit = computed(() => {
  const current = account?.limitBytes ?? 0n
  const used = account?.usedBytes ?? 0n
  return current > used ? current : used
})
const stepDown = computed(() => {
  const value = limitBytes.value
  if (value === undefined || value <= minimumLimit.value) return
  const stepped = value - limitStep
  return formatGB(stepped > minimumLimit.value ? stepped : minimumLimit.value, locale.value, false)
})
const stepUp = computed(() => {
  const value = limitBytes.value
  if (value === undefined) return
  const stepped = value + limitStep
  const next = formatGB(stepped > minimumLimit.value ? stepped : minimumLimit.value, locale.value, false)
  if (parseLimitGB(next) !== undefined) return next
})
const rules = computed<FormRules<typeof model>>(() => ({
  name: [{ required: true, pattern: /^[a-z][a-z0-9]{3,7}$/, message: t('subaccounts.nameRule') }],
  password: [{ required: true, pattern: /^(?=.*[A-Za-z])(?=.*\d)[A-Za-z0-9]{6,12}$/, message: t('subaccounts.passwordRule') }],
  remark: [{ validator: (_rule, value: string, done) => {
    done(remarkBytes(value.trim()) > remarkByteLimit ? new Error(t('subaccounts.remarkRule')) : undefined)
  } }],
  limitGB: [{ validator: (_rule, _value, done) => {
    const limit = limitBytes.value
    if (!model.limited) return done()
    if (limit === undefined) return done(new Error(t('subaccounts.limitRule')))
    if (limit === account?.limitBytes) return done()
    if (account && limit < account.usedBytes) return done(new Error(t('subaccounts.belowUsage')))
    if (account?.limitBytes !== undefined && limit < account.limitBytes) return done(new Error(t('subaccounts.lowerLimit')))
    done()
  } }],
}))

function generatePassword() {
  const letters = 'abcdefghjkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ'
  const digits = '23456789'
  const chars = letters + digits
  const random = crypto.getRandomValues(new Uint32Array(10))
  const password = Array.from(random, (value, index) => {
    const alphabet = index === 0 ? letters : index === 1 ? digits : chars
    return alphabet[value % alphabet.length]
  })
  model.password = password.join('')
  fieldErrors.password = undefined
  void form.value?.validateField('password').catch(() => {})
}

function changeLimit(value: string | undefined) {
  if (value === undefined) return
  model.limitGB = value
  fieldErrors.limitGB = undefined
  void form.value?.validateField('limitGB').catch(() => {})
}

async function save() {
  if (submitting.value) return
  submitting.value = true
  failure.value = undefined
  for (const field of Object.values(errorFields)) fieldErrors[field] = undefined
  try {
    if (form.value && !await form.value.validate().catch(() => false)) return
    const { action } = props
    const remark = model.remark.trim()
    let saved: Subaccount
    switch (action.kind) {
      case 'create':
        saved = await createSubaccount({ name: model.name, password: model.password, remark, active: model.active, limitBytes: limitBytes.value })
        break
      case 'edit': {
        const changedRemark = remark !== action.account.remark ? remark : undefined
        const active = model.active !== action.account.active ? model.active : undefined
        const limit = limitBytes.value !== action.account.limitBytes ? { bytes: limitBytes.value } : undefined
        if (changedRemark === undefined && active === undefined && !limit) {
          emit('close')
          return
        }
        saved = await updateSubaccount(action.account.clientId, { kind: 'details', remark: changedRemark, active, limit })
        break
      }
      case 'password':
        saved = await updateSubaccount(action.account.clientId, { kind: 'password', password: model.password })
        break
      case 'status':
        saved = await updateSubaccount(action.account.clientId, { kind: 'details', active: false })
        break
    }
    emit('saved', saved)
  } catch (error) {
    const reason = error instanceof ApiError ? error.reason : ''
    const field = errorFields[reason]
    if (field) fieldErrors[field] = reason
    else failure.value = error
    if (reason === 'SUBACCOUNT_LIMIT_REACHED') emit('quotaReached')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <ElDialog
    :model-value="true"
    :title="title"
    footer-class="subaccount-dialog-footer"
    :style="{
      '--el-dialog-padding-primary': 'var(--space-5)',
      '--el-dialog-border-radius': 'var(--dashboard-card-radius, var(--radius-card))',
      '--el-border-radius-base': 'var(--space-2)',
    }"
    width="min(var(--form-dialog-width), calc(100vw - var(--space-6)))"
    align-center
    :show-close="!submitting"
    :close-on-click-modal="false"
    :close-on-press-escape="!submitting"
    @update:model-value="emit('close')"
  >
    <template #header="{ titleId }">
      <h2 :id="titleId" class="dialog-title">{{ title }}</h2>
    </template>
    <ElAlert v-if="failure" class="error" :title="errorMessage(failure)" type="error" :closable="false" show-icon />
    <template v-if="action.kind === 'status'">
      <p class="disable-impact">
        <WarningFilled aria-hidden="true" />
        {{ t('subaccounts.disableImpact', { name: account?.name }) }}
      </p>
    </template>
    <ElForm v-else ref="form" :model="model" :rules="rules" :disabled="submitting" :validate-on-rule-change="false" label-position="top" @submit.prevent="save">
      <ElFormItem v-if="action.kind === 'create'" :label="t('subaccounts.name')" prop="name" :error="fieldErrors.name && t(`subaccounts.errors.${fieldErrors.name}`)">
        <ElInput v-model="model.name" :placeholder="t('subaccounts.namePlaceholder')" autocomplete="off" maxlength="8" @input="fieldErrors.name = undefined" />
        <p class="hint">{{ t('subaccounts.nameRule') }}</p>
      </ElFormItem>
      <ElFormItem v-else :label="t('subaccounts.name')">
        <code class="username">{{ account?.name }}</code>
      </ElFormItem>

      <ElFormItem v-if="action.kind !== 'edit'" :label="action.kind === 'password' ? t('subaccounts.newPassword') : t('subaccounts.password')" prop="password" :error="fieldErrors.password && t(`subaccounts.errors.${fieldErrors.password}`)">
        <div class="control-row">
          <ElInput v-model="model.password" type="password" show-password autocomplete="new-password" maxlength="12" @input="fieldErrors.password = undefined" />
          <ElButton class="dialog-button" :icon="Refresh" @click="generatePassword">{{ t('subaccounts.generate') }}</ElButton>
        </div>
        <p class="hint">{{ t('subaccounts.passwordRule') }}</p>
      </ElFormItem>

      <template v-if="action.kind !== 'password'">
        <ElFormItem :label="t('subaccounts.remark')" prop="remark" :error="fieldErrors.remark && t(`subaccounts.errors.${fieldErrors.remark}`)">
          <ElInput v-model="model.remark" :placeholder="t('subaccounts.remarkPlaceholder')" :maxlength="remarkByteLimit"
            :count-graphemes="remarkBytes" show-word-limit @input="fieldErrors.remark = undefined" />
        </ElFormItem>
        <ElFormItem :label="t('subaccounts.limit')" prop="limitGB" :error="fieldErrors.limitGB && t(`subaccounts.errors.${fieldErrors.limitGB}`)">
          <div class="control-row limit-row">
            <ElSwitch v-model="model.limited" :aria-label="t('subaccounts.limit')" :active-text="t('subaccounts.limited')" :inactive-text="t('subaccounts.unlimited')" inline-prompt
              @change="fieldErrors.limitGB = undefined; form?.clearValidate('limitGB')" />
            <ElInput v-if="model.limited" v-model="model.limitGB" inputmode="decimal" :aria-label="t('subaccounts.limitInGB')" @input="fieldErrors.limitGB = undefined">
              <template #prepend>
                <ElButton :icon="Minus" :aria-label="t('subaccounts.decreaseLimit')" :disabled="stepDown === undefined" @click="changeLimit(stepDown)" />
              </template>
              <template #append>
                <ElButton :icon="Plus" :aria-label="t('subaccounts.increaseLimit')" :disabled="stepUp === undefined" @click="changeLimit(stepUp)" />
              </template>
            </ElInput>
            <span v-if="model.limited" class="unit">GB</span>
          </div>
          <p class="hint">{{ account?.limitBytes === undefined ? t('subaccounts.limitImpact') : t('subaccounts.limitEditHint', { used: formatGB(account.usedBytes, locale) }) }}</p>
        </ElFormItem>
        <ElFormItem :label="t('subaccounts.status')">
          <ElSwitch v-model="model.active" :active-text="t('subaccounts.active')" :inactive-text="t('subaccounts.inactive')" />
        </ElFormItem>
      </template>

      <div v-if="action.kind !== 'edit'" class="preview">
        <span>{{ t('subaccounts.username') }}</span>
        <strong class="username">{{ primaryAccount }}.{{ account?.name || model.name || t('subaccounts.name') }}</strong>
        <p v-if="action.kind === 'password'" class="hint">{{ t('subaccounts.passwordImpact') }}</p>
      </div>
    </ElForm>
    <template #footer>
      <div class="dialog-actions">
        <ElButton class="dialog-button" :disabled="submitting" @click="emit('close')">{{ t('subaccounts.cancel') }}</ElButton>
        <ElButton v-if="action.kind === 'status'" class="dialog-button" type="primary" :loading="submitting" @click="save">
          {{ t('subaccounts.disable') }}
        </ElButton>
        <ElButton v-else class="dialog-button" type="primary" :loading="submitting" @click="save">
          {{ action.kind === 'create' ? t('subaccounts.create') : t('subaccounts.save') }}
        </ElButton>
      </div>
    </template>
  </ElDialog>
</template>

<style scoped>
.dialog-title {
  color: var(--color-text);
  font-size: var(--font-size-brand);
  font-weight: var(--font-weight-bold);
  line-height: var(--line-height-tight);
}
.error { margin-bottom: var(--space-4); }
.disable-impact { display: flex; align-items: flex-start; gap: var(--space-3); }
.disable-impact svg { flex: none; width: var(--space-5); color: var(--color-warning-text); }
.control-row { display: flex; align-items: center; gap: var(--space-2); width: 100%; }
.control-row > button { flex: none; }
.limit-row { flex-wrap: wrap; }
.limit-row > .el-input { width: min(100%, calc(var(--subaccount-metric-width) - var(--space-7))); }
.limit-row button { padding-inline: var(--space-2); }
.unit { color: var(--color-text-secondary); }
.hint { width: 100%; margin-top: var(--space-1); color: var(--color-text-secondary); font-size: var(--font-size-xs); line-height: var(--line-height-normal); }
.username { font-family: var(--font-mono); overflow-wrap: anywhere; }
.preview {
  display: grid;
  gap: var(--space-1);
  padding: var(--space-3);
  background: var(--color-surface-muted);
  border-radius: var(--space-2);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  line-height: 1.25;
}
.preview strong { color: var(--color-text); font-size: var(--font-size-md); }
:global(.subaccount-dialog-footer) { padding-top: calc(var(--space-4) + var(--space-1)); }
.dialog-actions { display: flex; justify-content: flex-end; gap: var(--space-2); }
.dialog-button { margin: 0; padding-inline: var(--space-3); font-size: var(--font-size-sm); }
</style>
