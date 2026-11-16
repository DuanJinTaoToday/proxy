<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ChartColumn, CircleHelp, List, ReceiptText, Network, Package } from 'lucide-vue-next'
import type { WalletTenant } from './api/wallet'
import DashboardShell from './DashboardShell.vue'
import WalletRecords from './WalletRecords.vue'
import TenantSubaccounts from './TenantSubaccounts.vue'
import TenantDailyUsage from './TenantDailyUsage.vue'
import TenantTrafficAlert from './TenantTrafficAlert.vue'
import TrafficCheckout from './TrafficCheckout.vue'
import WalletOverview from './WalletOverview.vue'
import StaticCheckout from './StaticCheckout.vue'
import TenantStaticIPs from './TenantStaticIPs.vue'
import { listProducts } from './api/product'
import { Button } from './ui/button'

defineProps<{ token: string; account: WalletTenant }>()
const emit = defineEmits<{ logout: []; expired: []; wallet: [WalletTenant] }>()
const route = useRoute()
const router = useRouter()
const showPackages = computed(() => route.query.view === 'packages')
const showUsage = computed(() => route.query.view === 'usage')
const showSubaccounts = ref(false)
const showStatic = computed(() => route.query.view === 'static')
const showIPs = computed(() => route.query.view === 'ips')
const staticOnSale = ref(false)
const controller = new AbortController()
onMounted(async () => {
  const products = await listProducts(controller.signal).catch(() => [])
  staticOnSale.value = products.some(product => product.kind === 'static' && product.can_purchase)
})
onUnmounted(() => controller.abort())
const showWallet = computed(() => !showPackages.value && !showUsage.value && !showStatic.value && !showIPs.value && !showSubaccounts.value)
const title = computed(() => showStatic.value ? '购买静态 IP' : showIPs.value ? '我的 IP' : showPackages.value ? '购买流量套餐' : showUsage.value ? '日用量' : showSubaccounts.value ? '动态住宅代理' : '钱包与订单')
const tab = ref<'entries' | 'orders'>('entries')

function show(view: 'packages' | 'usage' | 'static' | 'ips' | 'subaccounts' | 'entries' | 'orders') {
  showSubaccounts.value = view === 'subaccounts'
  if (view === 'entries' || view === 'orders') tab.value = view
  void router.replace({ path: '/', query: view === 'packages' || view === 'usage' || view === 'static' || view === 'ips' ? { view } : {} })
}
</script>

<template>
  <DashboardShell brand="代理控制台" area="租户控制台" :title="title" :account-name="account.name" @logout="emit('logout')">
    <template #navigation="{ closeSidebar }">
      <nav aria-label="租户导航">
        <p class="nav-label">动态住宅代理</p>
        <Button variant="ghost" type="button" :aria-current="showPackages ? 'page' : undefined" @click="show('packages'); closeSidebar()"><Package />流量套餐</Button>
        <Button variant="ghost" type="button" :aria-current="!showPackages && !showUsage && showSubaccounts ? 'page' : undefined" @click="show('subaccounts'); closeSidebar()"><Network />代理子账号</Button>
        <Button variant="ghost" type="button" :aria-current="showUsage ? 'page' : undefined" @click="show('usage'); closeSidebar()"><ChartColumn />日用量</Button>
        <p class="nav-label">静态住宅代理</p>
        <Button v-if="staticOnSale" variant="ghost" type="button" :aria-current="showStatic ? 'page' : undefined" @click="show('static'); closeSidebar()"><Package />购买静态 IP</Button>
        <Button variant="ghost" type="button" :aria-current="showIPs ? 'page' : undefined" @click="show('ips'); closeSidebar()"><Network />我的 IP</Button>
        <p class="nav-label">钱包与订单</p>
        <Button variant="ghost" type="button" :aria-current="showWallet && tab === 'entries' ? 'page' : undefined" @click="show('entries'); closeSidebar()"><List />钱包流水</Button>
        <Button variant="ghost" type="button" :aria-current="showWallet && tab === 'orders' ? 'page' : undefined" @click="show('orders'); closeSidebar()"><ReceiptText />订单记录</Button>
      </nav>
    </template>

    <TrafficCheckout v-if="showPackages" :token="token" :account="account" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <StaticCheckout v-else-if="showStatic" :token="token" :tenant-id="account.tenant_id" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <TenantStaticIPs v-else-if="showIPs" :token="token" :tenant-id="account.tenant_id" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <TenantDailyUsage v-else-if="showUsage" :token="token" @expired="emit('expired')" />
    <TenantSubaccounts v-else-if="showSubaccounts" :token="token" :primary-account="account.primary_account_name" @expired="emit('expired')" />
    <WalletRecords v-else v-model:view="tab" :token="token" @expired="emit('expired')">
      <template #overview="{ recentEntries }">
        <TenantStaticIPs :token="token" :tenant-id="account.tenant_id" summary @expired="emit('expired')" />
        <TenantTrafficAlert :token="token" @expired="emit('expired')" />
        <WalletOverview :balance="account.available_minor" :entries="recentEntries" />
      </template>
      <template #records-help>
        <p class="support-note"><CircleHelp :size="15" />充值、开票和售后请线下联系运营。</p>
      </template>
    </WalletRecords>
  </DashboardShell>
</template>
