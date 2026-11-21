<script setup lang="ts">
import { sessionPhone } from '@/features/auth'
import { OrderRecords } from '@/features/orders'
</script>

<template>
  <OrderRecords :key="sessionPhone" />
</template>
