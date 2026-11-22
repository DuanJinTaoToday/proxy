<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElSkeleton } from 'element-plus'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'
import type { OpsHistoryState } from './useOpsHistoryPage'

defineProps<{ label: string, state: OpsHistoryState<unknown>, page: number }>()
defineEmits<{ retry: [], previous: [], next: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()
</script>

<template>
  <ElCard role="region" :aria-label="label" class="ops-history" shadow="never">
    <div v-if="$slots.filters" class="ops-history-toolbar"><slot name="filters" /></div>
    <ElSkeleton v-if="state.kind === 'loading'" class="ops-history-placeholder" animated :rows="5" />
    <div v-else-if="state.kind === 'failed'" class="ops-history-placeholder">
      <ElAlert type="error" :title="errorMessage(state.error)" show-icon :closable="false">
        <ElButton size="small" @click="$emit('retry')">{{ t('orders.retry') }}</ElButton>
      </ElAlert>
    </div>
    <template v-else><slot /></template>
    <nav v-if="state.kind === 'ready' || page > 1" class="ops-history-footer" :aria-label="label">
      <span>{{ t('wallet.page', { page }) }}</span>
      <div v-if="page > 1 || (state.kind === 'ready' && state.nextBeforeId)">
        <ElButton size="small" :disabled="page === 1 || state.kind === 'loading'" @click="$emit('previous')">{{ t('orders.previous') }}</ElButton>
        <ElButton size="small" :disabled="state.kind !== 'ready' || !state.nextBeforeId" @click="$emit('next')">{{ t('orders.next') }}</ElButton>
      </div>
    </nav>
  </ElCard>
</template>

<style src="./opsHistory.css"></style>
