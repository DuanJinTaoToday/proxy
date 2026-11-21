<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, ElPopover, type FormInstance, type FormRules } from 'element-plus'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { normalizePhone, safeRedirect } from './rules'
import { register, signIn } from './session'

// 登录与注册共用一张表单：手机号加密码。成功后回到登录前访问的站内地址，缺省进入控制台。
const props = defineProps<{ mode: 'login' | 'register' }>()

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const formRef = ref<FormInstance>()
const form = reactive({ phone: '', password: '' })
const submitting = ref(false)
const failure = ref('')
// 注册已成功但自动登录失败：提示账号已创建并引导去登录，避免重复注册。
const registeredOnly = ref(false)
const fieldErrors = reactive({ phone: '', password: '' })

const isRegister = computed(() => props.mode === 'register')
// 在登录和注册之间切换时带上回跳地址。
const otherPage = computed(() => ({ path: `/${locale.value}/${isRegister.value ? 'login' : 'register'}`, query: route.query.redirect ? { redirect: route.query.redirect } : {} }))

const rules = computed<FormRules<typeof form>>(() => ({
  phone: [
    { required: true, message: t('auth.phoneRequired'), trigger: 'blur' },
    { validator: (_rule, value: string, done) => done(normalizePhone(value) ? undefined : new Error(t('auth.phoneInvalid'))), trigger: 'blur' },
  ],
  password: isRegister.value
    ? [{ required: true, message: t('auth.passwordRequired'), trigger: 'blur' }, { validator: (_rule, value: string, done) => done(passwordProblem(value)), trigger: 'blur' }]
    : [{ required: true, message: t('auth.passwordRequired'), trigger: 'blur' }],
}))

// 与控制面一致：至少 12 个字符（按字符而非 UTF-16 单元计），最多 1024 字节。
function passwordProblem(value: string) {
  if ([...value].length < 12) return new Error(t('auth.passwordTooShort'))
  if (new TextEncoder().encode(value).length > 1024) return new Error(t('auth.passwordTooLong'))
}

// 控制面能定位到字段的错误显示在对应输入框下，其余显示在表单顶部。修改输入后清除上次的服务端提示。
const fieldOf: Record<string, keyof typeof fieldErrors> = { INVALID_PHONE: 'phone', PHONE_ALREADY_REGISTERED: 'phone', INVALID_PASSWORD: 'password' }
watch(() => form.phone, () => { fieldErrors.phone = '' })
watch(() => form.password, () => { fieldErrors.password = '' })

async function submit() {
  failure.value = ''
  registeredOnly.value = false
  fieldErrors.phone = ''
  fieldErrors.password = ''
  if (!await formRef.value?.validate().catch(() => false)) return
  submitting.value = true
  try {
    if (isRegister.value) await register(form.phone, form.password)
  } catch (error) {
    showError(error)
    submitting.value = false
    return
  }
  try {
    await signIn(form.phone, form.password)
    await router.replace(safeRedirect(route.query.redirect))
  } catch (error) {
    if (isRegister.value) {
      registeredOnly.value = true
      failure.value = t('auth.registeredSignInFailed', { reason: errorMessage(error) })
    } else {
      showError(error)
    }
  } finally {
    submitting.value = false
  }
}

function showError(error: unknown) {
  const field = error instanceof ApiError ? fieldOf[error.reason] : undefined
  if (field) fieldErrors[field] = errorMessage(error)
  else failure.value = errorMessage(error)
}
</script>

<template>
  <section class="auth-form">
    <h1 class="auth-form__title">{{ t(isRegister ? 'auth.registerTitle' : 'auth.loginTitle') }}</h1>
    <p class="auth-form__subtitle">{{ t(isRegister ? 'auth.registerSubtitle' : 'auth.loginSubtitle') }}</p>

    <ElAlert v-if="failure" :type="registeredOnly ? 'warning' : 'error'" :title="failure" :closable="false" show-icon class="auth-form__alert">
      <LinkButton v-if="registeredOnly" :to="otherPage" type="primary" size="small">{{ t('auth.loginLink') }}</LinkButton>
    </ElAlert>

    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="submit">
      <ElFormItem :label="t('auth.phone')" prop="phone" :error="fieldErrors.phone">
        <ElInput v-model="form.phone" autocomplete="tel" inputmode="tel" :placeholder="t('auth.phonePlaceholder')">
          <template #prepend>+86</template>
        </ElInput>
      </ElFormItem>
      <ElFormItem prop="password" :error="fieldErrors.password">
        <template #label>
          <span>{{ t('auth.password') }}</span>
          <span v-if="isRegister" class="auth-form__hint">{{ t('auth.passwordHint') }}</span>
        </template>
        <ElInput v-model="form.password" type="password" show-password :autocomplete="isRegister ? 'new-password' : 'current-password'" :placeholder="t('auth.passwordPlaceholder')" />
      </ElFormItem>
      <ElButton type="primary" native-type="submit" :loading="submitting" class="auth-form__submit">
        {{ t(isRegister ? 'auth.submitRegister' : 'auth.submitLogin') }}
      </ElButton>
    </ElForm>

    <div class="auth-form__footer">
      <span>{{ t(isRegister ? 'auth.haveAccount' : 'auth.noAccount') }}<LinkButton :to="otherPage" text type="primary">{{ t(isRegister ? 'auth.loginLink' : 'auth.registerLink') }}</LinkButton></span>
      <ElPopover v-if="!isRegister" placement="bottom-end" trigger="click" :width="280" :content="t('auth.forgotDetail')">
        <template #reference><ElButton text>{{ t('auth.forgot') }}</ElButton></template>
      </ElPopover>
    </div>
  </section>
</template>

<style scoped>
.auth-form {
  width: 100%;
  max-width: var(--auth-card-width);
  margin-inline: auto;
  padding: var(--space-6);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--shadow-card);
}
.auth-form__title { font-size: var(--font-size-title); font-weight: var(--font-weight-bold); line-height: var(--line-height-tight); }
.auth-form__subtitle { margin: var(--space-2) 0 var(--space-5); color: var(--color-text-secondary); }
.auth-form__alert { margin-bottom: var(--space-4); }
.auth-form__hint { margin-left: var(--space-2); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.auth-form__submit { width: 100%; margin-top: var(--space-2); }
.auth-form__footer { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-2); margin-top: var(--space-4); color: var(--color-text-secondary); }

@media (max-width: 767px) {
  .auth-form { padding: var(--space-5) var(--space-4); }
}
</style>
