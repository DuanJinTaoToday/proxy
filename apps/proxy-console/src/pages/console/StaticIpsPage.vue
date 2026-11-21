<script setup lang="ts">
import { sessionPhone } from '@/features/auth'
import { StaticIps } from '@/features/static-ips'
</script>

<template>
  <StaticIps :key="sessionPhone" :account-phone="sessionPhone" />
</template>
