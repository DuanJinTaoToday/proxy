<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElAlert, ElButton, ElDialog, ElForm, ElFormItem, ElInputNumber, ElMessage, ElMessageBox, ElSwitch, ElTable, ElTableColumn, ElTag, ElTooltip } from 'element-plus'
import { formatExpiry, formatMoney } from './demo'
import { billingOnly, canBill, operator } from './ops'
import { basePerGbCents, productAudit, productOnSale, tick, trafficPackages, type SaleProduct, type TrafficPackage } from './state'

type Row = { row: any }
const saleProducts: { key: SaleProduct, name: string, hint: string }[] = [
  { key: 'dynamic', name: '动态住宅代理-GB', hint: '按 GB 售卖流量包，SOCKS5 隧道接入。' },
  { key: 'static', name: '静态住宅代理', hint: '皇冠交付，售价直接取皇冠目录价，不在这里调整。' },
]

function audit(target: string, before: string, after: string) {
  productAudit.value.unshift({ at: tick(), operator: operator.value.name, target, before, after })
}

async function toggleSale(key: SaleProduct, name: string) {
  const next = !productOnSale.value[key]
  const ok = await ElMessageBox.confirm(
    next ? `开启后官网和客户控制台展示报价和购买入口。请确认「${name}」已通过验收、报价就绪。` : `关闭后官网和控制台标记「暂未开放购买」，不再接受新订单。已成交的订单和已到账的资源不受影响。`,
    next ? `开售${name}？` : `暂停销售${name}？`,
    { confirmButtonText: next ? '开售' : '暂停销售', cancelButtonText: '取消', type: next ? 'info' : 'warning' },
  ).then(() => true, () => false)
  if (!ok) return
  productOnSale.value[key] = next
  audit(`${name} · 开售`, next ? '暂未开放' : '开售', next ? '开售' : '暂未开放')
  ElMessage.success(next ? `${name}已开售` : `${name}已暂停销售`)
}

const perGb = (pkg: TrafficPackage) => pkg.cents / pkg.gb
const discount = (pkg: TrafficPackage) => Math.round((1 - perGb(pkg) / basePerGbCents.value) * 100)
const listedCount = computed(() => trafficPackages.value.filter(p => p.listed).length)

async function toggleListed(pkg: TrafficPackage) {
  const next = !pkg.listed
  if (!next && listedCount.value === 1) {
    const ok = await ElMessageBox.confirm('下架后没有可售的流量包，客户将无法购买动态流量。', '下架最后一档？', { confirmButtonText: '仍然下架', cancelButtonText: '取消', type: 'warning' }).then(() => true, () => false)
    if (!ok) return
  }
  pkg.listed = next
  audit(`${pkg.gb} GB 流量包 · 状态`, next ? '已下架' : '上架', next ? '上架' : '已下架')
  ElMessage.success(`${pkg.gb} GB 流量包已${next ? '上架' : '下架'}`)
}

const editing = ref<TrafficPackage>()
const priceYuan = ref<number>()
function edit(pkg: TrafficPackage) {
  editing.value = pkg
  priceYuan.value = pkg.cents / 100
}
const newCents = computed(() => Math.round((priceYuan.value ?? 0) * 100))
function savePrice() {
  const pkg = editing.value!
  if (newCents.value !== pkg.cents) {
    audit(`${pkg.gb} GB 流量包 · 售价`, formatMoney(pkg.cents), formatMoney(newCents.value))
    pkg.cents = newCents.value
    ElMessage.success('售价已更新，新订单按新价格成交')
  }
  editing.value = undefined
}
</script>

<template>
  <div class="page-head"><h1>商品与定价</h1></div>
  <ElAlert v-if="!canBill" type="info" :closable="false" show-icon class="page-alert" :title="`当前角色只能查看。开售、调价和上下架${billingOnly}。`" />

  <div class="sale-grid">
    <section v-for="p in saleProducts" :key="p.key" class="details-card sale-card">
      <div>
        <h2>{{ p.name }}</h2>
        <p class="card-desc">{{ p.hint }}</p>
      </div>
      <div class="sale-switch">
        <ElTag :type="productOnSale[p.key] ? 'success' : 'info'" disable-transitions>{{ productOnSale[p.key] ? '开售中' : '暂未开放购买' }}</ElTag>
        <ElTooltip :disabled="canBill" :content="billingOnly" placement="top">
          <ElSwitch :model-value="productOnSale[p.key]" :disabled="!canBill" :aria-label="`${p.name}开售`" @change="toggleSale(p.key, p.name)" />
        </ElTooltip>
      </div>
    </section>
  </div>

  <section class="table-card" aria-labelledby="package-title">
    <div class="table-toolbar">
      <div>
        <h2 id="package-title" class="card-title">动态流量包</h2>
        <p class="card-desc">改价只影响新订单，已成交订单按下单时锁定的容量和价格。下架后官网和控制台隐藏该档，已到账流量照常使用。</p>
      </div>
    </div>
    <ElTable :data="trafficPackages" row-key="id" class="ip-table">
      <ElTableColumn label="容量" min-width="100">
        <template #default="{ row }: Row"><strong>{{ row.gb }} GB</strong></template>
      </ElTableColumn>
      <ElTableColumn label="售价" min-width="110" align="right">
        <template #default="{ row }: Row"><span class="ledger-amount">{{ formatMoney(row.cents) }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="每 GB 单价" min-width="110" align="right">
        <template #default="{ row }: Row"><span class="ledger-amount">{{ formatMoney(perGb(row)) }}</span></template>
      </ElTableColumn>
      <ElTableColumn label="相比 5 GB 档" min-width="120" align="right">
        <template #default="{ row }: Row">{{ discount(row) > 0 ? `低 ${discount(row)}%` : discount(row) < 0 ? `高 ${-discount(row)}%` : '基准' }}</template>
      </ElTableColumn>
      <ElTableColumn label="状态" min-width="90">
        <template #default="{ row }: Row"><ElTag :type="row.listed ? 'success' : 'info'" size="small" disable-transitions>{{ row.listed ? '上架' : '已下架' }}</ElTag></template>
      </ElTableColumn>
      <ElTableColumn label="" width="140" align="right">
        <template #default="{ row }: Row">
          <ElTooltip :disabled="canBill" :content="billingOnly" placement="left">
            <span class="row-actions">
              <ElButton link type="primary" :disabled="!canBill" @click="edit(row)">调价</ElButton>
              <ElButton link :type="row.listed ? 'danger' : 'primary'" :disabled="!canBill" @click="toggleListed(row)">{{ row.listed ? '下架' : '上架' }}</ElButton>
            </span>
          </ElTooltip>
        </template>
      </ElTableColumn>
    </ElTable>
  </section>

  <section class="table-card" aria-labelledby="audit-title">
    <div class="table-toolbar"><h2 id="audit-title" class="card-title">修改记录</h2></div>
    <ElTable :data="productAudit" class="ip-table">
      <ElTableColumn label="时间" min-width="150">
        <template #default="{ row }: Row">{{ formatExpiry(row.at) }}</template>
      </ElTableColumn>
      <ElTableColumn prop="target" label="修改项" min-width="200" />
      <ElTableColumn label="修改前 → 后" min-width="200">
        <template #default="{ row }: Row"><span class="audit-before">{{ row.before }}</span> → <strong>{{ row.after }}</strong></template>
      </ElTableColumn>
      <ElTableColumn prop="operator" label="操作人" width="100" />
    </ElTable>
  </section>

  <ElDialog :model-value="Boolean(editing)" :title="`调整 ${editing?.gb} GB 流量包售价`" width="min(420px, calc(100vw - 32px))" align-center @update:model-value="editing = undefined">
    <ElForm v-if="editing" label-position="top" @submit.prevent>
      <ElFormItem label="售价（元）">
        <ElInputNumber v-model="priceYuan" :min="0.01" :precision="2" :controls="false" class="amount-input" />
      </ElFormItem>
      <dl class="summary-total compact">
        <div><dt>当前售价</dt><dd>{{ formatMoney(editing.cents) }}</dd></div>
        <div><dt>调整后每 GB</dt><dd>{{ formatMoney(newCents / editing.gb) }}</dd></div>
      </dl>
    </ElForm>
    <template #footer>
      <ElButton @click="editing = undefined">取消</ElButton>
      <ElButton type="primary" :disabled="!newCents" @click="savePrice">保存</ElButton>
    </template>
  </ElDialog>
</template>
