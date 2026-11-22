<script setup lang="ts">
import { onScopeDispose, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElIcon, ElPopover, ElSkeleton } from 'element-plus'
import { Bottom, Service, Top, Wallet } from '@element-plus/icons-vue'
import { getWalletSummary } from '@/api/wallet'
import { formatYuan } from '@/shared/lib/money'

const { t, locale } = useI18n()
const lowBalanceMinor = 5000n
type Summary = Awaited<ReturnType<typeof getWalletSummary>>
type SummaryState = { kind: 'loading' | 'failed' } | { kind: 'ready', summary: Summary }
const state = shallowRef<SummaryState>({ kind: 'loading' })
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

async function load() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  try {
    const summary = await getWalletSummary(controller.signal)
    if (!controller.signal.aborted) state.value = { kind: 'ready', summary }
  } catch {
    if (!controller.signal.aborted) state.value = { kind: 'failed' }
  }
}

void load()
</script>

<template>
  <div class="wallet-overview">
    <div v-if="state.kind === 'loading'" role="status" :aria-label="t('wallet.loadingSummary')">
      <ElSkeleton animated :rows="3" />
    </div>
    <ElAlert v-else-if="state.kind === 'failed'" :title="t('wallet.summaryFailed')" type="error" :closable="false" show-icon>
      <ElButton size="small" @click="load">{{ t('wallet.retry') }}</ElButton>
    </ElAlert>
    <div v-else-if="state.kind === 'ready'" class="wallet-overview__cards">
      <section class="wallet-overview__balance" aria-labelledby="wallet-available-title">
        <h2 id="wallet-available-title"><ElIcon><Wallet /></ElIcon>{{ t('wallet.available') }}</h2>
        <p class="wallet-overview__available tabular">{{ formatYuan(state.summary.availableMinor, locale) }}</p>
        <p class="wallet-overview__balance-hint" :class="{ 'is-low': state.summary.availableMinor < lowBalanceMinor }">
          {{ t(state.summary.availableMinor < lowBalanceMinor ? 'wallet.lowBalanceHint' : 'wallet.balanceHint') }}
        </p>
        <ElPopover placement="bottom-start" trigger="click" width="min(var(--recharge-panel-width), calc(100vw - var(--space-5)))">
          <template #reference>
            <ElButton class="wallet-overview__top-up" :icon="Service" type="primary">{{ t('wallet.recharge') }}</ElButton>
          </template>
          <strong>{{ t('wallet.rechargeTitle') }}</strong>
          <p class="wallet-overview__instructions">{{ t('wallet.rechargeInstructions') }}</p>
        </ElPopover>
      </section>

      <ElCard class="wallet-overview__metric" body-class="wallet-overview__metric-body" role="region" aria-labelledby="wallet-recharge-title" shadow="never">
        <div>
          <h2 id="wallet-recharge-title">{{ t('wallet.totalRecharge') }}</h2>
          <p class="wallet-overview__total tabular">{{ formatYuan(state.summary.totalRechargeMinor, locale) }}</p>
          <p class="wallet-overview__hint">{{ t('wallet.rechargeHint') }}</p>
        </div>
        <ElIcon class="wallet-overview__icon wallet-overview__icon--recharge"><Top /></ElIcon>
      </ElCard>

      <ElCard class="wallet-overview__metric" body-class="wallet-overview__metric-body" role="region" aria-labelledby="wallet-spend-title" shadow="never">
        <div>
          <h2 id="wallet-spend-title">{{ t('wallet.totalSpend') }}</h2>
          <p class="wallet-overview__total tabular">{{ formatYuan(state.summary.totalSpendMinor, locale) }}</p>
          <p class="wallet-overview__hint">{{ t('wallet.spendHint') }}</p>
        </div>
        <ElIcon class="wallet-overview__icon wallet-overview__icon--spend"><Bottom /></ElIcon>
      </ElCard>
    </div>
  </div>
</template>

<style scoped>
.wallet-overview { margin-block-end: var(--space-4); line-height: normal; }
.wallet-overview__cards {
  display: grid;
  grid-template-columns: minmax(0, 1.4fr) repeat(2, minmax(0, 1fr));
  gap: var(--space-4);
}
.wallet-overview__balance {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: var(--space-2);
  padding: var(--tenant-metric-inset-inline) var(--dashboard-stat-inset-block);
  border-radius: var(--dashboard-card-radius);
  background: var(--color-shell);
  color: var(--color-shell-text);
}
.wallet-overview__balance h2 {
  display: flex;
  align-items: center;
  gap: var(--space-2);
}
.wallet-overview__balance h2, .wallet-overview__metric h2 {
  font-size: var(--font-size-sm);
  font-weight: var(--font-weight-semibold);
}
.wallet-overview__available, .wallet-overview__total {
  font-weight: var(--font-weight-dashboard-display);
  letter-spacing: -.03em;
  overflow-wrap: anywhere;
}
.wallet-overview__available {
  color: var(--color-on-primary);
  font-size: var(--wallet-balance-size);
  line-height: var(--tabs-height);
}
.wallet-overview__balance-hint, .wallet-overview__hint {
  font-size: var(--font-size-xs);
}
.wallet-overview__balance-hint {
  margin-block-end: var(--dashboard-compact-gap);
  color: var(--color-shell-muted);
}
.wallet-overview__balance-hint.is-low { color: var(--color-wallet-low); }
.wallet-overview__instructions { margin-block-start: var(--space-2); color: var(--color-text-secondary); }
.wallet-overview__top-up {
  padding-inline: var(--space-3);
  font-size: var(--font-size-sm);
  --el-button-bg-color: var(--color-accent);
  --el-button-border-color: var(--color-accent);
  --el-button-text-color: var(--color-shell);
  --el-button-hover-bg-color: var(--color-primary-soft);
  --el-button-hover-border-color: var(--color-primary-soft);
  --el-button-hover-text-color: var(--color-shell);
  --el-button-active-bg-color: var(--color-primary-border);
  --el-button-active-border-color: var(--color-primary-border);
  --el-button-active-text-color: var(--color-shell);
}
.wallet-overview__metric { border-radius: var(--dashboard-card-radius); min-block-size: var(--wallet-overview-min-height); }
.wallet-overview__metric :deep(.wallet-overview__metric-body) {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--tenant-metric-inset-block) var(--tenant-metric-inset-inline);
}
.wallet-overview__metric h2 { line-height: var(--dashboard-stat-inset-block); }
.wallet-overview__total {
  margin-block-start: var(--space-1);
  font-size: var(--font-size-dashboard-display);
  line-height: var(--font-size-display);
}
.wallet-overview__hint { margin-block-start: var(--dashboard-tight-gap); color: var(--color-text-secondary); }
.wallet-overview__icon {
  flex: none;
  width: var(--control-height);
  height: var(--control-height);
  border-radius: var(--tenant-metric-icon-radius);
  font-size: var(--font-size-xl);
}
.wallet-overview__icon--recharge { background: var(--color-success-bg); color: var(--color-success-text); }
.wallet-overview__icon--spend { background: var(--color-warning-bg); color: var(--color-warning-text); }
@media (max-width: 950px) {
  .wallet-overview__cards { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  .wallet-overview__balance { grid-column: 1 / -1; }
}
@media (max-width: 680px) {
  .wallet-overview__metric :deep(.wallet-overview__metric-body) { flex-wrap: wrap; padding-inline: var(--space-3); }
}
</style>
