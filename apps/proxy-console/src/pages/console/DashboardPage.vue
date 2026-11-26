<script setup lang="ts">
import { Dashboard } from '@/features/dashboard'
import { sessionEmail } from '@/features/auth'
</script>

<template>
  <Dashboard :key="sessionEmail" />
</template>
