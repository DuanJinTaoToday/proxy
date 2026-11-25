<script setup lang="ts">
import { ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElForm, ElFormItem, ElInput } from 'element-plus'
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
    <ElCard shadow="never" class="operator-login__card">
      <div class="operator-login__brand">
        <BrandMark />
        <span class="operator-login__badge">{{ t('app.opsBadge') }}</span>
      </div>
      <h1>{{ t('auth.opsLoginTitle') }}</h1>
      <ElForm label-position="top" @submit.prevent="submit">
        <ElAlert v-if="error" type="error" :title="errorMessage(error)" :closable="false" show-icon class="operator-login__error" />
        <ElFormItem :label="t('auth.opsUsername')" for="operator-username" required>
          <ElInput id="operator-username" v-model="username" autocomplete="username" maxlength="128" />
        </ElFormItem>
        <ElFormItem :label="t('auth.password')" for="operator-password" required>
          <ElInput id="operator-password" v-model="password" type="password" autocomplete="current-password" maxlength="1024" show-password />
        </ElFormItem>
        <ElButton type="primary" native-type="submit" :loading="busy" :disabled="!username.trim() || !password">{{ t('header.login') }}</ElButton>
      </ElForm>
    </ElCard>
  </main>
</template>

<style scoped>
.operator-login {
  display: grid;
  place-items: start center;
  min-height: 100dvh;
  padding: var(--space-7) var(--space-4);
}
.operator-login__card { width: min(100%, var(--auth-card-width)); }
.operator-login__brand {
  display: inline-flex;
  align-items: center;
  gap: var(--ops-brand-gap);
  margin-block-end: var(--space-4);
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
.operator-login h1 { margin-block-end: var(--space-5); }
.operator-login__error { margin-block-end: var(--space-4); }
.operator-login .el-button { width: 100%; }
</style>
