<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElTag } from 'element-plus'
import { getPendingOrderCount } from '@/api/orders'
import { sessionPhone } from '@/features/auth'
import RouteTabs from '@/app/RouteTabs.vue'

const route = useRoute()
const { t, n } = useI18n()
const pendingCount = ref(0)

watch([() => route.name, sessionPhone], async ([, phone], previous, onCleanup) => {
  if (phone !== previous?.[1]) pendingCount.value = 0
  if (!phone) return
  const request = new AbortController()
  onCleanup(() => request.abort())
  try {
    const count = await getPendingOrderCount(request.signal)
    if (!request.signal.aborted) pendingCount.value = count
  } catch {
    if (!request.signal.aborted) pendingCount.value = 0
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
  </RouteTabs>
</template>

<style scoped>
.pending-count { margin-inline-start: var(--space-2); }
</style>
