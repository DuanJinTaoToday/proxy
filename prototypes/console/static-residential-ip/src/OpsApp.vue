<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { ElDropdown, ElDropdownItem, ElDropdownMenu, ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Box, Check, DataLine, Document, Monitor, Switch, Tickets, User, Warning } from '@element-plus/icons-vue'
import OpsWorkbench from './OpsWorkbench.vue'
import OpsTenants from './OpsTenants.vue'
import OpsOrders from './OpsOrders.vue'
import OpsLedger from './OpsLedger.vue'
import OpsProducts from './OpsProducts.vue'
import OpsUsage from './OpsUsage.vue'
import { needsAction, openUsageIssues, operator, opsRoles, role, setRole, type OpsRole } from './ops'
import { resetDemoData } from './persist'

type Page = 'workbench' | 'tenants' | 'orders' | 'ledger' | 'products' | 'usage'
interface NavItem { page: Page, name: string, icon: Component, count?: () => number }

const sections: { name?: string, items: NavItem[] }[] = [
  { items: [{ page: 'workbench', name: '工作台', icon: Monitor, count: () => needsAction.value.length }] },
  { name: '客户', items: [{ page: 'tenants', name: '租户', icon: User }, { page: 'orders', name: '订单', icon: Tickets }, { page: 'ledger', name: '钱包流水', icon: Document }] },
  { name: '运营', items: [{ page: 'products', name: '商品与定价', icon: Box }, { page: 'usage', name: '用量异常', icon: Warning, count: () => openUsageIssues }] },
]
const pages: Record<Page, Component> = { workbench: OpsWorkbench, tenants: OpsTenants, orders: OpsOrders, ledger: OpsLedger, products: OpsProducts, usage: OpsUsage }
const navItems = sections.flatMap(s => s.items)

function pageFromHash(): Page {
  const hash = location.hash.slice(1).split('/')[0]
  return navItems.some(item => item.page === hash) ? hash as Page : 'workbench'
}

const page = ref<Page>(pageFromHash())
window.addEventListener('hashchange', () => { page.value = pageFromHash() })
const roleLabel = computed(() => opsRoles[role.value].title)

function onRole(next: OpsRole) {
  setRole(next)
  ElMessage.info(`已切换为「${opsRoles[next].title}」角色`)
}

async function onAccount(command: string) {
  if (command === 'reset') {
    const ok = await ElMessageBox.confirm('清空本浏览器里的演示订单、流水和定价改动，恢复初始数据。已打开的客户控制台标签页需要手动刷新。', '重置演示数据？', { confirmButtonText: '重置', cancelButtonText: '取消', type: 'warning' }).then(() => true, () => false)
    if (ok) resetDemoData()
    return
  }
  const ok = await ElMessageBox.confirm('退出后需要重新登录运营后台。', '退出登录？', { confirmButtonText: '退出登录', cancelButtonText: '取消', type: 'warning' }).then(() => true, () => false)
  if (ok) ElMessage.info('原型不接认证，正式版会回到运营登录页')
}
</script>

<template>
  <header class="topbar">
    <div class="brand"><span class="brand-mark"><Switch /></span><span>Proxy</span><span class="ops-badge">运营后台</span></div>
    <div class="top-tools">
      <a class="top-item" href="./" target="_blank" rel="noopener"><DataLine />打开客户控制台</a>
      <ElDropdown trigger="click" placement="bottom-end" @command="onRole">
        <button type="button" class="top-item demo-role" title="原型演示用，正式版按账号权限决定">
          演示角色：{{ roleLabel }}<ArrowDown class="chevron" />
        </button>
        <template #dropdown>
          <ElDropdownMenu>
            <ElDropdownItem v-for="(r, key) in opsRoles" :key="key" :command="key">
              <span class="menu-check"><Check v-if="key === role" /></span>{{ r.title }} · {{ r.name }}
            </ElDropdownItem>
          </ElDropdownMenu>
        </template>
      </ElDropdown>
      <ElDropdown trigger="click" placement="bottom-end" popper-class="account-popper" @command="onAccount">
        <button type="button" class="top-item top-account">{{ operator.name }}<ArrowDown class="chevron" /></button>
        <template #dropdown>
          <div class="account-head">
            <strong>{{ operator.name }}</strong>
            <span>{{ operator.title }}</span>
          </div>
          <ElDropdownMenu>
            <ElDropdownItem command="reset">重置演示数据</ElDropdownItem>
            <ElDropdownItem command="logout">退出登录</ElDropdownItem>
          </ElDropdownMenu>
        </template>
      </ElDropdown>
    </div>
  </header>

  <div class="workspace">
    <nav class="sidebar ops-sidebar" aria-label="运营后台导航">
      <div v-for="(section, index) in sections" :key="index" :class="{ 'nav-section': section.name }">
        <div v-if="section.name" class="nav-label">{{ section.name }}</div>
        <a v-for="item in section.items" :key="item.page" class="nav-item" :href="`#${item.page}`"
          :aria-current="item.page === page ? 'page' : undefined">
          <component :is="item.icon" />{{ item.name }}
          <span v-if="item.count?.()" class="nav-count">{{ item.count() }}</span>
        </a>
      </div>
    </nav>

    <main class="main-content">
      <component :is="pages[page]" :key="page" />
    </main>
  </div>
</template>
