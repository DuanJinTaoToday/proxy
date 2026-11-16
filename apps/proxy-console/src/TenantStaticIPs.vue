<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ApiError } from './api/control'
import { claimStaticIPExpiryNotices, staticIPConfig, staticIPExport, staticIPs } from './api/static-ip'
import type { StaticIP } from './api/static-ip'
import { Button } from './ui/button'
import StaticRenewal from './StaticRenewal.vue'
import type { WalletTenant } from './api/wallet'
import Table from './ui/table/Table.vue'
import TableHeader from './ui/table/TableHeader.vue'
import TableBody from './ui/table/TableBody.vue'
import TableRow from './ui/table/TableRow.vue'
import TableHead from './ui/table/TableHead.vue'
import TableCell from './ui/table/TableCell.vue'

const { token, tenantId } = defineProps<{ token: string; tenantId: string }>()
const emit = defineEmits<{ expired: []; wallet: [WalletTenant] }>()
const controller = new AbortController()
const allocations = ref<StaticIP[]>([])
const checked = ref<string[]>([])
const loading = ref(true)
const error = ref(false)
const feedback = ref('')
const expiryNotice = ref(0)
const now = ref(Date.now())
const timer = window.setInterval(() => { now.value = Date.now() }, 60_000)
const available = (ip: StaticIP) => Date.parse(ip.expires_at) > now.value
const nearingExpiry = (ip: StaticIP) => available(ip) && Date.parse(ip.expires_at) - now.value <= 5 * 86400_000
const activeCount = computed(() => allocations.value.filter(available).length)
const expiringCount = computed(() => allocations.value.filter(nearingExpiry).length)
const selected = computed(() => allocations.value.filter(ip => available(ip) && checked.value.includes(ip.supplier_item_id)))
const formatter = new Intl.DateTimeFormat('zh-CN', { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Shanghai' })
const line = (ip: StaticIP) => [ip.ip.includes(':') ? '[' + ip.ip + ']' : ip.ip, ip.port, ip.username, ip.password].join(':')

watch(expiringCount, async (count) => {
  if (!count) return
  const notice = await claimStaticIPExpiryNotices(token, controller.signal).catch(() => undefined)
  if (notice?.count) expiryNotice.value = notice.count
})

async function load() {
  loading.value = true
  error.value = false
  try {
    allocations.value = (await staticIPs(token, controller.signal)).allocations ?? []
  } catch (failure) {
    if (controller.signal.aborted) return
    if (failure instanceof ApiError && failure.status === 401) emit('expired')
    else error.value = true
  } finally {
    loading.value = false
  }
}

async function copy(ip: StaticIP) {
  try {
    const config = await staticIPConfig(token, ip.supplier_item_id, controller.signal)
    await navigator.clipboard.writeText(line(config))
    feedback.value = '已复制。'
  } catch {
    feedback.value = '复制失败，请检查剪贴板权限。'
  }
}

async function exportTXT() {
  if (!selected.value.length) return
  try {
    const blob = await staticIPExport(token, selected.value.map(ip => ip.supplier_item_id), controller.signal)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = 'static-ips.txt'
    link.click()
    window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch {
    feedback.value = '导出失败，请刷新列表并检查所选 IP 是否仍可用。'
  }
}

onMounted(load)
onUnmounted(() => { controller.abort(); window.clearInterval(timer) })
</script>

<template>
  <section class="panel wallet-section" aria-label="静态 IP">
    <p v-if="loading" role="status">正在加载静态 IP…</p>
    <div v-else-if="error">
      <p role="alert">暂时无法查询静态 IP。</p>
      <Button type="button" variant="outline" @click="load">重新加载</Button>
    </div>
    <template v-else>
      <p v-if="expiryNotice" role="status">{{ expiryNotice }} 个静态 IP 将在 5 天内到期。</p>
      <p>可用静态 IP {{ activeCount }} 个<span v-if="expiringCount">，5 天内到期 {{ expiringCount }} 个</span>。</p>
      <p v-if="!allocations.length">暂无交付记录。</p>
      <template v-else>
        <Button type="button" variant="outline" :disabled="!selected.length" @click="exportTXT">导出所选 IP 的 TXT</Button>
        <p v-if="feedback" role="status">{{ feedback }}</p>
        <div class="table-scroll">
          <Table>
            <TableHeader><TableRow><TableHead scope="col">选择</TableHead><TableHead scope="col">IP</TableHead><TableHead scope="col">到期时间（北京时间）</TableHead><TableHead scope="col">状态</TableHead><TableHead scope="col">操作</TableHead></TableRow></TableHeader>
            <TableBody>
              <TableRow v-for="ip in allocations" :key="ip.supplier_item_id">
                <TableCell><input v-model="checked" type="checkbox" :value="ip.supplier_item_id" :disabled="!available(ip)" :aria-label="'选择 ' + ip.ip" /></TableCell>
                <TableCell>{{ ip.ip }}:{{ ip.port }}</TableCell>
                <TableCell>{{ formatter.format(new Date(ip.expires_at)) }}</TableCell>
                <TableCell>{{ !available(ip) ? '已到期' : nearingExpiry(ip) ? '即将到期' : '可用' }}</TableCell>
                <TableCell><Button type="button" variant="outline" :disabled="!available(ip)" @click="copy(ip)">复制接入信息</Button></TableCell>
              </TableRow>
            </TableBody>
          </Table>
        </div>
      </template>
    </template>
    <StaticRenewal :token="token" :tenant-id="tenantId" :selected="selected.map(ip => ip.supplier_item_id)"
      @updated="load" @expired="emit('expired')" @wallet="emit('wallet', $event)" />
  </section>
</template>
