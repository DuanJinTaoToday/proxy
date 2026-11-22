<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { OpsPendingOrders } from '@/features/orders'

const { t } = useI18n()
</script>

<template>
  <h1 class="ops-page-title">{{ t('nav.opsOrders') }}</h1>
  <OpsPendingOrders />
</template>
