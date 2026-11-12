<script setup lang="ts">
import { onUnmounted, ref, shallowRef, watch } from 'vue'
import { ApiError, extractProxy, getDailyUsage } from './api/control'
import type { DailyUsage, ProxyEndpoint } from './api/control'
import { bytes, formatMB, formatTime } from './format'

const props = defineProps<{ tenantId: string; token: string; dates: { start: string; end: string } }>()
const emit = defineEmits<{ expired: [] }>()
const count = ref(1)
const daily = shallowRef<DailyUsage>()
const endpoints = shallowRef<ProxyEndpoint[]>([])
const extracting = ref(false)
const extractError = ref('')
const usageError = ref('')
const extraction = new AbortController()
onUnmounted(() => extraction.abort())

watch(() => props.dates, async ({ start, end }, _, onCleanup) => {
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  daily.value = undefined
  usageError.value = ''
  try {
    daily.value = await getDailyUsage(props.tenantId, start, end, props.token, controller.signal, 'api_extraction_mock')
  } catch (error) {
    if (controller.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else if (!(error instanceof ApiError && error.status === 404)) usageError.value = '无法加载模拟日用量。'
  }
}, { immediate: true })

async function extract() {
  if (extracting.value) return
  extracting.value = true
  endpoints.value = []
  extractError.value = ''
  try {
    const reply = await extractProxy(props.tenantId, count.value, props.token, extraction.signal)
    endpoints.value = reply.ipList
  } catch (error) {
    if (extraction.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else extractError.value = error instanceof ApiError && error.status === 403
      ? '只有当前租户的管理员可以提取。'
      : '提取失败，请重试。'
  } finally {
    extracting.value = false
  }
}
</script>

<template>
  <p v-if="usageError" class="panel state error" role="alert">{{ usageError }}</p>
  <section v-if="daily" class="panel" aria-labelledby="extraction-demo-title">
    <div class="section-head">
      <div>
        <h2 id="extraction-demo-title">代理提取演示</h2>
        <p class="muted">模拟 SOCKS5 入口，仅用于演示。</p>
      </div>
      <form class="filters" @submit.prevent="extract">
        <label>提取数量<input v-model.number="count" type="number" min="1" max="100" step="1" required></label>
        <button type="submit" :disabled="extracting">{{ extracting ? '提取中…' : '提取代理' }}</button>
      </form>
    </div>
    <p v-if="extractError" class="error" role="alert">{{ extractError }}</p>
    <div v-if="endpoints.length" class="table-wrap" tabindex="0" role="region" aria-label="模拟代理入口">
      <table>
        <thead><tr><th scope="col">地址</th><th scope="col">用户名</th><th scope="col">密码</th><th scope="col">到期时间</th></tr></thead>
        <tbody>
          <tr v-for="endpoint in endpoints" :key="endpoint.host">
            <th scope="row">{{ endpoint.host }}:{{ endpoint.port }}</th>
            <td>{{ endpoint.username }}</td><td>{{ endpoint.password }}</td><td>{{ formatTime(endpoint.expiresAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <div class="section-head"><h2>模拟日用量</h2></div>
    <p v-if="!daily.days?.length" class="empty">所选日期范围暂无模拟用量。</p>
    <div v-else class="table-wrap" tabindex="0" role="region" aria-label="模拟日用量">
      <table>
        <thead><tr><th scope="col">日期</th><th scope="col">合计（MB）</th></tr></thead>
        <tbody>
          <tr v-for="{ date, amount } in daily.days" :key="date">
            <th scope="row">{{ date }}</th><td>{{ formatMB(bytes(amount?.total_bytes)) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>
