<script setup lang="ts">
import { CommissionRecords, OpsCommissionRates } from '@/features/users'

defineProps<{ tenantId: string }>()
</script>

<template>
  <div class="ops-tenant-commissions">
    <OpsCommissionRates />
    <CommissionRecords mode="ops" :tenant-id="tenantId" />
  </div>
</template>

<style scoped>
.ops-tenant-commissions {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-4);
  min-width: 0;
}
</style>
