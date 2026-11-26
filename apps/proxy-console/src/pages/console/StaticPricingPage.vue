<script setup lang="ts">
import { sessionEmail } from '@/features/auth'
import { StaticPricing } from '@/features/static-pricing'
</script>

<template>
  <StaticPricing :key="sessionEmail" :account-key="sessionEmail" />
</template>
