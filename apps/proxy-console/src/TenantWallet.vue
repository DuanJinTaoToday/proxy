<script setup lang="ts">
import { ref } from 'vue'
import { CircleHelp, List, ReceiptText } from 'lucide-vue-next'
import type { WalletTenant } from './api/wallet'
import DashboardShell from './DashboardShell.vue'
import WalletRecords from './WalletRecords.vue'
import WalletOverview from './WalletOverview.vue'
import { Button } from './ui/button'

defineProps<{ token: string; account: WalletTenant }>()
const emit = defineEmits<{ logout: []; expired: [] }>()
const tab = ref<'entries' | 'orders'>('entries')
</script>

<template>
  <DashboardShell brand="代理控制台" area="租户控制台" title="钱包与订单" :account-name="account.name" @logout="emit('logout')">
    <template #navigation="{ closeSidebar }">
      <nav aria-label="钱包导航">
        <p class="nav-label">钱包与订单</p>
        <Button variant="ghost" type="button" :aria-current="tab === 'entries' ? 'page' : undefined" @click="tab = 'entries'; closeSidebar()"><List />钱包流水</Button>
        <Button variant="ghost" type="button" :aria-current="tab === 'orders' ? 'page' : undefined" @click="tab = 'orders'; closeSidebar()"><ReceiptText />订单记录</Button>
      </nav>
    </template>

    <WalletRecords v-model:view="tab" :token="token" @expired="emit('expired')">
      <template #overview="{ recentEntries }">
        <WalletOverview :balance="account.available_minor" :entries="recentEntries" />
      </template>
      <template #records-help>
        <p class="support-note"><CircleHelp :size="15" />充值、开票和售后请线下联系运营。</p>
      </template>
    </WalletRecords>
  </DashboardShell>
</template>
