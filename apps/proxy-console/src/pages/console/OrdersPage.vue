<script setup lang="ts">
import { sessionEmail } from '@/features/auth'
import { OrderRecords } from '@/features/orders'

const emit = defineEmits<{ pendingCount: [count: number] }>()
</script>

<template>
  <OrderRecords :key="sessionEmail" @pending-count="emit('pendingCount', $event)" />
</template>
