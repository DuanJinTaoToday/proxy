<script setup lang="ts">
import { onScopeDispose, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElIcon, ElPopover, ElSkeleton } from 'element-plus'
import { Bottom, Service, Top, Wallet } from '@element-plus/icons-vue'
import { getWalletSummary } from '@/api/wallet'
import { formatYuan } from '@/shared/lib/formatters'

const { t, locale } = useI18n()
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
      <ElCard class="wallet-overview__balance" role="region" aria-labelledby="wallet-available-title" shadow="never">
        <div class="wallet-overview__heading">
          <h2 id="wallet-available-title">{{ t('wallet.available') }}</h2>
          <ElIcon><Wallet /></ElIcon>
        </div>
        <p class="wallet-overview__available tabular">{{ formatYuan(state.summary.availableMinor, locale) }}</p>
        <p class="wallet-overview__balance-hint">{{ t('wallet.balanceHint') }}</p>
        <ElPopover placement="bottom-start" trigger="click" width="min(var(--recharge-panel-width), calc(100vw - var(--space-5)))">
          <template #reference>
            <ElButton class="wallet-overview__top-up" :icon="Service" type="primary">{{ t('wallet.recharge') }}</ElButton>
          </template>
          <strong>{{ t('wallet.rechargeTitle') }}</strong>
          <p>{{ t('wallet.rechargeInstructions') }}</p>
        </ElPopover>
      </ElCard>

      <ElCard role="region" aria-labelledby="wallet-recharge-title" shadow="never">
        <div class="wallet-overview__heading">
          <h2 id="wallet-recharge-title">{{ t('wallet.totalRecharge') }}</h2>
          <ElIcon class="wallet-overview__icon wallet-overview__icon--recharge"><Top /></ElIcon>
        </div>
        <p class="wallet-overview__total tabular">{{ formatYuan(state.summary.totalRechargeMinor, locale) }}</p>
        <p class="wallet-overview__hint">{{ t('wallet.rechargeHint') }}</p>
      </ElCard>

      <ElCard role="region" aria-labelledby="wallet-spend-title" shadow="never">
        <div class="wallet-overview__heading">
          <h2 id="wallet-spend-title">{{ t('wallet.totalSpend') }}</h2>
          <ElIcon class="wallet-overview__icon wallet-overview__icon--spend"><Bottom /></ElIcon>
        </div>
        <p class="wallet-overview__total tabular">{{ formatYuan(state.summary.totalSpendMinor, locale) }}</p>
        <p class="wallet-overview__hint">{{ t('wallet.spendHint') }}</p>
      </ElCard>
    </div>
  </div>
</template>

<style scoped>
.wallet-overview { margin-block-end: var(--space-4); }
.wallet-overview__cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, var(--wallet-summary-card-width)), 1fr));
  gap: var(--space-4);
}
.wallet-overview__balance {
  --el-card-bg-color: var(--color-shell);
  --el-card-border-color: var(--color-shell);
  color: var(--color-shell-text);
}
.wallet-overview__heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
}
.wallet-overview__heading h2 { font-size: var(--font-size-md); font-weight: var(--font-weight-semibold); }
.wallet-overview__available,
.wallet-overview__total {
  margin-block-start: var(--space-2);
  font-weight: var(--font-weight-bold);
  line-height: var(--line-height-tight);
  overflow-wrap: anywhere;
}
.wallet-overview__available { color: var(--color-on-primary); font-size: var(--font-size-display); }
.wallet-overview__total { font-size: var(--font-size-title); }
.wallet-overview__hint,
.wallet-overview__balance-hint {
  margin-block-start: var(--space-2);
  font-size: var(--font-size-sm);
}
.wallet-overview__hint { color: var(--color-text-secondary); }
.wallet-overview__balance-hint { color: var(--color-shell-muted); margin-block-end: var(--space-3); }
.wallet-overview__top-up {
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
.wallet-overview__icon {
  flex: none;
  width: var(--brand-mark-size);
  height: var(--brand-mark-size);
  border-radius: var(--radius-control);
}
.wallet-overview__icon--recharge { background: var(--color-success-bg); color: var(--color-success-text); }
.wallet-overview__icon--spend { background: var(--color-info-bg); color: var(--color-info-text); }
</style>
