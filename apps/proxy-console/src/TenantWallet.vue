<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId, watch } from 'vue'
import type { Component, Ref } from 'vue'
import { useRoute } from 'vue-router'
import { ChartColumn, ChevronRight, CircleHelp, Globe, House, Network, Package, Server, UserRound, Users, Wallet } from 'lucide-vue-next'
import type { WalletTenant } from './api/wallet'
import type { Overview } from './api/overview'
import { listProducts } from './api/product'
import kunxingMark from './assets/kunxing-mark.png'
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
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'

defineProps<{ token: string; account: WalletTenant; loginAccount: string }>()
const emit = defineEmits<{ logout: []; expired: []; wallet: [WalletTenant] }>()
const route = useRoute()
const titles = {
  overview: '仪表板',
  packages: '购买流量套餐',
  subaccounts: '子账户',
  usage: '日用量',
  static: '购买静态 IP',
  ips: '我的 IP',
  wallet: '鲲星钱包',
  account: '我的账户',
}
type TenantView = keyof typeof titles
const view = computed(() => {
  const value = route.query.view
  return typeof value === 'string' && Object.hasOwn(titles, value) ? value as TenantView : 'overview'
})
const tab = ref<'entries' | 'orders'>('entries')
const products = ref<Pick<Overview, 'products'>['products']>([])
const dynamicOnSale = computed(() => products.value.some(product => product.kind === 'dynamic' && product.can_purchase))
const staticOnSale = computed(() => products.value.some(product => product.kind === 'static' && product.can_purchase))
type ProductKind = 'dynamic' | 'static'
type ProductLink = { label: string; icon: Component } & (
  { view: TenantView; available?: Ref<boolean> } | { href: string }
)
const productGroups: { kind: ProductKind; label: string; icon: Component; links: ProductLink[] }[] = [
  {
    kind: 'dynamic',
    label: '动态住宅代理-GB',
    icon: Globe,
    links: [
      { view: 'packages', label: '流量套餐', icon: Package, available: dynamicOnSale },
      { view: 'usage', label: titles.usage, icon: ChartColumn },
      { href: '/#socks5', label: 'SOCKS5 接入说明', icon: CircleHelp },
    ],
  },
  {
    kind: 'static',
    label: '静态住宅代理-IP',
    icon: Server,
    links: [
      { view: 'static', label: titles.static, icon: Package, available: staticOnSale },
      { view: 'ips', label: titles.ips, icon: Network },
    ],
  },
]
const productNavId = useId()
const expandedProducts = ref<Record<ProductKind, boolean>>({ dynamic: false, static: false })
const activeProduct = computed(() => productGroups.find(group =>
  group.links.some(link => 'view' in link && link.view === view.value),
)?.kind)

watch(() => route.fullPath, () => {
  if (activeProduct.value) expandedProducts.value[activeProduct.value] = true
}, { immediate: true })
watch(view, current => {
  if (current === 'wallet') tab.value = 'entries'
})

function toggleProduct(kind: ProductKind, sidebarExpanded: boolean, revealSidebar: () => void) {
  expandedProducts.value[kind] = !sidebarExpanded || !expandedProducts.value[kind]
  revealSidebar()
}

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
  <DashboardShell brand="鲲星代理" :brand-logo="kunxingMark" area="租户控制台" :title="titles[view]" :account-name="loginAccount || account.name" @logout="emit('logout')">
    <template #account-links="{ item }">
      <component :is="item" as-child>
        <RouterLink class="account-menu-item" :to="{ query: { view: 'account' } }" :aria-current="view === 'account' ? 'page' : undefined">
          <UserRound :size="17" aria-hidden="true" />{{ titles.account }}
        </RouterLink>
      </component>
      <component :is="item" as-child>
        <RouterLink class="account-menu-item" :to="{ query: { view: 'wallet' } }" :aria-current="view === 'wallet' ? 'page' : undefined" @click="tab = 'entries'">
          <Wallet :size="17" aria-hidden="true" />{{ titles.wallet }}
        </RouterLink>
      </component>
    </template>
    <template #navigation="{ closeSidebar, revealSidebar, expanded }">
      <nav class="tenant-nav" aria-label="租户导航">
        <Button as-child variant="ghost">
          <RouterLink :to="{ name: 'console' }" :aria-current="view === 'overview' ? 'page' : undefined"
            :aria-label="titles.overview" :title="titles.overview" @click="closeSidebar">
            <House aria-hidden="true" /><span class="tenant-nav-text">{{ titles.overview }}</span>
          </RouterLink>
        </Button>
        <p class="nav-label">代理</p>
        <div v-for="group in productGroups" :key="group.kind">
          <Button variant="ghost" type="button" class="tenant-product-toggle"
            :class="{ 'tenant-product-active': activeProduct === group.kind }"
            :aria-controls="`${productNavId}-${group.kind}`"
            :aria-expanded="expanded && expandedProducts[group.kind]"
            :aria-label="group.label" :title="group.label"
            @click="toggleProduct(group.kind, expanded, revealSidebar)">
            <component :is="group.icon" aria-hidden="true" />
            <span class="tenant-nav-text">{{ group.label }}</span>
            <ChevronRight class="tenant-product-chevron" aria-hidden="true" />
          </Button>
          <ul v-show="expanded && expandedProducts[group.kind]" :id="`${productNavId}-${group.kind}`" class="tenant-product-links">
            <template v-for="link in group.links" :key="link.label">
              <li v-if="!('view' in link) || !link.available || link.available.value">
                <Button as-child variant="ghost">
                  <RouterLink :to="'view' in link ? { query: { view: link.view } } : link.href"
                    :aria-current="'view' in link && view === link.view ? 'page' : undefined" @click="closeSidebar">
                    <component :is="link.icon" aria-hidden="true" />{{ link.label }}
                  </RouterLink>
                </Button>
              </li>
            </template>
          </ul>
        </div>
        <Button as-child variant="ghost">
          <RouterLink :to="{ query: { view: 'subaccounts' } }" :aria-current="view === 'subaccounts' ? 'page' : undefined"
            :aria-label="titles.subaccounts" :title="titles.subaccounts" @click="closeSidebar">
            <Users aria-hidden="true" /><span class="tenant-nav-text">{{ titles.subaccounts }}</span>
          </RouterLink>
        </Button>
        <p class="nav-label">管理</p>
        <Button as-child variant="ghost">
          <RouterLink :to="{ query: { view: 'wallet' } }" :aria-current="view === 'wallet' ? 'page' : undefined"
            :aria-label="titles.wallet" :title="titles.wallet" @click="tab = 'entries'; closeSidebar()">
            <Wallet aria-hidden="true" /><span class="tenant-nav-text">{{ titles.wallet }}</span>
          </RouterLink>
        </Button>
        <Button as-child variant="ghost">
          <RouterLink :to="{ query: { view: 'account' } }" :aria-current="view === 'account' ? 'page' : undefined"
            :aria-label="titles.account" :title="titles.account" @click="closeSidebar">
            <UserRound aria-hidden="true" /><span class="tenant-nav-text">{{ titles.account }}</span>
          </RouterLink>
        </Button>
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
    <Card v-else-if="view === 'account'">
      <CardContent>
        <dl class="record-details">
          <div><dt>登录手机号</dt><dd>{{ loginAccount || '重新登录后可查看' }}</dd></div>
          <div><dt>租户名称</dt><dd>{{ account.name }}</dd></div>
          <div><dt>主账号名</dt><dd>{{ account.primary_account_name }}</dd></div>
        </dl>
      </CardContent>
    </Card>
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
