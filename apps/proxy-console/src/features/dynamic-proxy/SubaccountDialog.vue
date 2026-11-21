<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElSwitch, type FormInstance, type FormRules } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { createSubaccount, updateSubaccount, type Subaccount } from '@/api/subaccounts'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { formatGB, parseLimitGB } from './trafficUnits'

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
  active: true,
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

const statusLabel = computed(() => account?.active ? t('subaccounts.disable') : t('subaccounts.enable'))
const title = computed(() => {
  switch (props.action.kind) {
    case 'create': return t('subaccounts.createTitle')
    case 'edit': return t('subaccounts.editTitle')
    case 'password': return t('subaccounts.changePassword')
    case 'status': return account?.active
      ? t('subaccounts.disableTitle', { name: account.name })
      : t('subaccounts.enableTitle', { name: account?.name })
  }
})
const limitBytes = computed(() => model.limited ? parseLimitGB(model.limitGB) : undefined)
const rules = computed<FormRules<typeof model>>(() => ({
  name: [{ required: true, pattern: /^[a-z][a-z0-9]{3,7}$/, message: t('subaccounts.nameRule') }],
  password: [{ required: true, pattern: /^(?=.*[A-Za-z])(?=.*\d)[A-Za-z0-9]{6,12}$/, message: t('subaccounts.passwordRule') }],
  remark: [{ validator: (_rule, value: string, done) => {
    done(new TextEncoder().encode(value.trim()).length > 512 ? new Error(t('subaccounts.remarkRule')) : undefined)
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
        const limit = limitBytes.value !== action.account.limitBytes ? { bytes: limitBytes.value } : undefined
        if (changedRemark === undefined && !limit) {
          emit('close')
          return
        }
        saved = await updateSubaccount(action.account.clientId, { kind: 'details', remark: changedRemark, limit })
        break
      }
      case 'password':
        saved = await updateSubaccount(action.account.clientId, { kind: 'password', password: model.password })
        break
      case 'status':
        saved = await updateSubaccount(action.account.clientId, { kind: 'status', active: !action.account.active })
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
    width="min(var(--form-dialog-width), calc(100vw - var(--space-6)))"
    align-center
    :show-close="!submitting"
    :close-on-click-modal="false"
    :close-on-press-escape="!submitting"
    @update:model-value="emit('close')"
  >
    <ElAlert v-if="failure" class="error" :title="errorMessage(failure)" type="error" :closable="false" show-icon />
    <template v-if="action.kind === 'status'">
      <p>{{ account?.active ? t('subaccounts.disableImpact') : t('subaccounts.enableImpact') }}</p>
    </template>
    <ElForm v-else ref="form" :model="model" :rules="rules" :disabled="submitting" :validate-on-rule-change="false" label-position="top" @submit.prevent="save">
      <ElFormItem v-if="action.kind === 'create'" :label="t('subaccounts.name')" prop="name" :error="fieldErrors.name && t(`subaccounts.errors.${fieldErrors.name}`)">
        <ElInput v-model="model.name" autocomplete="off" maxlength="8" @input="fieldErrors.name = undefined" />
        <p class="hint">{{ t('subaccounts.nameRule') }}</p>
      </ElFormItem>
      <ElFormItem v-else :label="t('subaccounts.name')">
        <code>{{ account?.name }}</code>
      </ElFormItem>

      <ElFormItem v-if="action.kind !== 'edit'" :label="action.kind === 'password' ? t('subaccounts.newPassword') : t('subaccounts.password')" prop="password" :error="fieldErrors.password && t(`subaccounts.errors.${fieldErrors.password}`)">
        <div class="control-row">
          <ElInput v-model="model.password" type="password" show-password autocomplete="new-password" maxlength="12" @input="fieldErrors.password = undefined" />
          <ElButton :icon="Refresh" @click="generatePassword">{{ t('subaccounts.generate') }}</ElButton>
        </div>
        <p class="hint">{{ t('subaccounts.passwordRule') }}</p>
      </ElFormItem>

      <template v-if="action.kind !== 'password'">
        <ElFormItem :label="t('subaccounts.remark')" prop="remark" :error="fieldErrors.remark && t(`subaccounts.errors.${fieldErrors.remark}`)">
          <ElInput v-model="model.remark" :placeholder="t('subaccounts.remarkPlaceholder')" @input="fieldErrors.remark = undefined" />
        </ElFormItem>
        <ElFormItem :label="t('subaccounts.limit')" prop="limitGB" :error="fieldErrors.limitGB && t(`subaccounts.errors.${fieldErrors.limitGB}`)">
          <div class="control-row">
            <ElSwitch v-model="model.limited" :aria-label="t('subaccounts.limit')" :active-text="t('subaccounts.limited')" :inactive-text="t('subaccounts.unlimited')"
              @change="fieldErrors.limitGB = undefined; form?.clearValidate('limitGB')" />
            <ElInput v-if="model.limited" v-model="model.limitGB" inputmode="decimal" :aria-label="t('subaccounts.limitInGB')" @input="fieldErrors.limitGB = undefined">
              <template #append>GB</template>
            </ElInput>
          </div>
          <p class="hint">{{ account?.limitBytes === undefined ? t('subaccounts.limitImpact') : t('subaccounts.limitEditHint', { used: formatGB(account.usedBytes, locale) }) }}</p>
        </ElFormItem>
        <ElFormItem v-if="action.kind === 'create'" :label="t('subaccounts.status')">
          <ElSwitch v-model="model.active" :active-text="t('subaccounts.active')" :inactive-text="t('subaccounts.inactive')" />
        </ElFormItem>
      </template>

      <div v-if="action.kind !== 'edit'" class="preview">
        <span>{{ t('subaccounts.username') }}</span>
        <code>{{ primaryAccount }}.{{ account?.name || model.name || t('subaccounts.name') }}</code>
        <p v-if="action.kind === 'password'" class="hint">{{ t('subaccounts.passwordImpact') }}</p>
      </div>
    </ElForm>
    <template #footer>
      <ElButton :disabled="submitting" @click="emit('close')">{{ t('subaccounts.cancel') }}</ElButton>
      <ElButton :type="action.kind === 'status' && account?.active ? 'danger' : 'primary'" :loading="submitting" @click="save">
        {{ action.kind === 'status' ? statusLabel : action.kind === 'create' ? t('subaccounts.create') : t('subaccounts.save') }}
      </ElButton>
    </template>
  </ElDialog>
</template>

<style scoped>
.error { margin-bottom: var(--space-4); }
.control-row { display: flex; align-items: center; gap: var(--space-3); width: 100%; }
.control-row > button { flex: none; }
.hint { width: 100%; margin-top: var(--space-1); color: var(--color-text-secondary); font-size: var(--font-size-xs); line-height: var(--line-height-normal); }
.preview {
  display: grid;
  gap: var(--space-1);
  padding: var(--space-3);
  background: var(--color-surface-muted);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-control);
}
</style>
