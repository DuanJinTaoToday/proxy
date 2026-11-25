<script setup lang="ts">
import { sessionEmail } from '@/features/auth'
import { StaticIps } from '@/features/static-ips'
</script>

<template>
  <StaticIps :key="sessionEmail" :account-key="sessionEmail" />
</template>
