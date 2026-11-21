<script setup lang="ts">
import { Dashboard } from '@/features/dashboard'
import { sessionPhone } from '@/features/auth'
</script>

<template>
  <Dashboard :key="sessionPhone" />
</template>
