<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElAlert, ElButton, ElDialog, ElEmpty, ElMessage } from 'element-plus'
import { CircleCheck } from '@element-plus/icons-vue'
import { formatMoney } from './demo'
import { balanceCents, basePerGbCents, productOnSale, recordTrafficPurchase, trafficBalanceGb, trafficPackages, type TrafficPackage } from './state'

const emit = defineEmits<{ purchased: [] }>()
const listed = computed(() => trafficPackages.value.filter(pkg => pkg.listed))
const selectedId = ref(listed.value[1]?.id ?? listed.value[0]?.id)
const selected = computed(() => listed.value.find(pkg => pkg.id === selectedId.value) ?? listed.value[0])
const confirmOpen = ref(false)

const perGb = (pkg: TrafficPackage) => pkg.cents / pkg.gb
const discount = (pkg: TrafficPackage) => Math.round((1 - perGb(pkg) / basePerGbCents.value) * 100)
const shortfall = computed(() => selected.value ? Math.max(0, selected.value.cents - balanceCents.value) : 0)
const gb = (value: number) => `${value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} GB`
const afterGb = computed(() => trafficBalanceGb.value + (selected.value?.gb ?? 0))

function submit() {
  const pkg = selected.value!
  recordTrafficPurchase(pkg)
  confirmOpen.value = false
  ElMessage.success(`已到账 ${pkg.gb} GB 流量`)
  emit('purchased')
}
</script>

<template>
  <section v-if="!productOnSale.dynamic || !selected" class="details-card">
    <ElEmpty :description="productOnSale.dynamic ? '流量包暂时全部下架，请稍后再来' : '动态住宅代理-GB 暂未开放购买'" :image-size="80" />
  </section>
  <div v-else class="traffic-pricing">
    <div class="package-grid" role="radiogroup" aria-label="选择流量包">
      <button v-for="pkg in listed" :key="pkg.id" type="button" role="radio" class="package-card"
        :aria-checked="selected.id === pkg.id" @click="selectedId = pkg.id">
        <span v-if="discount(pkg)" class="package-badge">每 GB 省 {{ discount(pkg) }}%</span>
        <span class="package-size">{{ pkg.gb }} <small>GB</small></span>
        <span class="package-price">{{ formatMoney(pkg.cents) }}</span>
        <span class="package-unit">{{ formatMoney(perGb(pkg)) }} / GB</span>
        <ul class="package-points">
          <li><CircleCheck />流量长期有效，用完为止</li>
          <li><CircleCheck />所有子账号共用</li>
          <li><CircleCheck />SOCKS5 接入，按实际字节扣减</li>
        </ul>
      </button>
    </div>

    <section class="details-card package-checkout">
      <dl class="summary-total">
        <div><dt>已选套餐</dt><dd>{{ selected.gb }} GB 流量包</dd></div>
        <div><dt>到账后可用流量</dt><dd>{{ gb(trafficBalanceGb) }} → {{ gb(afterGb) }}</dd></div>
        <div><dt>钱包余额</dt><dd>{{ formatMoney(balanceCents) }}</dd></div>
        <div class="grand"><dt>应付</dt><dd>{{ formatMoney(selected.cents) }}</dd></div>
      </dl>
      <ElAlert v-if="shortfall" type="warning" :closable="false" show-icon
        :title="`余额不足，还差 ${formatMoney(shortfall)}`" description="请联系运营为钱包充值后再购买。" />
      <ElButton type="primary" size="large" class="summary-submit" :disabled="shortfall > 0" @click="confirmOpen = true">立即购买</ElButton>
      <p class="form-hint">一笔订单购买一份套餐，提交成功后按锁定的容量和价格成交。1 GB = 10⁹ 字节。</p>
    </section>

    <ElDialog v-model="confirmOpen" title="确认购买流量" width="min(480px, calc(100vw - 32px))" align-center>
      <dl class="summary-total compact">
        <div><dt>套餐</dt><dd>动态住宅代理-GB · {{ selected.gb }} GB</dd></div>
        <div><dt>有效期</dt><dd>长期有效</dd></div>
        <div><dt>支付后余额</dt><dd>{{ formatMoney(balanceCents - selected.cents) }}</dd></div>
        <div class="grand"><dt>应付</dt><dd>{{ formatMoney(selected.cents) }}</dd></div>
      </dl>
      <p v-if="trafficBalanceGb < 0" class="form-hint">当前欠 {{ gb(-trafficBalanceGb) }}，到账后先抵扣欠量。</p>
      <template #footer>
        <ElButton @click="confirmOpen = false">返回修改</ElButton>
        <ElButton type="primary" @click="submit">确认支付</ElButton>
      </template>
    </ElDialog>
  </div>
</template>
