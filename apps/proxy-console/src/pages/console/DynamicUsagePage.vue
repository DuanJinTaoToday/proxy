<script setup lang="ts">
import { sessionPhone } from '@/features/auth'
import { DynamicUsage } from '@/features/dynamic-usage'
</script>

<template>
  <DynamicUsage :key="sessionPhone" />
</template>
