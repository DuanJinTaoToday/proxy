import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { applyLocale, isLocale, preferredLocale } from './i18n'

declare module 'vue-router' {
  interface RouteMeta {
    // 页面标题和菜单使用的翻译键。
    titleKey?: string
    // 带页签的页面：子路由各自独立，可直接访问、刷新和后退。
    tabs?: { name: string, labelKey: string }[]
  }
}

const PublicLayout = () => import('./layouts/PublicLayout.vue')
const ConsoleLayout = () => import('./layouts/ConsoleLayout.vue')
const OpsLayout = () => import('./layouts/OpsLayout.vue')
const PendingPage = () => import('@/pages/PendingPage.vue')
const TabbedPage = () => import('@/pages/TabbedPage.vue')

function tabbed(path: string, prefix: string, titleKey: string, tabs: [string, string][]): RouteRecordRaw {
  return {
    path,
    component: TabbedPage,
    meta: { titleKey, tabs: tabs.map(([child, labelKey]) => ({ name: `${prefix}-${child}`, labelKey })) },
    redirect: { name: `${prefix}-${tabs[0][0]}` },
    children: tabs.map(([child, labelKey]) => ({ path: child, name: `${prefix}-${child}`, component: PendingPage, meta: { titleKey: labelKey } })),
  }
}

const page = (path: string, name: string, titleKey: string): RouteRecordRaw => ({ path, name, component: PendingPage, meta: { titleKey } })

export const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/zh-CN/' },
  {
    path: '/:locale(zh-CN|en)',
    component: PublicLayout,
    children: [
      { path: '', name: 'home', component: () => import('@/pages/public/HomePage.vue'), meta: { titleKey: 'nav.home' } },
      page('login', 'login', 'auth.loginTitle'),
      page('register', 'register', 'auth.registerTitle'),
    ],
  },
  {
    path: '/console',
    component: ConsoleLayout,
    children: [
      page('', 'console-dashboard', 'nav.dashboard'),
      tabbed('dynamic', 'console-dynamic', 'nav.dynamic', [['setup', 'tabs.dynamicSetup'], ['pricing', 'tabs.dynamicPricing'], ['usage', 'tabs.dynamicUsage'], ['guide', 'tabs.guide']]),
      tabbed('static', 'console-static', 'nav.static', [['ips', 'tabs.staticIps'], ['pricing', 'tabs.staticPricing'], ['guide', 'tabs.guide']]),
      page('sub-accounts', 'console-sub-accounts', 'nav.subAccounts'),
      page('wallet', 'console-wallet', 'nav.wallet'),
      tabbed('account', 'console-account', 'nav.account', [['profile', 'tabs.accountProfile'], ['orders', 'tabs.accountOrders']]),
    ],
  },
  {
    path: '/ops',
    component: OpsLayout,
    children: [
      { path: '', redirect: { name: 'ops-tenants' } },
      page('tenants', 'ops-tenants', 'nav.opsTenants'),
      page('tenants/:tenantId', 'ops-tenant', 'nav.opsTenants'),
      tabbed('products', 'ops-products', 'nav.opsProducts', [['dynamic', 'tabs.productsDynamic'], ['static', 'tabs.productsStatic']]),
      page('orders', 'ops-orders', 'nav.opsOrders'),
    ],
  },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/pages/NotFoundPage.vue'), meta: { titleKey: 'notFound.title' } },
]

export const router = createRouter({ history: createWebHistory(), routes })

// 官网以 URL 的语言为准；控制台和其他页面使用浏览器记住的偏好。
router.beforeEach(to => {
  const locale = to.params.locale
  applyLocale(isLocale(locale) ? locale : preferredLocale())
})
