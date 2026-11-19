import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import Prototype from './Prototype.vue'
import '@console/style.css'
import '@console/ui/theme.css'
import './prototype.css'

if (import.meta.env.DEV) {
  const path = '/my-ips-prototype.html'
  const router = createRouter({
    history: createWebHistory(),
    routes: [{ path, component: Prototype }, { path: '/', redirect: path }],
  })
  const app = createApp(Prototype).use(router)
  void router.isReady().then(() => app.mount('#app'))
}
