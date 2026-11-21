<script setup lang="ts">
import { AuthForm } from '@/features/auth'

defineProps<{ mode: 'login' | 'register' }>()
</script>

<template>
  <AuthForm :key="mode" :mode="mode" />
</template>
