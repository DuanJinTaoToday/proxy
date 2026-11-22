<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, ElMessage, ElOption, ElSelect } from 'element-plus'
import OpsDialog from '@/shared/ui/OpsDialog.vue'
import { CopyDocument } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { resetOpsTenantPassword, type OpsTenant } from '@/api/opsTenants'
import { currentOperator, hasOperatorPermission } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const props = defineProps<{ tenant: Pick<OpsTenant, 'id' | 'name' | 'account'> }>()
const open = defineModel<boolean>({ required: true })
const { t } = useI18n()
const errorMessage = useErrorMessage()

const methods = ['phone', 'transfer', 'license']
const verificationMethod = ref('')
const verificationNote = ref('')
const pending = ref(false)
const failure = shallowRef<unknown>()
const receipt = shallowRef<{ password: string, operator: string, method: string }>()
let candidate = ''

const validForm = computed(() => Boolean(verificationMethod.value && verificationNote.value.trim()))

watch(open, (visible) => {
  if (visible) return
  verificationMethod.value = ''
  verificationNote.value = ''
  candidate = ''
  receipt.value = undefined
  failure.value = undefined
})

function temporaryPassword() {
  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_'
  return Array.from(crypto.getRandomValues(new Uint8Array(18)), byte => alphabet.charAt(byte % alphabet.length)).join('')
}

async function submit() {
  const operator = currentOperator.value
  if (!operator || !hasOperatorPermission('manage_tenant_accounts') || !validForm.value || pending.value || receipt.value) return
  const method = verificationMethod.value
  pending.value = true
  failure.value = undefined
  try {
    // Keep the same candidate if the operator retries after losing the response.
    candidate ||= temporaryPassword()
    await resetOpsTenantPassword({ tenantId: props.tenant.id, password: candidate })
    receipt.value = { password: candidate, operator: operator.username, method }
    candidate = ''
  } catch (error) {
    if (error instanceof ApiError && error.status >= 400 && error.status < 500) candidate = ''
    failure.value = error
  } finally {
    pending.value = false
  }
}

async function copy(password: string) {
  try {
    await navigator.clipboard.writeText(password)
    ElMessage.success(t('users.reset.copied'))
  } catch {
    ElMessage.error(t('users.reset.copyFailed'))
  }
}
</script>

<template>
  <OpsDialog v-model="open" :title="t('users.reset.title')" width="min(var(--ops-dialog-width), calc(100vw - var(--space-6)))"
    :close-on-click-modal="!pending" :close-on-press-escape="!pending" :show-close="!pending">
    <template v-if="!receipt">
      <ElAlert type="warning" :closable="false" show-icon :title="t('users.reset.verifyTitle')"
        :description="t('users.reset.verifyDescription')" class="reset-alert" />
      <ElAlert v-if="failure" type="error" :closable="false" show-icon :title="errorMessage(failure)" class="reset-alert" />
      <dl class="reset-target">
        <div><dt>{{ t('users.reset.tenant') }}</dt><dd>{{ tenant.name }}</dd></div>
        <div><dt>{{ t('users.primaryAccount') }}</dt><dd class="mono">{{ tenant.account }}</dd></div>
      </dl>
      <ElForm label-position="top" @submit.prevent="submit">
        <ElFormItem :label="t('users.reset.method')" for="reset-verification-method" required>
          <ElSelect id="reset-verification-method" v-model="verificationMethod" :placeholder="t('users.reset.selectMethod')" :disabled="pending">
            <ElOption v-for="method in methods" :key="method" :value="method" :label="t(`users.reset.methods.${method}`)" />
          </ElSelect>
        </ElFormItem>
        <ElFormItem :label="t('users.reset.note')" for="reset-verification-note" required>
          <ElInput id="reset-verification-note" v-model="verificationNote" type="textarea" :rows="2" maxlength="200"
            :placeholder="t('users.reset.noteExample')" :disabled="pending" />
        </ElFormItem>
      </ElForm>
    </template>
    <template v-else>
      <p class="reset-hint">{{ t('users.reset.resultHint') }}</p>
      <div class="temp-password">
        <span class="mono">{{ receipt.password }}</span>
        <ElButton :icon="CopyDocument" @click="copy(receipt.password)">{{ t('users.reset.copy') }}</ElButton>
      </div>
      <p class="reset-hint">{{ t('users.reset.resultOperator', { operator: receipt.operator, method: t(`users.reset.methods.${receipt.method}`) }) }}</p>
    </template>
    <template #footer>
      <template v-if="!receipt">
        <ElButton class="ops-dialog-action" :disabled="pending" @click="open = false">{{ t('auth.cancel') }}</ElButton>
        <ElButton class="ops-dialog-action" type="danger" :disabled="!validForm" :loading="pending" @click="submit">{{ t('users.reset.confirm') }}</ElButton>
      </template>
      <ElButton class="ops-dialog-action" v-else type="primary" @click="open = false">{{ t('users.reset.finish') }}</ElButton>
    </template>
  </OpsDialog>
</template>

<style scoped>
.reset-alert { margin-block-end: var(--space-4); }
.reset-target { display: grid; gap: var(--space-1); margin: 0 0 var(--space-4); }
.reset-target div { display: grid; grid-template-columns: minmax(max-content, 1fr) 2fr; gap: var(--space-3); }
.reset-target dt, .reset-hint { color: var(--color-text-secondary); }
.reset-target dd { margin: 0; overflow-wrap: anywhere; }
.reset-hint { margin-block-start: var(--space-1); font-size: var(--font-size-xs); }
.temp-password { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-3); margin-block: var(--space-3); padding: var(--space-3) var(--space-4); border-radius: var(--radius-card); background: var(--color-surface-muted); font-size: var(--font-size-xl); }
.temp-password .mono { overflow-wrap: anywhere; }
</style>
