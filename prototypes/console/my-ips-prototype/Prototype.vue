<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowDown, ChevronRight, Clock3, Copy, Download, Globe2, LayoutDashboard, ListOrdered, Network, Search, Wallet } from 'lucide-vue-next'
import DashboardShell from '@console/DashboardShell.vue'
import brandLogo from '@console/assets/kunxing-mark.png'
import Badge from '@console/ui/badge/Badge.vue'
import { Button } from '@console/ui/button'
import Dialog from '@console/ui/dialog/Dialog.vue'
import DialogContent from '@console/ui/dialog/DialogContent.vue'
import DialogTitle from '@console/ui/dialog/DialogTitle.vue'
import Table from '@console/ui/table/Table.vue'
import TableHeader from '@console/ui/table/TableHeader.vue'
import TableBody from '@console/ui/table/TableBody.vue'
import TableRow from '@console/ui/table/TableRow.vue'
import TableHead from '@console/ui/table/TableHead.vue'
import TableCell from '@console/ui/table/TableCell.vue'

// Both layouts use the same simulated allocations and renewal flow.
type Status = '可用' | '即将到期' | '续费确认中' | '已到期'
type Allocation = { host: string; expires: string; status: Status }
const allocations = ref<Allocation[]>([
  { host: '203.0.113.24', expires: '11-22 18:00', status: '即将到期' },
  { host: '203.0.113.58', expires: '11-24 09:30', status: '即将到期' },
  { host: '198.51.100.16', expires: '12-07 10:00', status: '可用' },
  { host: '198.51.100.42', expires: '12-12 16:20', status: '续费确认中' },
  { host: '192.0.2.35', expires: '12-18 13:45', status: '可用' },
  { host: '192.0.2.81', expires: '11-17 15:00', status: '已到期' },
])
const statuses = ['全部', '即将到期', '可用', '续费确认中', '已到期'] as const
const filter = ref<(typeof statuses)[number]>('全部')
const query = ref('')
const ascending = ref(true)
const checked = ref<string[]>([])
const focusedHost = ref(allocations.value[0]?.host)
const feedback = ref('')
const route = useRoute()
const router = useRouter()
const split = computed(() => route.query.variant === 'details')
const nearingCount = computed(() => allocations.value.filter(ip => ip.status === '即将到期').length)
const filtered = computed(() => {
  const matches = allocations.value.filter(ip =>
    (filter.value === '全部' || ip.status === filter.value) && ip.host.includes(query.value.trim()),
  )
  return matches.sort((a, b) => ascending.value
    ? a.expires.localeCompare(b.expires)
    : b.expires.localeCompare(a.expires))
})
const focused = computed(() => filtered.value.find(ip => ip.host === focusedHost.value) ?? filtered.value[0])
const eligible = computed(() => filtered.value.filter(ip => ip.status !== '已到期'))
const selected = computed(() => eligible.value.filter(ip => checked.value.includes(ip.host)))
const renewable = computed(() => selected.value.filter(ip => ip.status !== '续费确认中'))
const selectionBlocked = computed(() => renewable.value.length !== selected.value.length)
const allChecked = computed(() => eligible.value.length > 0 && selected.value.length === eligible.value.length)
const renewal = ref<{ phase: 'term' | 'quote' | 'pending'; items: Allocation[] } | null>(null)
const term = ref(30)
const unitPrice = computed(() => term.value * 1.6)
const currency = (amount: number) => new Intl.NumberFormat('zh-CN', { style: 'currency', currency: 'CNY' }).format(amount)

function setFilter(value: (typeof statuses)[number]) {
  filter.value = value
  checked.value = []
}

function changeFilter(event: Event) {
  if (event.target instanceof HTMLSelectElement) {
    setFilter(statuses[event.target.selectedIndex] ?? '全部')
  }
}

function toggleAll() {
  checked.value = allChecked.value ? [] : eligible.value.map(ip => ip.host)
}

function setVariant(value: 'table' | 'details') {
  void router.replace({ query: { variant: value } })
}

function credential(ip: Allocation) {
  return `${ip.host}:1080:demo-user:demo-pass`
}

async function copy(ip: Allocation) {
  try {
    await navigator.clipboard.writeText(credential(ip))
    feedback.value = `已复制 ${ip.host} 的接入信息`
  } catch {
    feedback.value = '复制失败，请检查浏览器的剪贴板权限。'
  }
}

function exportTXT() {
  const file = new Blob([selected.value.map(credential).join('\n') + '\n'], { type: 'text/plain' })
  const url = URL.createObjectURL(file)
  const link = document.createElement('a')
  link.href = url
  link.download = 'static-ips-demo.txt'
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  feedback.value = `已导出 ${selected.value.length} 个 IP`
}

function openRenewal(items: Allocation[]) {
  term.value = 30
  renewal.value = { phase: 'term', items }
}

function submitRenewal() {
  if (renewal.value?.phase !== 'quote') return
  const hosts = new Set(renewal.value.items.map(ip => ip.host))
  allocations.value = allocations.value.map(ip =>
    hosts.has(ip.host) ? { ...ip, status: '续费确认中' } : ip,
  )
  checked.value = []
  renewal.value = { ...renewal.value, phase: 'pending' }
}

function statusVariant(status: Status) {
  return status === '已到期' ? 'secondary' : 'outline'
}
</script>

<template>
  <DashboardShell brand="鲲星代理" :brand-logo="brandLogo" area="静态住宅代理" title="我的 IP" account-name="体验账号">
    <template #navigation>
      <nav class="ip-preview-nav" aria-label="控制台导航示意">
        <span class="nav-label">工作台</span>
        <span><LayoutDashboard />账户概览</span>
        <span><Wallet />钱包</span>
        <span><ListOrdered />订单记录</span>
        <span class="nav-label">代理服务</span>
        <span><Network />动态住宅代理</span>
        <span class="ip-preview-nav-parent"><Globe2 />静态住宅代理</span>
        <span class="ip-preview-nav-current" aria-current="page">我的 IP</span>
      </nav>
    </template>

    <div class="ip-preview">
      <div class="ip-preview-intro">
        <p>查看已交付的静态住宅 IP，复制接入信息或续费保留原 IP。</p>
        <span>交互原型 · 模拟数据</span>
      </div>

      <div v-if="nearingCount && filter !== '即将到期'" class="ip-preview-notice">
        <Clock3 :size="17" />
        <p>{{ nearingCount }} 个 IP 将在 5 天内到期。</p>
        <Button size="sm" variant="ghost" @click="setFilter('即将到期')">查看即将到期 <ChevronRight data-icon="inline-end" /></Button>
      </div>

      <div class="ip-preview-layout" :class="{ 'ip-preview-split': split }">
        <section class="ip-preview-panel" aria-label="静态 IP 列表">
          <div class="ip-preview-filters">
            <div class="ip-preview-search">
              <Search :size="16" />
              <input v-model="query" type="search" placeholder="搜索 IP 地址" aria-label="搜索 IP 地址" @input="checked = []">
            </div>
            <select :value="filter" aria-label="筛选状态" @change="changeFilter">
              <option v-for="status in statuses" :key="status">{{ status }}</option>
            </select>
            <span class="ip-preview-batch">
              <Button size="sm" variant="outline" :disabled="!selected.length" @click="exportTXT"><Download data-icon="inline-start" /> 导出 TXT</Button>
              <Button size="sm" class="ip-preview-primary" :disabled="!renewable.length || selectionBlocked" @click="openRenewal(renewable)">批量续费</Button>
            </span>
          </div>
          <p v-if="selectionBlocked" class="ip-preview-hint">所选 IP 中有续费待确认，请取消勾选后再继续。</p>

          <Table class="ip-preview-table">
            <TableHeader>
              <TableRow>
                <TableHead scope="col"><input type="checkbox" aria-label="全选可用 IP" :checked="allChecked" :indeterminate="selected.length > 0 && !allChecked" :disabled="!eligible.length" @change="toggleAll"></TableHead>
                <TableHead scope="col">IP 地址</TableHead>
                <TableHead v-if="!split" scope="col">端口</TableHead>
                <TableHead scope="col" :aria-sort="ascending ? 'ascending' : 'descending'">
                  <Button size="sm" variant="ghost" class="ip-preview-sort" @click="ascending = !ascending">
                    到期时间 <ArrowDown :class="{ 'ip-preview-sort-desc': !ascending }" data-icon="inline-end" />
                  </Button>
                </TableHead>
                <TableHead scope="col">状态</TableHead>
                <TableHead v-if="!split" scope="col" class="ip-preview-row-actions">操作</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              <TableRow v-for="ip in filtered" :key="ip.host" :data-focused="split && focused?.host === ip.host">
                <TableCell><input v-model="checked" type="checkbox" :value="ip.host" :aria-label="'选择 ' + ip.host" :disabled="ip.status === '已到期'"></TableCell>
                <TableCell>
                  <Button v-if="split" size="sm" variant="ghost" class="ip-preview-address" :aria-pressed="focused?.host === ip.host" @click="focusedHost = ip.host">{{ ip.host }}</Button>
                  <span v-else class="ip-preview-address">{{ ip.host }}</span>
                </TableCell>
                <TableCell v-if="!split" class="ip-preview-numerals">1080</TableCell>
                <TableCell class="ip-preview-numerals">2026-{{ ip.expires }}</TableCell>
                <TableCell><Badge :variant="statusVariant(ip.status)" class="ip-preview-status" :data-status="ip.status">{{ ip.status }}</Badge></TableCell>
                <TableCell v-if="!split">
                  <div class="ip-preview-row-actions">
                    <Button size="sm" variant="ghost" :disabled="ip.status === '已到期'" @click="copy(ip)"><Copy data-icon="inline-start" /> 复制</Button>
                    <Button v-if="ip.status === '续费确认中'" size="sm" variant="ghost" @click="renewal = { phase: 'pending', items: [ip] }">查看结果</Button>
                    <Button v-else size="sm" variant="ghost" :disabled="ip.status === '已到期'" @click="openRenewal([ip])">续费</Button>
                  </div>
                </TableCell>
              </TableRow>
              <TableRow v-if="!filtered.length">
                <TableCell :colspan="split ? 4 : 6" class="ip-preview-empty">没有符合条件的 IP。<Button size="sm" variant="ghost" @click="query = ''; setFilter('全部')">清除筛选</Button></TableCell>
              </TableRow>
            </TableBody>
          </Table>
          <div class="ip-preview-table-footer">
            <span>共 {{ filtered.length }} 个 IP<span v-if="selected.length">，已选 {{ selected.length }} 个</span></span>
            <span>到期时间为北京时间（UTC+8）</span>
          </div>
        </section>

        <aside v-if="split && focused" class="ip-preview-panel ip-preview-details" aria-label="接入详情">
          <div><h2>接入详情</h2><p class="ip-preview-address">{{ focused.host }}:1080</p></div>
          <Badge :variant="statusVariant(focused.status)" class="ip-preview-status" :data-status="focused.status">{{ focused.status }}</Badge>
          <dl>
            <div><dt>协议</dt><dd>SOCKS5</dd></div>
            <div><dt>到期时间</dt><dd>2026-{{ focused.expires }}</dd></div>
            <div><dt>用户名</dt><dd>{{ focused.status === '已到期' ? '—' : 'demo-user' }}</dd></div>
            <div><dt>密码</dt><dd>{{ focused.status === '已到期' ? '—' : '••••••••' }}</dd></div>
          </dl>
          <div class="ip-preview-detail-actions">
            <Button size="sm" variant="outline" :disabled="focused.status === '已到期'" @click="copy(focused)"><Copy data-icon="inline-start" /> 复制接入信息</Button>
            <Button v-if="focused.status === '续费确认中'" size="sm" variant="outline" @click="renewal = { phase: 'pending', items: [focused] }">查看结果</Button>
            <Button v-else size="sm" class="ip-preview-primary" :disabled="focused.status === '已到期'" @click="openRenewal([focused])">续费</Button>
          </div>
        </aside>
      </div>
      <p class="ip-preview-feedback" role="status">{{ feedback }}</p>

      <nav class="ip-preview-switch" aria-label="切换原型方案">
        <span>原型方案</span>
        <Button size="sm" variant="ghost" :aria-pressed="!split" @click="setVariant('table')">A · 集中列表</Button>
        <Button size="sm" variant="ghost" :aria-pressed="split" @click="setVariant('details')">B · 列表加详情</Button>
      </nav>
    </div>
  </DashboardShell>

  <Dialog :open="Boolean(renewal)" @update:open="value => { if (!value) renewal = null }">
    <DialogContent v-if="renewal" class="ip-preview-dialog" :show-close-button="false" aria-describedby="ip-preview-renewal-description">
      <DialogTitle>{{ renewal.phase === 'pending' ? '续费确认中' : '保号续费' }}</DialogTitle>
      <p id="ip-preview-renewal-description">
        {{ renewal.phase === 'pending' ? '供应商正在确认结果。等待期间请勿重复提交。' : '续费成功后保留原 IP，从原到期时间顺延。' }}
      </p>
      <label v-if="renewal.phase !== 'pending'">续费租期
        <select v-model.number="term" @change="renewal.phase = 'term'">
          <option :value="30">30 天</option><option :value="90">90 天</option>
          <option :value="180">180 天</option><option :value="360">360 天</option>
        </select>
      </label>
      <ul class="ip-preview-quote-list">
        <li v-for="ip in renewal.items" :key="ip.host">
          <span class="ip-preview-address">{{ ip.host }}</span>
          <span v-if="renewal.phase === 'quote'">{{ currency(unitPrice) }}</span>
        </li>
      </ul>
      <p v-if="renewal.phase === 'quote'" class="ip-preview-total"><span>预计扣款</span><strong>{{ currency(unitPrice * renewal.items.length) }}</strong></p>
      <div class="ip-preview-dialog-actions">
        <Button variant="outline" @click="renewal = null">{{ renewal.phase === 'pending' ? '关闭' : '取消' }}</Button>
        <Button v-if="renewal.phase === 'term'" @click="renewal.phase = 'quote'">获取续费报价</Button>
        <Button v-else-if="renewal.phase === 'quote'" @click="submitRenewal">确认保号续费</Button>
      </div>
    </DialogContent>
  </Dialog>
</template>
