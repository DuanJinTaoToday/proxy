<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElEmpty, ElInput, ElMessage, ElOption, ElProgress, ElSelect, ElSkeleton, ElSwitch, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { CopyDocument, Hide, Plus, Search, View } from '@element-plus/icons-vue'
import { getAccount } from '@/api/access'
import { ApiError } from '@/api/client'
import { listSubaccounts, revealSubaccountPassword, type Subaccount } from '@/api/subaccounts'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import PageHeader from '@/shared/ui/PageHeader.vue'
import SubaccountDialog, { type SubaccountAction } from './SubaccountDialog.vue'
import { formatGB } from './trafficUnits'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const quota = 10
const accounts = ref<Subaccount[]>()
const primaryAccount = ref('')
const loading = ref(false)
const failure = ref<unknown>()
const keyword = ref('')
const status = ref<'all' | 'active' | 'inactive' | 'capped'>('all')
const action = ref<SubaccountAction>()
const password = ref<{ clientId: string, value: string }>()
const passwordPending = ref<string>()
let listRequest: AbortController | undefined
let passwordRequest: AbortController | undefined

const quotaFull = computed(() => (accounts.value?.length ?? 0) >= quota)
const activeCount = computed(() => accounts.value?.filter(account => account.active).length ?? 0)
const totalUsage = computed(() => accounts.value?.reduce((sum, account) => sum + account.usedBytes, 0n) ?? 0n)
const formatUsage = (bytes: bigint) => t('subaccounts.gb', { value: formatGB(bytes, locale.value) })
const username = (account: Subaccount) => `${primaryAccount.value}.${account.name}`
const capped = (account: Subaccount) => account.limitBytes !== undefined && account.usedBytes >= account.limitBytes
const dateFormat = computed(() => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Shanghai' }))
const visible = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  return (accounts.value ?? []).filter(account =>
    (!term || username(account).includes(term) || account.remark.toLowerCase().includes(term))
    && (status.value === 'all'
      || (status.value === 'active' && account.active)
      || (status.value === 'inactive' && !account.active)
      || (status.value === 'capped' && capped(account))))
})

function usagePercent(account: Subaccount) {
  if (capped(account)) return 100
  return account.limitBytes ? Number(account.usedBytes * 100n / account.limitBytes) : 0
}

// Element Plus does not carry the table's row type through to column slots.
const isAccount = (row: unknown): row is Subaccount => visible.value.some(account => account === row)

async function loadAccounts() {
  listRequest?.abort()
  const request = new AbortController()
  listRequest = request
  loading.value = true
  failure.value = undefined
  try {
    const [list, tenant] = await Promise.all([listSubaccounts(request.signal), getAccount(request.signal)])
    accounts.value = list
    primaryAccount.value = tenant.primaryAccountName
  } catch (error) {
    if (!request.signal.aborted) failure.value = error
    request.abort()
  } finally {
    if (listRequest === request) loading.value = false
  }
}

function resetFilters() {
  keyword.value = ''
  status.value = 'all'
}

function saveAccount(saved: Subaccount) {
  const list = accounts.value ?? []
  accounts.value = list.some(account => account.clientId === saved.clientId)
    ? list.map(account => account.clientId === saved.clientId ? saved : account)
    : [...list, saved].sort((a, b) => a.name.localeCompare(b.name))
  if (action.value?.kind === 'password') clearPassword()
  action.value = undefined
  ElMessage.success(t('subaccounts.saved', { name: saved.name }))
}

async function copy(value: string) {
  try {
    await navigator.clipboard.writeText(value)
    ElMessage.success(t('subaccounts.copied'))
  } catch {
    ElMessage.error(t('subaccounts.copyFailed'))
  }
}

function clearPassword() {
  passwordRequest?.abort()
  passwordRequest = undefined
  passwordPending.value = undefined
  password.value = undefined
}

async function readPassword(account: Subaccount, purpose: 'show' | 'copy') {
  const current = password.value
  if (purpose === 'copy' && current?.clientId === account.clientId) {
    await copy(current.value)
    return
  }
  if (purpose === 'copy' && (!navigator.clipboard?.write || !window.ClipboardItem)) {
    ElMessage.error(t('subaccounts.revealToCopy'))
    return
  }
  clearPassword()
  if (purpose === 'show' && current?.clientId === account.clientId) return

  const request = new AbortController()
  passwordRequest = request
  passwordPending.value = account.clientId
  try {
    const pending = revealSubaccountPassword(account.clientId, request.signal)
    if (purpose === 'copy') {
      // Start writing during the click so Safari can keep the user gesture while the password loads.
      const content = pending.then(value => new Blob([value], { type: 'text/plain' }))
      await Promise.all([pending, navigator.clipboard.write([new ClipboardItem({ 'text/plain': content })])])
      if (!request.signal.aborted) ElMessage.success(t('subaccounts.copied'))
    } else {
      const value = await pending
      if (!request.signal.aborted) password.value = { clientId: account.clientId, value }
    }
  } catch (error) {
    if (!request.signal.aborted) {
      ElMessage.error(purpose === 'copy' && !(error instanceof ApiError) ? t('subaccounts.copyFailed') : errorMessage(error))
    }
  } finally {
    if (passwordRequest === request) {
      passwordRequest = undefined
      passwordPending.value = undefined
    }
  }
}

watch([keyword, status], clearPassword)
onScopeDispose(() => {
  listRequest?.abort()
  clearPassword()
})
void loadAccounts()
</script>

<template>
  <PageHeader :title="t('nav.subAccounts')">
    <template #actions>
      <ElButton type="primary" :icon="Plus" :disabled="!accounts || quotaFull || loading || !!failure" @click="action = { kind: 'create' }">
        {{ t('subaccounts.createAction') }}
      </ElButton>
    </template>
  </PageHeader>

  <ElAlert type="info" class="notice" :title="t('subaccounts.intro')" :closable="false" show-icon>
    <p v-if="primaryAccount">{{ t('subaccounts.usernameHint', { account: primaryAccount }) }}</p>
  </ElAlert>

  <ElAlert v-if="failure" type="error" class="notice" :title="errorMessage(failure)" :closable="false" show-icon>
    <ElButton @click="loadAccounts">{{ t('subaccounts.retry') }}</ElButton>
  </ElAlert>
  <ElSkeleton v-if="loading" :rows="4" animated :aria-label="t('subaccounts.loading')" />
  <template v-else-if="accounts && !failure">
    <ElAlert v-if="quotaFull" class="notice" type="warning" :title="t('subaccounts.quotaFull', { quota })" :closable="false" show-icon />
    <div class="metrics">
      <section class="metric">
        <h2>{{ t('subaccounts.quota') }}</h2>
        <p class="metric-value">{{ t('subaccounts.quotaValue', { used: accounts.length, quota }) }}</p>
        <p>{{ t('subaccounts.quotaHint') }}</p>
      </section>
      <section class="metric">
        <h2>{{ t('subaccounts.active') }}</h2>
        <p class="metric-value">{{ activeCount }}</p>
        <p>{{ t('subaccounts.activeHint') }}</p>
      </section>
      <section class="metric">
        <h2>{{ t('subaccounts.usage') }}</h2>
        <p class="metric-value">{{ formatUsage(totalUsage) }}</p>
        <p>{{ t('subaccounts.usageHint') }}</p>
      </section>
    </div>

    <section class="table-card" :aria-label="t('subaccounts.list')">
      <div class="toolbar">
        <ElInput v-model="keyword" class="search" :prefix-icon="Search" :placeholder="t('subaccounts.search')" :aria-label="t('subaccounts.search')" clearable />
        <ElSelect v-model="status" class="status-filter" :aria-label="t('subaccounts.filter')">
          <ElOption value="all" :label="t('subaccounts.allStatuses')" />
          <ElOption value="active" :label="t('subaccounts.active')" />
          <ElOption value="inactive" :label="t('subaccounts.inactive')" />
          <ElOption value="capped" :label="t('subaccounts.capped')" />
        </ElSelect>
      </div>

      <ElTable :data="visible" row-key="clientId">
        <ElTableColumn :label="t('subaccounts.name')" min-width="220">
          <template #default="{ row }">
            <template v-if="isAccount(row)">
              <strong>{{ row.name }}</strong>
              <div class="inline">
                <code>{{ username(row) }}</code>
                <ElButton link :icon="CopyDocument" :aria-label="t('subaccounts.copyUsername')" @click="copy(username(row))" />
              </div>
              <p class="secondary remark">{{ row.remark || t('subaccounts.noRemark') }}</p>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.password')" min-width="155">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="inline">
              <code>{{ password && password.clientId === row.clientId ? password.value : '••••••••' }}</code>
              <ElButton link :icon="password?.clientId === row.clientId ? Hide : View" :loading="passwordPending === row.clientId"
                :aria-label="password?.clientId === row.clientId ? t('subaccounts.hidePassword') : t('subaccounts.showPassword')" @click="readPassword(row, 'show')" />
              <ElButton link :icon="CopyDocument" :disabled="passwordPending === row.clientId" :aria-label="t('subaccounts.copyPassword')" @click="readPassword(row, 'copy')" />
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.usageAndLimit')" min-width="170">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="usage-cell numeric">
              <span>{{ t('subaccounts.usageValue', { used: formatUsage(row.usedBytes), limit: row.limitBytes === undefined ? t('subaccounts.unlimited') : formatUsage(row.limitBytes) }) }}</span>
              <ElProgress v-if="row.limitBytes !== undefined" :percentage="usagePercent(row)" :show-text="false"
                :status="capped(row) ? 'exception' : usagePercent(row) >= 80 ? 'warning' : undefined" />
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.status')" min-width="120">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="inline">
              <ElSwitch :model-value="row.active" :aria-label="row.active ? t('subaccounts.disableTitle', { name: row.name }) : t('subaccounts.enableTitle', { name: row.name })"
                :before-change="() => { action = { kind: 'status', account: row }; return false }" />
              <ElTag v-if="row.active && capped(row)" type="danger" size="small">{{ t('subaccounts.capped') }}</ElTag>
              <span v-else class="secondary">{{ row.active ? t('subaccounts.active') : t('subaccounts.inactive') }}</span>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.createdAt')" min-width="150">
          <template #default="{ row }">
            <template v-if="isAccount(row)">
              <time v-if="row.createdAt" :datetime="row.createdAt" class="numeric">{{ dateFormat.format(new Date(row.createdAt)) }}</time>
              <span v-else class="secondary">{{ t('subaccounts.unknownDate') }}</span>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.actions')" min-width="155">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="inline actions">
              <ElButton link type="primary" @click="action = { kind: 'edit', account: row }">{{ t('subaccounts.edit') }}</ElButton>
              <ElButton link type="primary" @click="action = { kind: 'password', account: row }">{{ t('subaccounts.changePassword') }}</ElButton>
            </div>
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty v-if="accounts.length" :description="t('subaccounts.noMatches')">
            <ElButton @click="resetFilters">{{ t('subaccounts.clearFilters') }}</ElButton>
          </ElEmpty>
          <ElEmpty v-else :description="t('subaccounts.noAccounts')">
            <ElButton type="primary" @click="action = { kind: 'create' }">{{ t('subaccounts.createFirst') }}</ElButton>
          </ElEmpty>
        </template>
      </ElTable>
      <p class="table-footer">{{ t('subaccounts.visibleCount', { visible: visible.length, total: accounts.length }) }}</p>
    </section>
  </template>

  <SubaccountDialog v-if="action" :action="action" :primary-account="primaryAccount"
    @close="action = undefined" @saved="saveAccount" @quota-reached="loadAccounts" />
</template>

<style scoped>
.notice { margin-bottom: var(--space-4); }
.metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--subaccount-metric-width)), 1fr));
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}
.metric, .table-card {
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  background: var(--color-surface);
  box-shadow: var(--shadow-card);
}
.metric { padding: var(--space-4); color: var(--color-text-secondary); }
.metric h2 { font-size: var(--font-size-sm); font-weight: var(--font-weight-medium); }
.metric-value { margin-block: var(--space-1); color: var(--color-text); font-size: var(--font-size-title); font-weight: var(--font-weight-bold); }
.table-card { min-width: 0; overflow: hidden; }
.toolbar { display: flex; flex-wrap: wrap; gap: var(--space-3); padding: var(--space-4); }
.search, .status-filter { width: min(100%, var(--subaccount-metric-width)); }
.inline { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-1); }
.inline > button { margin: 0; }
.actions { column-gap: var(--space-3); }
.usage-cell { display: grid; gap: var(--space-1); }
.metric-value, .numeric { font-variant-numeric: tabular-nums; }
.secondary, .table-footer { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.remark { white-space: pre-wrap; overflow-wrap: anywhere; }
.table-footer { padding: var(--space-3) var(--space-4); }
</style>
