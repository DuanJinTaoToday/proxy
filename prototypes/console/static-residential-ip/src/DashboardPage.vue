<script setup lang="ts">
import { computed, ref, type Component } from 'vue'
import { ElButton, ElPopover } from 'element-plus'
import { ArrowRight, Bottom, CircleCheck, Clock, Connection, Loading, Service, Sort, Wallet, WarningFilled } from '@element-plus/icons-vue'
import UsageChart from './UsageChart.vue'
import { expiringWindowDays, formatExpiry, formatMoney, priceCents, statusOf } from './demo'
import { balanceCents, currentIp, dailyUsage, ips, ledger, orders, trafficBalanceGb, trafficThresholdGb, usageUpdatedAt } from './state'

interface Todo { id: string, level: 'critical' | 'warning' | 'info', icon: Component, title: string, detail: string, action: string, to: string }

const showTable = ref(false)
const usable = computed(() => ips.value.filter(ip => statusOf(ip) !== 'expired'))
const expiring = computed(() => ips.value.filter(ip => statusOf(ip) === 'expiring'))
const nextExpiry = computed(() => usable.value.reduce<number | undefined>((min, ip) => min === undefined || ip.expiry < min ? ip.expiry : min, undefined))
const processing = computed(() => orders.value.filter(o => o.status === 'processing'))
const renewalCost = computed(() => expiring.value.reduce((s, ip) => s + (priceCents(ip.prices, 'yp1') ?? 0), 0))
const totalRecharge = computed(() => ledger.value.filter(e => e.kind === 'recharge').reduce((s, e) => s + e.amountCents, 0))
const weekTotal = computed(() => dailyUsage.reduce((s, d) => s + d.gb, 0))
const trafficLow = computed(() => trafficBalanceGb.value <= trafficThresholdGb.value)
const range = computed(() => `${dailyUsage[0].day.replaceAll('-', '/')} – ${dailyUsage.at(-1)!.day.slice(5).replace('-', '/')}`)
const gb = (value: number) => `${value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} GB`

const todos = computed<Todo[]>(() => ([
  currentIp.mainland && { id: 'ip', level: 'critical', icon: WarningFilled, title: '当前网络无法使用', detail: `你的 IP 归属地为${currentIp.location}，所有代理产品均不支持在中国大陆网络环境下使用。`, action: '查看当前 IP', to: '#account/profile' },
  trafficLow.value && {
    id: 'traffic', level: trafficBalanceGb.value < 0 ? 'critical' : 'warning', icon: Sort,
    title: trafficBalanceGb.value < 0 ? '动态流量已欠量' : '动态流量偏低',
    detail: trafficBalanceGb.value < 0
      ? `已欠 ${gb(-trafficBalanceGb.value)}，新连接已暂停，购买后先抵扣欠量。`
      : `剩余 ${gb(trafficBalanceGb.value)}，低于提醒阈值 ${trafficThresholdGb.value} GB。`,
    action: '购买流量', to: '#dynamic/pricing',
  },
  expiring.value.length && { id: 'expiring', level: 'warning', icon: Clock, title: `${expiring.value.length} 个静态 IP 即将到期`, detail: `${expiringWindowDays} 天内到期，到期后需重新购买。`, action: '去续费', to: '#static' },
  expiring.value.length && balanceCents.value < renewalCost.value && { id: 'wallet', level: 'warning', icon: Wallet, title: '余额不足以续费', detail: `按月卡续费即将到期的 IP 还差 ${formatMoney(renewalCost.value - balanceCents.value)}。`, action: '查看钱包', to: '#wallet' },
  processing.value.length && { id: 'processing', level: 'info', icon: Loading, title: `${processing.value.length} 笔订单确认中`, detail: '供应商正在确认结果，金额已预扣。', action: '查看订单', to: '#account/orders' },
] as (Todo | false | 0)[]).filter((t): t is Todo => Boolean(t)))
</script>

<template>
  <div class="page-head"><h1>仪表板</h1></div>

  <div class="stat-grid">
    <section class="stat-card">
      <header><span>钱包余额</span><span class="stat-badge"><Wallet />人民币</span></header>
      <p class="stat-value">{{ formatMoney(balanceCents) }}</p>
      <footer>
        <strong>可用于购买和续费</strong>
        <span>累计充值 {{ formatMoney(totalRecharge) }}</span>
        <div class="stat-actions">
          <ElPopover placement="bottom-start" trigger="click" :width="280">
            <template #reference><ElButton type="primary" size="small" :icon="Service">联系运营充值</ElButton></template>
            <strong>如何充值</strong>
            <p class="notification-copy">首版暂不支持在线支付。请联系客户经理转账，运营确认到账后为钱包充值。</p>
          </ElPopover>
          <ElButton size="small" tag="a" href="#wallet">查看流水</ElButton>
        </div>
      </footer>
    </section>

    <section class="stat-card">
      <header>
        <span>动态住宅代理-GB · 可用流量</span>
        <span v-if="trafficLow" class="stat-badge warn"><Bottom />低于 {{ trafficThresholdGb }} GB</span>
        <span v-else class="stat-badge"><CircleCheck />充足</span>
      </header>
      <p class="stat-value" :class="{ bad: trafficBalanceGb < 0 }">{{ gb(trafficBalanceGb) }}</p>
      <footer>
        <strong>近 7 天消耗 {{ gb(weekTotal) }}</strong>
        <span>按 GB 购买，流量长期有效</span>
        <div class="stat-actions">
          <ElButton size="small" tag="a" href="#dynamic/pricing">购买流量<ArrowRight class="btn-arrow" /></ElButton>
        </div>
      </footer>
    </section>

    <section class="stat-card">
      <header>
        <span>静态住宅代理 · 可用 IP</span>
        <span v-if="expiring.length" class="stat-badge warn"><Clock />{{ expiring.length }} 个即将到期</span>
        <span v-else class="stat-badge"><Connection />正常</span>
      </header>
      <p class="stat-value">{{ usable.length }} <small>个</small></p>
      <footer>
        <strong v-if="nextExpiry">最近到期 {{ formatExpiry(nextExpiry) }}</strong>
        <strong v-else>还没有静态 IP</strong>
        <span>按 IP 数量和租期购买</span>
        <div class="stat-actions">
          <ElButton size="small" tag="a" href="#static/pricing">购买静态 IP<ArrowRight class="btn-arrow" /></ElButton>
        </div>
      </footer>
    </section>
  </div>

  <div class="dash-main">
    <section class="details-card usage-card" aria-labelledby="usage-title">
      <div class="usage-head">
        <div>
          <h2 id="usage-title">动态代理近 7 天消耗</h2>
          <p class="usage-desc">7 天合计扣减 <strong>{{ gb(weekTotal) }}</strong> · 数据更新于 {{ formatExpiry(usageUpdatedAt) }}</p>
        </div>
        <span class="usage-range">{{ range }}</span>
      </div>
      <UsageChart v-if="!showTable" :data="dailyUsage" />
      <table v-else class="usage-table">
        <thead><tr><th scope="col">业务日</th><th scope="col">扣减流量</th></tr></thead>
        <tbody><tr v-for="d in dailyUsage" :key="d.day"><td>{{ d.day.replaceAll('-', '/') }}</td><td>{{ gb(d.gb) }}</td></tr></tbody>
      </table>
      <div class="usage-foot">
        <span>业务日按北京时间 · 1 GB = 10⁹ 字节 · <ElButton link type="primary" size="small" @click="showTable = !showTable">{{ showTable ? '查看图表' : '查看数据表' }}</ElButton></span>
        <a href="#dynamic/usage" class="reminder-link">查看数据统计<ArrowRight /></a>
      </div>
    </section>

    <section class="details-card todo-card" aria-labelledby="todo-title">
      <div class="todo-head">
        <h2 id="todo-title">待处理</h2>
        <span v-if="todos.length" class="tab-count">{{ todos.length }}</span>
      </div>
      <ul v-if="todos.length" class="todo-list">
        <li v-for="t in todos" :key="t.id" class="todo-item" :class="t.level">
          <span class="todo-icon"><component :is="t.icon" /></span>
          <div class="todo-text">
            <strong>{{ t.title }}</strong>
            <p>{{ t.detail }}</p>
            <a :href="t.to" class="reminder-link">{{ t.action }}<ArrowRight /></a>
          </div>
        </li>
      </ul>
      <div v-else class="todo-empty"><CircleCheck /><span>暂无待处理事项</span></div>
    </section>
  </div>
</template>
