<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElPopover, ElSkeleton } from 'element-plus'
import { ArrowRight, Bottom, CircleCheck, Clock, Connection, Loading, Service, Sort, Wallet, WarningFilled } from '@element-plus/icons-vue'
import type { RouteLocationRaw } from 'vue-router'
import { getAccessLocation } from '@/api/accessLocation'
import { getOverview, listActiveStaticExpiries } from '@/api/dashboard'
import { getPendingOrderCount } from '@/api/orders'
import { listRenewalTerms, quoteStaticRenewal } from '@/api/staticIps'
import { getTrafficAlert } from '@/api/trafficAlert'
import { getDailyUsage } from '@/api/usage'
import { getWalletSummary } from '@/api/wallet'
import { trafficNoticeRevision } from '@/features/notifications'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { formatYuan } from '@/shared/lib/money'
import { bytesPerGB, formatGB } from '@/shared/lib/trafficUnits'
import LinkButton from '@/shared/ui/LinkButton.vue'
import DailyUsageChart from '@/shared/ui/DailyUsageChart.vue'
import { usageWeek } from './format'

const { t, locale } = useI18n()
const showTable = ref(false)
const trafficPricing = { name: 'console-dynamic-pricing' }
const staticIps = { name: 'console-static-ips' }
const walletPage = { name: 'console-wallet' }
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

async function renewalShortfall(ids: string[], walletMinor: bigint, signal: AbortSignal) {
  if (!ids.length) return 0n
  try {
    const terms = await listRenewalTerms(ids, signal)
    const monthly = terms.find(term => term.days === 30)
    if (!monthly) return 0n
    const quote = await quoteStaticRenewal(ids, monthly.code, signal)
    return quote.lines.reduce((total, line) => total + line.unitMinor, 0n) - walletMinor
  } catch {
    return 0n
  }
}

async function readDashboard(signal: AbortSignal, language: string) {
  const dates = usageWeek()
  const overview = await getOverview(signal)
  const [usage, alert, activeExpiries, pending, wallet, location] = await Promise.all([
    getDailyUsage({ start: dates[0], end: dates[6] }, signal),
    getTrafficAlert(signal),
    listActiveStaticExpiries(overview.activeStaticIpCount, signal),
    getPendingOrderCount(signal),
    getWalletSummary(signal).catch(() => undefined),
    getAccessLocation({ locale: language, signal }).catch(() => undefined),
  ])
  const usageByDate = new Map(usage.map(day => [day.date, day]))
  const eligibleExpiries = activeExpiries.slice(0, overview.expiringIpCount)
    .filter(allocation => !allocation.pendingRenewal)
  return {
    ...overview,
    dates,
    threshold: alert.thresholdBytes,
    lowTraffic: alert.thresholdBytes !== undefined && alert.availableBytes <= alert.thresholdBytes,
    expiry: activeExpiries[0]?.expiresAt,
    pending,
    wallet,
    location,
    eligibleExpiries,
    days: usage.length ? dates.map(date => ({ date, bytes: usageByDate.get(date)?.bytes ?? 0n })) : [],
    updatedAt: usage.reduce((latest, day) => Math.max(latest, Date.parse(day.updatedAt ?? '') || 0), 0),
  }
}

type DashboardState =
  | { kind: 'loading' | 'failed' }
  | { kind: 'ready', data: Awaited<ReturnType<typeof readDashboard>> }
const state = shallowRef<DashboardState>({ kind: 'loading' })
const shortfall = ref(0n)

async function load() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  shortfall.value = 0n
  try {
    const data = await readDashboard(controller.signal, locale.value)
    if (controller.signal.aborted) return
    state.value = { kind: 'ready', data }
    const missing = await renewalShortfall(data.eligibleExpiries.map(allocation => allocation.id), data.walletMinor, controller.signal)
    if (!controller.signal.aborted) shortfall.value = missing
  } catch {
    if (!controller.signal.aborted) {
      state.value = { kind: 'failed' }
      controller.abort()
    }
  }
}

watch([trafficNoticeRevision, locale], load, { immediate: true })

const data = computed(() => state.value.kind === 'ready' ? state.value.data : undefined)
const usageTotal = computed(() => data.value?.days.reduce((total, day) => total + day.bytes, 0n) ?? 0n)
const dateLabel = (date: string) => locale.value === 'zh-CN' ? date.replaceAll('-', '/') : date
const rangeLabel = computed(() => {
  const dates = data.value?.dates
  return dates ? dateLabel(dates[0]) + ' – ' + dateLabel(dates[6]).slice(5) : ''
})
const gb = (bytes: bigint) => t('dashboard.gb', { value: formatGB(bytes, locale.value) })
const displayGB = (bytes: bigint) => {
  const absolute = bytes < 0n ? -bytes : bytes
  const hundredths = (absolute * 100n + bytesPerGB / 2n) / bytesPerGB
  const whole = hundredths / 100n
  const major = bytes < 0n ? -whole : whole
  const fraction = hundredths % 100n
  const number = new Intl.NumberFormat(locale.value)
  const decimal = number.formatToParts(0.1).find(part => part.type === 'decimal')?.value ?? '.'
  return t('dashboard.gb', {
    value: number.format(major === 0n && bytes < 0n ? -0 : major) + decimal + fraction.toString().padStart(2, '0'),
  })
}
const money = (minor: bigint) => formatYuan(minor, locale.value)
const dateTime = (date: Date) => locale.value === 'zh-CN'
  ? new Intl.DateTimeFormat('sv-SE', { dateStyle: 'short', timeStyle: 'short', timeZone: 'Asia/Shanghai' })
    .format(date).replaceAll('-', '/')
  : shanghaiDateTime(date, locale.value)

interface Todo {
  id: string
  level: 'critical' | 'warning' | 'info'
  icon: Component
  title: string
  detail: string
  action: string
  to: RouteLocationRaw
}

const todos = computed<Todo[]>(() => {
  const current = data.value
  if (!current) return []
  const items: Todo[] = []
  if (current.location?.isMainlandChina) items.push({
    id: 'network', level: 'critical', icon: WarningFilled,
    title: t('dashboard.networkTitle'), detail: t('dashboard.networkDetail', { location: current.location.location }),
    action: t('dashboard.viewLocation'), to: { name: 'console-account-profile' },
  })
  if (current.trafficBytes < 0n || current.lowTraffic) {
    const debt = current.trafficBytes < 0n
    items.push({
      id: 'traffic', level: debt ? 'critical' : 'warning', icon: Sort,
      title: t(debt ? 'dashboard.todoDebt' : 'dashboard.todoLow'),
      detail: debt
        ? t('dashboard.debtDetail', { amount: gb(-current.trafficBytes) })
        : t('dashboard.lowDetail', { amount: displayGB(current.trafficBytes), threshold: gb(current.threshold ?? 0n) }),
      action: t(current.canBuyTraffic ? 'dashboard.buyTraffic' : 'dashboard.viewPricing'), to: trafficPricing,
    })
  }
  if (current.expiringIpCount) items.push({
    id: 'expiry', level: 'warning', icon: Clock,
    title: t('dashboard.todoExpiry', current.expiringIpCount), detail: t('dashboard.expiryDetail'),
    action: t('dashboard.renew'), to: staticIps,
  })
  if (shortfall.value > 0n) items.push({
    id: 'wallet', level: 'warning', icon: Wallet,
    title: t('dashboard.renewalShortfall'), detail: t('dashboard.renewalShortfallDetail', { amount: money(shortfall.value) }),
    action: t('dashboard.viewWallet'), to: walletPage,
  })
  if (current.pending) items.push({
    id: 'orders', level: 'info', icon: Loading,
    title: t('dashboard.todoOrders', current.pending), detail: t('dashboard.ordersDetail'),
    action: t('dashboard.viewOrders'), to: { name: 'console-account-orders', query: { status: 'pending' } },
  })
  return items
})
</script>

<template>
  <h1 class="dashboard-title">{{ t('nav.dashboard') }}</h1>
  <div v-if="state.kind === 'loading'" role="status" :aria-label="t('dashboard.loading')">
    <ElSkeleton animated :rows="6" />
  </div>
  <ElAlert v-else-if="state.kind === 'failed'" :title="t('dashboard.failed')" type="error" :closable="false" show-icon>
    <ElButton size="small" @click="load">{{ t('dashboard.retry') }}</ElButton>
  </ElAlert>
  <div v-else-if="data" class="dashboard">
    <div class="stat-grid">
      <section class="stat-card" :aria-label="t('dashboard.walletTitle')">
        <header><h2>{{ t('dashboard.walletTitle') }}</h2><span class="stat-badge"><Wallet />{{ t('dashboard.currency') }}</span></header>
        <p class="stat-value">{{ money(data.walletMinor) }}</p>
        <footer>
          <strong>{{ t('dashboard.walletDetail') }}</strong>
          <span v-if="data.wallet">{{ t('dashboard.totalRecharge', { amount: money(data.wallet.totalRechargeMinor) }) }}</span>
          <div class="stat-actions">
            <ElPopover placement="bottom-start" trigger="click" width="min(var(--recharge-panel-width), calc(100vw - var(--space-5)))">
              <template #reference><ElButton type="primary" size="small" :icon="Service">{{ t('dashboard.recharge') }}</ElButton></template>
              <strong>{{ t('dashboard.rechargeTitle') }}</strong>
              <p class="recharge-copy">{{ t('dashboard.rechargeInstructions') }}</p>
            </ElPopover>
            <LinkButton :to="walletPage" size="small">{{ t('dashboard.walletAction') }}</LinkButton>
          </div>
        </footer>
      </section>

      <section class="stat-card" :aria-label="t('dashboard.trafficTitle')">
        <header>
          <h2>{{ t('dashboard.trafficProduct') }}</h2>
          <span v-if="data.lowTraffic && data.threshold !== undefined" class="stat-badge warning"><Bottom />{{ t('dashboard.threshold', { amount: gb(data.threshold) }) }}</span>
          <span v-else-if="data.trafficBytes < 0n" class="stat-badge warning"><Bottom />{{ t('dashboard.negative') }}</span>
          <span v-else class="stat-badge"><CircleCheck />{{ t('dashboard.sufficient') }}</span>
        </header>
        <p class="stat-value" :class="{ debt: data.trafficBytes < 0n }">
          {{ data.trafficBytes < 0n ? t('dashboard.debt', { amount: gb(-data.trafficBytes) }) : displayGB(data.trafficBytes) }}
        </p>
        <footer>
          <strong>{{ t('dashboard.weekTotal', { amount: displayGB(usageTotal) }) }}</strong>
          <span>{{ t(data.canBuyTraffic ? 'dashboard.trafficDetail' : 'dashboard.offSale') }}</span>
          <div class="stat-actions">
            <LinkButton :to="trafficPricing" size="small">{{ t(data.canBuyTraffic ? 'dashboard.buyTraffic' : 'dashboard.viewPricing') }}<ArrowRight class="button-arrow" /></LinkButton>
          </div>
        </footer>
      </section>

      <section class="stat-card" :aria-label="t('dashboard.staticTitle')">
        <header>
          <h2>{{ t('dashboard.staticProduct') }}</h2>
          <span v-if="data.expiringIpCount" class="stat-badge warning"><Clock />{{ t('dashboard.expiring', data.expiringIpCount) }}</span>
          <span v-else class="stat-badge"><Connection />{{ t('dashboard.normal') }}</span>
        </header>
        <p class="stat-value">{{ new Intl.NumberFormat(locale).format(data.activeStaticIpCount) }} <small>{{ t('dashboard.ipUnit', data.activeStaticIpCount) }}</small></p>
        <footer>
          <strong v-if="data.expiry">{{ t('dashboard.nextExpiry') }} <time :datetime="data.expiry.toISOString()">{{ dateTime(data.expiry) }}</time></strong>
          <strong v-else>{{ t('dashboard.noExpiry') }}</strong>
          <span>{{ t('dashboard.staticDetail') }}</span>
          <div class="stat-actions">
            <LinkButton :to="{ name: 'console-static-pricing' }" size="small">{{ t(data.canBuyStatic ? 'dashboard.buyStatic' : 'dashboard.viewPricing') }}<ArrowRight class="button-arrow" /></LinkButton>
          </div>
        </footer>
      </section>
    </div>

    <div class="dash-main">
      <section class="details-card usage-card" aria-labelledby="dashboard-usage-title">
        <div class="usage-head">
          <div>
            <h2 id="dashboard-usage-title">{{ t('dashboard.usageTitle') }}</h2>
            <p v-if="data.days.length" class="usage-desc">
              {{ t('dashboard.usageTotal') }} <strong>{{ displayGB(usageTotal) }}</strong>
              <template v-if="data.updatedAt">
                · {{ t('dashboard.updated') }} <time :datetime="new Date(data.updatedAt).toISOString()">{{ dateTime(new Date(data.updatedAt)) }}</time>
              </template>
            </p>
          </div>
          <span v-if="data.days.length" class="usage-range">{{ rangeLabel }}</span>
        </div>
        <template v-if="data.days.length">
          <table v-if="showTable" class="usage-table">
            <thead><tr><th scope="col">{{ t('dashboard.businessDay') }}</th><th scope="col">{{ t('dashboard.usageAmount') }}</th></tr></thead>
            <tbody>
              <tr v-for="day in data.days" :key="day.date">
                <td>{{ dateLabel(day.date) }}</td><td>{{ displayGB(day.bytes) }}</td>
              </tr>
            </tbody>
          </table>
          <DailyUsageChart v-else :days="data.days" :label="t('dashboard.chartLabel')" :format-bytes="displayGB" :format-tick="gb" :amount-label="t('dashboard.usageDebit')" />
        </template>
        <div v-else class="dashboard-empty usage-empty"><CircleCheck /><span>{{ t('dashboard.noUsage') }}</span></div>
        <footer class="usage-foot">
          <span>{{ t('dashboard.usageBasis') }}<template v-if="data.days.length"> · <ElButton link type="primary" size="small" @click="showTable = !showTable">{{ t(showTable ? 'dashboard.showChart' : 'dashboard.showTable') }}</ElButton></template></span>
          <RouterLink class="reminder-link" :to="{ name: 'console-dynamic-usage' }">{{ t('dashboard.viewUsage') }}<ArrowRight /></RouterLink>
        </footer>
      </section>

      <section class="details-card todo-card" aria-labelledby="dashboard-todo-title">
        <header class="todo-head">
          <h2 id="dashboard-todo-title">{{ t('dashboard.todoTitle') }}</h2>
          <span v-if="todos.length" class="todo-count">{{ todos.length }}</span>
        </header>
        <ul v-if="todos.length" class="todo-list">
          <li v-for="item in todos" :key="item.id" class="todo-item" :class="item.level">
            <span class="todo-icon"><component :is="item.icon" /></span>
            <div class="todo-text">
              <strong>{{ item.title }}</strong>
              <p>{{ item.detail }}</p>
              <RouterLink class="reminder-link" :to="item.to">{{ item.action }}<ArrowRight /></RouterLink>
            </div>
          </li>
        </ul>
        <div v-else class="dashboard-empty"><CircleCheck /><span>{{ t('dashboard.noTodos') }}</span></div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.dashboard-title { inline-size: fit-content; max-inline-size: 100%; margin: 0 0 var(--dashboard-title-gap); font-size: var(--font-size-dashboard-display); line-height: var(--dashboard-title-height); letter-spacing: -.035em; font-weight: var(--font-weight-dashboard-display); }
.dashboard { line-height: normal; }
.stat-grid, .dash-main { display: grid; gap: var(--space-4); }
.stat-grid { grid-template-columns: repeat(3, minmax(0, 1fr)); margin-block-end: var(--space-4); }
.dash-main { grid-template-columns: minmax(0, 1fr) var(--dashboard-todo-width); align-items: stretch; }
.stat-card, .details-card {
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--dashboard-card-radius);
}
.stat-card {
  display: flex;
  flex-direction: column;
  gap: var(--dashboard-compact-gap);
  padding: var(--dashboard-stat-inset-block) var(--dashboard-stat-inset-inline);
  background: linear-gradient(to top, var(--color-dashboard-card-tint), var(--color-surface) 70%);
  box-shadow: var(--shadow-card);
}
.dashboard :is(h2, p) { margin: 0; }
.stat-card header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--space-2);
}
.stat-card h2 { color: var(--color-text-secondary); font-size: var(--font-size-sm); font-weight: var(--font-weight-regular); }
.dashboard svg { width: 1em; height: 1em; flex: none; }
.stat-badge {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: 1px 8px;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--radius-control);
  background: var(--color-surface);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-medium);
  line-height: 20px;
}
.stat-badge.warning { border-color: var(--color-warning-border); background: var(--color-warning-bg); color: var(--color-warning-text); }
.stat-value { font-size: var(--font-size-dashboard-display); font-weight: var(--font-weight-dashboard-display); line-height: var(--dashboard-value-height); letter-spacing: -.03em; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; }
.stat-value small { font-size: var(--font-size-md); font-weight: var(--font-weight-medium); color: var(--color-text-secondary); letter-spacing: 0; }
.stat-value.debt { color: var(--color-dashboard-critical); }
.stat-card footer { display: flex; flex-direction: column; gap: var(--dashboard-tight-gap); margin-block-start: var(--dashboard-medium-gap); font-size: var(--font-size-sm); }
.stat-card footer strong { font-weight: var(--font-weight-semibold); }
.stat-card footer > span, .recharge-copy { color: var(--color-text-secondary); }
.recharge-copy { margin-top: var(--space-2); }
.stat-actions { display: flex; flex-wrap: wrap; gap: var(--space-2); margin-block-start: var(--dashboard-loose-gap); }
.stat-actions :deep(.el-button), .usage-foot :deep(.el-button) { padding-inline: var(--space-3); font-size: var(--font-size-sm); }
.stat-actions :deep(.el-button) { height: auto; min-height: var(--space-5); padding-block: var(--dashboard-action-inset-block); margin-inline: 0; border-radius: var(--radius-usage-tooltip); white-space: normal; text-decoration: none; }
.button-arrow { margin-inline-start: var(--dashboard-compact-gap); }
.details-card { padding: var(--space-5); background: var(--color-surface); }
.details-card h2 { font-size: var(--font-size-lg); line-height: var(--space-5); }
.usage-head, .usage-foot { display: flex; flex-wrap: wrap; justify-content: space-between; }
.usage-head { align-items: flex-start; gap: var(--space-3); margin-block-end: var(--space-2); }
.dashboard .usage-desc { margin-block-start: 7px; color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.usage-desc strong { color: var(--color-text); font-variant-numeric: tabular-nums; }
.usage-range { padding: 4px 10px; border: var(--border-width) solid var(--color-border); border-radius: var(--radius-control); background: var(--color-surface-muted); color: var(--color-text-secondary); font-size: var(--font-size-xs); font-variant-numeric: tabular-nums; }
.usage-foot { gap: var(--space-2); margin-block-start: var(--space-2); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.usage-foot :deep(.el-button) { height: auto; padding-inline: var(--dashboard-tight-gap); }
.usage-table { width: 100%; border-collapse: collapse; font-size: var(--font-size-sm); font-variant-numeric: tabular-nums; }
.usage-table :is(th, td) { padding: 9px 12px; border-block-end: 1px solid var(--color-border); text-align: start; }
.usage-table th { color: var(--color-text-secondary); background: var(--color-surface-muted); font-weight: var(--font-weight-semibold); }
.usage-table :is(th, td):last-child { text-align: end; }
.reminder-link { display: inline-flex; align-items: center; gap: var(--space-1); color: var(--color-primary); font-size: var(--font-size-sm); font-weight: var(--font-weight-semibold); text-decoration: none; }
.reminder-link:hover { text-decoration: underline; }
.todo-card { display: flex; flex-direction: column; }
.todo-head { display: flex; align-items: center; gap: var(--dashboard-medium-gap); margin-block-end: var(--space-3); }
.todo-count { display: inline-grid; place-items: center; min-width: var(--dashboard-count-size); height: var(--dashboard-count-size); padding-inline: 5px; border-radius: 9px; background: var(--color-primary-soft); color: var(--color-primary); font-size: var(--font-size-usage-axis); font-weight: var(--font-weight-bold); }
.todo-list { display: flex; flex-direction: column; margin: 0; padding: 0; list-style: none; }
.todo-item { display: grid; grid-template-columns: var(--space-6) minmax(0, 1fr); gap: var(--space-3); padding-block: var(--dashboard-loose-gap); }
.todo-item + .todo-item { border-block-start: 1px solid var(--color-border); }
.todo-icon { display: inline-grid; place-items: center; width: var(--space-6); height: var(--space-6); border-radius: var(--radius-usage-tooltip); font-size: var(--font-size-lg); }
.critical .todo-icon { background: var(--color-dashboard-critical-bg); color: var(--color-dashboard-critical); }
.warning .todo-icon { background: var(--color-warning-bg); color: var(--color-warning-text); }
.info .todo-icon { background: var(--color-info-bg); color: var(--color-info-text); }
.info .todo-icon svg { animation: dashboard-order-pending 1.6s linear infinite; }
@keyframes dashboard-order-pending { to { transform: rotate(360deg); } }
.todo-text { display: flex; flex-direction: column; align-items: flex-start; gap: var(--dashboard-tight-gap); }
.todo-text strong { font-size: var(--font-size-md); }
.dashboard .todo-text p { margin-block-end: var(--space-1); color: var(--color-text-secondary); font-size: var(--font-size-xs); line-height: var(--dashboard-count-size); }
.dashboard-empty { display: flex; flex: 1; flex-direction: column; align-items: center; justify-content: center; gap: var(--space-2); min-height: var(--dashboard-empty-height); color: var(--color-text-secondary); font-size: var(--font-size-sm); }
.dashboard .dashboard-empty svg { width: 28px; height: 28px; color: var(--color-primary); }
.usage-empty { min-height: var(--dashboard-chart-empty-height); }
@media (max-width: 1180px) { .dash-main { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 950px) { .stat-grid { grid-template-columns: minmax(0, 1fr); } }
@media (max-width: 680px) { .dashboard-title { font-size: var(--font-size-title); } }
@media (prefers-reduced-motion: reduce) { .info .todo-icon svg { animation: none; } }
</style>
