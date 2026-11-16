<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CircleHelp, List, ReceiptText, Network, Package } from 'lucide-vue-next'
import type { WalletTenant } from './api/wallet'
import DashboardShell from './DashboardShell.vue'
import WalletRecords from './WalletRecords.vue'
import TenantSubaccounts from './TenantSubaccounts.vue'
import TrafficCheckout from './TrafficCheckout.vue'
import WalletOverview from './WalletOverview.vue'
import { Button } from './ui/button'

defineProps<{ token: string; account: WalletTenant }>()
const emit = defineEmits<{ logout: []; expired: []; wallet: [WalletTenant] }>()
const route = useRoute()
const router = useRouter()
const showPackages = computed(() => route.query.view === 'packages')
const showSubaccounts = ref(false)
const tab = ref<'entries' | 'orders'>('entries')

function show(view: 'packages' | 'subaccounts' | 'entries' | 'orders') {
  showSubaccounts.value = view === 'subaccounts'
  if (view === 'entries' || view === 'orders') tab.value = view
  void router.replace({ path: '/', query: view === 'packages' ? { view } : {} })
}
</script>

<template>
  <DashboardShell brand="代理控制台" area="租户控制台" :title="showPackages ? '购买流量套餐' : showSubaccounts ? '动态住宅代理' : '钱包与订单'" :account-name="account.name" @logout="emit('logout')">
    <template #navigation="{ closeSidebar }">
      <nav aria-label="租户导航">
        <p class="nav-label">动态住宅代理</p>
        <Button variant="ghost" type="button" :aria-current="showPackages ? 'page' : undefined" @click="show('packages'); closeSidebar()"><Package />流量套餐</Button>
        <Button variant="ghost" type="button" :aria-current="!showPackages && showSubaccounts ? 'page' : undefined" @click="show('subaccounts'); closeSidebar()"><Network />代理子账号</Button>
        <p class="nav-label">钱包与订单</p>
        <Button variant="ghost" type="button" :aria-current="!showPackages && !showSubaccounts && tab === 'entries' ? 'page' : undefined" @click="show('entries'); closeSidebar()"><List />钱包流水</Button>
        <Button variant="ghost" type="button" :aria-current="!showPackages && !showSubaccounts && tab === 'orders' ? 'page' : undefined" @click="show('orders'); closeSidebar()"><ReceiptText />订单记录</Button>
      </nav>
    </template>

    <TrafficCheckout v-if="showPackages" :token="token" :account="account" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <TenantSubaccounts v-else-if="showSubaccounts" :token="token" :primary-account="account.primary_account_name" @expired="emit('expired')" />
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
