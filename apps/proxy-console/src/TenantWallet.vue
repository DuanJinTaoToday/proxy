<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { ChartColumn, CircleHelp, House, List, ReceiptText, Network, Package } from 'lucide-vue-next'
import type { WalletTenant } from './api/wallet'
import type { Overview } from './api/overview'
import { listProducts } from './api/product'
import AccountOverview from './AccountOverview.vue'
import DashboardShell from './DashboardShell.vue'
import WalletRecords from './WalletRecords.vue'
import TenantSubaccounts from './TenantSubaccounts.vue'
import TenantDailyUsage from './TenantDailyUsage.vue'
import TenantTrafficAlert from './TenantTrafficAlert.vue'
import TrafficCheckout from './TrafficCheckout.vue'
import WalletOverview from './WalletOverview.vue'
import StaticCheckout from './StaticCheckout.vue'
import TenantStaticIPs from './TenantStaticIPs.vue'
import { Button } from './ui/button'

defineProps<{ token: string; account: WalletTenant }>()
const emit = defineEmits<{ logout: []; expired: []; wallet: [WalletTenant] }>()
const route = useRoute()
const titles = {
  overview: '账户概览',
  packages: '购买流量套餐',
  subaccounts: '动态住宅代理',
  usage: '日用量',
  static: '购买静态 IP',
  ips: '我的 IP',
  wallet: '钱包与订单',
}
const view = computed(() => {
  const value = route.query.view
  return typeof value === 'string' && Object.hasOwn(titles, value) ? value as keyof typeof titles : 'overview'
})
const tab = ref<'entries' | 'orders'>('entries')
const products = ref<Pick<Overview, 'products'>['products']>([])
const dynamicOnSale = computed(() => products.value.some(product => product.kind === 'dynamic' && product.can_purchase))
const staticOnSale = computed(() => products.value.some(product => product.kind === 'static' && product.can_purchase))
const controller = new AbortController()

onMounted(async () => {
  if (view.value === 'overview') return
  const catalog = await listProducts(controller.signal).catch(() => [])
  if (!controller.signal.aborted && products.value.length === 0) {
    products.value = catalog.map(product => ({
      kind: product.kind, open: Boolean(product.open), can_purchase: Boolean(product.can_purchase),
    }))
  }
})
onUnmounted(() => controller.abort())
</script>

<template>
  <DashboardShell brand="代理控制台" area="租户控制台" :title="titles[view]" :account-name="account.name" @logout="emit('logout')">
    <template #navigation="{ closeSidebar }">
      <nav aria-label="租户导航">
        <Button as-child variant="ghost"><RouterLink :to="{ name: 'console' }" :aria-current="view === 'overview' ? 'page' : undefined" @click="closeSidebar"><House />账户概览</RouterLink></Button>
        <p class="nav-label">动态住宅代理</p>
        <Button v-if="dynamicOnSale" as-child variant="ghost"><RouterLink :to="{ query: { view: 'packages' } }" :aria-current="view === 'packages' ? 'page' : undefined" @click="closeSidebar"><Package />流量套餐</RouterLink></Button>
        <Button as-child variant="ghost"><RouterLink :to="{ query: { view: 'subaccounts' } }" :aria-current="view === 'subaccounts' ? 'page' : undefined" @click="closeSidebar"><Network />代理子账号</RouterLink></Button>
        <Button as-child variant="ghost"><RouterLink :to="{ query: { view: 'usage' } }" :aria-current="view === 'usage' ? 'page' : undefined" @click="closeSidebar"><ChartColumn />日用量</RouterLink></Button>
        <p class="nav-label">静态住宅代理</p>
        <Button v-if="staticOnSale" as-child variant="ghost"><RouterLink :to="{ query: { view: 'static' } }" :aria-current="view === 'static' ? 'page' : undefined" @click="closeSidebar"><Package />购买静态 IP</RouterLink></Button>
        <Button as-child variant="ghost"><RouterLink :to="{ query: { view: 'ips' } }" :aria-current="view === 'ips' ? 'page' : undefined" @click="closeSidebar"><Network />我的 IP</RouterLink></Button>
        <p class="nav-label">钱包与订单</p>
        <Button as-child variant="ghost"><RouterLink :to="{ query: { view: 'wallet' } }" :aria-current="view === 'wallet' && tab === 'entries' ? 'page' : undefined" @click="tab = 'entries'; closeSidebar()"><List />钱包流水</RouterLink></Button>
        <Button as-child variant="ghost"><RouterLink :to="{ query: { view: 'wallet' } }" :aria-current="view === 'wallet' && tab === 'orders' ? 'page' : undefined" @click="tab = 'orders'; closeSidebar()"><ReceiptText />订单记录</RouterLink></Button>
        <p class="nav-label">官网</p>
        <Button as-child variant="ghost"><RouterLink to="/#socks5"><CircleHelp />SOCKS5 接入说明</RouterLink></Button>
      </nav>
    </template>

    <AccountOverview v-if="view === 'overview'" :token="token" @expired="emit('expired')" @loaded="products = $event.products" />
    <TrafficCheckout v-else-if="view === 'packages'" :token="token" :account="account" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <StaticCheckout v-else-if="view === 'static'" :token="token" :tenant-id="account.tenant_id" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <TenantStaticIPs v-else-if="view === 'ips'" :token="token" :tenant-id="account.tenant_id" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
    <template v-else-if="view === 'usage'">
      <TenantTrafficAlert :token="token" @expired="emit('expired')" />
      <TenantDailyUsage :token="token" @expired="emit('expired')" />
    </template>
    <TenantSubaccounts v-else-if="view === 'subaccounts'" :key="String(route.query.create)"
      :token="token" :primary-account="account.primary_account_name" :start-creating="route.query.create === '1'" @expired="emit('expired')" />
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
