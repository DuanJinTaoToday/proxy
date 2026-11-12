<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { ApiError, getBalance, getDailyUsage } from './api/control'
import type { Balance, DailyUsage } from './api/control'
import { bytes, defaultDates, formatMB, formatTime, validDates } from './format'

const props = defineProps<{ tenantId: string; token: string }>()
const emit = defineEmits<{ expired: [] }>()
const draft = ref(defaultDates())
const dates = ref({ ...draft.value })
const dateError = ref('')
const loadError = ref('')
const loading = ref(false)
const result = shallowRef<{ usage: DailyUsage; balance: Balance } | null>(null)

const balance = computed(() => result.value?.balance)
const available = computed(() => bytes(balance.value?.available_bytes))
const low = computed(() => available.value < bytes(balance.value?.warning_threshold_bytes))
const denied = computed(() => available.value < bytes(balance.value?.admission_threshold_bytes))
const days = computed(() => result.value?.usage.days ?? [])
const grants = computed(() => balance.value?.grants ?? [])

function search() {
  const { start, end } = draft.value
  if (!validDates(start, end)) {
    dateError.value = '请选择有效的起止日期，范围最多 93 天。'
    return
  }
  dateError.value = ''
  dates.value = { start, end }
}

watch(dates, async ({ start, end }, _, onCleanup) => {
  const controller = new AbortController()
  onCleanup(() => controller.abort())
  loading.value = true
  loadError.value = ''
  result.value = null
  const { tenantId, token } = props
  try {
    const [usage, balance] = await Promise.all([
      getDailyUsage(tenantId, start, end, token, controller.signal),
      getBalance(tenantId, token, controller.signal),
    ])
    if (!controller.signal.aborted) result.value = { usage, balance }
  } catch (error) {
    if (controller.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else loadError.value = error instanceof ApiError && error.status === 403
      ? '你没有权限查看此租户。'
      : '无法加载用量或额度，请重试。'
    loading.value = false
    controller.abort()
  } finally {
    if (!controller.signal.aborted) loading.value = false
  }
}, { immediate: true })
</script>

<template>
  <form class="panel filters" @submit.prevent="search">
    <label>开始日期<input v-model="draft.start" type="date" required></label>
    <label>结束日期<input v-model="draft.end" type="date" required></label>
    <button type="submit">查询用量</button>
    <p v-if="dateError" role="alert" class="error">{{ dateError }}</p>
  </form>

  <p v-if="loading" class="panel state" role="status">正在加载用量和额度…</p>
  <div v-else-if="loadError" class="panel state">
    <p role="alert" class="error">{{ loadError }}</p>
    <button class="secondary" type="button" @click="dates = { ...dates }">重新加载</button>
  </div>
  <template v-else-if="result && balance">
    <section class="panel overview" aria-labelledby="balance-title">
      <div>
        <h2 id="balance-title">剩余额度</h2>
        <p class="balance" :class="{ debt: available < 0n }">{{ formatMB(available) }} <small>MB</small></p>
        <p v-if="available < 0n" class="error">欠费 {{ formatMB(bytes(balance.overdraft_bytes)) }} MB</p>
      </div>
      <div v-if="low || denied" class="notices" role="status">
        <p v-if="low">剩余额度低于提醒阈值（{{ formatMB(bytes(balance.warning_threshold_bytes)) }} MB）。</p>
        <p v-if="denied">剩余额度低于准入门槛（{{ formatMB(bytes(balance.admission_threshold_bytes)) }} MB），新连接会被拒绝。</p>
      </div>
    </section>

    <section class="panel" aria-labelledby="usage-title">
      <div class="section-head">
        <div>
          <h2 id="usage-title">每日流量</h2>
          <p class="muted">{{ dates.start }} 至 {{ dates.end }} · 上海自然日</p>
        </div>
        <p class="muted">最近入库 {{ formatTime(result.usage.last_received_at) }}</p>
      </div>
      <p v-if="!days.length" class="empty" role="status">所选日期范围暂无用量。</p>
      <div v-else class="table-wrap" tabindex="0" role="region" aria-label="每日流量表">
        <table>
          <thead><tr><th scope="col">日期</th><th scope="col">上行（MB）</th><th scope="col">下行（MB）</th><th scope="col">合计（MB）</th><th scope="col">计费（MB）</th></tr></thead>
          <tbody>
            <tr v-for="{ date, amount } in days" :key="date">
              <th scope="row">{{ date }}</th>
              <td>{{ formatMB(bytes(amount?.input_bytes)) }}</td>
              <td>{{ formatMB(bytes(amount?.output_bytes)) }}</td>
              <td>{{ formatMB(bytes(amount?.total_bytes)) }}</td>
              <td>{{ formatMB(bytes(amount?.billable_bytes)) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel" aria-labelledby="grant-title">
      <div class="section-head"><h2 id="grant-title">生效中的流量授予</h2></div>
      <p v-if="!grants.length" class="empty">暂无生效中的流量授予。</p>
      <div v-else class="table-wrap" tabindex="0" role="region" aria-label="流量授予表">
        <table>
          <thead><tr><th scope="col">生效时间</th><th scope="col">到期时间</th><th scope="col">授予（MB）</th><th scope="col">已用（MB）</th></tr></thead>
          <tbody>
            <tr v-for="grant in grants" :key="grant.grant_id">
              <th scope="row">{{ formatTime(grant.starts_at) }}</th>
              <td>{{ grant.expires_at ? formatTime(grant.expires_at) : '无到期时间' }}</td>
              <td>{{ formatMB(bytes(grant.granted_bytes)) }}</td>
              <td>{{ formatMB(bytes(grant.used_bytes)) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </template>
</template>
