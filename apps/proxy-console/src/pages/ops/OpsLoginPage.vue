<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElForm, ElFormItem, ElInput } from 'element-plus'
import { safeOperatorRedirect, signInOperator } from '@/features/auth'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import BrandMark from '@/shared/ui/BrandMark.vue'

const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const errorMessage = useErrorMessage()
const username = ref('')
const password = ref('')
const busy = ref(false)
const error = ref<unknown>()

async function submit() {
  if (!username.value.trim() || !password.value || busy.value) return
  busy.value = true
  error.value = undefined
  try {
    await signInOperator(username.value, password.value)
    await router.replace(safeOperatorRedirect(route.query.redirect))
  } catch (failure) {
    error.value = failure
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="operator-login">
    <section class="operator-login__card">
      <div class="operator-login__accent" aria-hidden="true" />
      <div class="operator-login__inner">
        <div class="operator-login__brand">
          <BrandMark />
          <span class="operator-login__badge">{{ t('app.opsBadge') }}</span>
        </div>
        <h1>{{ t('auth.opsLoginTitle') }}</h1>
        <ElForm label-position="top" size="large" @submit.prevent="submit">
          <ElAlert v-if="error" type="error" :title="errorMessage(error)" :closable="false" show-icon class="operator-login__error" />
          <ElFormItem :label="t('auth.opsUsername')" for="operator-username" required>
            <ElInput id="operator-username" v-model="username" name="username" autocomplete="username" spellcheck="false" autocorrect="off" autocapitalize="none" enterkeyhint="next" maxlength="128" />
          </ElFormItem>
          <ElFormItem :label="t('auth.password')" for="operator-password" required>
            <ElInput id="operator-password" v-model="password" name="password" type="password" autocomplete="current-password" enterkeyhint="done" maxlength="1024" show-password />
          </ElFormItem>
          <ElButton type="primary" native-type="submit" class="operator-login__submit" :loading="busy" :disabled="!username.trim() || !password">{{ t('header.login') }}</ElButton>
        </ElForm>
      </div>
    </section>
  </main>
</template>

<style scoped>
.operator-login {
  display: grid;
  place-items: center;
  min-height: 100dvh;
  padding: var(--space-7) var(--space-4);
  background: var(--auth-page-bg);
}
.operator-login__card {
  width: min(100%, var(--auth-card-width));
  overflow: hidden;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--auth-panel-radius);
  background: var(--color-surface);
  box-shadow: var(--auth-panel-shadow);
}
.operator-login__accent {
  height: var(--space-1);
  background: linear-gradient(90deg, var(--color-primary) 0%, var(--auth-aside-cyan) 100%);
}
.operator-login__inner {
  padding: var(--space-6);
}
.operator-login__brand {
  display: inline-flex;
  align-items: center;
  gap: var(--ops-brand-gap);
  margin-block-end: var(--space-5);
  font-size: var(--ops-brand-size);
  font-weight: var(--font-weight-heavy);
}
.operator-login__badge {
  padding: var(--ops-badge-offset) var(--space-2);
  border-radius: var(--radius-control);
  background: var(--color-shell);
  color: var(--color-accent);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-semibold);
  line-height: var(--ops-badge-line-height);
  white-space: nowrap;
}
.operator-login h1 {
  margin-block-end: var(--space-5);
  font-size: var(--font-size-title);
  font-weight: var(--font-weight-bold);
  letter-spacing: var(--brand-wordmark-tracking);
  color: var(--color-text);
}
.operator-login__error { margin-block-end: var(--space-4); }
.operator-login__submit {
  width: 100%;
  height: var(--auth-submit-height);
  margin-top: var(--space-2);
  font-weight: var(--font-weight-semibold);
}

@media (max-width: 480px) {
  .operator-login { align-items: start; padding-block-start: var(--space-6); }
  .operator-login__inner { padding: var(--space-5) var(--space-4); }
}
</style>
