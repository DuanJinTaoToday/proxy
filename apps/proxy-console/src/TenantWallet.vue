<script setup lang="ts">
import { ref } from 'vue'
import { CircleHelp, List, ReceiptText, Network } from 'lucide-vue-next'
import type { WalletTenant } from './api/wallet'
import DashboardShell from './DashboardShell.vue'
import WalletRecords from './WalletRecords.vue'
import TenantSubaccounts from './TenantSubaccounts.vue'
import WalletOverview from './WalletOverview.vue'
import { Button } from './ui/button'

defineProps<{ token: string; account: WalletTenant }>()
const emit = defineEmits<{ logout: []; expired: [] }>()
const showSubaccounts = ref(false)
const tab = ref<'entries' | 'orders'>('entries')
</script>

<template>
  <DashboardShell brand="代理控制台" area="租户控制台" :title="showSubaccounts ? '动态住宅代理' : '钱包与订单'" :account-name="account.name" @logout="emit('logout')">
    <template #navigation="{ closeSidebar }">
      <nav aria-label="租户导航">
        <p class="nav-label">动态住宅代理</p>
        <Button variant="ghost" type="button" :aria-current="showSubaccounts ? 'page' : undefined" @click="showSubaccounts = true; closeSidebar()"><Network />代理子账号</Button>
        <p class="nav-label">钱包与订单</p>
        <Button variant="ghost" type="button" :aria-current="!showSubaccounts && tab === 'entries' ? 'page' : undefined" @click="showSubaccounts = false; tab = 'entries'; closeSidebar()"><List />钱包流水</Button>
        <Button variant="ghost" type="button" :aria-current="!showSubaccounts && tab === 'orders' ? 'page' : undefined" @click="showSubaccounts = false; tab = 'orders'; closeSidebar()"><ReceiptText />订单记录</Button>
      </nav>
    </template>

    <TenantSubaccounts v-if="showSubaccounts" :token="token" :primary-account="account.primary_account_name" @expired="emit('expired')" />
    <WalletRecords v-else v-model:view="tab" :token="token" @expired="emit('expired')">
      <template #overview="{ recentEntries }">
        <WalletOverview :balance="account.available_minor" :entries="recentEntries" />
      </template>
      <template #records-help>
        <p class="support-note"><CircleHelp :size="15" />充值、开票和售后请线下联系运营。</p>
      </template>
    </WalletRecords>
  </DashboardShell>
</template>
