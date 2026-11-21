<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { sessionPhone } from '@/features/auth'
import { WalletLedger, WalletOverview } from '@/features/wallet'
import PageHeader from '@/shared/ui/PageHeader.vue'

const { t } = useI18n()
</script>

<template>
  <PageHeader :title="t('nav.wallet')" />
  <WalletOverview :key="`overview-${sessionPhone}`" />
  <WalletLedger :key="`ledger-${sessionPhone}`" />
</template>
