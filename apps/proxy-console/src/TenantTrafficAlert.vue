<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { ApiError } from './api/control'
import { getTrafficAlert, setTrafficAlert, type TrafficAlert } from './api/usage'
import { dateTime, flowAmount } from './wallet-format'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'
import Input from './ui/input/Input.vue'
import FieldLabel from './ui/field/FieldLabel.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ expired: [] }>()
const controller = new AbortController()
const state = ref<TrafficAlert>()
const threshold = ref('')
const busy = ref(false)
const error = ref('')
let refreshing = false
let timer: number | undefined

function inputValue(alert: TrafficAlert) {
  return alert.threshold_bytes === undefined ? '' : flowAmount(alert.threshold_bytes, true)
}

async function update(action: () => Promise<TrafficAlert>, failure: string) {
  if (busy.value) return
  busy.value = true
  error.value = ''
  try {
    const alert = await action()
    state.value = alert
    threshold.value = inputValue(alert)
  } catch (cause) {
    if (controller.signal.aborted) return
    if (cause instanceof ApiError && cause.status === 401) emit('expired')
    else error.value = failure
  } finally {
    busy.value = false
  }
}

function load() {
  return update(() => getTrafficAlert(props.token, controller.signal), '无法加载动态流量余额。')
}

function save() {
  const input = threshold.value.trim()
  let bytes: string | null = null
  if (input) {
    const [whole, fraction = ''] = input.split('.')
    const amount = BigInt(whole) * 1_000_000_000n + BigInt(fraction.padEnd(9, '0'))
    if (amount > 9_223_372_036_854_775_807n) {
      error.value = '阈值超出可设置范围。'
      return
    }
    bytes = String(amount)
  }
  return update(() => setTrafficAlert(props.token, bytes, controller.signal), '无法保存低余额阈值。')
}

async function refresh() {
  const previous = state.value
  if (!previous || busy.value || refreshing || document.hidden) return
  refreshing = true
  try {
    const alert = await getTrafficAlert(props.token, controller.signal)
    if (controller.signal.aborted || busy.value || state.value !== previous) return
    if (threshold.value === inputValue(previous)) threshold.value = inputValue(alert)
    state.value = alert
  } catch (cause) {
    if (!controller.signal.aborted && cause instanceof ApiError && cause.status === 401) emit('expired')
  } finally {
    refreshing = false
  }
}

onMounted(() => {
  void load()
  window.addEventListener('focus', refresh)
  timer = window.setInterval(refresh, 30_000)
})
onUnmounted(() => {
  controller.abort()
  window.removeEventListener('focus', refresh)
  window.clearInterval(timer)
})
</script>

<template>
  <Card class="dashboard-card mb-4 gap-4">
    <CardHeader>
      <h2 class="font-semibold">动态流量余额</h2>
      <p v-if="state" class="text-lg font-semibold tabular-nums">{{ flowAmount(state.available_bytes) }}</p>
    </CardHeader>
    <CardContent class="space-y-3">
      <p v-if="state?.triggered_at" :key="state.occurrence" class="text-sm text-amber-800">
        流量余额已达到或低于 {{ flowAmount(state.threshold_bytes) }}。提醒时间 {{ dateTime(state.triggered_at) }}（上海）。
      </p>
      <form v-if="state" class="flex flex-wrap items-end gap-3" @submit.prevent="save">
        <div class="grid gap-2">
          <FieldLabel for="traffic-alert-threshold">提醒阈值（GB，留空停用）</FieldLabel>
          <Input id="traffic-alert-threshold" v-model="threshold" inputmode="decimal" pattern="[0-9]+([.][0-9]{1,9})?"
            maxlength="20" :disabled="busy" />
        </div>
        <Button type="submit" variant="outline" :disabled="busy">{{ busy ? '正在保存…' : '保存阈值' }}</Button>
      </form>
      <p v-else-if="busy" role="status" class="text-sm text-muted-foreground">正在加载余额…</p>
      <p v-if="error" role="alert" class="text-sm text-destructive">{{ error }}</p>
      <Button v-if="!state && error" type="button" variant="outline" :disabled="busy" @click="load">重试</Button>
    </CardContent>
  </Card>
</template>
