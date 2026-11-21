<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElInput, ElMessage, ElOption, ElRadioButton, ElRadioGroup, ElSelect, ElSkeleton } from 'element-plus'
import { CopyDocument, Hide, View } from '@element-plus/icons-vue'
import { getOverview } from '@/api/dashboard'
import { getSubaccountProxyConfig, listSubaccounts, type Subaccount } from '@/api/subaccounts'
import { formatGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import LinkButton from '@/shared/ui/LinkButton.vue'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const accounts = ref<Subaccount[]>()
const clientId = ref('')
const mode = ref('default')
const sid = ref('')
const revealed = ref(false)
const credentials = ref<Awaited<ReturnType<typeof getSubaccountProxyConfig>>>()
const listError = ref<unknown>()
const configError = ref<unknown>()
const balance = ref<{ kind: 'ready', bytes: bigint } | { kind: 'error', error: unknown }>()
let listRequest: AbortController | undefined
let configRequest: AbortController | undefined
let balanceRequest: AbortController | undefined

const invalidSid = computed(() => mode.value === 'explicit' && !/^[a-z0-9]{1,32}$/.test(sid.value))
const username = computed(() => {
  if (!credentials.value || invalidSid.value) return ''
  return mode.value === 'explicit' ? `${credentials.value.username}-sid-${sid.value}` : credentials.value.username
})
const mask = '••••••••'
const fields = computed(() => {
  const config = credentials.value
  return config ? [
    { key: 'host', value: config.host },
    { key: 'port', value: String(config.port) },
    { key: 'username', value: username.value },
    { key: 'password', value: config.password },
  ] : []
})

function connection(showPassword: boolean) {
  const config = credentials.value
  if (!config || !username.value) return ''
  const host = config.host.includes(':') && !config.host.startsWith('[') ? `[${config.host}]` : config.host
  const password = showPassword ? encodeURIComponent(config.password) : mask
  return `socks5h://${encodeURIComponent(username.value)}:${password}@${host}:${config.port}`
}

const curl = (showPassword: boolean) => {
  const url = connection(showPassword)
  return url ? `curl -x "${url}" https://api.ipify.org` : ''
}

async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    ElMessage.success(t('dynamicSetup.copied'))
  } catch {
    ElMessage.error(t('dynamicSetup.copyFailed'))
  }
}

async function loadCredentials(id: string) {
  configRequest?.abort()
  credentials.value = undefined
  configError.value = undefined
  revealed.value = false
  if (!id) return
  const controller = new AbortController()
  configRequest = controller
  try {
    const config = await getSubaccountProxyConfig(id, controller.signal)
    if (controller.signal.aborted) return
    if (!config.explicit_sid) mode.value = 'default'
    credentials.value = config
  } catch (error) {
    if (!controller.signal.aborted) configError.value = error
  }
}

watch(clientId, loadCredentials, { flush: 'sync' })

async function loadAccounts() {
  listRequest?.abort()
  configRequest?.abort()
  const selected = clientId.value
  clientId.value = ''
  accounts.value = undefined
  listError.value = undefined
  const controller = new AbortController()
  listRequest = controller
  try {
    const list = await listSubaccounts(controller.signal)
    if (controller.signal.aborted) return
    accounts.value = list.filter(account => account.active)
    clientId.value = accounts.value.find(account => account.clientId === selected)?.clientId ?? accounts.value[0]?.clientId ?? ''
  } catch (error) {
    if (!controller.signal.aborted) listError.value = error
  }
}

async function loadBalance() {
  balanceRequest?.abort()
  const controller = new AbortController()
  balanceRequest = controller
  balance.value = undefined
  try {
    const overview = await getOverview(controller.signal)
    if (!controller.signal.aborted) balance.value = { kind: 'ready', bytes: overview.trafficBytes }
  } catch (error) {
    if (!controller.signal.aborted) balance.value = { kind: 'error', error }
  }
}

onScopeDispose(() => {
  listRequest?.abort()
  configRequest?.abort()
  balanceRequest?.abort()
})
void loadAccounts()
void loadBalance()
</script>

<template>
  <div class="setup">
    <ElCard shadow="never">
      <template #header>
        <div class="toolbar">
          <h2>{{ t('dynamicSetup.heading') }}</h2>
          <RouterLink :to="{ name: 'console-sub-accounts' }">{{ t('dynamicSetup.manage') }}</RouterLink>
        </div>
      </template>

      <ElAlert v-if="listError" :title="errorMessage(listError)" type="error" :closable="false" show-icon>
        <ElButton size="small" @click="loadAccounts">{{ t('dynamicSetup.retry') }}</ElButton>
      </ElAlert>
      <ElSkeleton v-else-if="!accounts" animated :rows="4" />
      <ElAlert v-else-if="!accounts.length" :title="t('dynamicSetup.empty')" type="info" :closable="false" show-icon>
        <p>{{ t('dynamicSetup.emptyHint') }}</p>
        <LinkButton :to="{ name: 'console-sub-accounts' }" type="primary" size="small">{{ t('dynamicSetup.create') }}</LinkButton>
      </ElAlert>
      <template v-else>
        <div class="form-row">
          <label for="dynamic-subaccount">{{ t('dynamicSetup.account') }}</label>
          <ElSelect id="dynamic-subaccount" v-model="clientId">
            <ElOption v-for="account in accounts" :key="account.clientId" :value="account.clientId"
              :label="account.remark ? t('dynamicSetup.accountOption', { name: account.name, remark: account.remark }) : account.name" />
          </ElSelect>
        </div>
        <ElAlert v-if="configError" :title="errorMessage(configError)" type="error" :closable="false" show-icon>
          <ElButton size="small" @click="loadAccounts">{{ t('dynamicSetup.retry') }}</ElButton>
        </ElAlert>
        <ElSkeleton v-else-if="!credentials" animated :rows="4" />
        <template v-else>
          <div class="form-row">
            <span id="dynamic-session-label">{{ t('dynamicSetup.session') }}</span>
            <ElRadioGroup v-model="mode" aria-labelledby="dynamic-session-label">
              <ElRadioButton value="default">{{ t('dynamicSetup.defaultSession') }}</ElRadioButton>
              <ElRadioButton v-if="credentials.explicit_sid" value="explicit">{{ t('dynamicSetup.explicitSession') }}</ElRadioButton>
            </ElRadioGroup>
          </div>
          <div v-if="mode === 'explicit'" class="form-row">
            <label for="dynamic-sid">{{ t('dynamicSetup.sid') }}</label>
            <div>
              <ElInput id="dynamic-sid" v-model="sid" :aria-invalid="invalidSid" aria-describedby="dynamic-sid-help"
                :placeholder="t('dynamicSetup.sidFormat')" />
              <p id="dynamic-sid-help" :class="{ invalid: invalidSid }">{{ t(invalidSid ? 'dynamicSetup.sidFormat' : 'dynamicSetup.sidHelp') }}</p>
            </div>
          </div>

          <dl class="credentials">
            <div v-for="field in fields" :key="field.key" class="credential">
              <dt>{{ t('dynamicSetup.' + field.key) }}</dt>
              <dd>
                <code>{{ field.key === 'password' && !revealed ? mask : field.value || '—' }}</code>
                <ElButton v-if="field.key === 'password'" link :icon="revealed ? Hide : View"
                  :aria-label="t(revealed ? 'dynamicSetup.hide' : 'dynamicSetup.reveal')" @click="revealed = !revealed" />
                <ElButton link :icon="CopyDocument" :disabled="!field.value" type="primary"
                  :aria-label="t('dynamicSetup.copyField', { field: t('dynamicSetup.' + field.key) })" @click="copy(field.value)" />
              </dd>
            </div>
          </dl>

          <section v-for="example in [{ key: 'connection', render: connection }, { key: 'curl', render: curl }]" :key="example.key" class="example">
            <div class="toolbar">
              <h3>{{ t('dynamicSetup.' + example.key) }}</h3>
              <ElButton link :icon="CopyDocument" type="primary" :disabled="!username"
                :aria-label="t('dynamicSetup.copyField', { field: t('dynamicSetup.' + example.key) })" @click="copy(example.render(true))">
                {{ t('dynamicSetup.copy') }}
              </ElButton>
            </div>
            <code>{{ example.render(revealed) || '—' }}</code>
          </section>
        </template>
      </template>
    </ElCard>

    <div class="setup-side">
      <ElCard shadow="never">
        <template #header><h2>{{ t('dynamicSetup.balance') }}</h2></template>
        <ElSkeleton v-if="!balance" animated :rows="1" />
        <ElAlert v-else-if="balance.kind === 'error'" :title="errorMessage(balance.error)" type="error" :closable="false">
          <ElButton size="small" @click="loadBalance">{{ t('dynamicSetup.retry') }}</ElButton>
        </ElAlert>
        <strong v-else class="tabular balance">{{ t('subaccounts.gb', { value: formatGB(balance.bytes, locale) }) }}</strong>
        <p>{{ t('dynamicSetup.balanceHint') }}</p>
        <LinkButton :to="{ name: 'console-dynamic-pricing' }" type="primary">{{ t('dynamicSetup.buy') }}</LinkButton>
      </ElCard>
      <slot name="aside" />
    </div>
  </div>
</template>

<style scoped>
.setup { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 1fr); align-items: start; gap: var(--space-4); }
.setup-side { display: grid; grid-template-columns: minmax(0, 1fr); gap: var(--space-4); }
.toolbar, .credential, .form-row { display: flex; align-items: start; flex-wrap: wrap; gap: var(--space-2) var(--space-4); }
.toolbar, .credential { justify-content: space-between; align-items: center; }
.form-row { margin-block-end: var(--space-4); }
.form-row > label, .form-row > span { align-self: center; min-inline-size: var(--account-name-width); }
.el-select { max-inline-size: var(--notification-panel-width); }
h2, h3 { margin: 0; font-size: var(--font-size-lg); }
h3 { font-size: var(--font-size-md); }
p { color: var(--color-text-secondary); }
.form-row p { margin: var(--space-1) 0 0; }
.invalid { color: var(--color-danger-text); }
.credentials { margin: var(--space-4) 0; }
.credential { border-block-end: var(--border-width) solid var(--color-border); padding-block: var(--space-2); }
dd { display: flex; align-items: center; flex-wrap: wrap; gap: var(--space-2); margin: 0; min-inline-size: 0; }
code { font-family: var(--font-mono); overflow-wrap: anywhere; }
.example { padding: var(--space-3); background: var(--color-surface-muted); border-radius: var(--radius-control); margin-block-start: var(--space-3); }
.example .toolbar { margin-block-end: var(--space-2); }
.balance { font-size: var(--font-size-title); }
@media (max-width: 900px) { .setup { grid-template-columns: minmax(0, 1fr); } }
</style>
