<script setup lang="ts">
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElEmpty, ElInput, ElMessage, ElOption, ElProgress, ElSelect, ElSkeleton, ElSwitch, ElTable, ElTableColumn, ElTag, ElTooltip } from 'element-plus'
import { CopyDocument, Hide, Plus, Search, View } from '@element-plus/icons-vue'
import { getAccount } from '@/api/access'
import { ApiError } from '@/api/client'
import { listSubaccounts, revealSubaccountPassword, updateSubaccount, type Subaccount } from '@/api/subaccounts'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import PageHeader from '@/shared/ui/PageHeader.vue'
import SubaccountDialog, { type SubaccountAction } from './SubaccountDialog.vue'
import { formatGB } from '@/shared/lib/trafficUnits'

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
const enabling = ref<string>()
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
const dateFormat = computed(() => new Intl.DateTimeFormat(locale.value, {
  year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  hourCycle: 'h23', timeZone: 'Asia/Shanghai',
}))
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
  const kind = action.value?.kind
  replaceAccount(saved)
  if (kind === 'password') clearPassword()
  action.value = undefined
  const message = kind === 'create' ? 'created' : kind === 'password' ? 'passwordSaved' : kind === 'status' ? 'disabled' : 'saved'
  ElMessage.success(t(`subaccounts.${message}`, { name: saved.name }))
}

function replaceAccount(saved: Subaccount) {
  const list = accounts.value ?? []
  accounts.value = list.some(account => account.clientId === saved.clientId)
    ? list.map(account => account.clientId === saved.clientId ? saved : account)
    : [...list, saved].sort((a, b) => a.name.localeCompare(b.name))
}

async function changeStatus(account: Subaccount) {
  if (account.active) {
    action.value = { kind: 'status', account }
    return
  }
  enabling.value = account.clientId
  try {
    const saved = await updateSubaccount(account.clientId, { kind: 'details', active: true })
    replaceAccount(saved)
    ElMessage.success(t('subaccounts.enabled', { name: saved.name }))
  } catch (error) {
    ElMessage.error(errorMessage(error))
  } finally {
    enabling.value = undefined
  }
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
      <ElTooltip :disabled="!quotaFull" :content="t('subaccounts.quotaFull', { quota })" :trigger="['hover', 'focus']" placement="bottom-end">
        <span :tabindex="quotaFull ? 0 : undefined" :aria-label="quotaFull ? t('subaccounts.quotaFull', { quota }) : undefined">
          <ElButton class="create-button" type="primary" :icon="Plus" :disabled="!accounts || quotaFull || loading || !!failure" @click="action = { kind: 'create' }">
            {{ t('subaccounts.createAction') }}
          </ElButton>
        </span>
      </ElTooltip>
    </template>
  </PageHeader>

  <ElAlert type="info" class="notice introduction" :title="t('subaccounts.intro')" :closable="false" show-icon>
    <i18n-t v-if="primaryAccount" keypath="subaccounts.usernameHint" tag="p" scope="global">
      <template #username><code>{{ primaryAccount }}.{{ t('subaccounts.name') }}</code></template>
      <template #session><code>{{ t('subaccounts.sessionSuffix') }}</code></template>
    </i18n-t>
  </ElAlert>

  <ElAlert v-if="failure" type="error" class="notice" :title="errorMessage(failure)" :closable="false" show-icon>
    <ElButton @click="loadAccounts">{{ t('subaccounts.retry') }}</ElButton>
  </ElAlert>
  <ElSkeleton v-if="loading" :rows="4" animated :aria-label="t('subaccounts.loading')" />
  <template v-else-if="accounts && !failure">
    <div class="metrics" role="group" :aria-label="t('subaccounts.overview')">
      <section class="metric">
        <h2>{{ t('subaccounts.quota') }}</h2>
        <i18n-t keypath="subaccounts.quotaValue" class="metric-value" tag="p" scope="global">
          <template #used>{{ accounts.length }}</template>
          <template #quota><span class="metric-unit">/ {{ quota }}</span></template>
        </i18n-t>
        <p class="metric-hint">{{ t('subaccounts.quotaHint') }}</p>
      </section>
      <section class="metric">
        <h2>{{ t('subaccounts.activeMetric') }}</h2>
        <p class="metric-value">{{ activeCount }} <span class="metric-unit">{{ t('subaccounts.accountUnit', activeCount) }}</span></p>
        <p class="metric-hint">{{ t('subaccounts.activeHint') }}</p>
      </section>
      <section class="metric">
        <h2>{{ t('subaccounts.usage') }}</h2>
        <p class="metric-value">{{ formatUsage(totalUsage) }}</p>
        <p class="metric-hint">{{ t('subaccounts.usageHint') }}</p>
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

      <ElTable :data="visible" class="accounts-table" row-key="clientId"
        :header-cell-style="{ height: 'var(--tabs-height)', fontSize: 'var(--font-size-xs)', fontWeight: 'var(--font-weight-semibold)' }"
        :cell-style="{ height: 'calc(var(--space-6) + var(--space-5) + 2 * var(--border-width))' }">
        <ElTableColumn :label="t('subaccounts.account')" min-width="230">
          <template #default="{ row }">
            <template v-if="isAccount(row)">
              <span class="inline">
                <strong class="account-name">{{ row.name }}</strong>
                <ElTooltip :content="t('subaccounts.copyUsernameHint', { username: username(row) })" :trigger="['hover', 'focus']" placement="top">
                  <ElButton link :icon="CopyDocument" :aria-label="t('subaccounts.copyUsernameHint', { username: username(row) })" @click="copy(username(row))" />
                </ElTooltip>
              </span>
              <p class="secondary remark">{{ row.remark || t('subaccounts.noRemark') }}</p>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.password')" min-width="150">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="inline">
              <span class="password">{{ password && password.clientId === row.clientId ? password.value : '••••••••' }}</span>
              <ElButton link :icon="password?.clientId === row.clientId ? Hide : View" :loading="passwordPending === row.clientId"
                :aria-label="password?.clientId === row.clientId ? t('subaccounts.hidePassword') : t('subaccounts.showPassword')" @click="readPassword(row, 'show')" />
              <ElButton link :icon="CopyDocument" :disabled="passwordPending === row.clientId" :aria-label="t('subaccounts.copyPassword')" @click="readPassword(row, 'copy')" />
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.usageAndLimit')" min-width="200">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="usage-cell numeric">
              <i18n-t keypath="subaccounts.usageValue" tag="span" scope="global">
                <template #used><strong>{{ formatUsage(row.usedBytes) }}</strong></template>
                <template #limit>{{ row.limitBytes === undefined ? t('subaccounts.unlimited') : formatUsage(row.limitBytes) }}</template>
              </i18n-t>
              <ElProgress v-if="row.limitBytes !== undefined" :percentage="usagePercent(row)" :show-text="false" :stroke-width="5"
                :status="capped(row) ? 'exception' : usagePercent(row) >= 80 ? 'warning' : undefined" />
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.status')" min-width="140">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="inline status">
              <ElSwitch :model-value="row.active" :aria-label="row.active ? t('subaccounts.disableTitle', { name: row.name }) : t('subaccounts.enableTitle', { name: row.name })"
                :disabled="enabling !== undefined" :loading="enabling === row.clientId"
                :before-change="() => { void changeStatus(row); return false }" />
              <ElTag v-if="row.active && capped(row)" type="danger" size="small" disable-transitions>{{ t('subaccounts.capped') }}</ElTag>
              <span v-else class="secondary">{{ row.active ? t('subaccounts.active') : t('subaccounts.inactive') }}</span>
            </div>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.createdAt')" min-width="140">
          <template #default="{ row }">
            <template v-if="isAccount(row)">
              <time v-if="row.createdAt" :datetime="row.createdAt" class="numeric">{{ dateFormat.format(new Date(row.createdAt)) }}</time>
              <span v-else class="secondary">{{ t('subaccounts.unknownDate') }}</span>
            </template>
          </template>
        </ElTableColumn>
        <ElTableColumn :label="t('subaccounts.actions')" :width="locale === 'zh-CN' ? 150 : 170" fixed="right">
          <template #default="{ row }">
            <div v-if="isAccount(row)" class="inline actions">
              <ElButton link type="primary" :disabled="enabling === row.clientId" @click="action = { kind: 'edit', account: row }">{{ t('subaccounts.edit') }}</ElButton>
              <ElButton link type="primary" @click="action = { kind: 'password', account: row }">{{ t('subaccounts.changePassword') }}</ElButton>
            </div>
          </template>
        </ElTableColumn>
        <template #empty>
          <ElEmpty v-if="accounts.length" :description="t('subaccounts.noMatches')" :image-size="72">
            <ElButton @click="resetFilters">{{ t('subaccounts.clearFilters') }}</ElButton>
          </ElEmpty>
          <ElEmpty v-else :description="t('subaccounts.noAccounts')" :image-size="72">
            <ElButton type="primary" @click="action = { kind: 'create' }">{{ t('subaccounts.createFirst') }}</ElButton>
          </ElEmpty>
        </template>
      </ElTable>
      <p class="table-footer">{{ visible.length === accounts.length
        ? t('subaccounts.totalCount', { count: accounts.length }, accounts.length)
        : t('subaccounts.visibleCount', { visible: visible.length, total: accounts.length }) }}</p>
    </section>
  </template>

  <SubaccountDialog v-if="action" :action="action" :primary-account="primaryAccount"
    @close="action = undefined" @saved="saveAccount" @quota-reached="loadAccounts" />
</template>

<style scoped>
.notice { margin-bottom: var(--space-4); }
.notice.introduction {
  --el-alert-bg-color: var(--color-disabled-bg);
  --el-color-info: var(--color-text-secondary);
  border-radius: var(--radius-card);
}
.introduction p {
  margin-top: var(--space-1);
  font-size: var(--font-size-xs);
  line-height: var(--line-height-normal);
}
.introduction code {
  padding-inline: var(--space-1);
  background: var(--color-surface-muted);
  border-radius: var(--space-1);
  font: inherit;
  font-family: var(--font-mono);
  overflow-wrap: anywhere;
}
.create-button {
  --el-border-radius-base: var(--space-2);
  padding-inline: var(--space-3);
  font-size: var(--font-size-sm);
}
.metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: var(--space-4);
  margin-bottom: var(--space-4);
}
.metric, .table-card {
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--dashboard-card-radius, var(--radius-card));
  background: var(--color-surface);
}
.metric {
  padding: var(--space-4) calc(var(--space-4) + 2 * var(--border-width));
  overflow-wrap: anywhere;
}
.metric h2 { font-size: var(--font-size-sm); font-weight: var(--font-weight-semibold); }
.metric-value {
  margin-top: var(--space-1);
  font-size: var(--font-size-dashboard-display, var(--font-size-title));
  font-weight: var(--font-weight-bold);
  line-height: 1.2;
  letter-spacing: -.035em;
}
.metric-unit {
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  letter-spacing: normal;
  white-space: nowrap;
}
.metric-hint {
  margin-top: calc(2 * var(--border-width));
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}
.table-card { min-width: 0; overflow: hidden; }
.toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  padding: var(--space-3) var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
}
.search, .status-filter { --el-border-radius-base: var(--space-2); }
.search { width: min(100%, calc(var(--subaccount-metric-width) + var(--space-4) + var(--space-1))); }
.status-filter { width: min(100%, calc(var(--content-narrow-width) / 5 - var(--space-1))); }
.accounts-table {
  --el-table-header-bg-color: var(--color-surface-muted);
  --el-table-row-hover-bg-color: var(--color-surface-muted);
  --el-table-current-row-bg-color: var(--color-surface-muted);
  --el-table-header-text-color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
}
.inline { display: flex; flex-wrap: wrap; align-items: center; gap: calc(var(--space-1) + 2 * var(--border-width)); }
.inline > button { margin: 0; }
.inline > button { font-size: inherit; }
.actions { gap: 0; }
.account-name { color: var(--color-text); font-size: var(--font-size-md); font-weight: var(--font-weight-semibold); }
.password { font-family: var(--font-mono); }
.status { column-gap: var(--space-2); }
.usage-cell { display: grid; gap: calc(var(--space-1) + 2 * var(--border-width)); }
.usage-cell strong { color: var(--color-text); }
.metric-value, .numeric { font-variant-numeric: tabular-nums; }
.secondary, .table-footer { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.remark { white-space: pre-wrap; overflow-wrap: anywhere; }
.table-footer { padding: calc(var(--space-3) + 2 * var(--border-width)) var(--space-4); }
@media (max-width: 680px) {
  .metrics {
    grid-template-columns: repeat(auto-fit, minmax(min(100%, calc(var(--subaccount-metric-width) / 1.5)), 1fr));
    gap: var(--space-2);
  }
  .metric { padding-inline: var(--space-3); }
}
@media (max-width: 460px) {
  .metric:last-child { grid-column: 1 / -1; }
}
</style>
