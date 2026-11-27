<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { ElTabPane, ElTabs } from 'element-plus'
import { OpsPendingOrders } from '@/features/orders'
import { OpsStaticExchanges } from '@/features/ops-static-exchanges'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

type OrdersTab = 'orders' | 'static-exchanges'

const activeTab = computed<OrdersTab>({
  get: () => (route.query.tab === 'static-exchanges' ? 'static-exchanges' : 'orders'),
  set: (value) => {
    const query = { ...route.query }
    if (value === 'static-exchanges') {
      query.tab = 'static-exchanges'
    } else {
      delete query.tab
    }
    void router.replace({ query })
  },
})
</script>

<template>
  <h1 class="ops-page-title">{{ t('nav.opsOrders') }}</h1>
  <ElTabs v-model="activeTab" class="ops-orders-tabs">
    <ElTabPane name="orders" :label="t('opsExchange.tabs.pendingOrders')">
      <OpsPendingOrders />
    </ElTabPane>
    <ElTabPane name="static-exchanges" :label="t('opsExchange.tabs.staticExchanges')">
      <OpsStaticExchanges v-if="activeTab === 'static-exchanges'" />
    </ElTabPane>
  </ElTabs>
</template>

<style scoped>
.ops-orders-tabs {
  min-width: 0;
  max-width: 100%;
}
</style>
