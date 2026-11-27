<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElEmpty, ElInput, ElSkeleton, ElTable, ElTableColumn, ElTag } from 'element-plus'
import {
  isValidCustomerKey,
  listOpsTenantCommissions,
  listTenantCommissions,
  type CommissionRecord,
  type CommissionRecordStatus,
} from '@/api/commissions'
import { accountLoadFailed, currentAccount, hasOperatorPermission } from '@/features/auth'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { formatYuan } from '@/shared/lib/money'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { formatBpsPercent } from './commissionFormat'
import CommissionTransferForm from './CommissionTransferForm.vue'

type ListState =
  | { kind: 'idle' }
  | { kind: 'denied' }
  | { kind: 'loading' }
  | { kind: 'error', error: unknown }
  | {
    kind: 'ready'
    records: CommissionRecord[]
    nextBeforeId?: string
    loadingMore: boolean
    loadMoreError?: unknown
  }

interface SubjectKey {
  mode: 'tenant' | 'ops'
  tenantId: string
}

const props = defineProps<
  | { mode: 'tenant', tenantId?: never }
  | { mode: 'ops', tenantId: string }
>()

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const headingId = useId()
const filterInputId = useId()
const filterErrorId = useId()

const availableMinor = shallowRef<bigint | undefined>(undefined)
const filterInput = ref('')
const activeCustomerKey = ref<string | undefined>(undefined)
const validationError = ref<'invalid_uuid' | undefined>(undefined)
const listState = shallowRef<ListState>({ kind: 'loading' })

let activeRequest: AbortController | undefined
let requestSeq = 0
let lastSubjectKey: string | undefined

onScopeDispose(() => {
  activeRequest?.abort()
  activeRequest = undefined
})

const canReadOps = computed(() => hasOperatorPermission('read_operations'))

function resolveSubject(): SubjectKey | undefined {
  if (props.mode === 'ops') {
    if (!canReadOps.value || !props.tenantId) return undefined
    return { mode: 'ops', tenantId: props.tenantId }
  }
  const tenantId = currentAccount.value?.tenantId
  if (!tenantId) return undefined
  return { mode: 'tenant', tenantId }
}

function serializeSubject(subject: SubjectKey | undefined): string | undefined {
  return subject ? `${subject.mode}:${subject.tenantId}` : undefined
}

function isSameSubject(expected: SubjectKey): boolean {
  const current = resolveSubject()
  return current !== undefined && current.mode === expected.mode && current.tenantId === expected.tenantId
}

function fetchPage(subject: SubjectKey, customerKey: string | undefined, beforeId: string | undefined, signal: AbortSignal) {
  const query = { customerKey, beforeId }
  return subject.mode === 'ops'
    ? listOpsTenantCommissions(subject.tenantId, query, signal)
    : listTenantCommissions(query, signal)
}

function isStaleRequest(
  controller: AbortController,
  seq: number,
  subject: SubjectKey,
  customerKey: string | undefined,
): boolean {
  return controller.signal.aborted || seq !== requestSeq || !isSameSubject(subject) || activeCustomerKey.value !== customerKey
}

function emptySubjectState(): ListState {
  if (props.mode === 'ops' && !canReadOps.value) return { kind: 'denied' }
  if (props.mode === 'tenant' && !accountLoadFailed.value) return { kind: 'loading' }
  return { kind: 'idle' }
}

async function loadInitial(customerKey = activeCustomerKey.value) {
  activeRequest?.abort()
  activeRequest = undefined
  const seq = ++requestSeq
  const subject = resolveSubject()
  if (!subject) {
    listState.value = emptySubjectState()
    return
  }

  const controller = new AbortController()
  activeRequest = controller
  listState.value = { kind: 'loading' }
  try {
    const result = await fetchPage(subject, customerKey, undefined, controller.signal)
    if (isStaleRequest(controller, seq, subject, customerKey)) return
    availableMinor.value = result.availableMinor
    listState.value = {
      kind: 'ready',
      records: result.records,
      nextBeforeId: result.nextBeforeId,
      loadingMore: false,
    }
  } catch (error) {
    if (isStaleRequest(controller, seq, subject, customerKey)) return
    listState.value = { kind: 'error', error }
  }
}

async function loadMore() {
  if (listState.value.kind !== 'ready' || listState.value.loadingMore || !listState.value.nextBeforeId) return
  const subject = resolveSubject()
  if (!subject) return

  const current = listState.value
  const beforeId = current.nextBeforeId
  const customerKey = activeCustomerKey.value

  activeRequest?.abort()
  const seq = ++requestSeq
  const controller = new AbortController()
  activeRequest = controller
  listState.value = { ...current, loadingMore: true, loadMoreError: undefined }

  try {
    const result = await fetchPage(subject, customerKey, beforeId, controller.signal)
    if (isStaleRequest(controller, seq, subject, customerKey)) return
    const seen = new Set(current.records.map(record => record.id))
    const merged = [...current.records, ...result.records.filter(record => !seen.has(record.id))]
    availableMinor.value = result.availableMinor
    listState.value = {
      kind: 'ready',
      records: merged,
      nextBeforeId: result.nextBeforeId,
      loadingMore: false,
    }
  } catch (error) {
    if (isStaleRequest(controller, seq, subject, customerKey)) return
    listState.value = {
      ...current,
      loadingMore: false,
      loadMoreError: error,
    }
  }
}

function resetForSubjectChange() {
  activeRequest?.abort()
  activeRequest = undefined
  requestSeq++
  availableMinor.value = undefined
  filterInput.value = ''
  activeCustomerKey.value = undefined
  validationError.value = undefined
}

watch(
  [() => props.mode, () => props.mode === 'ops' ? props.tenantId : undefined, () => currentAccount.value?.tenantId, accountLoadFailed, canReadOps],
  () => {
    const subject = resolveSubject()
    const nextKey = serializeSubject(subject)
    if (nextKey !== lastSubjectKey) {
      lastSubjectKey = nextKey
      resetForSubjectChange()
      if (subject) {
        void loadInitial(undefined)
        return
      }
    }
    if (!subject) {
      listState.value = emptySubjectState()
    }
  },
  { immediate: true },
)

function onFilterInput(value: string) {
  filterInput.value = value
  if (validationError.value) validationError.value = undefined
}

function applyFilter() {
  const trimmed = filterInput.value.trim()
  filterInput.value = trimmed
  if (trimmed === '') {
    validationError.value = undefined
    activeCustomerKey.value = undefined
    void loadInitial(undefined)
    return
  }
  if (!isValidCustomerKey(trimmed)) {
    validationError.value = 'invalid_uuid'
    return
  }
  validationError.value = undefined
  activeCustomerKey.value = trimmed
  void loadInitial(trimmed)
}

function clearFilter() {
  filterInput.value = ''
  validationError.value = undefined
  const shouldReload = activeCustomerKey.value !== undefined || listState.value.kind === 'error'
  activeCustomerKey.value = undefined
  if (shouldReload) void loadInitial(undefined)
}

function statusTagType(status: CommissionRecordStatus): 'success' | 'info' | 'warning' {
  if (status === 'credited') return 'success'
  if (status === 'promotion_paused') return 'warning'
  return 'info'
}

const isInitialLoading = computed(() => listState.value.kind === 'loading')
const transferTenantId = computed(() => (props.mode === 'tenant' ? currentAccount.value?.tenantId : undefined))
</script>

<template>
  <ElCard class="commission-records" role="region" shadow="never" :aria-labelledby="headingId">
    <div class="commission-records__body">
      <div class="commission-records__header">
        <h2 :id="headingId" class="commission-records__heading">{{ t('users.commissions.title') }}</h2>
        <div v-if="availableMinor !== undefined" class="commission-records__balance">
          <span class="commission-records__balance-label">{{ t('users.commissions.availableBalance') }}</span>
          <strong class="commission-records__balance-amount tabular">{{ formatYuan(availableMinor, locale) }}</strong>
          <span class="commission-records__balance-hint">{{ t('users.commissions.availableBalanceHint') }}</span>
        </div>
      </div>

      <CommissionTransferForm
        v-if="transferTenantId"
        :key="transferTenantId"
        :tenant-id="transferTenantId"
        :available-minor="availableMinor"
        @refresh="loadInitial()"
      />

      <ElAlert
        v-if="listState.kind === 'denied'"
        type="warning"
        :title="t('errors.ACCESS_DENIED')"
        :closable="false"
        show-icon
      />

      <template v-else-if="listState.kind !== 'idle'">
        <form class="commission-records__filter" novalidate @submit.prevent="applyFilter">
          <div class="commission-records__filter-field">
            <label :for="filterInputId" class="commission-records__filter-label">
              {{ t('users.commissions.customerKeyLabel') }}
            </label>
            <div class="commission-records__filter-controls">
              <ElInput
                :id="filterInputId"
                :model-value="filterInput"
                class="commission-records__filter-input mono"
                :placeholder="t('users.commissions.customerKeyPlaceholder')"
                :aria-invalid="validationError ? 'true' : undefined"
                :aria-describedby="validationError ? filterErrorId : undefined"
                autocomplete="off"
                spellcheck="false"
                @update:model-value="onFilterInput"
              />
              <div class="commission-records__filter-actions">
                <ElButton class="commission-records__filter-btn" native-type="submit" type="primary" :loading="isInitialLoading">
                  {{ t('users.commissions.search') }}
                </ElButton>
                <ElButton
                  class="commission-records__filter-btn"
                  native-type="button"
                  :disabled="!filterInput && !activeCustomerKey && !validationError"
                  @click="clearFilter"
                >
                  {{ t('users.commissions.clear') }}
                </ElButton>
              </div>
            </div>
            <p v-if="validationError" :id="filterErrorId" class="commission-records__field-error" role="alert">
              {{ t('users.commissions.invalidCustomerKey') }}
            </p>
            <p v-else class="commission-records__filter-hint">
              {{ t('users.commissions.customerKeyHint') }}
            </p>
          </div>
        </form>

        <div v-if="listState.kind === 'loading'" role="status" :aria-label="t('users.commissions.loading')">
          <ElSkeleton animated :rows="4" />
        </div>

        <div v-else-if="listState.kind === 'error'" class="commission-records__error">
          <ElAlert
            type="error"
            :title="t('users.commissions.loadFailed')"
            :description="errorMessage(listState.error)"
            :closable="false"
            show-icon
          />
          <ElButton size="small" @click="loadInitial()">{{ t('auth.retry') }}</ElButton>
        </div>

        <template v-else-if="listState.kind === 'ready'">
          <div class="commission-records__table-wrap">
            <ElTable
              :data="listState.records"
              row-key="id"
              scrollbar-always-on
              class="commission-records__table"
              :aria-label="t('users.commissions.title')"
            >
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.product')" min-width="108">
                <template #default="{ row }">
                  <span class="commission-records__cell-text">{{ t(`users.commissions.products.${row.productKind}`) }}</span>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.commission')" min-width="90" align="right">
                <template #default="{ row }">
                  <strong
                    class="commission-records__amount tabular"
                    :class="{ 'commission-records__amount--credited': row.commissionMinor > 0n }"
                  >
                    {{ formatYuan(row.commissionMinor, locale) }}
                  </strong>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.status')" min-width="104">
                <template #default="{ row }">
                  <ElTag :type="statusTagType(row.status)" effect="light" size="small">
                    {{ t(`users.commissions.statuses.${row.status}`) }}
                  </ElTag>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.classification')" min-width="84">
                <template #default="{ row }">
                  <span class="commission-records__cell-text">{{ t(`users.commissions.classifications.${row.classification}`) }}</span>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.netDebit')" min-width="90" align="right">
                <template #default="{ row }">
                  <span class="tabular">{{ formatYuan(row.netDebitMinor, locale) }}</span>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.rate')" min-width="104" align="right">
                <template #default="{ row }">
                  <div class="commission-records__rate">
                    <span class="tabular">{{ formatBpsPercent(row.rateBps) }}</span>
                    <span class="commission-records__rate-meta">
                      {{ t('users.commissions.rateVersionTag', { version: row.rateVersion }) }}<template v-if="mode === 'ops'"> · {{ t('users.commissions.ruleVersionTag', { version: row.ruleVersion }) }}</template>
                    </span>
                  </div>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.settledAt')" min-width="124">
                <template #default="{ row }">
                  <time class="commission-records__cell-text tabular" :datetime="row.settledAt.toISOString()">
                    {{ shanghaiDateTime(row.settledAt, locale, 'short') }}
                  </time>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.source')" min-width="76">
                <template #default="{ row }">
                  <span class="commission-records__cell-text">{{ t(`users.commissions.sources.${row.referralSource}`) }}</span>
                </template>
              </ElTableColumn>
              <!-- @vue-generic {CommissionRecord} -->
              <ElTableColumn :label="t('users.commissions.columns.customerKey')" min-width="210">
                <template #default="{ row }">
                  <span class="commission-records__key mono">{{ row.customerKey }}</span>
                </template>
              </ElTableColumn>
              <template #empty>
                <ElEmpty
                  :description="t(activeCustomerKey ? 'users.commissions.emptyFiltered' : 'users.commissions.empty')"
                  :image-size="72"
                />
              </template>
            </ElTable>
          </div>

          <div v-if="listState.loadMoreError" class="commission-records__error">
            <ElAlert
              type="error"
              :title="t('users.commissions.loadMoreFailed')"
              :description="errorMessage(listState.loadMoreError)"
              :closable="false"
              show-icon
            />
            <ElButton size="small" :loading="listState.loadingMore" @click="loadMore">
              {{ t('auth.retry') }}
            </ElButton>
          </div>

          <div v-if="listState.nextBeforeId" class="commission-records__footer">
            <ElButton :loading="listState.loadingMore" :disabled="listState.loadingMore" @click="loadMore">
              {{ t('users.commissions.loadOlder') }}
            </ElButton>
          </div>
          <p v-else-if="listState.records.length > 0" class="commission-records__end">
            {{ t('users.commissions.allLoaded') }}
          </p>
        </template>
      </template>
    </div>
  </ElCard>
</template>

<style scoped>
.commission-records {
  --el-card-padding: var(--space-5);
  border-radius: var(--dashboard-card-radius);
  min-width: 0;
  max-width: 100%;
}

.commission-records__body {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-4);
  min-width: 0;
}

.commission-records__header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3) var(--space-4);
}

.commission-records__heading {
  margin: 0;
  font-size: var(--font-size-xl);
  line-height: var(--tenant-profile-heading-height);
  font-weight: var(--font-weight-semibold);
}

.commission-records__balance {
  display: grid;
  justify-items: end;
  gap: var(--space-1);
}

.commission-records__balance-label {
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.commission-records__balance-amount {
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--color-text);
}

.commission-records__balance-hint {
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.commission-records__filter {
  min-width: 0;
}

.commission-records__filter-field {
  display: grid;
  gap: var(--space-2);
  min-width: 0;
}

.commission-records__filter-label {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
  color: var(--color-text);
}

.commission-records__filter-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: var(--space-2);
  min-width: 0;
}

.commission-records__filter-input {
  flex: 1 1 18rem;
  min-width: 0;
  max-width: 28rem;
}

.commission-records__filter-actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-2);
  flex-shrink: 0;
}

.commission-records__filter-btn {
  margin-left: 0;
}

.commission-records__filter-hint {
  margin: 0;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.commission-records__field-error {
  margin: 0;
  color: var(--color-danger-text);
  font-size: var(--font-size-xs);
}

.commission-records__error {
  display: grid;
  justify-items: start;
  gap: var(--space-3);
}

.commission-records__table-wrap {
  width: 100%;
  min-width: 0;
  overflow-x: auto;
}

.commission-records__table {
  width: 100%;
}

.commission-records__cell-text {
  overflow-wrap: break-word;
}

.commission-records__key {
  font-size: var(--font-size-xs);
  line-height: var(--line-height-tight);
  user-select: all;
  word-break: break-all;
}

.commission-records__rate {
  display: grid;
  justify-items: end;
  gap: var(--space-1);
}

.commission-records__rate-meta {
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

.commission-records__amount {
  font-weight: var(--font-weight-semibold);
}

.commission-records__amount--credited {
  color: var(--color-success-text);
}

.commission-records__footer {
  display: flex;
  justify-content: center;
}

.commission-records__end {
  margin: 0;
  text-align: center;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
}

@media (max-width: 600px) {
  .commission-records {
    --el-card-padding: var(--space-4);
  }

  .commission-records__balance {
    justify-items: start;
  }

  .commission-records__filter-input {
    max-width: 100%;
  }
}
</style>
