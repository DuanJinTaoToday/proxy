<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElBadge, ElButton, ElEmpty, ElPopover, ElSkeleton } from 'element-plus'
import { Bell, Check, Close } from '@element-plus/icons-vue'
import { claimStaticExpiryNotices, getOverview, getTrafficAlert, listActiveStaticExpiries } from '@/api/dashboard'
import LinkButton from '@/shared/ui/LinkButton.vue'

const props = defineProps<{ accountKey: string }>()
const { t, locale } = useI18n()
const route = useRoute()
const bell = ref<InstanceType<typeof ElPopover>>()
const ready = ref(false)
const loading = ref(false)
const failed = ref(false)
const trafficOccurrence = ref<string>()
const expiringKeys = ref<string[]>([])
const controller = new AbortController()
onScopeDispose(() => controller.abort())

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
const unreadStatic = computed(() => expiringKeys.value.some(key => readState.value.staticStatus[key] === 'unread'))
const unread = computed(() => unreadTraffic.value || unreadStatic.value)

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

async function load() {
  if (!props.accountKey || loading.value) return
  loading.value = true
  failed.value = false
  try {
    const [overview, traffic] = await Promise.all([getOverview(controller.signal), getTrafficAlert(controller.signal)])
    if (controller.signal.aborted) return
    const active = overview.expiringIpCount > 0
      ? await listActiveStaticExpiries(overview.activeStaticIpCount, controller.signal)
      : []
    if (controller.signal.aborted) return
    const keys = active.slice(0, overview.expiringIpCount).map(allocation => allocation.key)
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
    expiringKeys.value = keys
    trafficOccurrence.value = traffic.occurrence
    ready.value = true
  } catch {
    if (!controller.signal.aborted) failed.value = true
  } finally {
    loading.value = false
  }
}

watch(() => route.path, load, { immediate: true })

function refreshOnOpen() {
  if (!failed.value) void load()
}

function markRead(kind: 'traffic' | 'static' | 'all') {
  const next = { ...readState.value }
  if (kind === 'traffic' || kind === 'all') next.trafficRead = trafficOccurrence.value
  if (kind === 'static' || kind === 'all') {
    next.staticStatus = Object.fromEntries(expiringKeys.value.map(key => [key, 'read']))
  }
  saveReadState(next)
  if (kind !== 'all') bell.value?.hide()
}
</script>

<template>
  <ElPopover ref="bell" placement="bottom-end" trigger="click" :teleported="false" width="min(var(--notification-panel-width), calc(100vw - var(--space-5)))" @before-enter="refreshOnOpen">
    <template #reference>
      <ElButton text circle :aria-label="t(unread ? 'notifications.unreadLabel' : 'header.notifications')">
        <ElBadge is-dot :hidden="!unread"><Bell class="notification-bell__icon" /></ElBadge>
      </ElButton>
    </template>

    <div class="notification-bell__panel">
      <div class="notification-bell__heading">
        <h2>{{ t('header.notifications') }}</h2>
        <div class="notification-bell__actions">
          <ElButton v-if="ready && !failed && unread" :disabled="loading" :icon="Check" link type="primary" @click="markRead('all')">
            {{ t('notifications.markAllRead') }}
          </ElButton>
          <ElButton text circle size="small" :icon="Close" :aria-label="t('notifications.close')" @click="bell?.hide()" />
        </div>
      </div>
      <ElEmpty v-if="!accountKey" :description="t('header.guestNotifications')" :image-size="56">
        <LinkButton :to="`/${locale}/login`" size="small">{{ t('header.login') }}</LinkButton>
      </ElEmpty>
      <div v-else-if="failed" class="notification-bell__failure">
        <p role="alert">{{ t('notifications.failed') }}</p>
        <ElButton size="small" @click="load">{{ t('notifications.retry') }}</ElButton>
      </div>
      <div v-else-if="!ready" role="status" :aria-label="t('notifications.loading')">
        <ElSkeleton animated :rows="2" />
      </div>
      <ul v-else-if="trafficOccurrence || expiringKeys.length">
        <li v-if="trafficOccurrence">
          <RouterLink :to="{ name: 'console-dynamic-pricing' }" @click="markRead('traffic')">
            <ElBadge is-dot :hidden="!unreadTraffic"><strong>{{ t('notifications.lowTraffic') }}</strong></ElBadge>
            <span>{{ t('notifications.trafficAction') }}</span>
          </RouterLink>
        </li>
        <li v-if="expiringKeys.length">
          <RouterLink :to="{ name: 'console-static-ips' }" @click="markRead('static')">
            <ElBadge is-dot :hidden="!unreadStatic"><strong class="tabular">{{ t('notifications.expiring', expiringKeys.length) }}</strong></ElBadge>
            <span>{{ t('notifications.staticAction') }}</span>
          </RouterLink>
        </li>
      </ul>
      <ElEmpty v-else :description="t('notifications.empty')" :image-size="56" />
    </div>
  </ElPopover>
</template>

<style scoped>
.notification-bell__icon { display: block; width: var(--icon-size); height: var(--icon-size); }
.notification-bell__heading, .notification-bell__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.notification-bell__actions { margin-inline-start: auto; }
.notification-bell__panel h2 { font-size: var(--font-size-md); font-weight: var(--font-weight-semibold); }
.notification-bell__panel ul { list-style: none; padding: 0; margin: var(--space-2) 0 0; }
.notification-bell__panel li + li { border-block-start: var(--border-width) solid var(--color-border); }
.notification-bell__panel a {
  display: grid;
  gap: var(--space-1);
  padding: var(--space-2);
  color: var(--color-text);
  text-decoration: none;
  overflow-wrap: anywhere;
}
.notification-bell__panel a:hover, .notification-bell__panel a:focus-visible { background: var(--color-surface-muted); }
.notification-bell__panel a > span { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.notification-bell__failure { display: grid; justify-items: start; gap: var(--space-2); margin-block-start: var(--space-2); }
</style>
