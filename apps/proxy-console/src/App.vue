<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowRight, Layers3, ReceiptText, Search, Wallet } from 'lucide-vue-next'
import { ApiError, login, loginOperator, register } from './api/control'
import { getWallet } from './api/wallet'
import type { WalletTenant } from './api/wallet'
import OperatorWallet from './OperatorWallet.vue'
import TenantWallet from './TenantWallet.vue'
import LoginForm from './LoginForm.vue'
import kunxingMark from './assets/kunxing-mark.png'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'
import './ui/theme.css'

const { operations } = defineProps<{ operations: boolean }>()
const sessionKey = operations ? 'proxy-console.operations-session' : 'proxy-console.session'
const operatorKey = 'proxy-console.operations-account'
const token = ref(sessionStorage.getItem(sessionKey) ?? '')
const operatorName = ref(operations ? sessionStorage.getItem(operatorKey) ?? '' : '')
const phone = ref('')
const route = useRoute()
const router = useRouter()
const registering = computed(() => !operations && route.query.register === '1')
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
  const username = phone.value.trim()
  const submittedPassword = password.value
  signingIn.value = true
  loginError.value = ''
  try {
    if (registering.value) {
      await register(username, submittedPassword)
      if (route.name === 'console' && registering.value) {
        await router.replace({ query: { ...route.query, register: undefined }, hash: route.hash })
      }
    }
    const authenticate = operations ? loginOperator : login
    const session = await authenticate(username, submittedPassword)
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

watch(registering, () => {
  password.value = ''
  loginError.value = ''
})

function switchMode() {
  return router.push({
    query: { ...route.query, register: registering.value ? undefined : '1' },
    hash: route.hash,
  })
}
</script>

<template>
  <OperatorWallet v-if="operations && token" :token="token" :operator-name="operatorName"
    @logout="endSession()" @expired="endSession(true)" />
  <TenantWallet v-else-if="!operations && token && account" :token="token" :account="account"
    @logout="endSession()" @expired="endSession(true)" @wallet="account = $event" />
  <main v-else id="console-auth-main" tabindex="-1" class="login-block" :class="{ 'tenant-login': !operations }">
    <a class="skip-link" href="#console-auth-main">跳到主要内容</a>
    <div class="login-aside">
      <RouterLink class="login-brand" to="/">
        <Layers3 v-if="operations" :size="26" aria-hidden="true" />
        <img v-else :src="kunxingMark" alt="" width="36" height="36" />
        {{ operations ? '住宅代理' : '鲲星代理' }}
      </RouterLink>
      <div v-if="operations" class="login-intro" data-enter>
        <p>管理订单与商品</p>
        <ul>
          <li><Search :size="19" />租户查询</li>
          <li><Wallet :size="19" />钱包充值</li>
          <li><ReceiptText :size="19" />静态 IP 订单</li>
        </ul>
      </div>
      <div v-else class="tenant-login-intro">
        <p>在一个控制台管理<span>住宅代理</span></p>
        <ul>
          <li><ArrowRight :size="17" aria-hidden="true" />动态住宅流量按 GB 购买</li>
          <li><ArrowRight :size="17" aria-hidden="true" />静态住宅 IP 按租期购买</li>
          <li><ArrowRight :size="17" aria-hidden="true" />管理代理子账号与接入配置</li>
          <li><ArrowRight :size="17" aria-hidden="true" />查看用量、账单和订单</li>
        </ul>
      </div>
    </div>
    <div class="login-workspace">
      <div class="login-content">
        <LoginForm v-if="!token" v-model:account="phone" v-model:password="password" :operations="operations"
          :registering="registering" :submitting="signingIn" :error="loginError" @submit="signIn" @switch-mode="switchMode" />
        <Card v-else class="border-solid">
          <CardHeader><h1 class="text-xl tracking-normal">租户控制台</h1></CardHeader>
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
    </div>
  </main>
</template>

<style scoped src="./login.css"></style>
