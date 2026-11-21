<script setup lang="ts">
import { sessionPhone } from '@/features/auth'
import StaticPricing from '@/features/static-pricing/StaticPricing.vue'
</script>

<template>
  <StaticPricing :key="sessionPhone" :account-key="sessionPhone" />
</template>
