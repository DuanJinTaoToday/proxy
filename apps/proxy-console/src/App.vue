<script setup lang="ts">
import { ref, watch } from 'vue'
import { ApiError, getAccount, login, register } from './api/control'
import type { Tenant } from './api/control'
const sessionKey = 'proxy-console.session'
const token = ref(sessionStorage.getItem(sessionKey) ?? '')
const phone = ref('')
const registering = ref(false)
const password = ref('')
const signingIn = ref(false)
const loginError = ref('')
const tenantError = ref('')
const account = ref<Tenant | null>(null)
const retry = ref(0)

function setSession(value: string) {
  if (value) sessionStorage.setItem(sessionKey, value)
  else sessionStorage.removeItem(sessionKey)
  token.value = value
}

function endSession(expired = false) {
  setSession('')
  loginError.value = expired ? '登录已失效，请重新登录。' : ''
}

async function signIn() {
  if (signingIn.value) return
  signingIn.value = true
  loginError.value = ''
  try {
    if (registering.value) {
      await register(phone.value.trim(), password.value)
      registering.value = false
    }
    setSession(await login(phone.value.trim(), password.value))
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 0
    if (status === 409) loginError.value = '该手机号已注册，请登录。'
    else if (status === 400) loginError.value = '请输入有效手机号和至少 12 位密码。'
    else if (status === 401) loginError.value = '手机号或密码错误。'
    else loginError.value = '操作失败，请稍后重试。'
  } finally {
    password.value = ''
    signingIn.value = false
  }
}

watch([token, retry], async ([session], _, onCleanup) => {
  account.value = null
  tenantError.value = ''
  if (!session) return

  const controller = new AbortController()
  onCleanup(() => controller.abort())
  try {
    const result = await getAccount(session, controller.signal)
    if (!controller.signal.aborted) account.value = result
  } catch (error) {
    if (controller.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) endSession(true)
    else tenantError.value = '无法加载租户信息。'
  }
}, { immediate: true, flush: 'sync' })

function switchMode() {
  registering.value = !registering.value
  password.value = ''
  loginError.value = ''
}
</script>

<template>
  <a class="skip-link" href="#main">跳到主要内容</a>
  <header class="topbar">
    <div class="shell topbar-inner">
      <div class="brand"><span class="brand-mark" aria-hidden="true"></span>代理控制台</div>
      <button v-if="token" class="secondary" type="button" @click="endSession()">退出登录</button>
    </div>
  </header>

  <main id="main" class="shell">
    <section v-if="!token" class="panel login-panel">
      <h1>{{ registering ? '注册代理控制台' : '登录控制台' }}</h1>
      <form class="login-form" @submit.prevent="signIn">
        <label>手机号<input v-model="phone" type="tel" autocomplete="username" placeholder="可带 +86" required></label>
        <label>密码<input v-model="password" type="password" :autocomplete="registering ? 'new-password' : 'current-password'" :minlength="registering ? 12 : undefined" required></label>
        <p v-if="loginError" role="alert" class="error">{{ loginError }}</p>
        <button type="submit" :disabled="signingIn">{{ signingIn ? '正在提交…' : registering ? '注册' : '登录' }}</button>
        <button class="secondary" type="button" :disabled="signingIn" @click="switchMode">{{ registering ? '已有账号，去登录' : '注册账号' }}</button>
        <p v-if="!registering">忘记密码？请联系运营核实租户归属后重置。</p>
      </form>
    </section>

    <template v-else>
      <div class="page-heading">
        <h1>租户信息</h1>
        <span v-if="account">{{ account.name }}</span>
      </div>
      <div v-if="tenantError" class="panel state">
        <p role="alert" class="error">{{ tenantError }}</p>
        <button class="secondary" type="button" @click="retry++">重新加载</button>
      </div>
      <p v-else-if="account === null" class="panel state" role="status">正在加载租户…</p>
    </template>
  </main>
</template>
