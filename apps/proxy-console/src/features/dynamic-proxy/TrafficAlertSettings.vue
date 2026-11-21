<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElForm, ElFormItem, ElInput, ElSkeleton } from 'element-plus'
import { getTrafficAlert, setTrafficAlert } from '@/api/trafficAlert'
import { refreshTrafficNotice } from '@/features/notifications'
import { formatGB, parseLimitGB } from '@/shared/lib/trafficUnits'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
type ThresholdState =
  | { kind: 'loading' }
  | { kind: 'failed', error: unknown }
  | { kind: 'ready', bytes: bigint | undefined }
type Action = 'save' | 'disable'
type UpdateState =
  | { kind: 'idle' }
  | { kind: 'saving' | 'succeeded', action: Action }
  | { kind: 'failed', action: Action, error: unknown }

const state = shallowRef<ThresholdState>({ kind: 'loading' })
const updateState = shallowRef<UpdateState>({ kind: 'idle' })
const input = ref('')
const invalid = ref(false)
const controller = new AbortController()
onScopeDispose(() => controller.abort())

const isConfigured = computed(() => state.value.kind === 'ready' && state.value.bytes !== undefined)

function showThreshold(bytes: bigint | undefined) {
  state.value = { kind: 'ready', bytes }
  input.value = bytes === undefined ? '' : formatGB(bytes, locale.value, false)
  invalid.value = false
}

async function load() {
  state.value = { kind: 'loading' }
  try {
    const alert = await getTrafficAlert(controller.signal)
    if (!controller.signal.aborted) showThreshold(alert.thresholdBytes)
  } catch (error) {
    if (!controller.signal.aborted) state.value = { kind: 'failed', error }
  }
}

async function update(action: Action) {
  if (state.value.kind !== 'ready' || updateState.value.kind === 'saving') return
  const bytes = action === 'save' ? parseLimitGB(input.value) : undefined
  invalid.value = action === 'save' && bytes === undefined
  if (invalid.value) return

  updateState.value = { kind: 'saving', action }
  try {
    const alert = await setTrafficAlert(bytes)
    refreshTrafficNotice()
    if (!controller.signal.aborted) {
      showThreshold(alert.thresholdBytes)
      updateState.value = { kind: 'succeeded', action }
    }
  } catch (error) {
    if (!controller.signal.aborted) updateState.value = { kind: 'failed', action, error }
  }
}

function clearFeedback() {
  invalid.value = false
  updateState.value = { kind: 'idle' }
}

void load()
</script>

<template>
  <ElCard role="region" aria-labelledby="traffic-alert-title" shadow="never">
    <h2 id="traffic-alert-title">{{ t('trafficAlert.title') }}</h2>
    <p class="traffic-alert__hint">{{ t('trafficAlert.hint') }}</p>
    <div v-if="state.kind === 'loading'" role="status" :aria-label="t('trafficAlert.loading')">
      <ElSkeleton animated :rows="2" />
    </div>
    <ElAlert v-else-if="state.kind === 'failed'" type="error" :closable="false" :title="t('trafficAlert.loadFailed')" show-icon>
      <p>{{ errorMessage(state.error) }}</p>
      <ElButton size="small" @click="load">{{ t('trafficAlert.retry') }}</ElButton>
    </ElAlert>
    <ElForm v-else label-position="top" @submit.prevent="update('save')">
      <ElFormItem :label="t('trafficAlert.threshold')" :error="invalid ? t('trafficAlert.invalid') : undefined">
        <ElInput
          v-model="input"
          class="traffic-alert__input tabular"
          inputmode="decimal"
          :aria-label="t('trafficAlert.threshold')"
          :disabled="updateState.kind === 'saving'"
          @input="clearFeedback"
        />
      </ElFormItem>
      <div class="traffic-alert__actions">
        <ElButton
          type="primary"
          native-type="submit"
          :loading="updateState.kind === 'saving' && updateState.action === 'save'"
          :disabled="updateState.kind === 'saving'"
        >
          {{ t('trafficAlert.save') }}
        </ElButton>
        <ElButton
          v-if="isConfigured"
          :loading="updateState.kind === 'saving' && updateState.action === 'disable'"
          :disabled="updateState.kind === 'saving'"
          @click="update('disable')"
        >
          {{ t('trafficAlert.disable') }}
        </ElButton>
      </div>
      <ElAlert
        v-if="updateState.kind === 'succeeded'"
        class="traffic-alert__feedback"
        type="success"
        :title="t(updateState.action === 'save' ? 'trafficAlert.saved' : 'trafficAlert.disabled')"
        :closable="false"
        show-icon
      />
      <ElAlert
        v-else-if="updateState.kind === 'failed'"
        class="traffic-alert__feedback"
        type="error"
        :title="t('trafficAlert.updateFailed')"
        :closable="false"
        show-icon
      >
        <p>{{ errorMessage(updateState.error) }}</p>
        <ElButton size="small" @click="update(updateState.action)">{{ t('trafficAlert.retry') }}</ElButton>
      </ElAlert>
    </ElForm>
  </ElCard>
</template>

<style scoped>
h2 { font-size: var(--font-size-lg); font-weight: var(--font-weight-semibold); }
.traffic-alert__hint { margin-block: var(--space-2) var(--space-4); color: var(--color-text-secondary); }
.traffic-alert__input { max-inline-size: var(--traffic-alert-input-width); }
.traffic-alert__actions { display: flex; flex-wrap: wrap; gap: var(--space-2); }
.traffic-alert__actions > button { margin-inline: 0; }
.traffic-alert__feedback { margin-block-start: var(--space-3); }
</style>
