<script setup lang="ts">
import { AccessLocationCard } from '@/features/users'
</script>

<template>
  <div class="account-profile">
    <AccessLocationCard />
  </div>
</template>

<style scoped>
.account-profile {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--account-profile-card-min-width)), 1fr));
  align-items: start;
  gap: var(--space-4);
}
</style>
