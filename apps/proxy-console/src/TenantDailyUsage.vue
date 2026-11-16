<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { Download } from 'lucide-vue-next'
import { ApiError } from './api/control'
import { listSubaccounts, type Subaccount } from './api/subaccounts'
import { exportDailyUsage, listDailyUsage, type UsageDay, type UsageFilter } from './api/usage'
import { dateTime, flowAmount } from './wallet-format'
import { Button } from './ui/button'
import Card from './ui/card/Card.vue'
import CardContent from './ui/card/CardContent.vue'
import CardHeader from './ui/card/CardHeader.vue'
import Input from './ui/input/Input.vue'
import FieldLabel from './ui/field/FieldLabel.vue'
import Table from './ui/table/Table.vue'
import TableBody from './ui/table/TableBody.vue'
import TableCell from './ui/table/TableCell.vue'
import TableHead from './ui/table/TableHead.vue'
import TableHeader from './ui/table/TableHeader.vue'
import TableRow from './ui/table/TableRow.vue'

const props = defineProps<{ token: string }>()
const emit = defineEmits<{ expired: [] }>()
const controller = new AbortController()
const today = new Intl.DateTimeFormat('sv-SE', { timeZone: 'Asia/Shanghai' }).format(new Date())
const filter = reactive<UsageFilter>({ start_date: today, end_date: today, client_id: '' })
const applied = ref<UsageFilter>()
const days = ref<UsageDay[]>([])
const accounts = ref<Subaccount[]>([])
const accountError = ref(false)
const loading = ref(false)
const exporting = ref(false)
const queryError = ref('')
const exportError = ref('')
const current = computed(() => applied.value?.start_date === filter.start_date &&
  applied.value?.end_date === filter.end_date && applied.value?.client_id === filter.client_id)

function failure(cause: unknown, fallback: string) {
  if (controller.signal.aborted) return ''
  if (cause instanceof ApiError && cause.status === 401) {
    emit('expired')
    return ''
  }
  if (cause instanceof ApiError && cause.reason === 'INVALID_DATE_RANGE') return '请选择有效的日期范围。'
  return fallback
}

async function loadAccounts() {
  accountError.value = false
  try {
    accounts.value = (await listSubaccounts(props.token, controller.signal)).accounts ?? []
  } catch (cause) {
    accountError.value = Boolean(failure(cause, '无法加载子账号。'))
  }
}

async function loadUsage() {
  if (loading.value) return
  const query = { ...filter }
  loading.value = true
  queryError.value = ''
  exportError.value = ''
  applied.value = undefined
  try {
    const result = await listDailyUsage(props.token, query, controller.signal)
    days.value = result.days
    applied.value = query
  } catch (cause) {
    queryError.value = failure(cause, '无法查询日用量，请重试。')
  } finally {
    loading.value = false
  }
}

async function download() {
  if (exporting.value || !current.value || !applied.value) return
  exporting.value = true
  exportError.value = ''
  try {
    const data = await exportDailyUsage(props.token, applied.value, controller.signal)
    if (controller.signal.aborted) return
    const url = URL.createObjectURL(data)
    const link = document.createElement('a')
    link.href = url
    link.download = 'daily-usage.csv'
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (cause) {
    exportError.value = failure(cause, '无法导出 CSV，请重试。')
  } finally {
    exporting.value = false
  }
}

onMounted(() => { void loadAccounts(); void loadUsage() })
onUnmounted(() => controller.abort())
</script>

<template>
  <Card class="dashboard-card gap-4">
    <CardHeader>
      <h2 class="font-semibold">动态流量日用量</h2>
      <p class="text-sm text-muted-foreground">按上海自然日汇总实际扣减的上行和下行流量。</p>
    </CardHeader>
    <CardContent class="space-y-4">
      <form class="flex flex-wrap items-end gap-3" @submit.prevent="loadUsage">
        <div class="grid gap-2">
          <FieldLabel for="usage-start">开始日期</FieldLabel>
          <Input id="usage-start" v-model="filter.start_date" type="date" required />
        </div>
        <div class="grid gap-2">
          <FieldLabel for="usage-end">结束日期</FieldLabel>
          <Input id="usage-end" v-model="filter.end_date" type="date" :min="filter.start_date" required />
        </div>
        <div class="grid min-w-36 gap-2">
          <FieldLabel for="usage-client">代理子账号</FieldLabel>
          <select id="usage-client" v-model="filter.client_id" class="h-9 rounded-md border bg-background px-3 text-sm">
            <option value="">全部子账号</option>
            <option v-for="account in accounts" :key="account.client_id" :value="account.client_id">
              {{ account.name }}{{ account.active ? '' : '（已禁用）' }}
            </option>
          </select>
        </div>
        <Button type="submit" :disabled="loading">查询</Button>
        <Button type="button" variant="outline" :disabled="!current || exporting" @click="download"><Download />导出 CSV</Button>
      </form>
      <div v-if="accountError" class="flex flex-wrap items-center gap-2">
        <p role="alert" class="text-sm text-destructive">无法加载子账号。</p>
        <Button type="button" size="sm" variant="outline" @click="loadAccounts">重试</Button>
      </div>
      <p v-if="exportError" role="alert" class="text-sm text-destructive">{{ exportError }}</p>
      <p v-if="loading" role="status" class="text-sm text-muted-foreground">正在查询用量…</p>
      <p v-else-if="queryError" role="alert" class="text-sm text-destructive">{{ queryError }}</p>
      <p v-else-if="!current" class="text-sm text-muted-foreground">筛选已修改，请查询以更新结果。</p>
      <p v-else-if="!days.length" class="text-sm text-muted-foreground">所选范围内暂无实扣用量。</p>
      <Table v-else>
        <TableHeader><TableRow><TableHead>日期</TableHead><TableHead>实扣流量</TableHead><TableHead>数据更新时间（上海）</TableHead></TableRow></TableHeader>
        <TableBody>
          <TableRow v-for="day in days" :key="day.date">
            <TableCell>{{ day.date }}</TableCell>
            <TableCell>{{ flowAmount(day.billable_bytes) }}</TableCell>
            <TableCell>{{ day.updated_at ? dateTime(day.updated_at) : '—' }}</TableCell>
          </TableRow>
        </TableBody>
      </Table>
    </CardContent>
  </Card>
</template>
