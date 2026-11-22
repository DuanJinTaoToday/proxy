<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElButton, ElEmpty, ElPopover, ElSkeleton } from 'element-plus'
import { Bell, Check, CircleCheck, Close } from '@element-plus/icons-vue'
import { claimStaticExpiryNotices, getOverview, listActiveStaticExpiries } from '@/api/dashboard'
import { getTrafficAlert } from '@/api/trafficAlert'
import { dayNumber, shanghaiDateTime, shanghaiToday } from '@/shared/lib/businessDates'
import { formatGB } from '@/shared/lib/trafficUnits'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { trafficNoticeRevision } from './trafficNotice'

const props = defineProps<{ accountKey: string }>()
const { t, locale } = useI18n()
const route = useRoute()
const open = ref(false)
const trafficLoading = ref(false)
const staticLoading = ref(false)
const trafficFailed = ref(false)
const staticFailed = ref(false)
const loading = computed(() => trafficLoading.value || staticLoading.value)
const failed = computed(() => trafficFailed.value || staticFailed.value)
const trafficAlert = shallowRef<Awaited<ReturnType<typeof getTrafficAlert>>>()
const trafficOccurrence = computed(() => trafficAlert.value?.occurrence)
const expiringAllocations = shallowRef<Awaited<ReturnType<typeof listActiveStaticExpiries>>>([])
const earliestExpiry = computed(() => expiringAllocations.value[0]?.expiresAt)
const controller = new AbortController()
let trafficRequest: AbortController | undefined
onScopeDispose(() => {
  controller.abort()
  trafficRequest?.abort()
})

const storageKey = `proxy-console.notifications.${props.accountKey}`
const claimKey = `${storageKey}.claim-pending`
let claimPending = false

if (props.accountKey) {
  try { claimPending = sessionStorage.getItem(claimKey) === '1' } catch { /* Fall back to in-memory state. */ }
}

function setClaimPending(pending: boolean) {
  claimPending = pending
  try {
    if (pending) sessionStorage.setItem(claimKey, '1')
    else sessionStorage.removeItem(claimKey)
  } catch { /* Fall back to in-memory state. */ }
}
type StaticReadStatus = 'read' | 'unread'
interface ReadState {
  trafficRead?: string
  staticStatus: Partial<Record<string, StaticReadStatus>>
}

function loadReadState(): ReadState {
  try {
    const stored = JSON.parse(localStorage.getItem(storageKey) ?? '{}')
    return {
      trafficRead: typeof stored?.trafficRead === 'string' ? stored.trafficRead : undefined,
      staticStatus: Object.fromEntries(Object.entries(stored?.staticStatus ?? {})
        .flatMap(([key, status]) => status === 'read' || status === 'unread' ? [[key, status]] : [])),
    }
  } catch {
    return { staticStatus: {} }
  }
}
const readState = shallowRef<ReadState>(props.accountKey ? loadReadState() : { staticStatus: {} })
const unreadTraffic = computed(() => trafficOccurrence.value !== undefined && trafficOccurrence.value !== readState.value.trafficRead)
const unreadStatic = computed(() => expiringAllocations.value.some(({ key }) => readState.value.staticStatus[key] === 'unread'))
const unread = computed(() => unreadTraffic.value || unreadStatic.value)
const trafficDetail = computed(() => {
  const alert = trafficAlert.value
  if (!alert) return ''
  const balance = alert.availableBytes
  const balanceText = t(balance < 0n ? 'notifications.trafficDebt' : 'notifications.trafficBalance', {
    value: formatGB(balance < 0n ? -balance : balance, locale.value),
  })
  if (alert.thresholdBytes === undefined) return balanceText
  return t('notifications.trafficDetail', { balance: balanceText, threshold: formatGB(alert.thresholdBytes, locale.value) })
})
const shanghaiDate = new Intl.DateTimeFormat('sv-SE', { dateStyle: 'short', timeZone: 'Asia/Shanghai' })

function relativeTriggerDate(date: Date) {
  const days = dayNumber(shanghaiDate.format(date)) - dayNumber(shanghaiToday())
  return days > 0
    ? shanghaiDateTime(date, locale.value)
    : new Intl.RelativeTimeFormat(locale.value, { numeric: 'auto' }).format(days, 'day')
}

function saveReadState(next: ReadState) {
  readState.value = next
  try { localStorage.setItem(storageKey, JSON.stringify(next)) } catch { /* 当前页面仍保留已读状态。 */ }
}

function syncReadState(event: StorageEvent) {
  if (event.key === storageKey) readState.value = loadReadState()
}
if (props.accountKey) {
  window.addEventListener('storage', syncReadState)
  onScopeDispose(() => window.removeEventListener('storage', syncReadState))
}

async function loadTraffic() {
  if (!props.accountKey) return
  trafficRequest?.abort()
  const request = new AbortController()
  trafficRequest = request
  trafficLoading.value = true
  trafficFailed.value = false
  try {
    const traffic = await getTrafficAlert(request.signal)
    if (!request.signal.aborted) trafficAlert.value = traffic
  } catch {
    if (!request.signal.aborted) {
      trafficAlert.value = undefined
      trafficFailed.value = true
    }
  } finally {
    if (!request.signal.aborted) trafficLoading.value = false
  }
}

async function loadStatic() {
  if (!props.accountKey || staticLoading.value) return
  staticLoading.value = true
  staticFailed.value = false
  try {
    const overview = await getOverview(controller.signal)
    if (controller.signal.aborted) return
    const active = overview.expiringIpCount > 0
      ? await listActiveStaticExpiries(overview.activeStaticIpCount, controller.signal)
      : []
    if (controller.signal.aborted) return
    const expiring = active.slice(0, overview.expiringIpCount)
    const keys = expiring.map(allocation => allocation.key)
    let newStatus: StaticReadStatus = 'read'
    if (keys.some(key => !readState.value.staticStatus[key])) {
      const recoverClaim = claimPending
      // The server can record the claim even if the response never reaches this tab.
      setClaimPending(true)
      const claimed = await claimStaticExpiryNotices()
      if (claimed > 0 || recoverClaim) newStatus = 'unread'
    }
    const previous = readState.value
    const staticStatus = Object.fromEntries(keys.map(key => [key, previous.staticStatus[key] ?? newStatus]))
    saveReadState({ ...previous, staticStatus })
    setClaimPending(false)
    if (controller.signal.aborted) return
    expiringAllocations.value = expiring
  } catch {
    if (!controller.signal.aborted) {
      expiringAllocations.value = []
      staticFailed.value = true
    }
  } finally {
    staticLoading.value = false
  }
}

function load() {
  if (!trafficLoading.value) void loadTraffic()
  void loadStatic()
}

watch(() => route.path, load, { immediate: true })
watch(trafficNoticeRevision, loadTraffic)

function refreshOnOpen() {
  if (!failed.value) void load()
}

function markRead(kind: 'traffic' | 'static' | 'all') {
  const next = { ...readState.value }
  if (kind === 'traffic' || kind === 'all') next.trafficRead = trafficOccurrence.value
  if (kind === 'static' || kind === 'all') {
    next.staticStatus = Object.fromEntries(expiringAllocations.value.map(({ key }) => [key, 'read']))
  }
  saveReadState(next)
  if (kind !== 'all') open.value = false
}
</script>

<template>
  <ElPopover
    v-model:visible="open"
    placement="bottom-end"
    trigger="click"
    :teleported="false"
    :show-arrow="false"
    :offset="10"
    width="min(var(--notification-panel-width), calc(100vw - var(--space-5)))"
    :popper-style="{ padding: 0, borderRadius: '14px', boxShadow: 'var(--shadow-overlay)' }"
    @before-enter="refreshOnOpen"
  >
    <template #reference>
      <button
        type="button"
        class="notification-bell__trigger"
        :class="{ 'is-open': open }"
        :aria-expanded="open"
        :aria-label="t(unread ? 'notifications.unreadLabel' : 'header.notifications')"
      >
        <Bell aria-hidden="true" />
        <span v-if="unread" class="notification-bell__badge" aria-hidden="true" />
      </button>
    </template>

    <div class="notification-bell__panel">
      <div class="notification-bell__heading">
        <h2>{{ t('header.notifications') }}</h2>
        <div class="notification-bell__actions">
          <button v-if="!failed && unread" type="button" :disabled="loading" @click="markRead('all')">
            <Check aria-hidden="true" />
            {{ t('notifications.markAllRead') }}
          </button>
          <button type="button" class="notification-bell__close" :aria-label="t('notifications.close')" @click="open = false">
            <Close aria-hidden="true" />
          </button>
        </div>
      </div>
      <ElEmpty v-if="!accountKey" :description="t('header.guestNotifications')" :image-size="56">
        <LinkButton :to="`/${locale}/login`" size="small">{{ t('header.login') }}</LinkButton>
      </ElEmpty>
      <template v-else>
        <div v-if="failed" class="notification-bell__failure">
          <p role="alert">{{ t('notifications.failed') }}</p>
          <ElButton size="small" @click="load">{{ t('notifications.retry') }}</ElButton>
        </div>
        <ul v-if="trafficOccurrence || earliestExpiry" class="notification-bell__list">
          <li v-if="trafficOccurrence">
            <RouterLink class="notification-bell__item" :class="{ 'is-unread': unreadTraffic }" :to="{ name: 'console-dynamic-pricing' }" @click="markRead('traffic')">
              <span class="notification-bell__dot" aria-hidden="true" />
              <span class="notification-bell__body">
                <strong>{{ t('notifications.lowTraffic') }}</strong>
                <span>{{ trafficDetail }}</span>
              </span>
              <time
                v-if="trafficAlert?.triggeredAt"
                class="notification-bell__time"
                :datetime="trafficAlert.triggeredAt.toISOString()"
                :title="shanghaiDateTime(trafficAlert.triggeredAt, locale)"
              >{{ relativeTriggerDate(trafficAlert.triggeredAt) }}</time>
            </RouterLink>
          </li>
          <li v-if="earliestExpiry">
            <RouterLink class="notification-bell__item" :class="{ 'is-unread': unreadStatic }" :to="{ name: 'console-static-ips' }" @click="markRead('static')">
              <span class="notification-bell__dot" aria-hidden="true" />
              <span class="notification-bell__body">
                <strong>{{ t('notifications.expiring') }}</strong>
                <i18n-t keypath="notifications.expiryDetail" tag="span" :plural="expiringAllocations.length">
                  <template #expiry>
                    <time :datetime="earliestExpiry.toISOString()">{{ shanghaiDateTime(earliestExpiry, locale) }}</time>
                  </template>
                </i18n-t>
              </span>
            </RouterLink>
          </li>
        </ul>
        <div v-else-if="!failed && loading" class="notification-bell__loading" role="status" :aria-label="t('notifications.loading')">
          <ElSkeleton animated :rows="2" />
        </div>
        <div v-else-if="!failed" class="notification-bell__empty">
          <CircleCheck aria-hidden="true" />
          <span>{{ t('notifications.empty') }}</span>
        </div>
      </template>
    </div>
  </ElPopover>
</template>

<style scoped>
.notification-bell__trigger {
  position: relative;
  display: inline-grid;
  place-items: center;
  flex: none;
  width: 38px;
  height: 38px;
  padding: 0;
  border: 0;
  border-radius: var(--radius-card);
  background: transparent;
  color: var(--notification-trigger-color, var(--color-text));
  cursor: pointer;
}
.notification-bell__trigger:hover, .notification-bell__trigger.is-open { background: var(--notification-trigger-hover, var(--color-primary-soft)); }
.notification-bell__trigger svg { width: var(--notification-bell-size); height: var(--notification-bell-size); }
.notification-bell__badge {
  position: absolute;
  top: 7px;
  right: var(--space-2);
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--color-notification-unread);
  box-shadow: 0 0 0 var(--focus-ring-width) var(--color-surface-muted);
}
.notification-bell__heading {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 14px var(--space-4);
  border-bottom: var(--border-width) solid var(--color-border);
}
.notification-bell__heading h2 { font-size: 15px; }
.notification-bell__actions { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; margin-inline-start: auto; }
.notification-bell__actions button {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  padding: var(--space-1) 6px;
  border: 0;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  cursor: pointer;
}
.notification-bell__actions button:disabled { color: var(--color-text-disabled); cursor: default; }
.notification-bell__actions button:hover:not(:disabled) { background: var(--color-surface-muted); color: var(--color-text); }
.notification-bell__actions .notification-bell__close { font-size: 15px; }
.notification-bell__actions svg { flex: none; width: 1em; height: 1em; }
.notification-bell__list {
  max-height: min(var(--notification-panel-width), 60vh);
  overflow-y: auto;
  list-style: none;
  padding: 0;
  margin: 0;
}
.notification-bell__list li + li { border-block-start: var(--border-width) solid var(--color-border); }
.notification-bell__item {
  display: grid;
  grid-template-columns: var(--space-2) minmax(0, 1fr) auto;
  align-items: start;
  gap: 10px;
  padding: 14px var(--space-4);
  color: var(--color-text);
  text-decoration: none;
  overflow-wrap: anywhere;
}
.notification-bell__item:hover { background: var(--color-surface-muted); }
.notification-bell__item:focus-visible { outline-offset: calc(-1 * var(--focus-ring-width)); }
.notification-bell__dot { width: 6px; height: 6px; margin-top: var(--space-2); border-radius: 50%; }
.notification-bell__item.is-unread .notification-bell__dot { background: var(--color-primary); }
.notification-bell__body { display: flex; flex-direction: column; gap: var(--space-1); min-width: 0; }
.notification-bell__item:not(.is-unread) strong { color: var(--color-text-secondary); font-weight: var(--font-weight-medium); }
.notification-bell__body > span, .notification-bell__time { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.notification-bell__body > span { line-height: 1.5; }
.notification-bell__time { margin-top: 2px; max-width: 8em; }
.notification-bell__empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  padding: 28px var(--space-4);
  color: var(--color-text-secondary);
  font-size: var(--font-size-sm);
}
.notification-bell__empty svg { width: var(--space-5); height: var(--space-5); color: var(--color-primary); }
.notification-bell__failure, .notification-bell__loading { padding: var(--space-3) var(--space-4); }
.notification-bell__failure { display: grid; justify-items: start; gap: var(--space-2); }
</style>
