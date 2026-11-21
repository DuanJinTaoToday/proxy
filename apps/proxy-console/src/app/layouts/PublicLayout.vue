<script setup lang="ts">
import AppHeader from './AppHeader.vue'
</script>

<template>
  <AppHeader />
  <main class="public-main">
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
