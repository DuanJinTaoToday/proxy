<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElAlert, ElButton, ElCheckbox, ElDialog, ElForm, ElFormItem, ElInput, ElInputNumber, ElMessage, ElRadio, ElRadioGroup, ElTag } from 'element-plus'
import { formatExpiry, formatMoney } from './demo'
import { canBill, hoursSince, operator, pendingStageLabels, stageOf, tenantById } from './ops'
import { books, orderKindLabels, resolveOrder } from './state'

// 对应 ResolveStaticPurchase / ResolveStaticRenewal：同步已确认的交付，或关闭剩余并退回预扣。
const props = defineProps<{ tenantId?: string, orderId?: string }>()
const open = defineModel<boolean>({ required: true })

const order = computed(() => props.tenantId ? books.value[props.tenantId]?.orders.find(o => o.id === props.orderId) : undefined)
const tenant = computed(() => props.tenantId ? tenantById(props.tenantId) : undefined)
const renewal = computed(() => order.value?.kind === 'static-renewal')
const stage = computed(() => order.value ? stageOf(order.value) : 'waiting')

const delivered = ref<number[]>([])
const mode = ref<'sync' | 'close'>('sync')
const confirmedFailure = ref(false)
const supplierRef = ref('')
const evidence = ref('')

watch(open, value => {
  if (!value || !order.value) return
  delivered.value = order.value.lines.map(l => l.delivered)
  mode.value = 'sync'
  confirmedFailure.value = false
  supplierRef.value = order.value.resolutions?.at(-1)?.supplierRef ?? ''
  evidence.value = ''
})

const refundCents = computed(() => order.value?.lines.reduce((s, l, i) => s + (l.quantity - (delivered.value[i] ?? 0)) * l.unitCents, 0) ?? 0)
const allDelivered = computed(() => order.value?.lines.every((l, i) => delivered.value[i] === l.quantity) ?? false)
const closeBlocked = computed(() => mode.value === 'close' && stage.value !== 'closable' && !confirmedFailure.value)
const changed = computed(() => order.value?.lines.some((l, i) => delivered.value[i] !== l.delivered) ?? false)
const error = computed(() => {
  if (!canBill.value) return '需要计费管理权限才能处理订单'
  if (!evidence.value.trim()) return '请填写核实依据'
  if (closeBlocked.value) return '未满 72 小时，只有确认无法交付时才能关闭'
  if (mode.value === 'sync' && !changed.value) return '交付数量没有变化'
  return ''
})

function submit() {
  if (!order.value || !props.tenantId || error.value) return
  const closing = mode.value === 'close' || allDelivered.value
  resolveOrder(props.tenantId, order.value.id, delivered.value, { operator: operator.value.name, evidence: evidence.value.trim(), supplierRef: supplierRef.value.trim() || undefined, closed: closing })
  open.value = false
  ElMessage.success(closing ? (refundCents.value ? `订单已关闭，已退回 ${formatMoney(refundCents.value)}` : '订单已完成') : '已同步交付结果，订单仍在确认中')
}
</script>

<template>
  <ElDialog v-model="open" title="核实静态订单" width="min(620px, calc(100vw - 32px))" align-center>
    <template v-if="order && tenant">
      <dl class="summary-total compact resolve-head">
        <div><dt>订单</dt><dd><span class="mono">{{ order.id }}</span> · {{ orderKindLabels[order.kind] }}</dd></div>
        <div><dt>租户</dt><dd>{{ tenant.name }} · {{ tenant.account }}</dd></div>
        <div><dt>下单时间</dt><dd>{{ formatExpiry(order.createdAt) }} · 已等待 {{ hoursSince(order.createdAt) }} 小时</dd></div>
        <div><dt>阶段</dt><dd><ElTag size="small" :type="stage === 'closable' ? 'danger' : stage === 'review' ? 'warning' : 'info'" disable-transitions>{{ pendingStageLabels[stage] }}</ElTag></dd></div>
      </dl>

      <ElForm label-position="top" class="resolve-form" @submit.prevent>
        <ElFormItem label="皇冠主订单号">
          <ElInput v-model="supplierRef" placeholder="留空沿用平台已有编号" maxlength="128" />
        </ElFormItem>

        <ElFormItem :label="renewal ? '已确认续费成功' : '已确认交付数量'">
          <table class="resolve-lines">
            <tbody>
              <tr v-for="(line, i) in order.lines" :key="i">
                <td>{{ line.label }}<span class="remaining">单价 {{ formatMoney(line.unitCents) }}</span></td>
                <td class="resolve-count">
                  <ElInputNumber v-model="delivered[i]" :min="line.delivered" :max="line.quantity" size="small" controls-position="right" />
                  <span>/ {{ line.quantity }}</span>
                </td>
              </tr>
            </tbody>
          </table>
          <p class="form-hint">只能增加，不能减少已确认的数量。</p>
        </ElFormItem>

        <ElFormItem label="处理方式">
          <ElRadioGroup v-model="mode" class="resolve-mode">
            <ElRadio value="sync">只同步交付结果，订单继续确认中</ElRadio>
            <ElRadio value="close">关闭剩余，退回未交付部分</ElRadio>
          </ElRadioGroup>
        </ElFormItem>
        <ElAlert v-if="mode === 'close' && stage !== 'closable'" type="warning" :closable="false" show-icon class="resolve-alert"
          title="下单未满 72 小时" description="只有拿到皇冠明确的失败结果时才能提前关闭。">
          <ElCheckbox v-model="confirmedFailure">已确认剩余部分无法交付</ElCheckbox>
        </ElAlert>

        <ElFormItem label="核实依据" required>
          <ElInput v-model="evidence" type="textarea" :rows="3" maxlength="500" show-word-limit
            placeholder="例如：皇冠后台查单结果、客服工单号、截图存档位置" />
        </ElFormItem>
      </ElForm>

      <dl v-if="mode === 'close' || allDelivered" class="summary-total compact">
        <div class="grand"><dt>退回钱包</dt><dd>{{ formatMoney(refundCents) }}</dd></div>
      </dl>
    </template>
    <template #footer>
      <span v-if="error" class="dialog-error">{{ error }}</span>
      <ElButton @click="open = false">取消</ElButton>
      <ElButton type="primary" :disabled="Boolean(error)" @click="submit">{{ mode === 'close' ? '确认关闭' : '保存' }}</ElButton>
    </template>
  </ElDialog>
</template>
