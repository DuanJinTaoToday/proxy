<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, ElOption, ElSelect } from 'element-plus'
import OpsDialog from '@/shared/ui/OpsDialog.vue'
import { ApiError } from '@/api/client'
import { sendOpsTenantRecoveryEmail, type OpsTenant } from '@/api/opsTenants'
import { currentOperator, emailAddress, hasOperatorPermission, isEmail } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { verificationMethods } from './verification'

// 运营发起的账号恢复：核实租户身份后，向客户确认的邮箱发送恢复链接。
// 客户通过链接设置新密码，之后用该邮箱登录。直接重置密码仍作为备用手段保留。
const props = defineProps<{ tenant: Pick<OpsTenant, 'id' | 'name' | 'account' | 'email'> }>()
const open = defineModel<boolean>({ required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const email = ref(props.tenant.email ?? '')
const verificationMethod = ref('')
const verificationNote = ref('')
const pending = ref(false)
const failure = shallowRef<unknown>()
const emailError = ref('')
const receipt = shallowRef<{ email: string, operator: string, method: string }>()

const emailInvalid = computed(() => Boolean(email.value.trim()) && !isEmail(email.value))
const validForm = computed(() => Boolean(isEmail(email.value) && verificationMethod.value && verificationNote.value.trim()))

watch(open, (visible) => {
  if (visible) return
  email.value = props.tenant.email ?? ''
  verificationMethod.value = ''
  verificationNote.value = ''
  receipt.value = undefined
  failure.value = undefined
  emailError.value = ''
})
watch(email, () => { emailError.value = '' })

async function submit() {
  const operator = currentOperator.value
  if (!operator || !hasOperatorPermission('manage_tenant_accounts') || !validForm.value || pending.value || receipt.value) return
  // 恢复邮件发往客户核实的原样地址，只去首尾空白、不改大小写；控制面自行归一化账号标识。
  const address = emailAddress(email.value)
  const method = verificationMethod.value
  pending.value = true
  failure.value = undefined
  try {
    await sendOpsTenantRecoveryEmail({ tenantId: props.tenant.id, email: address })
    receipt.value = { email: address, operator: operator.username, method }
  } catch (error) {
    if (error instanceof ApiError && error.reason === 'INVALID_EMAIL') emailError.value = errorMessage(error)
    else failure.value = error
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <OpsDialog v-model="open" :title="t('users.recover.title')" width="min(var(--ops-dialog-width), calc(100vw - var(--space-6)))"
    :close-on-click-modal="!pending" :close-on-press-escape="!pending" :show-close="!pending">
    <template v-if="!receipt">
      <ElAlert type="warning" :closable="false" show-icon :title="t('users.reset.verifyTitle')"
        :description="t('users.recover.verifyDescription')" class="recover-alert" />
      <ElAlert v-if="failure" type="error" :closable="false" show-icon :title="errorMessage(failure)" class="recover-alert" />
      <dl class="recover-target">
        <div><dt>{{ t('users.reset.tenant') }}</dt><dd>{{ tenant.name }}</dd></div>
        <div><dt>{{ t('users.primaryAccount') }}</dt><dd class="mono">{{ tenant.account }}</dd></div>
        <div><dt>{{ t('users.recover.currentEmail') }}</dt><dd>{{ tenant.email || t('users.recover.noEmail') }}</dd></div>
      </dl>
      <ElForm label-position="top" @submit.prevent="submit">
        <ElFormItem :label="t('users.recover.email')" for="recover-email" required
          :error="emailError || (emailInvalid ? t('auth.emailInvalid') : '')">
          <ElInput id="recover-email" v-model="email" type="email" autocomplete="off" maxlength="254"
            :placeholder="t('auth.emailPlaceholder')" :disabled="pending" />
        </ElFormItem>
        <ElFormItem :label="t('users.reset.method')" for="recover-verification-method" required>
          <ElSelect id="recover-verification-method" v-model="verificationMethod" :placeholder="t('users.reset.selectMethod')" :disabled="pending">
            <ElOption v-for="method in verificationMethods" :key="method" :value="method" :label="t(`users.reset.methods.${method}`)" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="t('users.reset.note')" for="recover-verification-note" required>
          <ElInput id="recover-verification-note" v-model="verificationNote" type="textarea" :rows="2" maxlength="200"
            :placeholder="t('users.reset.noteExample')" :disabled="pending" />
        </ElFormItem>
      </ElForm>
    </template>
    <template v-else>
      <ElAlert type="success" :closable="false" show-icon role="status" :title="t('users.recover.sent', { email: receipt.email })"
        :description="t('users.recover.sentDetail')" class="recover-alert" />
      <p class="recover-hint">{{ t('users.reset.resultOperator', { operator: receipt.operator, method: t(`users.reset.methods.${receipt.method}`) }) }}</p>
    </template>
    <template #footer>
      <template v-if="!receipt">
        <ElButton class="ops-dialog-action" :disabled="pending" @click="open = false">{{ t('auth.cancel') }}</ElButton>
        <ElButton class="ops-dialog-action" type="primary" :disabled="!validForm" :loading="pending" @click="submit">{{ t('users.recover.confirm') }}</ElButton>
      </template>
      <ElButton class="ops-dialog-action" v-else type="primary" @click="open = false">{{ t('users.reset.finish') }}</ElButton>
    </template>
  </OpsDialog>
</template>

<style scoped>
.recover-alert { margin-block-end: var(--space-4); }
.recover-target { display: grid; gap: var(--space-1); margin: 0 0 var(--space-4); }
.recover-target div { display: grid; grid-template-columns: minmax(max-content, 1fr) 2fr; gap: var(--space-3); }
.recover-target dt, .recover-hint { color: var(--color-text-secondary); }
.recover-target dd { margin: 0; overflow-wrap: anywhere; }
.recover-target .mono { font-size: var(--font-size-xs); }
.recover-hint { margin-block-start: var(--space-1); font-size: var(--font-size-xs); }
</style>
