import type { Component } from 'vue'
import { Box, Connection, CreditCard, House, Sort, Tickets, User, UserFilled, Warning } from '@element-plus/icons-vue'

export interface NavItem { path: string, labelKey: string, icon: Component }
export interface NavSection { labelKey?: string, items: NavItem[] }

export const consoleNav: NavSection[] = [
  { items: [{ path: '/console', labelKey: 'nav.dashboard', icon: House }] },
  {
    labelKey: 'nav.groupProxy',
    items: [
      { path: '/console/dynamic', labelKey: 'nav.dynamic', icon: Sort },
      { path: '/console/static', labelKey: 'nav.static', icon: Connection },
      { path: '/console/sub-accounts', labelKey: 'nav.subAccounts', icon: UserFilled },
    ],
  },
  {
    labelKey: 'nav.groupManage',
    items: [
      { path: '/console/wallet', labelKey: 'nav.wallet', icon: CreditCard },
      { path: '/console/account', labelKey: 'nav.account', icon: User },
    ],
  },
]

export const opsNav: NavSection[] = [
  {
    labelKey: 'nav.opsCustomers',
    items: [{ path: '/ops/tenants', labelKey: 'nav.opsTenants', icon: User }],
  },
  {
    labelKey: 'nav.opsOperations',
    items: [
      { path: '/ops/orders', labelKey: 'nav.opsOrders', icon: Tickets },
      { path: '/ops/products', labelKey: 'nav.opsProducts', icon: Box },
      { path: '/ops/commission-rates', labelKey: 'nav.opsCommissionRates', icon: CreditCard },
      { path: '/ops/usage', labelKey: 'nav.opsUsage', icon: Warning },
    ],
  },
]

// 当前路由归属的菜单项：取路径前缀最长的一项，子页和详情页高亮其一级菜单。
export function activeNavPath(sections: NavSection[], path: string) {
  return sections
    .flatMap(section => section.items.map(item => item.path))
    .filter(item => path === item || path.startsWith(`${item}/`))
    .sort((a, b) => b.length - a.length)[0] ?? ''
}
