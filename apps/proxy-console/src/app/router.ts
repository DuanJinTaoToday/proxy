import { createRouter, createWebHistory, type RouteLocationNormalizedLoaded, type RouteRecordRaw } from 'vue-router'
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
const RoutePlaceholder = () => import('./RoutePlaceholder.vue')
const RouteTabs = () => import('./RouteTabs.vue')

interface Tab { path: string, labelKey: string }

// 页签页：父路由显示标题和页签，第一个页签为默认子页，子路由名为「前缀-路径」。
function tabbed(path: string, prefix: string, titleKey: string, tabs: Tab[]): RouteRecordRaw {
  const name = (tab: Tab) => `${prefix}-${tab.path}`
  return {
    path,
    component: RouteTabs,
    meta: { titleKey, tabs: tabs.map(tab => ({ name: name(tab), labelKey: tab.labelKey })) },
    redirect: { name: name(tabs[0]) },
    children: tabs.map(tab => ({ path: tab.path, name: name(tab), component: RoutePlaceholder, meta: { titleKey: tab.labelKey } })),
  }
}

export const tabsParent = (route: RouteLocationNormalizedLoaded) => route.matched.find(record => record.meta.tabs)

const page = (path: string, name: string, titleKey: string): RouteRecordRaw => ({ path, name, component: RoutePlaceholder, meta: { titleKey } })

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
      tabbed('dynamic', 'console-dynamic', 'nav.dynamic', [{ path: 'setup', labelKey: 'tabs.dynamicSetup' }, { path: 'pricing', labelKey: 'tabs.dynamicPricing' }, { path: 'usage', labelKey: 'tabs.dynamicUsage' }, { path: 'guide', labelKey: 'tabs.guide' }]),
      tabbed('static', 'console-static', 'nav.static', [{ path: 'ips', labelKey: 'tabs.staticIps' }, { path: 'pricing', labelKey: 'tabs.staticPricing' }, { path: 'guide', labelKey: 'tabs.guide' }]),
      page('sub-accounts', 'console-sub-accounts', 'nav.subAccounts'),
      page('wallet', 'console-wallet', 'nav.wallet'),
      tabbed('account', 'console-account', 'nav.account', [{ path: 'profile', labelKey: 'tabs.accountProfile' }, { path: 'orders', labelKey: 'tabs.accountOrders' }]),
    ],
  },
  {
    path: '/ops',
    component: OpsLayout,
    children: [
      { path: '', redirect: { name: 'ops-tenants' } },
      page('tenants', 'ops-tenants', 'nav.opsTenants'),
      page('tenants/:tenantId', 'ops-tenant', 'nav.opsTenants'),
      tabbed('products', 'ops-products', 'nav.opsProducts', [{ path: 'dynamic', labelKey: 'tabs.productsDynamic' }, { path: 'static', labelKey: 'tabs.productsStatic' }]),
      page('orders', 'ops-orders', 'nav.opsOrders'),
    ],
  },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/pages/public/NotFoundPage.vue'), meta: { titleKey: 'notFound.title' } },
]

export const router = createRouter({ history: createWebHistory(), routes })

// 官网以 URL 的语言为准；控制台和其他页面使用浏览器记住的偏好。
router.beforeEach(to => {
  const locale = to.params.locale
  applyLocale(isLocale(locale) ? locale : preferredLocale())
})
