import { createApp } from 'vue'
import { createRouter, createWebHistory, RouterView } from 'vue-router'
import App from './App.vue'
import './style.css'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: App, props: { operations: false } },
    { path: '/operations', component: App, props: { operations: true } },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
})

createApp(RouterView).use(router).mount('#app')
