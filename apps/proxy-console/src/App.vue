<script setup lang="ts">
import { ref, watch } from 'vue'
import { ApiError, login, loginOperator, register } from './api/control'
import { getWallet } from './api/wallet'
import type { WalletTenant } from './api/wallet'
import OperatorWallet from './OperatorWallet.vue'
import WalletRecords from './WalletRecords.vue'
import { money } from './wallet-format'

const { operations } = defineProps<{ operations: boolean }>()
const sessionKey = operations ? 'proxy-console.operations-session' : 'proxy-console.session'
const operatorKey = 'proxy-console.operations-account'
const operatorName = ref(operations ? sessionStorage.getItem(operatorKey) ?? '' : '')
const token = ref(sessionStorage.getItem(sessionKey) ?? '')
const phone = ref('')
const registering = ref(false)
const password = ref('')
const signingIn = ref(false)
const loginError = ref('')
const tenantError = ref('')
const account = ref<WalletTenant | null>(null)
const retry = ref(0)

function setSession(value: string) {
  if (value) sessionStorage.setItem(sessionKey, value)
  else sessionStorage.removeItem(sessionKey)
  token.value = value
}

function endSession(expired = false) {
  if (operations) sessionStorage.removeItem(operatorKey)
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
    const authenticate = operations ? loginOperator : login
    const username = phone.value.trim()
    const session = await authenticate(username, password.value)
    if (operations) {
      operatorName.value = username
      sessionStorage.setItem(operatorKey, username)
    }
    setSession(session)
  } catch (error) {
    const status = error instanceof ApiError ? error.status : 0
    if (status === 409) loginError.value = '该手机号已注册，请登录。'
    else if (status === 400) loginError.value = operations ? '请检查账号和密码。' : '请输入有效手机号和至少 12 位密码。'
    else if (status === 401) loginError.value = operations ? '运营账号或密码错误。' : '手机号或密码错误。'
    else loginError.value = '操作失败，请稍后重试。'
  } finally {
    password.value = ''
    signingIn.value = false
  }
}

watch([token, retry], async ([session], _, onCleanup) => {
  account.value = null
  tenantError.value = ''
  if (!session || operations) return

  const controller = new AbortController()
  onCleanup(() => controller.abort())
  try {
    const result = await getWallet(session, controller.signal)
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
      <div class="brand"><span class="brand-mark" aria-hidden="true"></span>{{ operations ? '运营控制台' : '代理控制台' }}</div>
      <button v-if="token" class="secondary" type="button" @click="endSession()">退出登录</button>
    </div>
  </header>

  <main id="main" class="shell wallet-console">
    <section v-if="!token" class="panel login-panel">
      <h1>{{ operations ? '运营登录' : registering ? '注册代理控制台' : '登录控制台' }}</h1>
      <form class="login-form" @submit.prevent="signIn">
        <label v-if="operations">运营账号<input v-model="phone" autocomplete="username" required></label>
        <label v-else>手机号<input v-model="phone" type="tel" autocomplete="username" placeholder="可带 +86" required></label>
        <label>密码<input v-model="password" type="password" :autocomplete="registering ? 'new-password' : 'current-password'" :minlength="registering ? 12 : undefined" required></label>
        <p v-if="loginError" role="alert" class="error">{{ loginError }}</p>
        <button type="submit" :disabled="signingIn">{{ signingIn ? '正在提交…' : registering ? '注册' : '登录' }}</button>
        <button v-if="!operations" class="secondary" type="button" :disabled="signingIn" @click="switchMode">{{ registering ? '已有账号，去登录' : '注册账号' }}</button>
        <p v-if="!registering && !operations">忘记密码？请联系运营核实租户归属后重置。</p>
      </form>
    </section>

    <OperatorWallet v-else-if="operations" :token="token" :operator-name="operatorName" @expired="endSession(true)" />
    <template v-else>
      <div class="page-heading">
        <h1>钱包与订单</h1>
        <span v-if="account">{{ account.name }}</span>
      </div>
      <div v-if="tenantError" class="panel state">
        <p role="alert" class="error">{{ tenantError }}</p>
        <button class="secondary" type="button" @click="retry++">重新加载</button>
      </div>
      <p v-else-if="account === null" class="panel state" role="status">正在加载租户…</p>
      <template v-else>
        <section class="panel overview wallet-summary" aria-labelledby="wallet-balance">
          <div>
            <h2 id="wallet-balance">钱包余额</h2>
            <p class="balance">{{ money(account.available_minor) }}</p>
          </div>
          <p class="muted">充值、开票和售后请线下联系运营。</p>
        </section>
        <WalletRecords :token="token" @expired="endSession(true)" />
      </template>
    </template>
  </main>
</template>
