<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElButton, ElDialog, ElForm, ElFormItem, ElInput, ElInputNumber, ElResult } from 'element-plus'
import { formatExpiry, formatMoney } from './demo'
import { canBill, operator, tenantBalance, type Tenant } from './ops'
import { rechargeWallet, type LedgerEntry } from './state'

// 对应 QuoteWalletRecharge → RechargeWallet：先填写，再核对租户和到账后余额，确认后入账。
const props = defineProps<{ tenant: Tenant }>()
const open = defineModel<boolean>({ required: true })

const step = ref<'form' | 'confirm' | 'done'>('form')
const amount = ref<number>()
const reason = ref('')
const entry = ref<LedgerEntry>()

watch(open, value => {
  if (!value) return
  step.value = 'form'
  amount.value = undefined
  reason.value = ''
})

const cents = computed(() => Math.round((amount.value ?? 0) * 100))
const balance = computed(() => tenantBalance(props.tenant.id))
const formError = computed(() => !cents.value ? '请输入充值金额' : !reason.value.trim() ? '请填写充值原因' : '')

function submit() {
  entry.value = rechargeWallet(props.tenant.id, cents.value, reason.value.trim(), operator.value.name)
  step.value = 'done'
}
</script>

<template>
  <ElDialog v-model="open" :title="step === 'confirm' ? '核对充值信息' : '钱包充值'" width="min(480px, calc(100vw - 32px))" align-center>
    <ElForm v-if="step === 'form'" label-position="top" @submit.prevent>
      <ElFormItem label="租户"><span>{{ tenant.name }} · <span class="mono">{{ tenant.account }}</span></span></ElFormItem>
      <ElFormItem label="充值金额（元）" required>
        <ElInputNumber v-model="amount" :min="0.01" :max="100000" :precision="2" :controls="false" class="amount-input" placeholder="0.00" />
      </ElFormItem>
      <ElFormItem label="充值原因" required>
        <ElInput v-model="reason" type="textarea" :rows="2" maxlength="200" show-word-limit placeholder="例如：对公转账到账，回单尾号 3381" />
      </ElFormItem>
      <p class="form-hint">充值原因和操作人只在运营后台可见，客户流水显示为「运营充值」。</p>
    </ElForm>

    <template v-else-if="step === 'confirm'">
      <dl class="summary-total">
        <div><dt>租户名</dt><dd>{{ tenant.name }}</dd></div>
        <div><dt>主账号名</dt><dd class="mono">{{ tenant.account }}</dd></div>
        <div><dt>充值原因</dt><dd>{{ reason }}</dd></div>
        <div><dt>当前余额</dt><dd>{{ formatMoney(balance) }}</dd></div>
        <div><dt>到账后余额</dt><dd>{{ formatMoney(balance + cents) }}</dd></div>
        <div class="grand"><dt>充值金额</dt><dd>{{ formatMoney(cents) }}</dd></div>
      </dl>
      <p class="form-hint">确认后立即入账，无法撤销。操作人：{{ operator.name }}。</p>
    </template>

    <ElResult v-else icon="success" title="充值已到账" :sub-title="`${tenant.name} 当前余额 ${formatMoney(balance)}`">
      <template #extra>
        <dl v-if="entry" class="summary-total compact">
          <div><dt>流水号</dt><dd class="mono">{{ entry.id }}</dd></div>
          <div><dt>到账时间</dt><dd>{{ formatExpiry(entry.at) }}</dd></div>
          <div><dt>变动</dt><dd>{{ formatMoney(entry.balanceAfterCents - entry.amountCents) }} → {{ formatMoney(entry.balanceAfterCents) }}</dd></div>
        </dl>
      </template>
    </ElResult>

    <template #footer>
      <template v-if="step === 'form'">
        <span v-if="formError && amount !== undefined" class="dialog-error">{{ formError }}</span>
        <ElButton @click="open = false">取消</ElButton>
        <ElButton type="primary" :disabled="Boolean(formError) || !canBill" @click="step = 'confirm'">下一步</ElButton>
      </template>
      <template v-else-if="step === 'confirm'">
        <ElButton @click="step = 'form'">返回修改</ElButton>
        <ElButton type="primary" @click="submit">确认充值 {{ formatMoney(cents) }}</ElButton>
      </template>
      <ElButton v-else type="primary" @click="open = false">完成</ElButton>
    </template>
  </ElDialog>
</template>
