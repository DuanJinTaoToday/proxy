<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ChartColumn, CheckCircle2, CircleHelp, House, List, Network, Package, Plus, ReceiptText } from 'lucide-vue-next'
import { DialogDescription, DialogTrigger } from 'reka-ui'
import DashboardShell from '@console/DashboardShell.vue'
import TenantSubaccounts from '@console/TenantSubaccounts.vue'
import { listSubaccounts } from '@console/api/subaccounts'
import type { Subaccount } from '@console/api/subaccounts'
import { Button } from '@console/ui/button'
import Card from '@console/ui/card/Card.vue'
import Dialog from '@console/ui/dialog/Dialog.vue'
import DialogContent from '@console/ui/dialog/DialogContent.vue'
import DialogTitle from '@console/ui/dialog/DialogTitle.vue'
import CreateForm from './CreateForm.vue'

const route = useRoute()
const router = useRouter()
const variants = [
  { key: 'current', label: '现状' },
  { key: 'a', label: 'A 抽屉' },
  { key: 'b', label: 'B 独立页' },
]
const variant = computed(() => variants.find(option => option.key === route.query.variant)?.key ?? 'current')
const dedicated = computed(() => variant.value === 'b' && route.query.create === '1')
const drawer = ref(false)
const accounts = ref<Subaccount[]>([])
const state = ref<'loading' | 'ready' | 'error'>('loading')
const createdName = ref('')
const receipt = ref<HTMLElement>()
const launch = ref<HTMLElement>()

async function load() {
  state.value = 'loading'
  try {
    accounts.value = (await listSubaccounts('prototype', new AbortController().signal)).accounts ?? []
    state.value = 'ready'
  } catch {
    state.value = 'error'
  }
}

watch(variant, () => {
  drawer.value = false
  createdName.value = ''
  void load()
})
onMounted(load)

async function startDedicated() {
  createdName.value = ''
  await router.push({ query: { variant: 'b', create: '1' } })
  document.querySelector<HTMLInputElement>('.prototype-create input')?.focus()
}

async function leaveDedicated() {
  await router.replace({ query: { variant: 'b' } })
  launch.value?.querySelector('button')?.focus()
}

async function created(account: Subaccount) {
  accounts.value = [account, ...accounts.value]
  createdName.value = account.name
  if (dedicated.value) await router.replace({ query: { variant: 'b' } })
  else drawer.value = false
  await nextTick()
  receipt.value?.focus()
}

function gb(value: string) {
  const bytes = BigInt(value)
  const unit = 1_000_000_000n
  const fraction = String(bytes % unit).padStart(9, '0').replace(/0+$/, '')
  return String(bytes / unit) + (fraction ? '.' + fraction : '')
}

function switchWithArrow(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight'].includes(event.key)) return
  event.preventDefault()
  const step = event.key === 'ArrowLeft' ? -1 : 1
  const index = variants.findIndex(option => option.key === variant.value)
  const next = variants[(index + step + variants.length) % variants.length]
  if (next) void router.replace({ query: { variant: next.key } })
}
</script>

<template>
  <DashboardShell brand="住宅代理" area="租户控制台" :title="dedicated ? '创建子账号' : '代理子账号'" account-name="示例租户">
    <template #navigation="{ closeSidebar }">
      <nav aria-label="租户导航" class="prototype-tenant-nav">
        <Button variant="ghost" disabled><House />账户概览</Button>
        <p class="nav-label">动态住宅代理</p>
        <Button variant="ghost" disabled><Package />流量套餐</Button>
        <Button as-child variant="ghost"><RouterLink :to="{ query: { variant } }" aria-current="page" @click="closeSidebar"><Network />代理子账号</RouterLink></Button>
        <Button variant="ghost" disabled><ChartColumn />日用量</Button>
        <p class="nav-label">静态住宅代理</p>
        <Button variant="ghost" disabled><Package />购买静态 IP</Button>
        <Button variant="ghost" disabled><Network />我的 IP</Button>
        <p class="nav-label">钱包与订单</p>
        <Button variant="ghost" disabled><List />钱包流水</Button>
        <Button variant="ghost" disabled><ReceiptText />订单记录</Button>
        <p class="nav-label">官网</p>
        <Button variant="ghost" disabled><CircleHelp />SOCKS5 接入说明</Button>
      </nav>
    </template>

    <TenantSubaccounts v-if="variant === 'current'" token="prototype" primary-account="demo" />

    <section v-else-if="dedicated" class="prototype-create">
      <Button variant="ghost" type="button" class="prototype-back" @click="leaveDedicated"><ArrowLeft :size="16" />返回代理子账号</Button>
      <Card class="prototype-create-card">
        <div class="prototype-create-intro">
          <span class="prototype-icon"><Network :size="22" aria-hidden="true" /></span>
          <h2>账号信息</h2>
          <p class="prototype-help">为动态住宅代理设置独立账密。创建后可在子账号列表查看。</p>
          <p class="prototype-help prototype-slots">已使用 <strong>{{ accounts.length }} / 10</strong> 个名额</p>
        </div>
        <CreateForm @cancel="leaveDedicated" @created="created" />
      </Card>
    </section>

    <Dialog v-else v-model:open="drawer">
      <div class="prototype-stack">
        <div v-if="variant === 'b'" ref="launch" class="prototype-launch">
          <span class="prototype-icon"><Network :size="22" aria-hidden="true" /></span>
          <div><h2>管理动态代理接入</h2><p class="prototype-help">为不同项目分配独立账密和用量上限。</p></div>
          <Button type="button" class="prototype-primary" :disabled="state !== 'ready' || accounts.length >= 10" @click="startDedicated"><Plus :size="16" />创建子账号</Button>
        </div>

        <div v-if="createdName" ref="receipt" class="prototype-success" role="status" tabindex="-1">
          <CheckCircle2 :size="19" aria-hidden="true" /><span>已创建子账号 <strong>{{ createdName }}</strong>。</span>
        </div>

        <Card class="prototype-list">
          <div class="prototype-list-head">
            <div><h2>子账号列表</h2><p class="prototype-help">已使用 {{ accounts.length }} / 10 个名额</p></div>
            <DialogTrigger v-if="variant === 'a'" as-child>
              <Button type="button" class="prototype-primary" :disabled="state !== 'ready' || accounts.length >= 10" @click="createdName = ''"><Plus :size="16" />创建子账号</Button>
            </DialogTrigger>
          </div>
          <p v-if="state === 'loading'" class="prototype-list-state" role="status">正在加载子账号…</p>
          <div v-else-if="state === 'error'" class="prototype-list-state"><p role="alert">暂时无法加载子账号。</p><Button type="button" @click="load">重新加载</Button></div>
          <p v-else-if="!accounts.length" class="prototype-list-state">暂无子账号。</p>
          <template v-else>
            <div class="prototype-row prototype-columns" aria-hidden="true"><span>名称 / 备注</span><span>状态</span><span>累计用量 / 上限</span></div>
            <ul class="prototype-rows" aria-label="子账号">
              <li v-for="account in accounts" :key="account.client_id" class="prototype-row" :class="{ 'prototype-new': account.name === createdName }">
                <span><strong>{{ account.name }}</strong><small v-if="account.remark">{{ account.remark }}</small></span>
                <span class="prototype-badge" :class="{ 'prototype-active': account.active }">{{ account.active ? '启用' : '禁用' }}</span>
                <span class="prototype-amount"><span class="prototype-amount-label">累计用量 / 上限</span>{{ gb(account.used_bytes ?? '0') }} GB / {{ account.limit_bytes === undefined ? '不限' : gb(account.limit_bytes) + ' GB' }}</span>
              </li>
            </ul>
          </template>
        </Card>
      </div>
      <DialogContent v-if="variant === 'a'" class="prototype-drawer" :show-close-button="false"
        @close-auto-focus="event => { if (createdName) { event.preventDefault(); receipt?.focus() } }">
        <header class="prototype-drawer-header">
          <DialogTitle>创建子账号</DialogTitle>
          <DialogDescription class="prototype-help">为动态住宅代理设置独立账密。</DialogDescription>
        </header>
        <CreateForm @cancel="drawer = false" @created="created" />
      </DialogContent>
    </Dialog>
  </DashboardShell>

  <nav class="prototype-switch" aria-label="原型切换" @keydown="switchWithArrow">
    <span>交互原型</span>
    <RouterLink v-for="option in variants" :key="option.key" :to="{ query: { variant: option.key } }" replace
      :aria-current="variant === option.key ? 'page' : undefined">{{ option.label }}</RouterLink>
  </nav>
</template>
