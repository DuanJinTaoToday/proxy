<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElEmpty, ElIcon, ElPopover, ElSkeleton, ElTable, ElTableColumn } from 'element-plus'
import { Clock, Connection, CreditCard, Service, Sort, Tickets } from '@element-plus/icons-vue'
import { getDailyUsage, getOverview, getTrafficAlert, hasPendingOrders, listActiveStaticExpiries } from '@/api/dashboard'
import { formatGB } from '@/features/dynamic-proxy'
import LinkButton from '@/shared/ui/LinkButton.vue'
import PageHeader from '@/shared/ui/PageHeader.vue'
import UsageChart from './UsageChart.vue'
import { formatYuan, shanghaiDateTime } from '@/shared/lib/formatters'
import { usageWeek } from './format'

const { t, locale } = useI18n()
const showTable = ref(false)
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

async function readDashboard(signal: AbortSignal) {
  const dates = usageWeek()
  const overview = await getOverview(signal)
  const [usage, alert, activeExpiries, pending] = await Promise.all([
    getDailyUsage({ start: dates[0], end: dates[6], signal }),
    getTrafficAlert(signal),
    listActiveStaticExpiries(overview.activeStaticIpCount, signal),
    hasPendingOrders(signal),
  ])
  const usageByDate = new Map(usage.map(day => [day.date, day]))
  return {
    ...overview,
    lowTraffic: alert.thresholdBytes !== undefined && overview.trafficBytes <= alert.thresholdBytes,
    expiry: activeExpiries[0]?.expiresAt,
    pending,
    days: usage.length ? dates.map(date => ({ date, bytes: usageByDate.get(date)?.bytes ?? 0n })) : [],
    updatedAt: usage.reduce((latest, day) => Math.max(latest, Date.parse(day.updatedAt ?? '') || 0), 0),
  }
}

type DashboardState =
  | { kind: 'loading' | 'failed' }
  | { kind: 'ready', data: Awaited<ReturnType<typeof readDashboard>> }
const state = shallowRef<DashboardState>({ kind: 'loading' })

async function load() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  try {
    const data = await readDashboard(controller.signal)
    if (!controller.signal.aborted) state.value = { kind: 'ready', data }
  } catch {
    if (!controller.signal.aborted) {
      state.value = { kind: 'failed' }
      controller.abort()
    }
  }
}

void load()

const data = computed(() => state.value.kind === 'ready' ? state.value.data : undefined)
const usageTotal = computed(() => data.value?.days.reduce((total, day) => total + day.bytes, 0n) ?? 0n)
const gb = (bytes: bigint) => t('dashboard.gb', { value: formatGB(bytes, locale.value) })
const trafficPricing = { name: 'console-dynamic-pricing' }
const staticIps = { name: 'console-static-ips' }
</script>

<template>
  <PageHeader :title="t('nav.dashboard')" />
  <div v-if="state.kind === 'loading'" role="status" :aria-label="t('dashboard.loading')">
    <ElSkeleton animated :rows="6" />
  </div>
  <ElAlert v-else-if="state.kind === 'failed'" :title="t('dashboard.failed')" type="error" :closable="false" show-icon>
    <ElButton size="small" @click="load">{{ t('dashboard.retry') }}</ElButton>
  </ElAlert>
  <div v-else-if="data" class="dashboard">
    <div class="dashboard__stats">
      <ElCard role="region" shadow="never" body-class="dashboard__stat" aria-labelledby="wallet-balance-title">
        <header class="dashboard__card-heading">
          <h2 id="wallet-balance-title">{{ t('dashboard.walletTitle') }}</h2>
          <ElIcon class="dashboard__icon"><CreditCard /></ElIcon>
        </header>
        <p class="dashboard__value tabular">{{ formatYuan(data.walletMinor, locale) }}</p>
        <p class="dashboard__muted">{{ t('dashboard.walletDetail') }}</p>
        <div class="dashboard__actions">
          <ElPopover placement="bottom-start" trigger="click" width="min(var(--recharge-panel-width), calc(100vw - var(--space-5)))">
            <template #reference>
              <ElButton :icon="Service" size="small" type="primary">{{ t('dashboard.recharge') }}</ElButton>
            </template>
            <strong>{{ t('dashboard.rechargeTitle') }}</strong>
            <p>{{ t('dashboard.rechargeInstructions') }}</p>
          </ElPopover>
          <LinkButton :to="{ name: 'console-wallet' }" size="small">{{ t('dashboard.walletAction') }}</LinkButton>
        </div>
      </ElCard>

      <ElCard role="region" shadow="never" body-class="dashboard__stat" aria-labelledby="traffic-balance-title">
        <header class="dashboard__card-heading">
          <h2 id="traffic-balance-title">{{ t('dashboard.trafficTitle') }}</h2>
          <ElIcon class="dashboard__icon"><Sort /></ElIcon>
        </header>
        <p v-if="data.trafficBytes < 0n" class="dashboard__value dashboard__debt tabular">{{ t('dashboard.debt', { amount: gb(-data.trafficBytes) }) }}</p>
        <p v-else class="dashboard__value tabular">{{ gb(data.trafficBytes) }}</p>
        <p class="dashboard__muted">{{ t(data.canBuyTraffic ? 'dashboard.trafficDetail' : 'dashboard.offSale') }}</p>
        <div class="dashboard__actions">
          <LinkButton :to="trafficPricing" size="small">{{ t(data.canBuyTraffic ? 'dashboard.buyTraffic' : 'dashboard.viewPricing') }}</LinkButton>
        </div>
      </ElCard>

      <ElCard role="region" shadow="never" body-class="dashboard__stat" aria-labelledby="static-count-title">
        <header class="dashboard__card-heading">
          <h2 id="static-count-title">{{ t('dashboard.staticTitle') }}</h2>
          <ElIcon class="dashboard__icon"><Connection /></ElIcon>
        </header>
        <p class="dashboard__value tabular">{{ t('dashboard.ipCount', data.activeStaticIpCount) }}</p>
        <p v-if="data.expiry" class="dashboard__muted tabular">
          {{ t('dashboard.nextExpiry') }}
          <time :datetime="data.expiry.toISOString()">{{ shanghaiDateTime(data.expiry, locale) }}</time>
        </p>
        <p v-else class="dashboard__muted">{{ t('dashboard.noExpiry') }}</p>
        <div class="dashboard__actions">
          <LinkButton :to="{ name: 'console-static-pricing' }" size="small">{{ t(data.canBuyStatic ? 'dashboard.buyStatic' : 'dashboard.viewPricing') }}</LinkButton>
          <LinkButton :to="staticIps" size="small">{{ t('dashboard.viewIps') }}</LinkButton>
        </div>
      </ElCard>
    </div>

    <div class="dashboard__details">
      <ElCard role="region" shadow="never" aria-labelledby="dashboard-usage-title">
        <div class="dashboard__card-heading">
          <h2 id="dashboard-usage-title">{{ t('dashboard.usageTitle') }}</h2>
          <ElButton v-if="data.days.length" link type="primary" size="small" @click="showTable = !showTable">
            {{ t(showTable ? 'dashboard.showChart' : 'dashboard.showTable') }}
          </ElButton>
        </div>
        <template v-if="data.days.length">
          <p class="dashboard__muted tabular">
            {{ t('dashboard.usageTotal', { amount: gb(usageTotal) }) }}
            <template v-if="data.updatedAt">
              {{ t('dashboard.updated') }}
              <time :datetime="new Date(data.updatedAt).toISOString()">{{ shanghaiDateTime(new Date(data.updatedAt), locale) }}</time>
            </template>
          </p>
          <ElTable v-if="showTable" :data="data.days" :aria-label="t('dashboard.usageTitle')" class="dashboard__table tabular" size="small">
            <ElTableColumn prop="date" :label="t('dashboard.businessDay')" />
            <ElTableColumn :label="t('dashboard.usageAmount')">
              <template #default="{ row }">{{ gb(row.bytes) }}</template>
            </ElTableColumn>
          </ElTable>
          <UsageChart v-else :days="data.days" />
        </template>
        <ElEmpty v-else :description="t('dashboard.noUsage')" :image-size="56" />
        <footer class="dashboard__usage-footer">
          <span class="dashboard__muted">{{ t('dashboard.usageBasis') }}</span>
          <RouterLink :to="{ name: 'console-dynamic-usage' }">{{ t('dashboard.viewUsage') }}</RouterLink>
        </footer>
      </ElCard>

      <ElCard role="region" shadow="never" aria-labelledby="dashboard-todo-title">
        <div class="dashboard__card-heading"><h2 id="dashboard-todo-title">{{ t('dashboard.todoTitle') }}</h2></div>
        <ul v-if="data.trafficBytes < 0n || data.lowTraffic || data.expiringIpCount || data.pending" class="dashboard__todos">
          <li v-if="data.trafficBytes < 0n || data.lowTraffic">
            <ElIcon class="dashboard__icon" :class="data.trafficBytes < 0n ? 'dashboard__debt' : 'dashboard__warning'"><Sort /></ElIcon>
            <div>
              <strong>{{ t(data.trafficBytes < 0n ? 'dashboard.todoDebt' : 'dashboard.todoLow') }}</strong>
              <p class="dashboard__muted tabular">{{ data.trafficBytes < 0n ? t('dashboard.debtDetail') : t('dashboard.lowDetail', { amount: gb(data.trafficBytes) }) }}</p>
              <RouterLink :to="trafficPricing">{{ t(data.canBuyTraffic ? 'dashboard.buyTraffic' : 'dashboard.viewPricing') }}</RouterLink>
            </div>
          </li>
          <li v-if="data.expiringIpCount">
            <ElIcon class="dashboard__icon dashboard__warning"><Clock /></ElIcon>
            <div>
              <strong class="tabular">{{ t('dashboard.todoExpiry', data.expiringIpCount) }}</strong>
              <p class="dashboard__muted">{{ t('dashboard.expiryDetail') }}</p>
              <RouterLink :to="staticIps">{{ t('dashboard.renew') }}</RouterLink>
            </div>
          </li>
          <li v-if="data.pending">
            <ElIcon class="dashboard__icon dashboard__info"><Tickets /></ElIcon>
            <div>
              <strong>{{ t('dashboard.todoOrders') }}</strong>
              <p class="dashboard__muted">{{ t('dashboard.ordersDetail') }}</p>
              <RouterLink :to="{ name: 'console-account-orders', query: { status: 'pending' } }">{{ t('dashboard.viewOrders') }}</RouterLink>
            </div>
          </li>
        </ul>
        <ElEmpty v-else :description="t('dashboard.noTodos')" :image-size="56" />
      </ElCard>
    </div>
  </div>
</template>

<style scoped>
.dashboard, .dashboard__stats, .dashboard__details { display: grid; gap: var(--space-4); }
.dashboard__stats { grid-template-columns: repeat(3, minmax(0, 1fr)); }
.dashboard__details { grid-template-columns: minmax(0, 2fr) minmax(0, 1fr); align-items: start; }
.dashboard :deep(.dashboard__stat) {
  display: grid;
  grid-template-rows: auto auto 1fr auto;
  block-size: 100%;
}
.dashboard__card-heading, .dashboard__usage-footer, .dashboard__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.dashboard__card-heading h2 { font-size: var(--font-size-lg); font-weight: var(--font-weight-semibold); }
.dashboard__icon {
  flex: none;
  box-sizing: content-box;
  padding: var(--space-2);
  color: var(--color-primary);
  background: var(--color-primary-soft);
  border-radius: var(--radius-control);
}
.dashboard__value {
  margin-block: var(--space-2);
  font-size: var(--font-size-display);
  line-height: var(--line-height-tight);
  font-weight: var(--font-weight-bold);
  overflow-wrap: anywhere;
}
.dashboard__muted { color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.dashboard__card-heading + .dashboard__muted { margin-block: var(--space-2) var(--space-4); }
.dashboard__debt { color: var(--color-danger-text); }
.dashboard__icon.dashboard__debt { background: var(--color-danger-bg); }
.dashboard__warning { background: var(--color-warning-bg); color: var(--color-warning-text); }
.dashboard__info { background: var(--color-info-bg); color: var(--color-info-text); }
.dashboard__actions { justify-content: flex-start; margin-block-start: var(--space-3); }
.dashboard__actions > :is(a, button) { margin-inline: 0; white-space: normal; height: auto; min-height: var(--control-height-compact); }
.dashboard__usage-footer {
  margin-block-start: var(--space-4);
  padding-block-start: var(--space-3);
  border-block-start: var(--border-width) solid var(--color-border);
}
.dashboard a:not(.el-button) { color: var(--color-primary); font-weight: var(--font-weight-semibold); }
.dashboard__table { width: 100%; }
.dashboard__todos { list-style: none; padding: 0; margin: 0; }
.dashboard__todos li {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
  padding-block: var(--space-3);
}
.dashboard__todos li + li { border-block-start: var(--border-width) solid var(--color-border); }
.dashboard__todos p { margin-block: var(--space-1); }

@media (max-width: 1180px) {
  .dashboard__details { grid-template-columns: minmax(0, 1fr); }
}
@media (max-width: 950px) {
  .dashboard__stats { grid-template-columns: minmax(0, 1fr); }
}
</style>
