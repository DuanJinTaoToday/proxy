<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElDialog, ElEmpty, ElRadio, ElRadioGroup, ElSkeleton } from 'element-plus'
import { savingsPercent, unitPrice } from '@/features/catalog/trafficQuotes'
import { useTrafficCheckout } from '@/features/checkout/useTrafficCheckout'
import { formatYuan } from '@/shared/lib/money'
import { formatGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const { t, te, locale } = useI18n()
const errorMessage = useErrorMessage()
const route = useRoute()
const { confirmedTier, data, loadError, loading, openConfirmation, pending, receipt, refresh, rejection, selected, selectedId, submit, submitting } =
  useTrafficCheckout(typeof route.query.package === 'string' ? route.query.package : undefined)

const money = (minor: bigint) => formatYuan(minor, locale.value)
const gb = (bytes: bigint) => t('subaccounts.gb', { value: formatGB(bytes, locale.value) })
const base = computed(() => data.value?.catalog.packages.find(tier => tier.id === '5gb'))
const offerCards = computed(() => data.value?.catalog.packages.map(tier => ({
  ...tier,
  unitMinor: unitPrice(tier),
  savings: base.value ? savingsPercent(tier, base.value) : 0n,
})) ?? [])
const shortfall = computed(() => {
  const tier = confirmedTier.value ?? selected.value
  return data.value && tier && tier.priceMinor > data.value.walletMinor ? tier.priceMinor - data.value.walletMinor : 0n
})
const dialogOpen = computed({
  get: () => Boolean(confirmedTier.value),
  set: open => { if (!open && !submitting.value) confirmedTier.value = undefined },
})
const rejectionMessage = computed(() => {
  if (!rejection.value) return ''
  if (rejection.value === 'WALLET_INSUFFICIENT_FUNDS' && shortfall.value > 0n) {
    return t('trafficPricing.shortfall', { amount: money(shortfall.value) })
  }
  const key = 'trafficPricing.errors.' + rejection.value
  return t(te(key) ? key : 'trafficPricing.rejected')
})
</script>

<template>
  <div class="pricing tabular" :aria-busy="loading || submitting">
    <div v-if="receipt" role="status">
      <ElAlert type="success" :closable="false" show-icon
        :title="t('trafficPricing.receipt', { id: receipt.id, traffic: gb(receipt.deliveredBytes), amount: money(receipt.debitedMinor) })" />
    </div>
    <ElAlert v-if="pending && !submitting" type="warning" :closable="false" show-icon :title="t('trafficPricing.unknown')">
      <ElButton type="primary" :disabled="loading" @click="submit">{{ t('trafficPricing.retryPurchase') }}</ElButton>
    </ElAlert>
    <ElAlert v-if="rejectionMessage && !submitting" type="warning" :closable="false" show-icon :title="rejectionMessage" />
    <ElAlert v-if="loadError" :title="errorMessage(loadError)" type="error" :closable="false" show-icon>
      <ElButton size="small" @click="refresh">{{ t('trafficPricing.retryLoad') }}</ElButton>
    </ElAlert>
    <ElSkeleton v-if="!data && loading" animated :rows="5" />
    <template v-if="data && !loadError">
      <ElCard v-if="!data.catalog.open || !data.catalog.canPurchase || !selected" shadow="never">
        <ElEmpty :description="t(data.catalog.open ? 'trafficPricing.noPackages' : 'trafficPricing.closed')" />
      </ElCard>
      <template v-else>
        <ElRadioGroup :model-value="selected.id" :disabled="loading || submitting || Boolean(pending)"
          :aria-label="t('trafficPricing.choose')" class="offers" @change="selectedId = String($event)">
          <ElCard v-for="tier in offerCards" :key="tier.id" shadow="never" class="offer" :class="{ selected: tier.id === selected.id }">
            <ElRadio :value="tier.id" class="offer-label">
              {{ t('trafficPricing.package', { capacity: gb(tier.capacityBytes) }) }}
            </ElRadio>
            <strong class="price">{{ money(tier.priceMinor) }}</strong>
            <p>{{ t('trafficPricing.unitPrice', { amount: money(tier.unitMinor) }) }}</p>
            <p v-if="tier.savings > 0n" class="savings">
              {{ t('trafficPricing.savings', { percent: tier.savings.toString() }) }}
            </p>
            <p>{{ t('trafficPricing.validity') }} · {{ t('trafficPricing.shared') }}</p>
          </ElCard>
        </ElRadioGroup>

        <ElCard shadow="never">
          <dl class="summary">
            <div><dt>{{ t('trafficPricing.chosen') }}</dt><dd>{{ t('trafficPricing.package', { capacity: gb(selected.capacityBytes) }) }}</dd></div>
            <div><dt>{{ t('trafficPricing.traffic') }}</dt><dd>{{ gb(data.trafficBytes) }}</dd></div>
            <div><dt>{{ t('trafficPricing.afterTraffic') }}</dt><dd>{{ gb(data.trafficBytes + selected.capacityBytes) }}</dd></div>
            <div><dt>{{ t('trafficPricing.wallet') }}</dt><dd>{{ money(data.walletMinor) }}</dd></div>
            <div class="total"><dt>{{ t('trafficPricing.due') }}</dt><dd>{{ money(selected.priceMinor) }}</dd></div>
          </dl>
          <ElAlert v-if="shortfall > 0n && !pending" type="warning" :closable="false" show-icon
            :title="t('trafficPricing.shortfall', { amount: money(shortfall) })" />
          <ElButton class="buy" type="primary" :loading="loading || submitting" :disabled="Boolean(pending)" @click="openConfirmation">
            {{ t('trafficPricing.buy') }}
          </ElButton>
          <p>{{ t('trafficPricing.billing') }}</p>
        </ElCard>
      </template>
    </template>
    <ElDialog v-model="dialogOpen" :title="t('trafficPricing.confirm')" class="traffic-confirm"
      :show-close="!submitting" :close-on-click-modal="!submitting" :close-on-press-escape="!submitting">
      <template v-if="confirmedTier && data">
        <dl class="summary tabular">
          <div><dt>{{ t('trafficPricing.chosen') }}</dt><dd>{{ t('trafficPricing.package', { capacity: gb(confirmedTier.capacityBytes) }) }}</dd></div>
          <div><dt>{{ t('trafficPricing.validityLabel') }}</dt><dd>{{ t('trafficPricing.validity') }}</dd></div>
          <div><dt>{{ t('trafficPricing.afterWallet') }}</dt><dd>{{ shortfall > 0n ? '—' : money(data.walletMinor - confirmedTier.priceMinor) }}</dd></div>
          <div class="total"><dt>{{ t('trafficPricing.due') }}</dt><dd>{{ money(confirmedTier.priceMinor) }}</dd></div>
        </dl>
        <ElAlert v-if="shortfall > 0n" class="tabular" type="warning" :closable="false" show-icon
          :title="t('trafficPricing.shortfall', { amount: money(shortfall) })" />
      </template>
      <template #footer>
        <ElButton :disabled="submitting" @click="dialogOpen = false">{{ t('trafficPricing.back') }}</ElButton>
        <ElButton type="primary" :loading="submitting" :disabled="!confirmedTier || shortfall > 0n" @click="submit">
          {{ t('trafficPricing.pay') }}
        </ElButton>
      </template>
    </ElDialog>
  </div>
</template>

<style scoped>
.pricing { display: grid; gap: var(--space-4); }
.offers { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--subaccount-metric-width)), 1fr)); gap: var(--space-4); align-items: stretch; }
.offer-label { white-space: normal; height: auto; margin: 0 0 var(--space-3); font-weight: var(--font-weight-semibold); }
.offer.selected { border-color: var(--color-primary-border); background: var(--color-primary-soft); }
.price { display: block; font-size: var(--font-size-title); }
p { color: var(--color-text-secondary); margin: var(--space-2) 0 0; }
.savings { color: var(--color-success-text); font-weight: var(--font-weight-semibold); }
.summary { margin: 0 0 var(--space-4); }
.summary > div { display: flex; justify-content: space-between; flex-wrap: wrap; gap: var(--space-2); padding-block: var(--space-2); }
.summary dd { margin: 0; font-weight: var(--font-weight-semibold); }
.total { border-block-start: var(--border-width) solid var(--color-border); }
.total dd { color: var(--color-primary); font-size: var(--font-size-lg); }
.buy { margin-block-start: var(--space-3); }
.traffic-confirm { --el-dialog-width: min(var(--form-dialog-width), calc(100% - 2 * var(--space-4))); }
</style>
