<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ApiError } from './api/control'
import { createSubaccount, getSubaccountConfig, listSubaccounts, revealSubaccountPassword, updateSubaccount } from './api/subaccounts'
import type { ProxyConfig, Subaccount } from './api/subaccounts'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'
import Input from './ui/input/Input.vue'
import FieldLabel from './ui/field/FieldLabel.vue'
import Table from './ui/table/Table.vue'
import TableBody from './ui/table/TableBody.vue'
import TableCell from './ui/table/TableCell.vue'
import TableHead from './ui/table/TableHead.vue'
import TableHeader from './ui/table/TableHeader.vue'
import TableRow from './ui/table/TableRow.vue'

const props = defineProps<{ token: string; primaryAccount: string; startCreating?: boolean }>()
const emit = defineEmits<{ expired: [] }>()
const controller = new AbortController()
const accounts = ref<Subaccount[]>([])
const loading = ref(true)
const busy = ref(false)
const error = ref('')
const notice = ref('')
const editing = ref<Subaccount | 'new' | null>(null)
const name = ref('')
const password = ref('')
const remark = ref('')
const active = ref(true)
const limit = ref('')
const maxAccounts = 10
const GB = 1_000_000_000n

function asGB(bytes: string) {
  const value = BigInt(bytes)
  const fraction = String(value % GB).padStart(9, '0').replace(/0+$/, '')
  return String(value / GB) + (fraction ? '.' + fraction : '')
}

function asBytes(gb: string) {
  const [whole, fraction = ''] = gb.split('.')
  return String(BigInt(whole) * GB + BigInt(fraction.padEnd(9, '0')))
}

const credential = ref<{ username: string; password: string; config?: ProxyConfig } | null>(null)
const sessionID = ref('')
const sessionTime = ref('')
const invalidSessionID = computed(() => sessionID.value !== '' && !/^[a-z0-9]{1,32}$/.test(sessionID.value))
const invalidSessionTime = computed(() => sessionTime.value !== '' && !/^[1-9][0-9]{0,8}$/.test(sessionTime.value))
const connectionUsername = computed(() => {
  const config = credential.value?.config
  if (!config?.explicit_sidtime || invalidSessionID.value || invalidSessionTime.value) return credential.value?.username ?? ''
  return config.username + (sessionID.value ? '-sid-' + sessionID.value : '') +
    (sessionTime.value ? '-sidtime-' + sessionTime.value : '')
})


function showError(cause: unknown) {
  if (controller.signal.aborted) return
  if (cause instanceof ApiError) {
    if (cause.status === 401) {
      emit('expired')
      return
    }
    if (cause.reason === 'SUBACCOUNT_NAME_EXISTS') {
      error.value = '该子账号名称已存在。'
      return
    }
    if (cause.reason === 'SUBACCOUNT_LIMIT_REACHED') {
      error.value = '已达到 10 个子账号上限。'
      return
    }
    if (cause.reason === 'INVALID_SUBACCOUNT_LIMIT') {
      error.value = '累计上限只能调高或取消。'
      return
    }
    if (cause.status === 400) {
      error.value = '请检查子账号名称、密码和累计上限。'
      return
    }
  }
  error.value = '操作失败，请稍后重试。'
}

async function reload() {
  loading.value = true
  try {
    accounts.value = (await listSubaccounts(props.token, controller.signal)).accounts ?? []
  } catch (cause) {
    showError(cause)
  } finally {
    loading.value = false
  }
}

function edit(account?: Subaccount) {
  credential.value = null
  editing.value = account ?? 'new'
  name.value = account?.name ?? ''
  password.value = ''
  active.value = account ? Boolean(account.active) : true
  remark.value = account?.remark ?? ''
  limit.value = account?.limit_bytes === undefined ? '' : asGB(account.limit_bytes)
  error.value = ''
  notice.value = ''
}

function cancel() {
  editing.value = null
  password.value = ''
}

async function save() {
  const account = editing.value
  if (!account || busy.value) return
  busy.value = true
  error.value = ''
  try {
    const limit_bytes = limit.value ? asBytes(limit.value) : undefined
    if (account === 'new') {
      await createSubaccount(props.token, { name: name.value, password: password.value, remark: remark.value, active: active.value, limit_bytes })
    } else {
      const newPassword = password.value || undefined
      await updateSubaccount(props.token, account.client_id, {
        password: newPassword, remark: remark.value, active: active.value, limit_bytes,
        update_mask: newPassword ? 'remark,active,limitBytes,password' : 'remark,active,limitBytes',
      })
    }
    cancel()
    notice.value = '已保存。'
    await reload()
  } catch (cause) {
    showError(cause)
  } finally {
    busy.value = false
  }
}

async function reveal(account: Subaccount, config: boolean) {
  if (busy.value) return
  cancel()
  credential.value = null
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    if (config) {
      const result = await getSubaccountConfig(props.token, account.client_id, controller.signal)
      sessionID.value = ''
      sessionTime.value = ''
      credential.value = { username: result.username, password: result.password, config: result }
    } else {
      const result = await revealSubaccountPassword(props.token, account.client_id, controller.signal)
      credential.value = { username: props.primaryAccount + '.' + account.name, password: result.password }
    }
  } catch (cause) {
    showError(cause)
  } finally {
    busy.value = false
  }
}

async function copyConfig(config: ProxyConfig) {
  if (invalidSessionID.value || invalidSessionTime.value) return
  try {
    const host = config.host.includes(':') ? '[' + config.host + ']' : config.host
    await navigator.clipboard.writeText('socks5://' + connectionUsername.value + ':' + config.password + '@' + host + ':' + config.port)
    notice.value = '已复制接入配置。'
  } catch {
    error.value = '无法访问剪贴板，请手动复制。'
  }
}

onMounted(async () => {
  await reload()
  if (!controller.signal.aborted && !error.value && props.startCreating && accounts.value.length < maxAccounts) edit()
})
onBeforeUnmount(() => {
  controller.abort()
  password.value = ''
  credential.value = null
})
</script>

<template>
  <section class="grid gap-4" aria-label="代理子账号">
    <Card>
      <CardHeader class="flex flex-row items-center justify-between gap-3">
        <h2 class="text-base font-semibold">代理子账号（{{ accounts.length }}/10）</h2>
        <Button type="button" :disabled="loading || busy || accounts.length >= maxAccounts" @click="edit()">创建子账号</Button>
      </CardHeader>
      <CardContent class="overflow-x-auto">
        <p v-if="loading" role="status">正在加载子账号…</p>
        <p v-else-if="!accounts.length">暂无子账号。</p>
        <Table v-else class="w-full min-w-[650px] text-left text-sm">
          <TableHeader><TableRow><TableHead>名称</TableHead><TableHead>备注</TableHead><TableHead>状态</TableHead><TableHead>累计用量 / 上限（GB）</TableHead><TableHead>操作</TableHead></TableRow></TableHeader>
          <TableBody>
            <TableRow v-for="account in accounts" :key="account.client_id" class="border-t">
              <TableCell class="py-2 pr-4">{{ account.name }}</TableCell><TableCell class="max-w-40 truncate pr-4" :title="account.remark">{{ account.remark || '—' }}</TableCell>
              <TableCell class="pr-4">{{ account.active ? '启用' : '禁用' }}</TableCell>
              <TableCell class="pr-4">{{ asGB(account.used_bytes ?? '0') }} / {{ account.limit_bytes === undefined ? '不限' : asGB(account.limit_bytes) }}</TableCell>
              <TableCell class="whitespace-nowrap">
                <Button type="button" size="sm" variant="ghost" :disabled="busy" @click="edit(account)">编辑</Button>
                <Button type="button" size="sm" variant="ghost" :disabled="busy" @click="reveal(account, false)">查看账密</Button>
                <Button v-if="account.active" type="button" size="sm" variant="ghost" :disabled="busy" @click="reveal(account, true)">接入配置</Button>
              </TableCell>
            </TableRow>
          </TableBody>
        </Table>
      </CardContent>
    </Card>

    <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
    <p v-if="notice" role="status" class="text-sm">{{ notice }}</p>

    <Card v-if="editing">
      <CardHeader><h2 class="font-semibold">{{ editing === 'new' ? '创建子账号' : '编辑 ' + editing.name }}</h2></CardHeader>
      <CardContent>
        <form class="grid max-w-lg gap-3" @submit.prevent="save">
          <div v-if="editing === 'new'" class="grid gap-1">
            <FieldLabel for="subaccount-name">名称（4–8 位，以小写字母开头，其余为小写字母或数字）</FieldLabel>
            <Input id="subaccount-name" v-model="name" pattern="[a-z][a-z0-9]{3,7}" required />
          </div>
          <div class="grid gap-1">
            <FieldLabel for="subaccount-password">{{ editing === 'new' ? '密码' : '新密码（留空时保留原密码）' }}</FieldLabel>
            <Input id="subaccount-password" v-model="password" type="password" autocomplete="new-password"
              pattern="(?=.*[A-Za-z])(?=.*[0-9])[A-Za-z0-9]{6,12}" :required="editing === 'new'" />
            <span class="text-sm text-muted-foreground">6–12 位，须包含英文字母和数字。</span>
          </div>
          <div class="grid gap-1">
            <FieldLabel for="subaccount-remark">备注</FieldLabel>
            <Input id="subaccount-remark" v-model="remark" maxlength="512" />
          </div>
          <div class="grid gap-1">
            <FieldLabel for="subaccount-limit">累计上限（GB，留空不限）</FieldLabel>
            <Input id="subaccount-limit" v-model="limit" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,9})?" />
          </div>
          <label class="flex items-center gap-2"><input v-model="active" type="checkbox">启用</label>
          <div class="flex gap-2">
            <Button type="submit" :disabled="busy">保存</Button>
            <Button type="button" variant="outline" :disabled="busy" @click="cancel">取消</Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <Card v-if="credential">
      <CardHeader class="flex flex-row items-center justify-between gap-3">
        <h2 class="font-semibold">{{ credential.config ? 'SOCKS5 接入配置' : '子账号账密' }}</h2>
        <Button type="button" variant="ghost" @click="credential = null">隐藏</Button>
      </CardHeader>
      <CardContent class="grid gap-3 break-all text-sm">
        <p v-if="credential.config">网关地址 {{ credential.config.host }}，端口 {{ credential.config.port }}</p>
        <p>用户名 <code>{{ connectionUsername }}</code></p>
        <p>密码 <code>{{ credential.password }}</code></p>
        <template v-if="credential.config?.explicit_sidtime">
          <div v-if="credential.config.explicit_sid" class="grid gap-1">
            <FieldLabel for="connection-sid">会话标识 SID（可选）</FieldLabel>
            <Input id="connection-sid" v-model="sessionID" maxlength="32" :aria-invalid="invalidSessionID" />
            <p v-if="invalidSessionID" role="alert">请输入 1–32 位小写字母或数字。</p>
          </div>
          <div class="grid gap-1">
            <FieldLabel for="connection-sidtime">会话时长 sidtime（可选）</FieldLabel>
            <Input id="connection-sidtime" v-model="sessionTime" inputmode="numeric" maxlength="9" :aria-invalid="invalidSessionTime" />
            <p v-if="invalidSessionTime" role="alert">请输入 1–9 位正整数，首位不能为 0。</p>
          </div>
          <p>留空使用悟空默认行为。实际时长、到期处理和出口 IP 由悟空决定，重连可能更换出口 IP。</p>
        </template>
        <p v-else-if="credential.config?.explicit_sid">可在用户名后追加 <code>-sid-</code> 和 1–32 位小写字母或数字。</p>
        <Button v-if="credential.config" type="button" variant="outline" class="w-fit" :disabled="invalidSessionID || invalidSessionTime" @click="copyConfig(credential.config)">复制接入配置</Button>
      </CardContent>
    </Card>
  </section>
</template>
