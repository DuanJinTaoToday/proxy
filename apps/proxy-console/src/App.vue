<script setup lang="ts">
import { ref, watch } from 'vue'
import { GalleryVerticalEnd } from 'lucide-vue-next'
import { ApiError, login, loginOperator, register } from './api/control'
import { getWallet } from './api/wallet'
import type { WalletTenant } from './api/wallet'
import OperatorWallet from './OperatorWallet.vue'
import TenantWallet from './TenantWallet.vue'
import LoginForm from './LoginForm.vue'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'
import './ui/theme.css'

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
  <OperatorWallet v-if="operations && token" :token="token" :operator-name="operatorName"
    @logout="endSession()" @expired="endSession(true)" />
  <TenantWallet v-else-if="!operations && token && account" :token="token" :account="account"
    @logout="endSession()" @expired="endSession(true)" @wallet="account = $event" />
  <main v-else id="console-auth-main" tabindex="-1"
    class="login-block bg-muted text-foreground flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
    <a class="skip-link" href="#console-auth-main">跳到主要内容</a>
    <div class="flex w-full max-w-sm flex-col gap-6">
      <RouterLink v-if="!operations" to="/products#socks5" class="self-center text-sm underline">官网 SOCKS5 接入说明</RouterLink>
      <div class="flex items-center gap-2 self-center font-medium">
        <span class="bg-primary text-primary-foreground flex size-6 items-center justify-center rounded-md">
          <GalleryVerticalEnd class="size-4" />
        </span>
        {{ operations ? '运营控制台' : '代理控制台' }}
      </div>
      <LoginForm v-if="!token" v-model:account="phone" v-model:password="password" :operations="operations"
        :registering="registering" :submitting="signingIn" :error="loginError" @submit="signIn" @switch-mode="switchMode" />
      <Card v-else class="border-solid">
        <CardHeader class="text-center"><h1 class="text-xl tracking-normal">租户控制台</h1></CardHeader>
        <CardContent class="flex flex-col gap-3">
          <template v-if="tenantError">
            <p role="alert" class="text-sm text-destructive">{{ tenantError }}</p>
            <Button type="button" class="min-h-0 border-0" @click="retry++">重新加载</Button>
          </template>
          <p v-else class="text-sm text-muted-foreground" role="status">正在加载租户…</p>
          <Button type="button" variant="outline" class="min-h-0" @click="endSession()">退出登录</Button>
        </CardContent>
      </Card>
    </div>
  </main>
</template>
