<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElAlert, ElButton, ElEmpty, ElInput, ElMessage, ElOption, ElSelect, ElTable, ElTableColumn, ElTabPane, ElTabs, ElTooltip, type TableInstance } from 'element-plus'
import { CircleCheck, Clock, CopyDocument, Download, Loading, Search } from '@element-plus/icons-vue'
import RenewalDialog from './RenewalDialog.vue'
import StaticPricing from './StaticPricing.vue'
import { canRenew, credentialLine, endpoint, expiringWindowDays, formatExpiry, productLabels, remainingLabel, statusOf, type IpStatus, type StaticIp } from './demo'
import { ips } from './state'

type IpRow = { row: any }

const statusLabels: Record<IpStatus, string> = { available: '可用', expiring: '即将到期', renewing: '续费确认中', expired: '已到期' }
const statusFilters: { value: IpStatus | 'all', label: string }[] = [
  { value: 'all', label: '全部状态' },
  ...(Object.keys(statusLabels) as IpStatus[]).map(value => ({ value, label: statusLabels[value] })),
]

const tab = ref(location.hash.endsWith('/pricing') ? 'pricing' : 'ips')
const keyword = ref('')
const statusFilter = ref<IpStatus | 'all'>('all')
const selected = ref<StaticIp[]>([])
const table = ref<TableInstance>()
const renewalTargets = ref<StaticIp[]>([])
const renewalOpen = ref(false)

const usable = computed(() => ips.value.filter(ip => statusOf(ip) !== 'expired'))
const expiring = computed(() => ips.value.filter(ip => statusOf(ip) === 'expiring'))
const visibleIps = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  return ips.value.filter(ip =>
    (statusFilter.value === 'all' || statusOf(ip) === statusFilter.value)
    && (!term || endpoint(ip).includes(term) || ip.region.toLowerCase().includes(term)))
    .sort((a, b) => Number(statusOf(a) === 'expired') - Number(statusOf(b) === 'expired') || a.expiry - b.expiry)
})

function filterBy(status: IpStatus | 'all') {
  statusFilter.value = statusFilter.value === status ? 'all' : status
}

function resetFilters() {
  keyword.value = ''
  statusFilter.value = 'all'
}

function openRenewal(targets: StaticIp[]) {
  if (!targets.length) return
  renewalTargets.value = [...targets]
  renewalOpen.value = true
}

function onPurchased(count: number) {
  resetFilters()
  tab.value = 'ips'
  ElMessage.success(`已交付 ${count} 个 IP，已加入 IP 列表`)
}

async function copyLine(ip: StaticIp) {
  try {
    await navigator.clipboard.writeText(credentialLine(ip))
    ElMessage.success(`已复制 ${endpoint(ip)} 的接入信息`)
  } catch {
    ElMessage.error('复制失败，请检查浏览器剪贴板权限')
  }
}

function exportTxt() {
  const lines = selected.value.map(credentialLine).join('\r\n')
  const url = URL.createObjectURL(new Blob([lines + '\r\n'], { type: 'text/plain;charset=utf-8' }))
  const link = document.createElement('a')
  link.href = url
  link.download = 'static-ip-demo.txt'
  link.click()
  window.setTimeout(() => URL.revokeObjectURL(url), 1000)
  ElMessage.success(`已导出 ${selected.value.length} 个 IP`)
}
</script>

<template>
  <div class="page-head">
    <h1>静态住宅代理</h1>
    <ElButton type="primary" @click="tab = 'pricing'">购买静态 IP</ElButton>
  </div>

  <ElTabs v-model="tab" class="page-tabs">
    <ElTabPane label="IP 列表" name="ips">
      <ElAlert v-if="expiring.length" class="expiry-alert" type="warning" show-icon :closable="false"
        :title="`${expiring.length} 个 IP 将在 ${expiringWindowDays} 天内到期，到期后需重新购买。`">
        <ElButton link type="warning" @click="openRenewal(expiring)">立即续费</ElButton>
      </ElAlert>

      <div class="metrics" aria-label="静态 IP 概况">
        <button type="button" class="metric" :aria-pressed="statusFilter === 'available'" @click="filterBy('available')">
          <div><h2>可用 IP 数</h2><p class="metric-value">{{ usable.length }} <span>个</span></p><p class="metric-hint">含即将到期和续费确认中</p></div>
          <span class="metric-icon available"><CircleCheck /></span>
        </button>
        <button type="button" class="metric" :aria-pressed="statusFilter === 'expiring'" @click="filterBy('expiring')">
          <div><h2>即将到期 IP 数</h2><p class="metric-value">{{ expiring.length }} <span>个</span></p><p class="metric-hint">{{ expiringWindowDays }} 天内到期，点击筛选</p></div>
          <span class="metric-icon expiring"><Clock /></span>
        </button>
      </div>

      <section class="table-card" aria-label="静态 IP 列表">
        <div class="table-toolbar">
          <div class="table-filters">
            <ElInput v-model="keyword" class="filter-search" :prefix-icon="Search" clearable placeholder="搜索 IP 或地区" />
            <ElSelect v-model="statusFilter" class="filter-status" aria-label="按状态筛选">
              <ElOption v-for="option in statusFilters" :key="option.value" :value="option.value" :label="option.label" />
            </ElSelect>
          </div>
          <div class="table-actions">
            <span class="selection" role="status">已选择 {{ selected.length }} 个</span>
            <ElButton :disabled="!selected.length" @click="openRenewal(selected)">批量续费</ElButton>
            <ElButton :disabled="!selected.length" :icon="Download" @click="exportTxt">导出 TXT</ElButton>
          </div>
        </div>

        <ElTable ref="table" :data="visibleIps" row-key="address" class="ip-table"
          @selection-change="selected = $event">
          <ElTableColumn type="selection" width="52" :selectable="canRenew" reserve-selection />
          <ElTableColumn label="IP 地址" min-width="200">
            <template #default="{ row }: IpRow">
              <span class="endpoint-cell">
                <span class="endpoint">{{ endpoint(row) }}</span>
                <ElTooltip content="复制 IP:端口:用户名:密码" placement="top">
                  <ElButton link :icon="CopyDocument" :aria-label="`复制 ${endpoint(row)}`" @click="copyLine(row)" />
                </ElTooltip>
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="地区" min-width="170">
            <template #default="{ row }: IpRow">
              <span class="product-tag" :class="row.product">{{ productLabels[row.product as StaticIp['product']] }}</span>{{ row.region }}
              <span class="remaining">{{ row.isp }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="状态" min-width="130">
            <template #default="{ row }: IpRow">
              <span class="status-badge" :class="statusOf(row)">
                <Loading v-if="statusOf(row) === 'renewing'" class="spin" />{{ statusLabels[statusOf(row)] }}
              </span>
            </template>
          </ElTableColumn>
          <ElTableColumn prop="expiry" label="到期时间" min-width="180">
            <template #default="{ row }: IpRow">
              <time :datetime="new Date(row.expiry).toISOString()">{{ formatExpiry(row.expiry) }}</time>
              <span class="remaining" :class="statusOf(row)">{{ remainingLabel(row) }}</span>
            </template>
          </ElTableColumn>
          <ElTableColumn label="操作" width="120" fixed="right">
            <template #default="{ row }: IpRow">
              <ElTooltip :disabled="canRenew(row)" placement="top"
                :content="statusOf(row) === 'expired' ? '已到期的 IP 需重新购买' : '续费结果确认中，请勿重复提交'">
                <span><ElButton size="small" :disabled="!canRenew(row)" @click="openRenewal([row])">续费</ElButton></span>
              </ElTooltip>
            </template>
          </ElTableColumn>
          <template #empty>
            <ElEmpty description="没有符合条件的 IP" :image-size="72">
              <ElButton @click="resetFilters">清除筛选</ElButton>
            </ElEmpty>
          </template>
        </ElTable>
        <div class="table-footer">共 {{ visibleIps.length }} 条<template v-if="visibleIps.length !== ips.length">，全部 {{ ips.length }} 条</template></div>
      </section>
    </ElTabPane>

    <ElTabPane label="价格" name="pricing">
      <StaticPricing @purchased="onPurchased" />
    </ElTabPane>

    <ElTabPane label="使用教程" name="guide">
      <section class="details-card">
        <h2>使用静态住宅 IP</h2>
        <ol class="steps">
          <li><strong>查看接入信息</strong><p>在「IP 列表」查看 IP 地址、端口和到期时间，点击复制按钮获取完整接入信息。</p></li>
          <li><strong>配置你的代理客户端</strong><p>勾选可用的 IP 导出 TXT，每行格式为 <code>IP:端口:用户名:密码</code>。</p></li>
          <li><strong>到期前续费</strong><p>在 IP 对应行点击「续费」，或勾选多个 IP 后点击「批量续费」。到期后的 IP 需重新购买。</p></li>
        </ol>
        <ElButton @click="tab = 'ips'">返回 IP 列表</ElButton>
      </section>
    </ElTabPane>
  </ElTabs>
  <RenewalDialog v-model="renewalOpen" :ips="renewalTargets" @renewed="table?.clearSelection()" />
</template>
