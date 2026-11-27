<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from './AppHeader.vue'

const route = useRoute()
const authRoutes = new Set(['login', 'register', 'forgot-password', 'reset-password', 'recover-email'])
const marketing = computed(() => route.name === 'home' || route.name === 'pricing')
const marketingLight = computed(() => route.name === 'pricing')
const isAuth = computed(() => typeof route.name === 'string' && authRoutes.has(route.name))
</script>

<template>
  <AppHeader :marketing="marketing" :marketing-light="marketingLight" />
  <main :class="{ 'public-main': !marketing && !isAuth, 'public-auth': isAuth }">
    <RouterView />
  </main>
</template>

<style scoped>
.public-main {
  width: 100%;
  max-width: var(--content-max-width);
  margin-inline: auto;
  padding: var(--space-7) var(--page-padding);
}

.public-auth {
  display: grid;
  place-items: center;
  min-height: calc(100dvh - var(--header-height));
  padding: var(--space-6) var(--page-padding);
  background: var(--auth-page-bg);
}

@media (max-width: 767px) {
  .public-main { padding: var(--space-6) var(--space-3); }
  .public-auth {
    align-items: start;
    padding: var(--space-5) var(--space-3) var(--space-7);
  }
}
</style>
