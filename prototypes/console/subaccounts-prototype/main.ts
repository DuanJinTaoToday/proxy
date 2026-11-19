import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import type { Subaccount } from '@console/api/subaccounts'
import Prototype from './Prototype.vue'
import '@console/style.css'
import '@console/ui/theme.css'
import './prototype.css'

if (import.meta.env.DEV) {
  const accounts: Subaccount[] = [
    { client_id: 'qa01', name: 'qa01', remark: '测试环境', active: true, used_bytes: '2500000000', limit_bytes: '10000000000' },
    { client_id: 'ops01', name: 'ops01', remark: '运营工具', active: true, used_bytes: '1200000000' },
    { client_id: 'old01', name: 'old01', remark: '已归档项目', active: false },
  ]
  const originalFetch = window.fetch.bind(window)

  window.fetch = async (input, init) => {
    if (typeof input !== 'string' || !input.startsWith('/v1/')) return originalFetch(input, init)
    if (input !== '/v1/tenant/subaccounts') return Response.json({}, { status: 404 })
    if (init?.method === 'GET') return Response.json({ accounts })
    if (init?.method !== 'POST' || typeof init.body !== 'string') return Response.json({}, { status: 405 })

    const body: unknown = JSON.parse(init.body)
    if (!body || typeof body !== 'object' || !('name' in body) || typeof body.name !== 'string') {
      return Response.json({}, { status: 400 })
    }
    const name = body.name
    if (accounts.some(account => account.name === name)) {
      return Response.json({ reason: 'SUBACCOUNT_NAME_EXISTS' }, { status: 409 })
    }
    if (accounts.length >= 10) return Response.json({ reason: 'SUBACCOUNT_LIMIT_REACHED' }, { status: 409 })
    const account: Subaccount = {
      client_id: name,
      name,
      remark: 'remark' in body && typeof body.remark === 'string' ? body.remark : '',
      active: 'active' in body && body.active === true,
      used_bytes: '0',
      ...('limit_bytes' in body && typeof body.limit_bytes === 'string' ? { limit_bytes: body.limit_bytes } : {}),
    }
    accounts.unshift(account)
    return Response.json(account, { status: 201 })
  }

  const router = createRouter({
    history: createWebHistory(),
    routes: [
      { path: '/subaccounts-prototype.html', name: 'prototype', component: Prototype },
      { path: '/', redirect: { name: 'prototype' } },
    ],
  })
  const app = createApp(Prototype).use(router)
  void router.isReady().then(() => app.mount('#app'))
}
