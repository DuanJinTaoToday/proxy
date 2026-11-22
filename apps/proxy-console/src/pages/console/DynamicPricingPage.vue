<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElDialog, ElEmpty, ElSkeleton } from 'element-plus'
import { CircleCheck } from '@element-plus/icons-vue'
import { unitPrice } from '@/features/catalog'
import { useTrafficCheckout } from '@/features/checkout'
import { formatYuan } from '@/shared/lib/money'
import { formatGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const { t, te, locale } = useI18n()
const errorMessage = useErrorMessage()
const route = useRoute()
const { confirmedTier, data, loadError, loading, openConfirmation, pending, receipt, refresh, rejection, selected, selectedId, submit, submitting } =
  useTrafficCheckout(typeof route.query.package === 'string' ? route.query.package : undefined)

const money = (minor: bigint) => formatYuan(minor, locale.value)
const capacity = (bytes: bigint) => formatGB(bytes, locale.value)
const gb = (bytes: bigint) => t('subaccounts.gb', { value: capacity(bytes) })
const offerCards = computed(() => data.value?.catalog.packages.map(tier => ({
  ...tier,
  unitMinor: unitPrice(tier),
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
    <ElAlert v-if="pending && !confirmedTier" type="warning" :closable="false" show-icon :title="t('trafficPricing.unknown')">
      <ElButton type="primary" :loading="submitting" :disabled="loading" @click="submit">{{ t('trafficPricing.retryPurchase') }}</ElButton>
    </ElAlert>
    <ElAlert v-if="rejectionMessage && !submitting" type="warning" :closable="false" show-icon :title="rejectionMessage" />
    <ElAlert v-if="loadError" :title="errorMessage(loadError)" type="error" :closable="false" show-icon>
      <ElButton size="small" @click="refresh">{{ t('trafficPricing.retryLoad') }}</ElButton>
    </ElAlert>
    <ElSkeleton v-if="!data && loading" animated :rows="5" />
    <template v-if="data && !loadError">
      <ElCard v-if="!data.catalog.open || !data.catalog.canPurchase || !selected" class="checkout" shadow="never">
        <ElEmpty :description="t(data.catalog.open ? 'trafficPricing.noPackages' : 'trafficPricing.closed')" :image-size="80" />
      </ElCard>
      <div v-else class="traffic-purchase">
        <div role="radiogroup" :aria-label="t('trafficPricing.choose')" class="offers">
          <label v-for="tier in offerCards" :key="tier.id" class="offer" :class="{ selected: tier.id === selected.id }">
            <input type="radio" name="traffic-package" :value="tier.id" :checked="tier.id === selected.id"
              :disabled="loading || submitting || Boolean(pending)"
              :aria-label="t('trafficPricing.package', { capacity: gb(tier.capacityBytes) })" @change="selectedId = tier.id" />
            <span v-if="tier.discountPercent > 0" class="savings" :title="t('trafficPricing.savings', { percent: tier.discountPercent })">
              {{ t('trafficPricing.savingsBadge', { percent: tier.discountPercent }) }}
            </span>
            <strong class="offer-size">{{ capacity(tier.capacityBytes) }} <small>GB</small></strong>
            <strong class="price">{{ money(tier.priceMinor) }}</strong>
            <span class="unit">{{ t('trafficPricing.unitPrice', { amount: money(tier.unitMinor) }) }}</span>
            <span class="benefits" role="list">
              <span role="listitem"><CircleCheck aria-hidden="true" />{{ t('trafficPricing.durationBenefit') }}</span>
              <span role="listitem"><CircleCheck aria-hidden="true" />{{ t('trafficPricing.shared') }}</span>
              <span role="listitem"><CircleCheck aria-hidden="true" />{{ t('trafficPricing.connectionBenefit') }}</span>
            </span>
          </label>
        </div>

        <ElCard class="checkout" shadow="never">
          <dl class="summary">
            <div><dt>{{ t('trafficPricing.selected') }}</dt><dd>{{ t('trafficPricing.package', { capacity: gb(selected.capacityBytes) }) }}</dd></div>
            <div>
              <dt>{{ t('trafficPricing.afterTraffic') }}</dt>
              <dd>{{ t('trafficPricing.trafficChange', { before: gb(data.trafficBytes), after: gb(data.trafficBytes + selected.capacityBytes) }) }}</dd>
            </div>
            <div><dt>{{ t('trafficPricing.wallet') }}</dt><dd>{{ money(data.walletMinor) }}</dd></div>
            <div class="total"><dt>{{ t('trafficPricing.due') }}</dt><dd>{{ money(selected.priceMinor) }}</dd></div>
          </dl>
          <ElAlert v-if="shortfall > 0n && !pending" type="warning" :closable="false" show-icon
            :title="t('trafficPricing.shortfall', { amount: money(shortfall) })" />
          <ElButton class="buy" size="large" type="primary" :loading="loading || submitting" :disabled="Boolean(pending)" @click="openConfirmation">
            {{ t('trafficPricing.buy') }}
          </ElButton>
          <p class="billing">{{ t('trafficPricing.billing') }}</p>
        </ElCard>
      </div>
    </template>
    <ElDialog v-model="dialogOpen" :title="t('trafficPricing.confirm')" class="traffic-confirm" align-center
      :show-close="!submitting" :close-on-click-modal="!submitting" :close-on-press-escape="!submitting">
      <template v-if="confirmedTier && data">
        <dl class="summary tabular">
          <div><dt>{{ t('trafficPricing.chosen') }}</dt><dd>{{ t('trafficPricing.confirmedPackage', { capacity: gb(confirmedTier.capacityBytes) }) }}</dd></div>
          <div><dt>{{ t('trafficPricing.validityLabel') }}</dt><dd>{{ t('trafficPricing.validity') }}</dd></div>
          <div><dt>{{ t('trafficPricing.afterWallet') }}</dt><dd>{{ shortfall > 0n ? '—' : money(data.walletMinor - confirmedTier.priceMinor) }}</dd></div>
          <div class="total"><dt>{{ t('trafficPricing.due') }}</dt><dd>{{ money(confirmedTier.priceMinor) }}</dd></div>
        </dl>
        <ElAlert v-if="shortfall > 0n" class="tabular" type="warning" :closable="false" show-icon
          :title="t('trafficPricing.shortfall', { amount: money(shortfall) })" />
        <p v-if="data.trafficBytes < 0n" class="billing">
          {{ t('trafficPricing.trafficDebt', { amount: gb(-data.trafficBytes) }) }}
        </p>
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
.traffic-purchase { display: grid; grid-template-columns: minmax(0, 1fr) var(--dynamic-side-panel-width); gap: var(--space-4); align-items: start; }
.offers { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: var(--space-4); }
.offer { position: relative; display: flex; flex-direction: column; align-items: start; gap: var(--space-1); min-inline-size: 0; padding: var(--space-5) calc(var(--space-5) - var(--space-1)); border: var(--border-width) solid var(--color-border); border-radius: var(--radius-card); background: var(--color-surface); cursor: pointer; }
.offer:hover { border-color: var(--color-primary-border); }
.offer:has(input:focus-visible) { outline: var(--focus-ring-width) solid var(--color-focus); outline-offset: var(--space-1); }
.offer:has(input:disabled) { cursor: default; }
.offer.selected { border-color: var(--color-primary); box-shadow: 0 0 0 var(--border-width) var(--color-primary); background: var(--gradient-metric); }
.offer input { position: absolute; inline-size: var(--border-width); block-size: var(--border-width); margin: 0; opacity: 0; }
.offer-size { font-size: var(--font-size-metric); line-height: var(--line-height-tight); overflow-wrap: anywhere; }
.offer-size small { color: var(--color-text-secondary); font-size: var(--font-size-md); }
.price { font-size: var(--font-size-brand); overflow-wrap: anywhere; }
.unit, .benefits, .billing { color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.savings { position: absolute; inset-block-start: var(--space-3); inset-inline-end: var(--space-3); padding-inline: var(--space-2); color: var(--color-success-text); background: var(--color-primary-soft); border-radius: var(--radius-control); font-size: var(--font-size-xs); font-weight: var(--font-weight-semibold); }
.benefits { display: grid; gap: var(--space-1); padding: var(--space-3) 0 0; margin: var(--space-3) 0 0; inline-size: 100%; border-block-start: var(--border-width) solid var(--color-border); list-style: none; }
.benefits [role='listitem'] { display: flex; align-items: baseline; gap: var(--space-1); }
.benefits svg { flex: 0 0 var(--font-size-xs); inline-size: var(--font-size-xs); block-size: var(--font-size-xs); color: var(--color-primary); align-self: start; margin-block-start: var(--space-1); }
.checkout { --el-card-padding: var(--space-5); position: sticky; inset-block-start: var(--space-4); border-radius: var(--radius-card); }
.summary { display: grid; gap: var(--space-2); margin: 0 0 var(--space-3); font-size: var(--font-size-sm); }
.summary > div { display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; gap: var(--space-1) var(--space-2); }
.summary dt { color: var(--color-text-secondary); }
.summary dd { margin: 0; font-weight: var(--font-weight-semibold); }
.total dt { color: var(--color-text); font-weight: var(--font-weight-semibold); }
.total dd { font-size: var(--font-size-title); font-weight: var(--font-weight-bold); }
.buy { inline-size: 100%; }
.checkout .el-alert { margin-block-end: var(--space-3); }
.billing { margin: var(--space-5) 0 0; }
.traffic-confirm { --el-dialog-width: min(var(--traffic-confirm-width), calc(100% - 2 * var(--space-4))); }
@media (max-width: 1180px) {
  .traffic-purchase { grid-template-columns: minmax(0, 1fr); }
  .checkout { position: static; }
}
@media (max-width: 850px), (1180px < width <= 1360px) {
  .savings { position: static; margin-block-start: calc(-1 * var(--space-3)); align-self: end; }
  .offer-size { margin-block-start: var(--space-2); }
  .savings + .offer-size { margin-block-start: 0; }
}
@media (max-width: 750px) { .offer { padding-inline: var(--space-3); } }
@media (max-width: 360px) { .offers { grid-template-columns: minmax(0, 1fr); } }
</style>
