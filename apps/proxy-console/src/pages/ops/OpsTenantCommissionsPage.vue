<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { hasOperatorPermission } from '@/features/auth'
import { CommissionRecords } from '@/features/users'

defineProps<{ tenantId: string }>()

const { t } = useI18n()
const canAccessRates = computed(() => hasOperatorPermission('read_operations') || hasOperatorPermission('manage_billing'))
</script>

<template>
  <div class="ops-tenant-commissions">
    <div v-if="canAccessRates" class="ops-tenant-commissions__rates-link">
      <RouterLink :to="{ name: 'ops-commission-rates' }" class="ops-tenant-commissions__link">
        {{ t('users.commissions.rates.manageLink') }}
      </RouterLink>
    </div>
    <CommissionRecords mode="ops" :tenant-id="tenantId" />
  </div>
</template>

<style scoped>
.ops-tenant-commissions {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: var(--space-3);
  min-width: 0;
}

.ops-tenant-commissions__rates-link {
  display: flex;
  justify-content: flex-end;
}

.ops-tenant-commissions__link {
  color: var(--color-primary);
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-medium);
}

.ops-tenant-commissions__link:hover {
  text-decoration: underline;
}
</style>
