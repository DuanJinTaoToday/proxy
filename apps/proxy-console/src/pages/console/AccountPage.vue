<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton } from 'element-plus'
import { getPendingOrderCount } from '@/api/orders'
import { sessionEmail } from '@/features/auth'
import RouteTabs from '@/app/RouteTabs.vue'

const route = useRoute()
const { t, n } = useI18n()
const pendingCount = ref(0)
const countFailed = ref(false)
const retry = ref(0)

function applyPendingCount(count: number) {
  pendingCount.value = count
}

watch([() => route.name, sessionEmail, retry], async ([name, email], previous, onCleanup) => {
  if (email !== previous?.[1]) pendingCount.value = 0
  countFailed.value = false
  if (!email || name !== 'console-account-profile') return
  const request = new AbortController()
  onCleanup(() => request.abort())
  try {
    const count = await getPendingOrderCount(request.signal)
    if (!request.signal.aborted) applyPendingCount(count)
  } catch {
    if (!request.signal.aborted) {
      countFailed.value = true
    }
  }
}, { immediate: true })
</script>

<template>
  <RouteTabs>
    <template #label="{ tab }">
      {{ t(tab.labelKey) }}
      <span v-if="tab.name === 'console-account-orders' && pendingCount > 0"
        class="pending-count tabular" :aria-label="t('orders.pendingCount', pendingCount)">
        {{ n(pendingCount) }}
      </span>
    </template>
    <ElAlert v-if="countFailed" class="pending-count-alert" type="error" :title="t('orders.pendingCountFailed')"
      :closable="false" show-icon>
      <ElButton size="small" @click="retry++">{{ t('orders.retry') }}</ElButton>
    </ElAlert>
    <RouterView v-slot="{ Component }">
      <component :is="Component" v-if="route.name === 'console-account-orders'" @pending-count="applyPendingCount" />
      <component :is="Component" v-else />
    </RouterView>
  </RouteTabs>
</template>

<style scoped>
.pending-count {
  display: inline-grid;
  place-items: center;
  min-inline-size: var(--dashboard-count-size);
  block-size: var(--dashboard-count-size);
  margin-inline-start: var(--dashboard-compact-gap);
  padding-inline: var(--space-1);
  border-radius: 1em;
  background: var(--color-primary-soft);
  color: var(--color-primary);
  font-size: var(--font-size-usage-axis);
  font-weight: var(--font-weight-bold);
}
.pending-count-alert { margin-bottom: var(--space-4); }
</style>
