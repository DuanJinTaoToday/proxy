<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElAlert, ElButton, ElEmpty, ElInput, ElMessage, ElMessageBox, ElOption, ElProgress, ElSelect, ElSwitch, ElTable, ElTableColumn, ElTag, ElTooltip } from 'element-plus'
import { CopyDocument, Plus, Search, View, Hide } from '@element-plus/icons-vue'
import SubAccountDialog, { type SubAccountDialogMode } from './SubAccountDialog.vue'
import { formatExpiry } from './demo'
import { primaryAccount, socksUsername, subAccountQuota, subAccounts, upsertSubAccount, type SubAccount } from './state'

// ElTable 的插槽行类型是 DefaultRow，模板里按 SubAccount 读取字段。
type AccountRow = { row: any }
type StatusFilter = 'all' | 'active' | 'inactive' | 'capped'

const keyword = ref('')
const status = ref<StatusFilter>('all')
const revealed = ref(new Set<string>())
const dialogOpen = ref(false)
const dialogMode = ref<SubAccountDialogMode>('create')
const dialogAccount = ref<SubAccount>()

const full = computed(() => subAccounts.value.length >= subAccountQuota)
const capped = (a: SubAccount) => a.limitGb !== undefined && a.usedGb >= a.limitGb
const visible = computed(() => {
  const term = keyword.value.trim().toLowerCase()
  return subAccounts.value.filter(a =>
    (!term || a.name.includes(term) || a.remark.toLowerCase().includes(term))
    && (status.value === 'all'
      || (status.value === 'active' && a.active)
      || (status.value === 'inactive' && !a.active)
      || (status.value === 'capped' && capped(a))))
})
const activeCount = computed(() => subAccounts.value.filter(a => a.active).length)
const totalUsed = computed(() => subAccounts.value.reduce((s, a) => s + a.usedGb, 0))

const gb = (value: number) => `${value.toLocaleString('zh-CN', { maximumFractionDigits: 2 })} GB`
const percent = (a: SubAccount) => a.limitGb ? Math.min(100, Math.round(a.usedGb / a.limitGb * 100)) : 0

function openDialog(mode: SubAccountDialogMode, account?: SubAccount) {
  dialogMode.value = mode
  dialogAccount.value = account
  dialogOpen.value = true
}

function onSaved(account: SubAccount) {
  const creating = dialogMode.value === 'create'
  upsertSubAccount(account)
  ElMessage.success(creating ? `已创建子账号 ${account.name}` : dialogMode.value === 'password' ? `已修改 ${account.name} 的密码` : `已保存 ${account.name}`)
}

async function toggleActive(account: SubAccount, next: boolean) {
  if (!next) {
    const ok = await ElMessageBox.confirm(
      `禁用后 ${account.name} 的新连接会被拒绝，已有连接继续到正常结束并照常计量。历史用量保留，账号仍占用名额。`,
      `禁用 ${account.name}？`, { confirmButtonText: '禁用', cancelButtonText: '取消', type: 'warning' },
    ).then(() => true, () => false)
    if (!ok) return
  }
  upsertSubAccount({ ...account, active: next })
  ElMessage.success(`${account.name} 已${next ? '启用' : '禁用'}`)
}

function toggleReveal(name: string) {
  const next = new Set(revealed.value)
  if (next.has(name)) next.delete(name)
  else next.add(name)
  revealed.value = next
}

async function copy(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`已复制${label}`)
  } catch {
    ElMessage.error('复制失败，请检查浏览器剪贴板权限')
  }
}

function resetFilters() {
  keyword.value = ''
  status.value = 'all'
}
</script>

<template>
  <div class="page-head">
    <h1>代理子账号</h1>
    <ElTooltip :disabled="!full" :content="`每个账户最多 ${subAccountQuota} 个子账号，禁用的账号也占用名额`" placement="bottom-end">
      <span><ElButton type="primary" :icon="Plus" :disabled="full" @click="openDialog('create')">创建子账号</ElButton></span>
    </ElTooltip>
  </div>

  <ElAlert class="sub-intro" type="info" :closable="false" show-icon>
    <template #title>子账号用于动态住宅代理-GB 的 SOCKS5 接入，可为不同项目分配独立账密和用量上限。</template>
    <p>SOCKS5 用户名为 <code>{{ primaryAccount }}.子账号名</code>，可追加 <code>-sid-会话标识</code> 保持会话。静态住宅 IP 使用各自的交付账密，不使用子账号。</p>
  </ElAlert>

  <div class="metrics sub-metrics" aria-label="子账号概况">
    <section class="metric">
      <div><h2>已用名额</h2><p class="metric-value">{{ subAccounts.length }} <span>/ {{ subAccountQuota }}</span></p><p class="metric-hint">启用和禁用的账号都计入</p></div>
    </section>
    <section class="metric">
      <div><h2>启用中</h2><p class="metric-value">{{ activeCount }} <span>个</span></p><p class="metric-hint">禁用的账号无法建立新连接</p></div>
    </section>
    <section class="metric">
      <div><h2>累计用量</h2><p class="metric-value">{{ gb(totalUsed) }}</p><p class="metric-hint">所有子账号合计，1 GB = 10⁹ 字节</p></div>
    </section>
  </div>

  <section class="table-card" aria-label="子账号列表">
    <div class="table-toolbar">
      <div class="table-filters">
        <ElInput v-model="keyword" class="filter-search" :prefix-icon="Search" clearable placeholder="搜索子账号名或备注" />
        <ElSelect v-model="status" class="filter-status" aria-label="按状态筛选">
          <ElOption value="all" label="全部状态" />
          <ElOption value="active" label="启用" />
          <ElOption value="inactive" label="禁用" />
          <ElOption value="capped" label="已达上限" />
        </ElSelect>
      </div>
    </div>

    <ElTable :data="visible" row-key="name" class="ip-table">
      <ElTableColumn label="子账号" min-width="230">
        <template #default="{ row }: AccountRow">
          <span class="endpoint-cell">
            <span class="endpoint">{{ row.name }}</span>
            <ElTooltip :content="`复制 SOCKS5 用户名 ${socksUsername(row.name)}`" placement="top">
              <ElButton link :icon="CopyDocument" :aria-label="`复制 ${row.name} 的 SOCKS5 用户名`" @click="copy(socksUsername(row.name), ' SOCKS5 用户名')" />
            </ElTooltip>
          </span>
          <span class="remaining">{{ row.remark || '无备注' }}</span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="密码" min-width="150">
        <template #default="{ row }: AccountRow">
          <span class="endpoint-cell">
            <span class="mono-value">{{ revealed.has(row.name) ? row.password : '••••••••' }}</span>
            <ElButton link :icon="revealed.has(row.name) ? Hide : View" :aria-label="revealed.has(row.name) ? '隐藏密码' : '显示密码'" @click="toggleReveal(row.name)" />
            <ElButton link :icon="CopyDocument" aria-label="复制密码" @click="copy(row.password, '密码')" />
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="累计用量 / 上限" min-width="200">
        <template #default="{ row }: AccountRow">
          <div class="usage-cell">
            <span><strong>{{ gb(row.usedGb) }}</strong> / {{ row.limitGb === undefined ? '不限' : gb(row.limitGb) }}</span>
            <ElProgress v-if="row.limitGb !== undefined" :percentage="percent(row)" :show-text="false" :stroke-width="5"
              :status="capped(row) ? 'exception' : percent(row) >= 80 ? 'warning' : undefined" />
          </div>
        </template>
      </ElTableColumn>
      <ElTableColumn label="状态" min-width="140">
        <template #default="{ row }: AccountRow">
          <span class="status-cell">
            <ElSwitch :model-value="row.active" :aria-label="`${row.active ? '禁用' : '启用'} ${row.name}`" @update:model-value="toggleActive(row, $event as boolean)" />
            <ElTag v-if="capped(row) && row.active" type="danger" size="small" disable-transitions>已达上限</ElTag>
            <span v-else class="remaining">{{ row.active ? '启用' : '禁用' }}</span>
          </span>
        </template>
      </ElTableColumn>
      <ElTableColumn label="创建时间" min-width="140">
        <template #default="{ row }: AccountRow">{{ formatExpiry(row.createdAt) }}</template>
      </ElTableColumn>
      <ElTableColumn label="操作" width="150" fixed="right">
        <template #default="{ row }: AccountRow">
          <ElButton link type="primary" @click="openDialog('edit', row)">编辑</ElButton>
          <ElButton link type="primary" @click="openDialog('password', row)">修改密码</ElButton>
        </template>
      </ElTableColumn>
      <template #empty>
        <ElEmpty v-if="subAccounts.length" description="没有符合条件的子账号" :image-size="72">
          <ElButton @click="resetFilters">清除筛选</ElButton>
        </ElEmpty>
        <ElEmpty v-else description="还没有子账号，创建后即可接入动态住宅代理" :image-size="72">
          <ElButton type="primary" @click="openDialog('create')">创建子账号</ElButton>
        </ElEmpty>
      </template>
    </ElTable>
    <div class="table-footer">共 {{ visible.length }} 个<template v-if="visible.length !== subAccounts.length">，全部 {{ subAccounts.length }} 个</template></div>
  </section>

  <SubAccountDialog v-model="dialogOpen" :mode="dialogMode" :account="dialogAccount" @saved="onSaved" />
</template>
