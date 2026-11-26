<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { sessionEmail } from '@/features/auth'
import { OrderDrawer } from '@/features/orders'
import { WalletLedger, WalletOverview } from '@/features/wallet'
import PageHeader from '@/shared/ui/PageHeader.vue'

const { t } = useI18n()
const selectedOrderId = ref<string>()

function closeOrder() {
  selectedOrderId.value = undefined
}

watch(sessionEmail, closeOrder)
</script>

<template>
  <PageHeader :title="t('nav.wallet')" />
  <WalletOverview :key="`overview-${sessionEmail}`" />
  <WalletLedger :key="`ledger-${sessionEmail}`" @open-order="selectedOrderId = $event" />
  <OrderDrawer :order-id="selectedOrderId" @close="closeOrder" />
</template>
