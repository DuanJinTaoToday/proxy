import { cloneVNode, createApp, h, type VNode } from 'vue'
import { createRouter, createWebHistory, RouterView } from 'vue-router'
import type { RouteLocationNormalizedLoaded } from 'vue-router'
import App from './App.vue'
import PublicProducts from './PublicProducts.vue'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  scrollBehavior(to, _, saved) {
    return saved ?? (to.hash ? { el: to.hash } : { top: 0 })
  },
  routes: [
    { path: '/', name: 'home', component: PublicProducts, meta: { title: '鲲星代理' } },
    { path: '/products', redirect: { name: 'home' } },
    { path: '/console', name: 'console', component: App, props: { operations: false }, meta: { title: '租户控制台 · 鲲星代理' } },
    { path: '/operations', name: 'operations', component: App, props: { operations: true }, meta: { title: '运营控制台 · 住宅代理' } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

router.beforeEach(to => {
  if (to.name === 'home' && 'view' in to.query) return { name: 'console', query: to.query, hash: to.hash }
})

router.afterEach(to => {
  document.title = String(to.meta.title ?? '住宅代理')
})

createApp({
  render: () => h(RouterView, null, {
    default: ({ Component, route }: { Component: VNode; route: RouteLocationNormalizedLoaded }) =>
      Component && cloneVNode(Component, { key: route.name }),
  }),
}).use(router).mount('#app')
