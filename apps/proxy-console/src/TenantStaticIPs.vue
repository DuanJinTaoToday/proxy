<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { ApiError } from './api/control'
import { staticIPs } from './api/static-ip'
import type { StaticIP } from './api/static-ip'
import { Button } from './ui/button'

const { token, tenantId, summary } = defineProps<{ token: string; tenantId: string; summary?: boolean }>()
const emit = defineEmits<{ expired: [] }>()
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

watch([allocations, now], () => {
  const soon = allocations.value.filter(nearingExpiry)
  if (!soon.length) return
  try {
    const key = 'proxy-console.static-expiry.' + tenantId
    const previous = JSON.parse(localStorage.getItem(key) ?? '{}') as Record<string, string>
    const active = Object.fromEntries(soon.map(ip => [ip.supplier_item_id, ip.expires_at]))
    const count = soon.filter(ip => previous[ip.supplier_item_id] !== ip.expires_at).length
    if (count) expiryNotice.value = count
    localStorage.setItem(key, JSON.stringify(active))
  } catch {
    // Current expiry status and counts remain visible even without storage.
  }
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
    await navigator.clipboard.writeText(line(ip))
    feedback.value = '已复制。'
  } catch {
    feedback.value = '复制失败，请检查剪贴板权限。'
  }
}

function exportTXT() {
  if (!selected.value.length) return
  const url = URL.createObjectURL(new Blob([selected.value.map(line).join('\n') + '\n'], { type: 'text/plain;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = 'static-ips.txt'
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
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
      <RouterLink v-if="summary" :to="{ path: '/', query: { view: 'ips' } }">查看我的 IP</RouterLink>
      <p v-else-if="!allocations.length">暂无交付记录。</p>
      <template v-else>
        <Button type="button" variant="outline" :disabled="!selected.length" @click="exportTXT">导出所选 IP 的 TXT</Button>
        <p v-if="feedback" role="status">{{ feedback }}</p>
        <div class="table-scroll">
          <table>
            <thead><tr><th scope="col">选择</th><th scope="col">IP</th><th scope="col">到期时间（北京时间）</th><th scope="col">状态</th><th scope="col">操作</th></tr></thead>
            <tbody>
              <tr v-for="ip in allocations" :key="ip.supplier_item_id">
                <td><input v-model="checked" type="checkbox" :value="ip.supplier_item_id" :disabled="!available(ip)" :aria-label="'选择 ' + ip.ip" /></td>
                <td>{{ ip.ip }}:{{ ip.port }}</td>
                <td>{{ formatter.format(new Date(ip.expires_at)) }}</td>
                <td>{{ !available(ip) ? '已到期' : nearingExpiry(ip) ? '即将到期' : '可用' }}</td>
                <td><Button type="button" variant="outline" @click="copy(ip)">复制接入信息</Button></td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </template>
  </section>
</template>
