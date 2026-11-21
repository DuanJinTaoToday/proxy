<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElTag } from 'element-plus'
import { getPendingOrderCount } from '@/api/orders'
import { sessionPhone } from '@/features/auth'
import RouteTabs from '@/app/RouteTabs.vue'

const route = useRoute()
const { t, n } = useI18n()
const pendingCount = ref(0)
const countFailed = ref(false)
const retry = ref(0)

function applyPendingCount(count: number) {
  pendingCount.value = count
}

watch([() => route.name, sessionPhone, retry], async ([name, phone], previous, onCleanup) => {
  if (phone !== previous?.[1]) pendingCount.value = 0
  countFailed.value = false
  if (!phone || name !== 'console-account-profile') return
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
      <ElTag v-if="tab.name === 'console-account-orders' && pendingCount > 0" type="warning" round size="small"
        class="pending-count tabular" :aria-label="t('orders.pendingCount', pendingCount)" disable-transitions>
        {{ n(pendingCount) }}
      </ElTag>
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
.pending-count { margin-inline-start: var(--space-2); }
.pending-count-alert { margin-bottom: var(--space-4); }
</style>
