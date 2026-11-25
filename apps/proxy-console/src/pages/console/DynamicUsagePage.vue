<script setup lang="ts">
import { sessionEmail } from '@/features/auth'
import { DynamicUsage } from '@/features/dynamic-proxy'
</script>

<template>
  <DynamicUsage :key="sessionEmail" />
</template>
