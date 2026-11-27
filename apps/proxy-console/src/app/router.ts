import { createRouter, createWebHistory, type RouteLocationNormalizedLoaded, type RouteRecordRaw, type RouteRecordSingleView } from 'vue-router'
import { hasOperatorSession, hasSession, installOperatorSession, installSession, loadOperator, safeOperatorRedirect, safeRedirect, stashLinkToken, type LinkPurpose } from '@/features/auth'
import { applyLocale, isLocale, preferredLocale, rememberLocale } from './i18n'

declare module 'vue-router' {
  interface RouteMeta {
    // 页面标题和菜单使用的翻译键。
    titleKey?: string
    // 需要租户登录的入口，子路由继承。
    requiresAuth?: boolean
    // 仅游客访问：已登录时直接去回跳地址。
    guestOnly?: boolean
    requiresOperator?: boolean
    operatorGuestOnly?: boolean
    // 带页签的页面：子路由各自独立，可直接访问、刷新和后退。
    tabs?: { name: string, labelKey: string }[]
    tabAction?: { name: string, labelKey: string }
    // 邮件链接落地页携带的一次性令牌用途。
    linkToken?: LinkPurpose
  }
}

const PublicLayout = () => import('./layouts/PublicLayout.vue')
const ConsoleLayout = () => import('./layouts/ConsoleLayout.vue')
const OpsLayout = () => import('./layouts/OpsLayout.vue')
const RoutePlaceholder = () => import('./RoutePlaceholder.vue')
const RouteTabs = () => import('./RouteTabs.vue')
const AuthPage = () => import('@/pages/public/AuthPage.vue')

interface Tab { path: string, labelKey: string, component?: RouteRecordSingleView['component'] }

// 页签页：父路由显示标题和页签，第一个页签为默认子页，子路由名为「前缀-路径」。
function tabbed(path: string, prefix: string, titleKey: string, tabs: Tab[], options: { component?: RouteRecordSingleView['component'], action?: { name: string, labelKey: string } } = {}): RouteRecordRaw {
  const name = (tab: Tab) => `${prefix}-${tab.path}`
  return {
    path,
    component: options.component ?? RouteTabs,
    meta: { titleKey, tabAction: options.action, tabs: tabs.map(tab => ({ name: name(tab), labelKey: tab.labelKey })) },
    redirect: { name: name(tabs[0]) },
    children: tabs.map(tab => ({ path: tab.path, name: name(tab), component: tab.component ?? RoutePlaceholder, meta: { titleKey: tab.labelKey } })),
  }
}

export const tabsParent = (route: RouteLocationNormalizedLoaded) => route.matched.find(record => record.meta.tabs)

export const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/zh-CN/' },
  {
    path: '/:locale(zh-CN|en)',
    component: PublicLayout,
    children: [
      { path: '', name: 'home', component: () => import('@/pages/public/HomePage.vue'), meta: { titleKey: 'nav.home' } },
      { path: 'pricing', name: 'pricing', component: () => import('@/pages/public/PricingPage.vue'), meta: { titleKey: 'publicPricing.title' } },
      { path: 'login', name: 'login', component: AuthPage, props: { mode: 'login' }, meta: { titleKey: 'auth.loginTitle', guestOnly: true } },
      { path: 'register', name: 'register', component: AuthPage, props: { mode: 'register' }, meta: { titleKey: 'auth.registerTitle', guestOnly: true } },
      { path: 'forgot-password', name: 'forgot-password', component: AuthPage, props: { mode: 'forgot' }, meta: { titleKey: 'auth.forgotTitle', guestOnly: true } },
      // 邮件链接落地页，已登录时同样可用。
      { path: 'reset-password', name: 'reset-password', component: AuthPage, props: { mode: 'reset' }, meta: { titleKey: 'auth.reset.title', linkToken: 'reset' } },
      { path: 'recover-email', name: 'recover-email', component: AuthPage, props: { mode: 'recover' }, meta: { titleKey: 'auth.recover.title', linkToken: 'recover' } },
    ],
  },
  {
    path: '/console',
    component: ConsoleLayout,
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'console-dashboard', component: () => import('@/pages/console/DashboardPage.vue'), meta: { titleKey: 'nav.dashboard' } },
      tabbed('dynamic', 'console-dynamic', 'nav.dynamic', [
        { path: 'setup', labelKey: 'tabs.dynamicSetup', component: () => import('@/pages/console/DynamicSetupPage.vue') },
        { path: 'pricing', labelKey: 'tabs.dynamicPricing', component: () => import('@/pages/console/DynamicPricingPage.vue') },
        { path: 'usage', labelKey: 'tabs.dynamicUsage', component: () => import('@/pages/console/DynamicUsagePage.vue') },
        { path: 'guide', labelKey: 'tabs.guide', component: () => import('@/pages/console/DynamicGuidePage.vue') },
      ], { action: { name: 'console-dynamic-pricing', labelKey: 'dynamicSetup.buy' } }),
      tabbed('static', 'console-static', 'nav.static', [
        { path: 'ips', labelKey: 'tabs.staticIps', component: () => import('@/pages/console/StaticIpsPage.vue') },
        { path: 'pricing', labelKey: 'tabs.staticPricing', component: () => import('@/pages/console/StaticPricingPage.vue') },
        { path: 'guide', labelKey: 'tabs.guide', component: () => import('@/pages/console/StaticGuidePage.vue') },
      ], { action: { name: 'console-static-pricing', labelKey: 'staticIps.buy' } }),
      { path: 'sub-accounts', name: 'console-sub-accounts', component: () => import('@/pages/console/SubAccountsPage.vue'), meta: { titleKey: 'nav.subAccounts' } },
      { path: 'wallet', name: 'console-wallet', component: () => import('@/pages/console/WalletPage.vue'), meta: { titleKey: 'nav.wallet' } },
      tabbed('account', 'console-account', 'nav.account', [
        { path: 'profile', labelKey: 'tabs.accountProfile', component: () => import('@/pages/console/AccountProfilePage.vue') },
        { path: 'orders', labelKey: 'tabs.accountOrders', component: () => import('@/pages/console/OrdersPage.vue') },
      ], { component: () => import('@/pages/console/AccountPage.vue') }),
    ],
  },
  { path: '/ops/login', name: 'ops-login', component: () => import('@/pages/ops/OpsLoginPage.vue'), meta: { titleKey: 'auth.opsLoginTitle', operatorGuestOnly: true } },
  {
    path: '/ops',
    component: OpsLayout,
    meta: { requiresOperator: true },
    children: [
      { path: '', redirect: { name: 'ops-tenants' } },
      { path: 'tenants', name: 'ops-tenants', component: () => import('@/pages/ops/OpsTenantsPage.vue'), meta: { titleKey: 'nav.opsTenants' } },
      {
        path: 'tenants/:tenantId',
        component: () => import('@/pages/ops/OpsTenantPage.vue'),
        meta: { titleKey: 'nav.opsTenants' },
        children: [
          { path: '', name: 'ops-tenant', component: () => import('@/pages/ops/OpsTenantOrdersPage.vue') },
          { path: 'ledger', name: 'ops-tenant-ledger', component: () => import('@/pages/ops/OpsTenantLedgerPage.vue') },
          { path: 'commissions', name: 'ops-tenant-commissions', component: () => import('@/pages/ops/OpsTenantCommissionsPage.vue') },
        ],
      },
      { path: 'products', alias: ['products/dynamic', 'products/static'], name: 'ops-products', component: () => import('@/pages/ops/OpsProductsPage.vue'), meta: { titleKey: 'nav.opsProducts' } },
      { path: 'commission-rates', name: 'ops-commission-rates', component: () => import('@/pages/ops/OpsCommissionRatesPage.vue'), meta: { titleKey: 'nav.opsCommissionRates' } },
      { path: 'orders', name: 'ops-orders', component: () => import('@/pages/ops/OpsOrdersPage.vue'), meta: { titleKey: 'nav.opsOrders' } },
      { path: 'usage', name: 'ops-usage', component: () => import('@/pages/ops/OpsUsagePage.vue'), meta: { titleKey: 'nav.opsUsage' } },
    ],
  },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('@/pages/public/NotFoundPage.vue'), meta: { titleKey: 'notFound.title' } },
]

export const router = createRouter({ history: createWebHistory(), routes })

const loginLocation = (locale: string, redirect: string) => ({ path: `/${locale}/login`, query: { redirect } })
const needsAuth = (route: RouteLocationNormalizedLoaded) => route.matched.some(record => record.meta.requiresAuth)

// 官网以 URL 的语言为准，并记为偏好，从英文官网注册或登录后控制台沿用英文；控制台和其他页面使用记住的偏好。
// 未登录访问控制台时跳到登录页并带上回跳地址；已登录访问登录注册页时直接去回跳地址。
router.beforeEach(async to => {
  // 邮件链接的令牌在渲染前移出地址栏，替换当前历史记录，页面从本标签页暂存中读取。
  if (to.meta.linkToken && 'token' in to.query) {
    const { token, ...query } = to.query
    stashLinkToken(to.meta.linkToken, typeof token === 'string' ? token : '')
    return { path: to.path, query, hash: to.hash, replace: true }
  }
  const urlLocale = isLocale(to.params.locale) ? to.params.locale : undefined
  if (urlLocale) rememberLocale(urlLocale)
  const locale = urlLocale ?? preferredLocale()
  applyLocale(locale)
  if (to.meta.requiresOperator && !(await loadOperator())) return { name: 'ops-login', query: { redirect: to.fullPath } }
  if (to.meta.operatorGuestOnly && hasOperatorSession() && await loadOperator()) return safeOperatorRedirect(to.query.redirect)
  if (needsAuth(to) && !hasSession()) return loginLocation(locale, to.fullPath)
  if (to.meta.guestOnly && hasSession()) return safeRedirect(to.query.redirect)
})

// 会话失效时（令牌被拒绝或在其他标签页退出），受保护页面整页回到登录页并保留当前地址，内存缓存随之清空。
installSession(() => {
  const current = router.currentRoute.value
  if (needsAuth(current)) window.location.assign(router.resolve(loginLocation(preferredLocale(), current.fullPath)).href)
})

installOperatorSession(() => {
  const current = router.currentRoute.value
  if (current.meta.requiresOperator) {
    window.location.assign(router.resolve({ name: 'ops-login', query: { redirect: current.fullPath } }).href)
  }
})
