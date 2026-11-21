<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { ElEmpty } from 'element-plus'
import { Connection, CreditCard, House, Sort, User, UserFilled } from '@element-plus/icons-vue'
import AppHeader from './AppHeader.vue'
import DashboardPage from './DashboardPage.vue'
import DynamicPage from './DynamicPage.vue'
import StaticPage from './StaticPage.vue'
import WalletPage from './WalletPage.vue'
import AccountPage from './AccountPage.vue'
import SubAccountsPage from './SubAccountsPage.vue'

type Page = 'dashboard' | 'dynamic' | 'static' | 'sub-accounts' | 'wallet' | 'account'
interface NavItem { page: Page, name: string, icon: Component }

const sections: { name?: string, items: NavItem[] }[] = [
  { items: [{ page: 'dashboard', name: '仪表板', icon: House }] },
  { name: '代理', items: [{ page: 'dynamic', name: '动态住宅代理-GB', icon: Sort }, { page: 'static', name: '静态住宅代理', icon: Connection }, { page: 'sub-accounts', name: '代理子账号', icon: UserFilled }] },
  { name: '管理', items: [{ page: 'wallet', name: '钱包', icon: CreditCard }, { page: 'account', name: '我的账户', icon: User }] },
]
const pages: Partial<Record<Page, Component>> = { dashboard: DashboardPage, dynamic: DynamicPage, static: StaticPage, 'sub-accounts': SubAccountsPage, wallet: WalletPage, account: AccountPage }
const navItems = sections.flatMap(s => s.items)

function pageFromHash(): Page {
  const hash = location.hash.slice(1).split('/')[0]
  return navItems.some(item => item.page === hash) ? hash as Page : 'dashboard'
}

const page = ref<Page>(pageFromHash())
const current = computed(() => navItems.find(item => item.page === page.value)!)

window.addEventListener('hashchange', () => { page.value = pageFromHash() })

function go(next: Page) {
  location.hash = next
}
</script>

<template>
  <AppHeader />

  <div class="workspace">
    <nav class="sidebar" aria-label="控制台导航">
      <div v-for="(section, index) in sections" :key="index" :class="{ 'nav-section': section.name }">
        <div v-if="section.name" class="nav-label">{{ section.name }}</div>
        <a v-for="item in section.items" :key="item.page" class="nav-item" :href="`#${item.page}`"
          :aria-current="item.page === page ? 'page' : undefined" @click.prevent="go(item.page)">
          <component :is="item.icon" />{{ item.name }}
        </a>
      </div>
    </nav>

    <main class="main-content">
      <component :is="pages[page]" v-if="pages[page]" :key="page" />
      <template v-else>
        <div class="page-head"><h1>{{ current.name }}</h1></div>
        <section class="details-card">
          <ElEmpty description="这一页的原型尚未制作。" />
        </section>
      </template>
    </main>
  </div>
</template>
