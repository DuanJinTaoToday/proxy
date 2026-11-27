<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, type FormInstance, type FormItemRule, type FormRules } from 'element-plus'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import AuthCard from './AuthCard.vue'
import { useCooldown } from './emailLinks'
import { isEmail, passwordProblem, redirectQuery, safeRedirect } from './rules'
import { register, requestRegisterCode, signIn } from './session'

// 登录与注册共用一张表单：邮箱加密码，注册另需邮箱验证码。成功后回到登录前访问的站内地址，缺省进入控制台。
const props = defineProps<{ mode: 'login' | 'register' }>()

const route = useRoute()
const router = useRouter()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const formRef = ref<FormInstance>()
const isRegister = computed(() => props.mode === 'register')
const readSingleInvite = (value: unknown) => (typeof value === 'string' ? value.trim() : '')
const linkInviteCode = ref(isRegister.value ? readSingleInvite(route.query.invite) : '')
const inviteTouched = ref(false)
const form = reactive({ email: '', verificationCode: '', password: '', inviteCode: linkInviteCode.value })
const submitting = ref(false)
const sendingCode = ref(false)
const codeSent = ref(false)
const { remaining: cooldownLeft, start: startCooldown } = useCooldown()
const failure = ref('')
// 注册已成功但自动登录失败：提示账号已创建并引导去登录，避免重复注册。
const registeredOnly = ref(false)
const fieldErrors = reactive({ email: '', verificationCode: '', password: '', inviteCode: '' })

// 在认证页面之间切换时带上回跳地址。
const authPage = (page: string) => ({ path: `/${locale.value}/${page}`, query: redirectQuery(route.query.redirect) })
const otherPage = computed(() => authPage(isRegister.value ? 'login' : 'register'))

const passwordRule: FormItemRule = {
  validator: (_rule, value: string, done) => {
    const problem = passwordProblem(value)
    done(problem && new Error(t(problem === 'tooShort' ? 'auth.passwordTooShort' : 'auth.passwordTooLong')))
  },
  trigger: 'blur',
}

const rules = computed<FormRules<typeof form>>(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), trigger: 'blur' },
    { validator: (_rule, value: string, done) => done(isEmail(value) ? undefined : new Error(t('auth.emailInvalid'))), trigger: 'blur' },
  ],
  verificationCode: isRegister.value ? [{ required: true, whitespace: true, message: t('auth.codeRequired'), trigger: 'blur' }] : [],
  password: isRegister.value
    ? [{ required: true, message: t('auth.passwordRequired'), trigger: 'blur' }, passwordRule]
    : [{ required: true, message: t('auth.passwordRequired'), trigger: 'blur' }],
}))

// 控制面能定位到字段的错误显示在对应输入框下，其余显示在表单顶部。修改输入后清除上次的服务端提示。
const fieldOf: Record<string, keyof typeof fieldErrors> = {
  INVALID_EMAIL: 'email',
  EMAIL_ALREADY_REGISTERED: 'email',
  INVALID_VERIFICATION_CODE: 'verificationCode',
  INVALID_PASSWORD: 'password',
  INVALID_INVITE_CODE: 'inviteCode',
  PROMOTION_PAUSED: 'inviteCode',
}
const inviteErrorKey: Record<string, string> = {
  INVALID_INVITE_CODE: 'auth.inviteInvalid',
  PROMOTION_PAUSED: 'auth.invitePromotionPaused',
}
watch(() => form.email, () => { fieldErrors.email = '' })
watch(() => form.verificationCode, () => { fieldErrors.verificationCode = '' })
watch(() => form.password, () => { fieldErrors.password = '' })
watch(() => form.inviteCode, () => { fieldErrors.inviteCode = '' })
watch(() => route.query.invite, (value) => {
  if (!isRegister.value) return
  const next = readSingleInvite(value)
  linkInviteCode.value = next
  if (!inviteTouched.value) form.inviteCode = next
})

function updateInviteCode(value: string) {
  inviteTouched.value = true
  form.inviteCode = value
}

function clearInviteCode() {
  inviteTouched.value = true
  form.inviteCode = ''
  fieldErrors.inviteCode = ''
}

function registerInvitePayload() {
  if (inviteTouched.value) return { inviteCode: form.inviteCode.trim() }
  if (linkInviteCode.value) return { linkInviteCode: linkInviteCode.value }
  return undefined
}

function resetFeedback() {
  failure.value = ''
  registeredOnly.value = false
  fieldErrors.email = ''
  fieldErrors.verificationCode = ''
  fieldErrors.password = ''
  fieldErrors.inviteCode = ''
}

// 无论邮箱是否已注册都显示同一提示，避免借此探测账号。
async function sendCode() {
  resetFeedback()
  if (!await formRef.value?.validateField('email').catch(() => false)) return
  sendingCode.value = true
  try {
    await requestRegisterCode(form.email)
    codeSent.value = true
    startCooldown()
  } catch (error) {
    showError(error)
  } finally {
    sendingCode.value = false
  }
}

async function submit() {
  resetFeedback()
  if (!await formRef.value?.validate().catch(() => false)) return
  submitting.value = true
  try {
    if (isRegister.value) await register(form.email, form.password, form.verificationCode, registerInvitePayload())
  } catch (error) {
    showError(error)
    submitting.value = false
    return
  }
  try {
    await signIn(form.email, form.password)
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
  if (error instanceof ApiError) {
    const localKey = inviteErrorKey[error.reason]
    if (localKey) {
      fieldErrors.inviteCode = t(localKey)
      return
    }
    const field = fieldOf[error.reason]
    if (field) {
      fieldErrors[field] = errorMessage(error)
      return
    }
  }
  failure.value = errorMessage(error)
}
</script>

<template>
  <AuthCard :title="t(isRegister ? 'auth.registerTitle' : 'auth.loginTitle')" :subtitle="t(isRegister ? 'auth.registerSubtitle' : 'auth.loginSubtitle')">
    <ElAlert v-if="failure" :type="registeredOnly ? 'warning' : 'error'" :title="failure" :closable="false" show-icon class="auth-card__alert">
      <LinkButton v-if="registeredOnly" :to="otherPage" type="primary" size="small">{{ t('auth.loginLink') }}</LinkButton>
    </ElAlert>
    <ElAlert v-else-if="codeSent" type="success" :title="t('auth.codeSent')" :closable="false" show-icon class="auth-card__alert" role="status" />

    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="submit">
      <ElFormItem :label="t('auth.email')" prop="email" for="auth-email" :error="fieldErrors.email">
        <ElInput id="auth-email" v-model="form.email" name="email" type="email" autocomplete="username" inputmode="email" spellcheck="false" enterkeyhint="next" maxlength="254" :placeholder="t('auth.emailPlaceholder')" />
      </ElFormItem>
      <ElFormItem v-if="isRegister" :label="t('auth.code')" prop="verificationCode" for="auth-code" :error="fieldErrors.verificationCode">
        <div class="auth-code">
          <ElInput id="auth-code" v-model="form.verificationCode" name="verificationCode" autocomplete="one-time-code" inputmode="numeric" enterkeyhint="next" maxlength="32" :placeholder="t('auth.codePlaceholder')" />
          <ElButton class="auth-code__send" :loading="sendingCode" :disabled="cooldownLeft > 0" @click="sendCode">
            {{ cooldownLeft > 0 ? t('auth.resendIn', { seconds: cooldownLeft }) : t(codeSent ? 'auth.resendCode' : 'auth.sendCode') }}
          </ElButton>
        </div>
      </ElFormItem>
      <ElFormItem prop="password" for="auth-password" :error="fieldErrors.password">
        <template #label>
          <span>{{ t('auth.password') }}</span>
          <span v-if="isRegister" class="auth-card__hint">{{ t('auth.passwordHint') }}</span>
        </template>
        <ElInput id="auth-password" v-model="form.password" name="password" type="password" show-password :autocomplete="isRegister ? 'new-password' : 'current-password'" :enterkeyhint="isRegister ? 'next' : 'done'" :placeholder="t('auth.passwordPlaceholder')" />
      </ElFormItem>
      <ElFormItem v-if="isRegister" :label="t('auth.inviteCode')" prop="inviteCode" for="auth-invite-code" :error="fieldErrors.inviteCode">
        <div class="auth-invite">
          <ElInput
            id="auth-invite-code"
            :model-value="form.inviteCode"
            name="inviteCode"
            autocomplete="off"
            spellcheck="false"
            enterkeyhint="done"
            maxlength="64"
            :placeholder="t('auth.inviteCodePlaceholder')"
            @update:model-value="updateInviteCode"
          />
          <ElButton class="auth-invite__clear" :disabled="!form.inviteCode" @click="clearInviteCode">
            {{ t('auth.clearInviteCode') }}
          </ElButton>
        </div>
      </ElFormItem>
      <ElButton type="primary" native-type="submit" :loading="submitting" class="auth-card__submit">
        {{ t(isRegister ? 'auth.submitRegister' : 'auth.submitLogin') }}
      </ElButton>
    </ElForm>

    <div class="auth-card__footer">
      <span class="auth-card__footer-switch">{{ t(isRegister ? 'auth.haveAccount' : 'auth.noAccount') }}<LinkButton :to="otherPage" text type="primary" class="auth-card__footer-link">{{ t(isRegister ? 'auth.loginLink' : 'auth.registerLink') }}</LinkButton></span>
      <LinkButton v-if="!isRegister" :to="authPage('forgot-password')" text class="auth-card__footer-link">{{ t('auth.forgot') }}</LinkButton>
    </div>
  </AuthCard>
</template>

<style scoped>
.auth-code, .auth-invite { display: flex; flex-wrap: wrap; gap: var(--space-2); width: 100%; }
.auth-code > :first-child, .auth-invite > :first-child { flex: 1 1 10em; min-width: 0; }
.auth-code__send, .auth-invite__clear { margin-left: 0; }
</style>
