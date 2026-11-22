<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElDescriptions, ElDescriptionsItem, ElDrawer, ElSkeleton, ElTable, ElTableColumn, ElTag } from 'element-plus'
import { getOrder, type OrderItem } from '@/api/orders'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import { useOrderFormatting } from './format'

const props = defineProps<{ orderId?: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()
const { dateTime, delivery, itemDescription, money, statusTypes } = useOrderFormatting()

type DetailState =
  | { kind: 'loading' }
  | { kind: 'failed', error: unknown }
  | { kind: 'ready', order: Awaited<ReturnType<typeof getOrder>> }
const state = shallowRef<DetailState>({ kind: 'loading' })
const retry = ref(0)
const commonTerm = computed(() => {
  if (state.value.kind !== 'ready') return
  const { items } = state.value.order
  const days = items[0]?.termDays
  return days && items.every(item => item.termDays === days) ? days : undefined
})

watch([() => props.orderId, retry], async ([id], _previous, onCleanup) => {
  if (!id) return
  const request = new AbortController()
  onCleanup(() => request.abort())
  state.value = { kind: 'loading' }
  try {
    const order = await getOrder(id, request.signal)
    if (!request.signal.aborted) state.value = { kind: 'ready', order }
  } catch (error) {
    if (!request.signal.aborted) state.value = { kind: 'failed', error }
  }
}, { immediate: true })
</script>

<template>
  <ElDrawer :model-value="Boolean(props.orderId)" :title="t('orders.detailTitle')"
    size="min(var(--order-drawer-width), 100vw)" @update:model-value="open => { if (!open) emit('close') }">
    <div v-if="state.kind === 'loading'" role="status" :aria-label="t('orders.detailLoading')">
      <ElSkeleton animated :rows="6" />
    </div>
    <ElAlert v-else-if="state.kind === 'failed'" :title="t('orders.detailFailed')" type="error" show-icon :closable="false">
      <p>{{ errorMessage(state.error) }}</p>
      <ElButton size="small" @click="retry++">{{ t('orders.retry') }}</ElButton>
    </ElAlert>
    <div v-else class="order-detail">
      <ElAlert v-if="state.order.status === 'pending'" type="info" show-icon :closable="false"
        :title="t('orders.pendingTitle')" :description="t('orders.pendingDescription')" />
      <ElDescriptions :column="2" border size="small">
        <ElDescriptionsItem :label="t('orders.number')" :span="2">
          <span class="mono">{{ t('orders.orderNumber', { id: state.order.id }) }}</span>
        </ElDescriptionsItem>
        <ElDescriptionsItem :label="t('orders.type')">{{ t(`orders.kinds.${state.order.kind}`) }}</ElDescriptionsItem>
        <ElDescriptionsItem :label="t('orders.state')">
          <ElTag :type="statusTypes[state.order.status]" size="small" disable-transitions>{{ t(`orders.status.${state.order.status}`) }}</ElTag>
        </ElDescriptionsItem>
        <ElDescriptionsItem :label="t('orders.placedAt')">
          <time :datetime="state.order.createdAt.toISOString()">{{ dateTime(state.order.createdAt) }}</time>
        </ElDescriptionsItem>
        <ElDescriptionsItem v-if="commonTerm" :label="t('orders.rentalTerm')">{{ t('orders.termDays', commonTerm) }}</ElDescriptionsItem>
        <ElDescriptionsItem :label="t('orders.delivery')">
          <span class="tabular">{{ delivery(state.order) }}</span>
        </ElDescriptionsItem>
      </ElDescriptions>

      <section aria-labelledby="locked-items-title">
        <h3 id="locked-items-title">{{ t('orders.lockedItems') }}</h3>
        <ElTable :data="state.order.items" class="order-detail__table" row-key="position" size="small" :empty-text="t('orders.noItems')">
          <!-- @vue-generic {OrderItem} -->
          <ElTableColumn :label="t('orders.item')" min-width="130">
            <template #default="{ row }">
              <span class="tabular">{{ itemDescription(row) }}</span>
              <span v-if="row.termDays" class="order-note tabular">{{ t('orders.termDays', row.termDays) }}</span>
            </template>
          </ElTableColumn>
          <!-- @vue-generic {OrderItem} -->
          <ElTableColumn :label="t('orders.lineProgress')" min-width="120" align="right">
            <template #default="{ row }">
              <span class="tabular" :class="{ 'order-detail__partial': state.order.status !== 'pending' && row.deliveredCount < row.quantity }">
                {{ state.order.status === 'pending' && !row.deliveredCount
                  ? t('orders.status.pending')
                  : t('orders.quantityProgress', { delivered: row.deliveredCount, ordered: row.quantity }) }}
              </span>
            </template>
          </ElTableColumn>
          <!-- @vue-generic {OrderItem} -->
          <ElTableColumn :label="t('orders.unitPrice')" min-width="95" align="right">
            <template #default="{ row }"><span class="tabular">{{ money(row.unitMinor) }}</span></template>
          </ElTableColumn>
        </ElTable>
        <dl class="order-totals">
          <div><dt>{{ t('orders.lockedTotal') }}</dt><dd>{{ money(state.order.lockedMinor) }}</dd></div>
          <div v-if="state.order.prepaidMinor > 0n"><dt>{{ t('orders.prepaid') }}</dt><dd>{{ money(state.order.prepaidMinor) }}</dd></div>
          <div v-if="state.order.unsettledMinor > 0n"><dt>{{ t('orders.awaitingSettlement') }}</dt><dd>{{ money(state.order.unsettledMinor) }}</dd></div>
          <div><dt>{{ t('orders.refunded') }}</dt><dd>{{ money(state.order.refundedMinor) }}</dd></div>
          <div class="order-paid"><dt>{{ t('orders.paid') }}</dt><dd>{{ money(state.order.settledMinor) }}</dd></div>
        </dl>
      </section>

      <section aria-labelledby="order-ledger-title">
        <h3 id="order-ledger-title">{{ t('orders.walletActivity') }}</h3>
        <ul v-if="state.order.walletEntries.length" class="order-ledger">
          <li v-for="entry in state.order.walletEntries" :key="entry.id">
            <div>
              <strong>{{ t(`wallet.kinds.${entry.kind}`) }}</strong>
              <span class="order-note">{{ t('orders.ledgerNumber', { id: entry.id }) }} ·
                <time :datetime="entry.createdAt.toISOString()">{{ dateTime(entry.createdAt) }}</time>
              </span>
            </div>
            <span class="tabular" :class="{ 'credit-amount': entry.amountMinor > 0n }">
              {{ entry.amountMinor > 0n ? t('orders.creditAmount', { amount: money(entry.amountMinor) }) : money(entry.amountMinor) }}
            </span>
          </li>
        </ul>
        <p v-else class="order-note">{{ t('orders.noWalletEntries') }}</p>
      </section>
    </div>
  </ElDrawer>
</template>

<style scoped>
.order-detail { display: grid; gap: var(--space-5); }
.order-detail h3 { margin: 0 0 var(--space-2); font-size: var(--font-size-md); }
.order-detail__table { --el-table-header-bg-color: var(--color-surface-muted); }
.order-detail__partial { color: var(--color-warning-text); font-weight: var(--font-weight-semibold); }
.order-note { display: block; color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.order-totals {
  margin: var(--space-4) 0 0;
  padding-block-start: var(--space-2);
  border-block-start: var(--border-width) solid var(--color-border);
  font-size: var(--font-size-sm);
}
.order-totals dt { color: var(--color-text-secondary); }
.order-totals > div, .order-ledger > li {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: var(--space-3);
  padding-block: var(--space-1);
}
.order-totals dd { margin: 0; font-weight: var(--font-weight-semibold); font-variant-numeric: tabular-nums; }
.order-paid { border-top: var(--border-width) solid var(--color-border); font-weight: var(--font-weight-semibold); }
.order-paid dd { font-size: var(--font-size-title); }
.order-paid dt { color: var(--color-text); }
.order-ledger { list-style: none; margin: 0; padding: 0; border: var(--border-width) solid var(--color-border); border-radius: var(--radius-card); }
.order-ledger > li { padding: var(--space-2) var(--space-3); font-size: var(--font-size-sm); }
.order-ledger > li + li { border-top: var(--border-width) solid var(--color-border); }
.credit-amount { color: var(--color-success-text); }
</style>
