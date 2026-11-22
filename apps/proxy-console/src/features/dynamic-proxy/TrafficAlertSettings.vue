<script setup lang="ts">
import { computed, onScopeDispose, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElForm, ElFormItem, ElInput, ElSkeleton } from 'element-plus'
import { Minus, Plus } from '@element-plus/icons-vue'
import { getTrafficAlert, setTrafficAlert } from '@/api/trafficAlert'
import { refreshTrafficNotice } from '@/features/notifications'
import { bytesPerGB, formatGB, parseLimitGB } from '@/shared/lib/trafficUnits'
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
const inputBytes = computed(() => parseLimitGB(input.value))
const saving = computed(() => updateState.value.kind === 'saving')
const unchanged = computed(() => state.value.kind === 'ready' && inputBytes.value !== undefined && inputBytes.value === state.value.bytes)

function stepThreshold(direction: -1n | 1n) {
  const stepped = (inputBytes.value ?? 0n) + direction * 5n * bytesPerGB
  const value = formatGB(stepped < 0n ? 0n : stepped, locale.value, false)
  if (parseLimitGB(value) !== undefined) {
    input.value = value
    clearFeedback()
  }
}

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
  <ElCard class="traffic-alert" role="region" aria-labelledby="traffic-alert-title" shadow="never">
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
      <div class="traffic-alert__actions">
        <ElFormItem :error="invalid ? t('trafficAlert.invalid') : undefined">
          <ElInput
            v-model="input"
            class="traffic-alert__input tabular"
            inputmode="decimal"
            :aria-label="t('trafficAlert.threshold')"
            :disabled="saving"
            :input-style="{ textAlign: 'center' }"
            @input="clearFeedback"
          >
            <template #prepend>
              <ElButton class="traffic-alert__step" :aria-label="t('trafficAlert.decrease')" :icon="Minus" :disabled="saving || !inputBytes" @click="stepThreshold(-1n)" />
            </template>
            <template #append>
              <ElButton class="traffic-alert__step" :aria-label="t('trafficAlert.increase')" :icon="Plus" :disabled="saving" @click="stepThreshold(1n)" />
            </template>
          </ElInput>
          <span aria-hidden="true" class="traffic-alert__unit">GB</span>
        </ElFormItem>
        <ElButton
          native-type="submit"
          :loading="updateState.kind === 'saving' && updateState.action === 'save'"
          :disabled="saving || unchanged || !input.trim()"
          :aria-label="t('trafficAlert.save')"
        >
          {{ t('trafficAlert.saveShort') }}
        </ElButton>
      </div>
      <ElButton
        v-if="isConfigured"
        class="traffic-alert__disable"
        link
        :loading="updateState.kind === 'saving' && updateState.action === 'disable'"
        :disabled="saving"
        @click="update('disable')"
      >
        {{ t('trafficAlert.disable') }}
      </ElButton>
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
.traffic-alert { --el-card-padding: var(--space-5); border-radius: var(--radius-card); }
h2 { font-size: var(--font-size-xl); font-weight: var(--font-weight-semibold); }
.traffic-alert__hint { margin-block: var(--space-1) var(--space-3); color: var(--color-text-secondary); font-size: var(--font-size-xs); }
.traffic-alert__input { inline-size: var(--traffic-alert-stepper-width); max-inline-size: 100%; }
.traffic-alert__step { inline-size: var(--brand-mark-size); padding-inline: 0; }
.traffic-alert__actions { display: flex; align-items: flex-start; flex-wrap: wrap; gap: var(--space-2); }
.traffic-alert__actions .el-form-item { margin-block-end: 0; }
.traffic-alert__actions .el-form-item.is-error { margin-block-end: var(--space-6); }
.traffic-alert__unit { margin-inline-start: var(--space-2); color: var(--color-text-secondary); }
.traffic-alert__actions > button { margin-inline: 0; }
.traffic-alert__disable { margin-block-start: var(--space-3); }
.traffic-alert__feedback { margin-block-start: var(--space-3); }
</style>
