<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElButton, ElDialog, ElRadioButton, ElRadioGroup } from 'element-plus'
import { CircleCheckFilled } from '@element-plus/icons-vue'
import { dayMs, endpoint, formatExpiry, formatMoney, priceCents, productLabels, termsOf, type StaticIp } from './demo'
import { recordStaticRenewal } from './state'

const props = defineProps<{ ips: readonly StaticIp[] }>()
const emit = defineEmits<{ renewed: [] }>()
const open = defineModel<boolean>({ required: true })

const terms = computed(() => {
  const [first, ...rest] = props.ips
  if (!first) return []
  return termsOf(first.prices).filter(t => rest.every(ip => priceCents(ip.prices, t.code) !== undefined) && t.days >= 30)
})
const termCode = ref('yp1')
const term = computed(() => terms.value.find(t => t.code === termCode.value) ?? terms.value[0])
const done = ref(false)
const unit = (ip: StaticIp) => priceCents(ip.prices, term.value?.code ?? '') ?? 0
const cost = computed(() => props.ips.reduce((total, ip) => total + unit(ip), 0))

watch(open, (visible) => {
  if (!visible) return
  termCode.value = 'yp1'
  done.value = false
})

function confirm() {
  recordStaticRenewal(props.ips, term.value, unit)
  emit('renewed')
  done.value = true
}
</script>

<template>
  <ElDialog v-model="open" :title="done ? '续费结果' : '续费静态 IP'" width="min(600px, calc(100vw - 32px))" align-center>
    <template v-if="!done">
      <p class="dialog-intro">续费保留原 IP，租期从原到期时间顺延。提交时按现价逐个锁定。</p>
      <div class="renewal-terms">
        <span id="term-label">续费租期</span>
        <ElRadioGroup v-model="termCode" aria-labelledby="term-label">
          <ElRadioButton v-for="t in terms" :key="t.code" :value="t.code">{{ t.name }}</ElRadioButton>
        </ElRadioGroup>
      </div>
    </template>
    <p v-else class="dialog-intro">{{ ips.length }} 个 IP 续费成功，已从钱包扣款 {{ formatMoney(cost) }}。</p>

    <div v-if="term" class="quote">
      <div class="quote-heading">
        <span>续费 IP · {{ ips.length }} 个</span><span>{{ done ? '结果' : '价格' }}</span><span>续费后到期时间</span>
      </div>
      <div v-for="ip in ips" :key="ip.address" class="quote-item">
        <span><strong class="endpoint">{{ endpoint(ip) }}</strong><span class="product-tag" :class="ip.product">{{ productLabels[ip.product] }}</span></span>
        <span v-if="done" class="result-ok"><CircleCheckFilled />成功</span>
        <span v-else>{{ formatMoney(unit(ip)) }}</span>
        <span>{{ formatExpiry(ip.expiry + term.days * dayMs) }}</span>
      </div>
    </div>
    <div v-if="!done" class="quote-total"><span>预计支付</span><strong>{{ formatMoney(cost) }}</strong></div>

    <template #footer>
      <ElButton v-if="done" type="primary" @click="open = false">完成</ElButton>
      <template v-else>
        <ElButton @click="open = false">取消</ElButton>
        <ElButton type="primary" :disabled="!ips.length || !term" @click="confirm">确认续费</ElButton>
      </template>
    </template>
  </ElDialog>
</template>
