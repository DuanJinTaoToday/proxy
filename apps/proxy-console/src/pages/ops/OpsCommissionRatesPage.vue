<script setup lang="ts">
import { OpsCommissionRates } from '@/features/users'
</script>

<template>
  <div class="ops-commission-rates-page">
    <OpsCommissionRates heading-level="h1" />
  </div>
</template>

<style scoped>
.ops-commission-rates-page {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  min-width: 0;
}
</style>
