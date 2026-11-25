<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, ElResult, type FormInstance, type FormRules } from 'element-plus'
import { recoverEmailAccount, resetPassword } from '@/api/access'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import AuthCard from './AuthCard.vue'
import { dropLinkToken, readLinkToken, type LinkPurpose } from './emailLinks'
import { passwordProblem } from './rules'
import { clearSession } from './session'

// 邮件链接落地页：reset 为自助重置密码，recover 为运营发起的账号恢复。两者都用一次性令牌设置新密码。
// 令牌已由路由守卫移出地址栏，本页不加载任何站外资源。
const props = defineProps<{ purpose: LinkPurpose }>()

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const token = ref(readLinkToken(props.purpose))
const state = ref<'form' | 'done'>('form')
const formRef = ref<FormInstance>()
const form = reactive({ password: '', confirm: '' })
const submitting = ref(false)
const failure = ref('')
const passwordError = ref('')

const isReset = computed(() => props.purpose === 'reset')
const keyOf = (name: string) => `auth.${isReset.value ? 'reset' : 'recover'}.${name}`
const page = (name: string) => ({ path: `/${locale.value}/${name}` })

const rules = computed<FormRules<typeof form>>(() => ({
  password: [
    { required: true, message: t('auth.passwordRequired'), trigger: 'blur' },
    {
      validator: (_rule, value: string, done) => {
        const problem = passwordProblem(value)
        done(problem && new Error(t(problem === 'tooShort' ? 'auth.passwordTooShort' : 'auth.passwordTooLong')))
      },
      trigger: 'blur',
    },
  ],
  confirm: [
    { required: true, message: t('auth.confirmRequired'), trigger: 'blur' },
    { validator: (_rule, value: string, done) => done(value === form.password ? undefined : new Error(t('auth.confirmMismatch'))), trigger: 'blur' },
  ],
}))

async function submit() {
  failure.value = ''
  passwordError.value = ''
  if (!token.value || !await formRef.value?.validate().catch(() => false)) return
  submitting.value = true
  try {
    await (isReset.value ? resetPassword : recoverEmailAccount)(token.value, form.password)
    forgetToken()
    // 控制面已使该账号的旧会话失效，清除本地会话以便直接登录。
    clearSession()
    form.password = ''
    form.confirm = ''
    state.value = 'done'
  } catch (error) {
    if (error instanceof ApiError && error.reason === 'INVALID_ACCESS_TOKEN') forgetToken()
    else if (error instanceof ApiError && error.reason === 'INVALID_PASSWORD') passwordError.value = errorMessage(error)
    else failure.value = errorMessage(error)
  } finally {
    submitting.value = false
  }
}

function forgetToken() {
  dropLinkToken(props.purpose)
  token.value = ''
}
</script>

<template>
  <AuthCard v-if="state === 'done'" :title="t(keyOf('doneTitle'))">
    <ElResult icon="success" :sub-title="t(keyOf('doneBody'))" class="link-result">
      <template #extra>
        <LinkButton :to="page('login')" type="primary">{{ t('auth.goToLogin') }}</LinkButton>
      </template>
    </ElResult>
  </AuthCard>

  <AuthCard v-else-if="!token" :title="t(keyOf('invalidTitle'))">
    <ElAlert type="warning" :title="t(keyOf('invalidBody'))" :closable="false" show-icon class="auth-card__alert" />
    <div class="link-actions">
      <LinkButton :to="page('forgot-password')" type="primary">{{ t('auth.requestNewLink') }}</LinkButton>
      <LinkButton :to="page('login')">{{ t('auth.backToLogin') }}</LinkButton>
    </div>
  </AuthCard>

  <AuthCard v-else :title="t(keyOf('title'))" :subtitle="t(keyOf('subtitle'))">
    <ElAlert v-if="failure" type="error" :title="failure" :closable="false" show-icon class="auth-card__alert" />
    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="submit">
      <ElFormItem prop="password" for="link-password" :error="passwordError">
        <template #label>
          <span>{{ t('auth.newPassword') }}</span>
          <span class="auth-card__hint">{{ t('auth.passwordHint') }}</span>
        </template>
        <ElInput id="link-password" v-model="form.password" type="password" show-password autocomplete="new-password" :placeholder="t('auth.newPasswordPlaceholder')" @input="passwordError = ''" />
      </ElFormItem>
      <ElFormItem :label="t('auth.confirmPassword')" prop="confirm" for="link-confirm">
        <ElInput id="link-confirm" v-model="form.confirm" type="password" show-password autocomplete="new-password" :placeholder="t('auth.confirmPlaceholder')" />
      </ElFormItem>
      <ElButton type="primary" native-type="submit" :loading="submitting" class="auth-card__submit">{{ t(keyOf('submit')) }}</ElButton>
    </ElForm>
  </AuthCard>
</template>

<style scoped>
.link-result { padding: var(--space-4) 0 0; }
.link-actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.link-actions > * { margin-left: 0; }
</style>
