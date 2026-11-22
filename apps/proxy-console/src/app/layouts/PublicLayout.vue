<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from './AppHeader.vue'

const route = useRoute()
const marketing = computed(() => route.name === 'home')
</script>

<template>
  <AppHeader :marketing="marketing" />
  <main :class="{ 'public-main': !marketing }">
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

@media (max-width: 767px) {
  .public-main { padding: var(--space-6) var(--space-3); }
}
</style>
