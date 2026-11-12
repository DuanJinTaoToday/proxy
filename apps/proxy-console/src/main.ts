import { createApp, defineComponent } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import App from './App.vue'
import './style.css'

const routeContent = defineComponent({ render: () => null })
const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: routeContent },
    { path: '/tenants/:tenantId/usage', name: 'usage', component: routeContent },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

const app = createApp(App).use(router)
await router.isReady()
app.mount('#app')
