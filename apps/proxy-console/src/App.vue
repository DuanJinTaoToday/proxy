<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ApiError, listTenants, login } from './api/control'
import type { Tenant } from './api/control'
import UsageDashboard from './UsageDashboard.vue'

const route = useRoute()
const router = useRouter()
const sessionKey = 'proxy-console.session'
const token = ref(sessionStorage.getItem(sessionKey) ?? '')
const username = ref('')
const password = ref('')
const signingIn = ref(false)
const loginError = ref('')
const tenantError = ref('')
const tenants = ref<Tenant[] | null>(null)
const retry = ref(0)

const routeTenant = computed(() => typeof route.params.tenantId === 'string' ? route.params.tenantId : '')
const tenant = computed(() => tenants.value?.find((item) => item.id === routeTenant.value))

function setSession(value: string) {
  if (value) sessionStorage.setItem(sessionKey, value)
  else sessionStorage.removeItem(sessionKey)
  token.value = value
}

function endSession(expired = false) {
  setSession('')
  loginError.value = expired ? '登录已失效，请重新登录。' : ''
  if (!expired) void router.replace({ name: 'home' })
}

async function signIn() {
  if (signingIn.value) return
  signingIn.value = true
  loginError.value = ''
  try {
    setSession(await login(username.value.trim(), password.value))
  } catch (error) {
    loginError.value = error instanceof ApiError && error.status === 401
      ? '用户名或密码错误。'
      : '登录失败，请稍后重试。'
  } finally {
    password.value = ''
    signingIn.value = false
  }
}

watch([token, retry], async ([session], _, onCleanup) => {
  tenants.value = null
  tenantError.value = ''
  if (!session) return

  const controller = new AbortController()
  onCleanup(() => controller.abort())
  try {
    const result = await listTenants(session, controller.signal)
    if (!controller.signal.aborted) tenants.value = result
  } catch (error) {
    if (controller.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) endSession(true)
    else tenantError.value = '无法加载租户列表。'
  }
}, { immediate: true, flush: 'sync' })

watch([tenants, routeTenant], ([items, id]) => {
  const first = items?.[0]
  if (first && !id) void router.replace({ name: 'usage', params: { tenantId: first.id } })
})

function switchTenant(event: Event) {
  const id = (event.target as HTMLSelectElement).value
  void router.push({ name: 'usage', params: { tenantId: id } })
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
      <h1>登录控制台</h1>
      <form class="login-form" @submit.prevent="signIn">
        <label>用户名<input v-model="username" autocomplete="username" required></label>
        <label>密码<input v-model="password" type="password" autocomplete="current-password" required></label>
        <p v-if="loginError" role="alert" class="error">{{ loginError }}</p>
        <button type="submit" :disabled="signingIn">{{ signingIn ? '正在登录…' : '登录' }}</button>
      </form>
    </section>

    <template v-else>
      <div class="page-heading">
        <h1>日用量</h1>
        <label v-if="tenants?.length" class="tenant-picker">
          租户
          <select :value="tenant?.id ?? ''" @change="switchTenant">
            <option v-if="!tenant" value="" disabled>请选择租户</option>
            <option v-for="item in tenants" :key="item.id" :value="item.id">{{ item.name }}</option>
          </select>
        </label>
      </div>
      <div v-if="tenantError" class="panel state">
        <p role="alert" class="error">{{ tenantError }}</p>
        <button class="secondary" type="button" @click="retry++">重新加载</button>
      </div>
      <p v-else-if="tenants === null" class="panel state" role="status">正在加载租户…</p>
      <p v-else-if="routeTenant && !tenant" class="panel state" role="alert">你没有权限查看此租户。</p>
      <p v-else-if="!tenants.length" class="panel state" role="status">当前账号没有可访问的租户。</p>
      <UsageDashboard v-else-if="tenant" :key="tenant.id" :tenant-id="tenant.id" :token="token" @expired="endSession(true)" />
    </template>
  </main>
</template>
