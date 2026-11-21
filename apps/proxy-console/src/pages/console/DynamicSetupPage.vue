<script setup lang="ts">
import { DynamicSetup, TrafficAlertSettings } from '@/features/dynamic-proxy'
</script>

<template>
  <DynamicSetup>
    <template #aside><TrafficAlertSettings /></template>
  </DynamicSetup>
</template>
