<script setup lang="ts">
import { onScopeDispose, shallowRef, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElAlert, ElButton, ElCard, ElSkeleton } from 'element-plus'
import { getAccessLocation } from '@/api/accessLocation'

type AccessLocationState =
  | { kind: 'loading' }
  | { kind: 'failed' }
  | { kind: 'ready', data: Awaited<ReturnType<typeof getAccessLocation>> }

const { t, locale } = useI18n()
const headingId = useId()
const state = shallowRef<AccessLocationState>({ kind: 'loading' })
let request: AbortController | undefined
onScopeDispose(() => request?.abort())

async function load() {
  request?.abort()
  const controller = new AbortController()
  request = controller
  state.value = { kind: 'loading' }
  try {
    const data = await getAccessLocation({ locale: locale.value, signal: controller.signal })
    if (!controller.signal.aborted) state.value = { kind: 'ready', data }
  } catch {
    if (!controller.signal.aborted) state.value = { kind: 'failed' }
  }
}

watch(locale, load, { immediate: true })
</script>

<template>
  <ElCard class="access-location-card" role="region" shadow="never" :aria-labelledby="headingId">
    <div class="access-location">
      <h2 :id="headingId" class="access-location__heading">{{ t('accessLocation.title') }}</h2>
      <div v-if="state.kind === 'loading'" role="status" :aria-label="t('accessLocation.loading')">
        <ElSkeleton animated :rows="2" />
      </div>
      <ElAlert v-else-if="state.kind === 'failed'" :title="t('accessLocation.failed')" type="error" :closable="false" show-icon>
        <ElButton size="small" @click="load">{{ t('accessLocation.retry') }}</ElButton>
      </ElAlert>
      <template v-else>
        <dl class="access-location__fields">
          <div>
            <dt>{{ t('accessLocation.ip') }}</dt>
            <dd class="mono">{{ state.data.ip || t('accessLocation.unknown') }}</dd>
          </div>
          <div>
            <dt>{{ t('accessLocation.location') }}</dt>
            <dd>{{ state.data.location || t('accessLocation.unknown') }}</dd>
          </div>
        </dl>
        <ElAlert
          :type="state.data.isMainlandChina ? 'warning' : 'success'"
          :title="t(state.data.isMainlandChina ? 'accessLocation.mainland' : 'accessLocation.available')"
          :closable="false"
          show-icon
        />
      </template>
      <div class="access-location__notes">
        <p>{{ t('accessLocation.notice') }}</p>
        <a v-if="state.kind === 'ready' && state.data.location" href="https://db-ip.com">{{ t('accessLocation.attribution') }}</a>
      </div>
    </div>
  </ElCard>
</template>

<style scoped>
.access-location-card { --el-card-padding: var(--space-5); border-radius: var(--dashboard-card-radius); }
.access-location {
  display: grid;
  gap: var(--space-4);
}

.access-location__heading {
  font-size: var(--font-size-xl);
  line-height: var(--tenant-profile-heading-height);
  font-weight: var(--font-weight-semibold);
}

.access-location__fields {
  display: grid;
  gap: var(--space-3);
  margin: 0;
}

.access-location__fields > div {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-1) var(--space-4);
}

.access-location__fields > div + div {
  border-top: var(--border-width) solid var(--color-border);
  padding-top: var(--space-3);
}

.access-location__fields dt,
.access-location__notes {
  color: var(--color-text-secondary);
}

.access-location__fields dd {
  margin: 0;
  font-weight: var(--font-weight-semibold);
  overflow-wrap: anywhere;
}

.access-location__notes {
  display: grid;
  gap: var(--space-1);
  font-size: var(--font-size-sm);
}

.access-location__notes a {
  justify-self: start;
  text-decoration: underline;
}
</style>
