<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput, type FormInstance, type FormRules } from 'element-plus'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'
import AuthCard from './AuthCard.vue'
import { useCooldown } from './emailLinks'
import { isEmail, redirectQuery } from './rules'
import { requestPasswordReset } from './session'

// 自助找回密码：提交邮箱后发送重置链接。无论邮箱是否注册都显示同一结果。
const route = useRoute()
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()

const formRef = ref<FormInstance>()
const form = reactive({ email: '' })
const sending = ref(false)
const sent = ref(false)
const failure = ref('')
const emailError = ref('')
const { remaining: cooldownLeft, start: startCooldown } = useCooldown()

const loginPage = computed(() => ({ path: `/${locale.value}/login`, query: redirectQuery(route.query.redirect) }))
const rules = computed<FormRules<typeof form>>(() => ({
  email: [
    { required: true, message: t('auth.emailRequired'), trigger: 'blur' },
    { validator: (_rule, value: string, done) => done(isEmail(value) ? undefined : new Error(t('auth.emailInvalid'))), trigger: 'blur' },
  ],
}))

async function submit() {
  failure.value = ''
  emailError.value = ''
  if (cooldownLeft.value > 0 || !await formRef.value?.validate().catch(() => false)) return
  sending.value = true
  try {
    await requestPasswordReset(form.email)
    sent.value = true
    startCooldown()
  } catch (error) {
    if (error instanceof ApiError && error.reason === 'INVALID_EMAIL') emailError.value = errorMessage(error)
    else failure.value = errorMessage(error)
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <AuthCard :title="t('auth.forgotTitle')" :subtitle="t('auth.forgotSubtitle')">
    <ElAlert v-if="failure" type="error" :title="failure" :closable="false" show-icon class="auth-card__alert" />
    <ElAlert v-else-if="sent" type="success" :title="t('auth.resetSent')" :description="t('auth.resetSentDetail')" :closable="false" show-icon class="auth-card__alert" role="status" />

    <ElForm ref="formRef" :model="form" :rules="rules" label-position="top" size="large" @submit.prevent="submit">
      <ElFormItem :label="t('auth.email')" prop="email" for="forgot-email" :error="emailError">
        <ElInput id="forgot-email" v-model="form.email" type="email" autocomplete="email" inputmode="email" maxlength="254" :placeholder="t('auth.emailPlaceholder')" @input="emailError = ''" />
      </ElFormItem>
      <ElButton type="primary" native-type="submit" :loading="sending" :disabled="cooldownLeft > 0" class="auth-card__submit">
        {{ cooldownLeft > 0 ? t('auth.resendIn', { seconds: cooldownLeft }) : t(sent ? 'auth.resendResetLink' : 'auth.sendResetLink') }}
      </ElButton>
    </ElForm>

    <div class="auth-card__footer">
      <LinkButton :to="loginPage" text type="primary">{{ t('auth.backToLogin') }}</LinkButton>
    </div>
  </AuthCard>
</template>
