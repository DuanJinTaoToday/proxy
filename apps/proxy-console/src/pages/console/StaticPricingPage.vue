<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { sessionEmail } from '@/features/auth'
import { StaticPricing } from '@/features/static-pricing'
import { parseStaticPurchaseIntent, staticIntentQueryKeys, stripQueryKeys } from '@/features/checkout'

const route = useRoute()
const router = useRouter()
const intent = computed(() => parseStaticPurchaseIntent(route.query))

function consumeIntent() {
  const { nextQuery, changed } = stripQueryKeys(route.query, staticIntentQueryKeys)
  if (changed) void router.replace({ query: nextQuery })
}
</script>

<template>
  <StaticPricing :key="sessionEmail" :account-key="sessionEmail" :intent="intent" @consume-intent="consumeIntent" />
</template>
