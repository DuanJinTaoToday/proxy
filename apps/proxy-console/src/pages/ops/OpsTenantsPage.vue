<script setup lang="ts">
import { onScopeDispose, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElEmpty, ElInput } from 'element-plus'
import { Search } from '@element-plus/icons-vue'
import { ApiError } from '@/api/client'
import { findOpsTenant, type OpsTenant } from '@/api/opsTenants'
import { hasOperatorPermission } from '@/features/auth'
import { formatYuan } from '@/shared/lib/money'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
type SearchState = { kind: 'idle' | 'loading' | 'empty' } | { kind: 'error', error: unknown } | { kind: 'found', tenant: OpsTenant }
const query = ref('')
const state = shallowRef<SearchState>({ kind: 'idle' })
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

function clear() {
  request?.abort()
  state.value = { kind: 'idle' }
}

async function search() {
  const term = query.value.trim()
  clear()
  if (!term) return
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  try {
    const tenant = await findOpsTenant(term, controller.signal)
    if (!controller.signal.aborted) state.value = { kind: 'found', tenant }
  } catch (error) {
    if (controller.signal.aborted) return
    state.value = error instanceof ApiError && error.status === 404 ? { kind: 'empty' } : { kind: 'error', error }
  }
}
</script>

<template>
  <h1 class="ops-page-title">{{ t('nav.opsTenants') }}</h1>
  <ElAlert v-if="!hasOperatorPermission('read_operations')" type="warning" :title="t('errors.ACCESS_DENIED')" :closable="false" show-icon />
  <ElCard v-else shadow="never" class="tenant-search">
    <h2>{{ t('users.opsFindTenant') }}</h2>
    <p class="tenant-search__description">{{ t('users.opsSearchHint') }}</p>
    <form class="tenant-search__row" @submit.prevent="search">
      <ElInput v-model="query" size="large" :prefix-icon="Search" clearable maxlength="254"
        :aria-label="t('users.opsSearchLabel')" :placeholder="t('users.opsSearchPlaceholder')" @input="clear" />
      <ElButton class="tenant-search__button" type="primary" size="large" native-type="submit" :loading="state.kind === 'loading'">{{ t('users.opsSearch') }}</ElButton>
    </form>

    <RouterLink v-if="state.kind === 'found'" class="tenant-search__result" :to="{ name: 'ops-tenant', params: { tenantId: state.tenant.id } }">
      <div>
        <strong>{{ state.tenant.name }}</strong>
        <span class="mono">{{ state.tenant.account }}</span><template v-if="state.tenant.email"> · {{ state.tenant.email }}</template>
      </div>
      <span class="tabular">{{ t('users.opsSearchBalance', { amount: formatYuan(state.tenant.availableMinor, locale) }) }}</span>
    </RouterLink>
    <ElEmpty v-else-if="state.kind === 'empty'" class="tenant-search__empty" :description="t('users.opsNoTenant')" :image-size="64" />
    <ElAlert v-else-if="state.kind === 'error'" class="tenant-search__result" type="error" :title="errorMessage(state.error)" :closable="false" show-icon />
  </ElCard>
</template>

<style scoped>
.tenant-search { max-width: var(--content-narrow-width); --el-card-padding: var(--space-5); border-radius: var(--ops-card-radius); }
.tenant-search h2 { font-size: var(--font-size-xl); line-height: var(--ops-card-title-line-height); font-weight: var(--font-weight-bold); }
.tenant-search__description { margin-block-start: var(--space-1); color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.tenant-search__row, .tenant-search__result { display: flex; margin-block-start: var(--space-4); }
.tenant-search__row { gap: var(--space-2); }
.tenant-search__button { padding-inline: var(--space-3); font-size: var(--font-size-sm); }
.tenant-search__empty { --el-empty-description-margin-top: var(--space-5); }
.tenant-search__result {
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--ops-search-result-padding) var(--space-4);
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-card);
  overflow-wrap: anywhere;
}
a.tenant-search__result:hover { border-color: var(--color-primary-border); background: var(--color-surface-muted); }
.tenant-search__result div { color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.tenant-search__result strong {
  display: block;
  margin-block-end: var(--ops-detail-meta-gap);
  color: var(--color-text-secondary);
  font-size: var(--ops-tenant-result-name-size);
}
.tenant-search__result .mono { font-size: var(--font-size-xs); }
.tenant-search__result > span { font-weight: var(--font-weight-semibold); }
</style>
