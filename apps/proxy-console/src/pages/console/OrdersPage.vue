<script setup lang="ts">
import { sessionPhone } from '@/features/auth'
import { OrderRecords } from '@/features/orders'

const emit = defineEmits<{ pendingCount: [count: number] }>()
</script>

<template>
  <OrderRecords :key="sessionPhone" @pending-count="emit('pendingCount', $event)" />
</template>
