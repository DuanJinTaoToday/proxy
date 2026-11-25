<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElSkeleton, ElTabPane, ElTabs, ElTooltip } from 'element-plus'
import { ArrowLeft } from '@element-plus/icons-vue'
import { getOpsTenant, listOpsTenantOrders, type OpsTenant } from '@/api/opsTenants'
import { hasOperatorPermission } from '@/features/auth'
import { OrderDrawer, readOrderId } from '@/features/orders'
import { OpsRecoverEmailDialog, OpsResetPasswordDialog } from '@/features/users'
import { OpsRechargeDialog } from '@/features/wallet'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { formatYuan } from '@/shared/lib/money'
import { bytesPerGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const { t, locale, n } = useI18n()
const route = useRoute()
const router = useRouter()
const tab = computed(() => route.name === 'ops-tenant-ledger' ? 'ledger' : 'orders')
const orderId = computed(() => readOrderId(route.query.order))
const ledgerRevision = ref(0)
const pendingCount = ref(0)

watch([tab, () => route.params.tenantId], async ([section, id], _previous, onCleanup) => {
  if (section !== 'ledger' || typeof id !== 'string' || !hasOperatorPermission('read_operations')) return
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  try {
    const { pendingCount: count } = await listOpsTenantOrders(id, { status: 'pending' }, controller.signal)
    if (!controller.signal.aborted) pendingCount.value = count
  } catch {
    // The pending count is supplementary to the ledger.
  }
}, { immediate: true })

function selectTab(nextTab: string | number) {
  if (nextTab !== tab.value) {
    void router.push({ name: nextTab === 'ledger' ? 'ops-tenant-ledger' : 'ops-tenant' })
  }
}

function closeOrder() {
  void router.replace({ query: { ...route.query, order: undefined } })
}
const errorMessage = useErrorMessage()
type Detail = { kind: 'loading' } | { kind: 'error', error: unknown } | { kind: 'ready', tenant: OpsTenant }
const detail = shallowRef<Detail>({ kind: 'loading' })
const rechargeOpen = ref(false)
const resetOpen = ref(false)
const recoverOpen = ref(false)
const retry = ref(0)

watch([() => route.params.tenantId, retry], async ([id], _previous, onCleanup) => {
  if (typeof id !== 'string' || !hasOperatorPermission('read_operations')) return
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  rechargeOpen.value = false
  pendingCount.value = 0
  resetOpen.value = false
  recoverOpen.value = false
  detail.value = { kind: 'loading' }
  try {
    const tenant = await getOpsTenant(id, controller.signal)
    if (!controller.signal.aborted) detail.value = { kind: 'ready', tenant }
  } catch (error) {
    if (!controller.signal.aborted) detail.value = { kind: 'error', error }
  }
}, { immediate: true })

function updateBalance(availableMinor: bigint) {
  ledgerRevision.value++
  if (detail.value.kind === 'ready') detail.value = { kind: 'ready', tenant: { ...detail.value.tenant, availableMinor } }
}

function trafficBalance(bytes: bigint) {
  const absolute = bytes < 0n ? -bytes : bytes
  const hundredths = (absolute * 100n + bytesPerGB / 2n) / bytesPerGB
  const whole = hundredths / 100n
  const fraction = Number(hundredths % 100n)
  const value = bytes < 0n ? (whole === 0n ? -0 : -whole) : whole
  return new Intl.NumberFormat(locale.value, { minimumFractionDigits: 2 }).formatToParts(value)
    .map(part => part.type === 'fraction' ? new Intl.NumberFormat(locale.value, { minimumIntegerDigits: 2 }).format(fraction) : part.value).join('') + ' GB'
}
</script>

<template>
  <RouterLink :to="{ name: 'ops-tenants' }" class="tenant-back"><ArrowLeft aria-hidden="true" />{{ t('users.opsBack') }}</RouterLink>
  <ElAlert v-if="!hasOperatorPermission('read_operations')" type="warning" :title="t('errors.ACCESS_DENIED')" :closable="false" show-icon />
  <ElSkeleton v-else-if="detail.kind === 'loading'" animated :rows="3" />
  <ElAlert v-else-if="detail.kind === 'error'" type="error" :title="errorMessage(detail.error)" :closable="false" show-icon>
    <ElButton @click="retry++">{{ t('auth.retry') }}</ElButton>
  </ElAlert>
  <template v-else>
    <div class="tenant-head">
      <div>
        <h1 class="ops-page-title tenant-head__title">{{ detail.tenant.name }}</h1>
        <p class="tenant-head__meta">
          <span class="mono">{{ detail.tenant.account }}</span><template v-if="detail.tenant.email"> · {{ detail.tenant.email }}</template>
          <template v-if="detail.tenant.registeredAt"> · {{ t('users.opsRegisteredAt', { date: shanghaiDateTime(detail.tenant.registeredAt, locale, 'short') }) }}</template>
        </p>
      </div>
      <div class="tenant-head__actions">
        <slot name="actions" :tenant="detail.tenant" />
        <ElTooltip :disabled="hasOperatorPermission('manage_tenant_accounts')" :content="t('users.recover.permission')" placement="bottom-end">
          <span><ElButton class="tenant-head__button" :disabled="!hasOperatorPermission('manage_tenant_accounts')" @click="recoverOpen = true">{{ t('users.recover.title') }}</ElButton></span>
        </ElTooltip>
        <ElTooltip :disabled="hasOperatorPermission('manage_tenant_accounts')" :content="t('users.reset.permission')" placement="bottom-end">
          <span><ElButton class="tenant-head__button" :disabled="!hasOperatorPermission('manage_tenant_accounts')" @click="resetOpen = true">{{ t('users.reset.title') }}</ElButton></span>
        </ElTooltip>
        <ElTooltip :disabled="hasOperatorPermission('manage_billing')" :content="t('wallet.opsBillingOnly')" placement="bottom-end">
          <span><ElButton class="tenant-head__button" type="primary" :disabled="!hasOperatorPermission('manage_billing')" @click="rechargeOpen = true">{{ t('wallet.opsRecharge') }}</ElButton></span>
        </ElTooltip>
      </div>
    </div>

    <div class="tenant-metrics">
      <ElCard shadow="never" class="tenant-metrics__card">
        <h2>{{ t('users.opsWalletBalance') }}</h2>
        <p class="tenant-metrics__amount tabular">{{ formatYuan(detail.tenant.availableMinor, locale) }}</p>
        <p class="tenant-metrics__hint">{{ t('users.opsWalletHint') }}</p>
      </ElCard>
      <ElCard shadow="never" class="tenant-metrics__card">
        <h2>{{ t('users.opsTrafficBalance') }}</h2>
        <p class="tenant-metrics__amount tabular" :class="{ 'tenant-metrics__debt': detail.tenant.availableTrafficBytes < 0n }">{{ trafficBalance(detail.tenant.availableTrafficBytes) }}</p>
        <p class="tenant-metrics__hint">{{ t(detail.tenant.availableTrafficBytes < 0n ? 'users.opsTrafficDebt' : 'users.opsTrafficHint') }}</p>
      </ElCard>
    </div>

    <ElTabs class="tenant-history-tabs ops-page-tabs" :model-value="tab" @update:model-value="selectTab">
      <ElTabPane name="orders" class="tenant-history-panel" :label="t('users.opsOrders')">
        <template #label>
          <span :class="{ 'tenant-tab-label--active': tab === 'orders' }">{{ t('users.opsOrders') }}</span>
          <span v-if="pendingCount" class="tenant-pending-count"
            :aria-label="t('orders.pendingCount', pendingCount)">{{ n(pendingCount) }}</span>
        </template>
        <RouterView v-if="tab === 'orders'" v-slot="{ Component }">
          <component :is="Component" :key="detail.tenant.id" :tenant-id="detail.tenant.id" @pending-count="pendingCount = $event" />
        </RouterView>
      </ElTabPane>
      <ElTabPane name="ledger" class="tenant-history-panel" :label="t('wallet.ledger')">
        <template #label><span :class="{ 'tenant-tab-label--active': tab === 'ledger' }">{{ t('wallet.ledger') }}</span></template>
        <RouterView v-if="tab === 'ledger'" :key="`${detail.tenant.id}:${ledgerRevision}`" :tenant-id="detail.tenant.id" />
      </ElTabPane>
    </ElTabs>
    <OrderDrawer :key="detail.tenant.id" :tenant-id="detail.tenant.id" :order-id="orderId" @close="closeOrder" />
    <OpsRecoverEmailDialog v-if="hasOperatorPermission('manage_tenant_accounts')" :key="`recover-${detail.tenant.id}`" v-model="recoverOpen" :tenant="detail.tenant" />
    <OpsResetPasswordDialog v-if="hasOperatorPermission('manage_tenant_accounts')" :key="detail.tenant.id" v-model="resetOpen" :tenant="detail.tenant" />
    <OpsRechargeDialog :key="detail.tenant.id" v-model="rechargeOpen" :tenant="detail.tenant" @recharged="updateBalance" />
  </template>
</template>

<style scoped>
.tenant-history-tabs { --el-tabs-header-height: var(--tabs-height); }
.tenant-history-panel { padding-block-start: var(--ops-tenant-history-gap); }
.tenant-tab-label--active { font-weight: var(--font-weight-bold); }
.tenant-pending-count {
  display: inline-grid;
  place-items: center;
  min-inline-size: var(--ops-usage-count-size);
  min-block-size: var(--ops-usage-count-size);
  margin-inline-start: var(--ops-usage-count-gap);
  padding-inline: var(--space-1);
  border-radius: var(--ops-usage-count-size);
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: var(--ops-usage-count-font);
  font-weight: var(--font-weight-bold);
}
.tenant-back { display: inline-flex; align-items: center; gap: var(--space-1); margin-block-end: var(--space-2); color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.tenant-back svg { width: 1em; }
.tenant-back:hover { color: var(--color-primary); }
.tenant-head { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: var(--space-4); margin-block-end: var(--ops-page-heading-gap); }
.tenant-head__title { margin: 0; overflow-wrap: anywhere; }
.tenant-head__meta { margin-block-start: var(--ops-detail-meta-gap); font-size: var(--font-size-sm); color: var(--color-text-secondary); overflow-wrap: anywhere; line-height: normal; }
.tenant-head__meta .mono { font-size: var(--font-size-xs); }
.tenant-head__actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.tenant-head__button { padding-inline: var(--space-3); font-size: var(--font-size-sm); }
.tenant-metrics { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--wallet-summary-card-width)), 1fr)); gap: var(--space-4); margin-block-end: var(--space-2); }
.tenant-metrics__card { --el-card-padding: var(--ops-detail-card-block-padding) var(--ops-detail-card-inline-padding); border-radius: var(--ops-card-radius); }
.tenant-metrics__card h2 { font-size: var(--font-size-sm); font-weight: var(--font-weight-semibold); }
.tenant-metrics__amount { margin-block-start: var(--space-1); font-size: var(--ops-display-size); font-weight: var(--font-weight-heavy); line-height: calc(var(--space-6) + var(--space-1)); letter-spacing: var(--ops-display-tracking); overflow-wrap: anywhere; }
.tenant-metrics__debt { color: var(--color-warning-text); }
.tenant-metrics__hint { margin-block-start: var(--ops-detail-meta-gap); line-height: normal; color: var(--color-text-secondary); font-size: var(--font-size-xs); }
</style>
