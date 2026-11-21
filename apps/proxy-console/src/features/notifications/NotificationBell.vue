<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElBadge, ElButton, ElEmpty, ElPopover, ElSkeleton } from 'element-plus'
import { Bell } from '@element-plus/icons-vue'
import { claimStaticExpiryNotices, getOverview, getTrafficAlert } from '@/api/dashboard'
import LinkButton from '@/shared/ui/LinkButton.vue'

const props = defineProps<{ accountKey: string }>()
const { t, locale } = useI18n()
const route = useRoute()
const bell = ref<InstanceType<typeof ElPopover>>()
const ready = ref(false)
const loading = ref(false)
const failed = ref(false)
const trafficOccurrence = ref<string>()
const expiring = ref(0)
const controller = new AbortController()
onScopeDispose(() => controller.abort())

const storageKey = `proxy-console.notifications.${props.accountKey}`
interface ReadState {
  trafficRead?: string
  staticUnread?: boolean
}

function loadReadState(): ReadState {
  try {
    const stored = JSON.parse(localStorage.getItem(storageKey) ?? '{}')
    return { trafficRead: stored?.trafficRead, staticUnread: stored?.staticUnread === true }
  } catch {
    return {}
  }
}
const readState = shallowRef(props.accountKey ? loadReadState() : {})
const unreadTraffic = computed(() => trafficOccurrence.value !== undefined && trafficOccurrence.value !== readState.value.trafficRead)
const unreadStatic = computed(() => expiring.value > 0 && readState.value.staticUnread === true)
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
    const count = overview.expiringIpCount
    const claimed = count > 0 ? await claimStaticExpiryNotices() : 0
    // 领取会改变服务端状态。即使用户已换页，也保存旧账号成功领取的结果。
    const previous = loadReadState()
    const staticUnread = count > 0 && (claimed > 0 || previous.staticUnread === true)
    if (staticUnread !== Boolean(previous.staticUnread)) saveReadState({ ...previous, staticUnread })
    if (controller.signal.aborted) return
    expiring.value = count
    trafficOccurrence.value = traffic.occurrence
    ready.value = true
  } catch {
    if (!controller.signal.aborted) failed.value = true
  } finally {
    loading.value = false
  }
}

watch(() => route.path, load, { immediate: true })

function markRead(kind: 'traffic' | 'static') {
  if (kind === 'traffic') saveReadState({ ...readState.value, trafficRead: trafficOccurrence.value })
  else saveReadState({ ...readState.value, staticUnread: false })
  bell.value?.hide()
}
</script>

<template>
  <ElPopover ref="bell" placement="bottom-end" trigger="click" :teleported="false" width="min(300px, calc(100vw - 24px))" @before-enter="load">
    <template #reference>
      <ElButton text circle :aria-label="t(unread ? 'notifications.unreadLabel' : 'header.notifications')">
        <ElBadge is-dot :hidden="!unread"><Bell class="notification-bell__icon" /></ElBadge>
      </ElButton>
    </template>

    <div class="notification-bell__panel">
      <h2>{{ t('header.notifications') }}</h2>
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
      <ul v-else-if="trafficOccurrence || expiring">
        <li v-if="trafficOccurrence">
          <RouterLink :to="{ name: 'console-dynamic-pricing' }" @click="markRead('traffic')">
            <ElBadge is-dot :hidden="!unreadTraffic"><strong>{{ t('notifications.lowTraffic') }}</strong></ElBadge>
            <span>{{ t('notifications.trafficAction') }}</span>
          </RouterLink>
        </li>
        <li v-if="expiring">
          <RouterLink :to="{ name: 'console-static-ips' }" @click="markRead('static')">
            <ElBadge is-dot :hidden="!unreadStatic"><strong class="tabular">{{ t('notifications.expiring', expiring) }}</strong></ElBadge>
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
