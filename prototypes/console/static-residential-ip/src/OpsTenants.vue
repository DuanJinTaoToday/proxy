<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { ElAlert, ElButton, ElDialog, ElEmpty, ElForm, ElFormItem, ElInput, ElMessage, ElOption, ElSelect, ElTabPane, ElTabs, ElTooltip } from 'element-plus'
import { ArrowLeft, CopyDocument, Search } from '@element-plus/icons-vue'
import OpsOrders from './OpsOrders.vue'
import OpsLedger from './OpsLedger.vue'
import OpsRechargeDialog from './OpsRechargeDialog.vue'
import { formatExpiry, formatMoney } from './demo'
import { billingOnly, canBill, findTenant, operator, tenantBalance, tenants, tenantTrafficGb } from './ops'
import { books } from './state'

const idFromHash = () => location.hash.split('/')[1]
const tenantId = ref(idFromHash())
const onHash = () => { tenantId.value = idFromHash() }
window.addEventListener('hashchange', onHash)
onBeforeUnmount(() => window.removeEventListener('hashchange', onHash))

const tenant = computed(() => tenants.find(t => t.id === tenantId.value))
const query = ref('')
const searched = ref(false)
const result = computed(() => searched.value ? findTenant(query.value) : undefined)
const recent = tenants.slice(0, 3)

function search() {
  searched.value = Boolean(query.value.trim())
}

const tab = ref('orders')
const rechargeOpen = ref(false)
const processing = computed(() => tenant.value ? books.value[tenant.value.id].orders.filter(o => o.status === 'processing').length : 0)
const gb = (value: number) => `${value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} GB`

// 重置登录密码：运营核实租户归属后生成临时密码，只显示一次。
const resetOpen = ref(false)
const verifyBy = ref('')
const verifyNote = ref('')
const tempPassword = ref('')
const verifyOptions = ['注册手机号回拨核实', '对公转账回单核实', '营业执照与联系人核实']

function openReset() {
  verifyBy.value = ''
  verifyNote.value = ''
  tempPassword.value = ''
  resetOpen.value = true
}

function resetPassword() {
  const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'
  tempPassword.value = Array.from(crypto.getRandomValues(new Uint32Array(14)), n => chars[n % chars.length]).join('')
}

async function copy(text: string) {
  await navigator.clipboard.writeText(text).then(() => ElMessage.success('已复制临时密码'), () => ElMessage.error('复制失败，请检查浏览器剪贴板权限'))
}
</script>

<template>
  <template v-if="tenant">
    <a href="#tenants" class="back-link"><ArrowLeft />返回租户查找</a>
    <div class="page-head">
      <div>
        <h1>{{ tenant.name }}</h1>
        <p class="page-sub"><span class="mono">{{ tenant.account }}</span> · {{ tenant.phone }} · 注册于 {{ formatExpiry(tenant.registeredAt) }}</p>
      </div>
      <div class="head-actions">
        <ElButton @click="openReset">重置登录密码</ElButton>
        <ElTooltip :disabled="canBill" :content="billingOnly" placement="bottom-end">
          <span><ElButton type="primary" :disabled="!canBill" @click="rechargeOpen = true">钱包充值</ElButton></span>
        </ElTooltip>
      </div>
    </div>

    <div class="metrics tenant-metrics">
      <section class="metric static">
        <div><h2>钱包可用余额</h2><p class="metric-value">{{ formatMoney(tenantBalance(tenant.id)) }}</p><p class="metric-hint">购买和续费从余额扣款</p></div>
      </section>
      <section class="metric static">
        <div>
          <h2>动态流量余额</h2>
          <p class="metric-value" :class="{ bad: tenantTrafficGb(tenant.id) < 0 }">{{ gb(tenantTrafficGb(tenant.id)) }}</p>
          <p class="metric-hint">{{ tenantTrafficGb(tenant.id) < 0 ? '负值为待抵扣的用量' : '1 GB = 10⁹ 字节' }}</p>
        </div>
      </section>
    </div>

    <ElTabs v-model="tab" class="page-tabs">
      <ElTabPane name="orders">
        <template #label>订单<span v-if="processing" class="tab-count">{{ processing }}</span></template>
        <OpsOrders :tenant-id="tenant.id" />
      </ElTabPane>
      <ElTabPane label="钱包流水" name="ledger">
        <OpsLedger :tenant-id="tenant.id" />
      </ElTabPane>
    </ElTabs>

    <OpsRechargeDialog v-model="rechargeOpen" :tenant="tenant" />

    <ElDialog v-model="resetOpen" title="重置登录密码" width="min(480px, calc(100vw - 32px))" align-center>
      <template v-if="!tempPassword">
        <ElAlert type="warning" :closable="false" show-icon title="先核实对方是租户本人"
          description="忘记密码由运营核实租户归属后协助重置。重置后旧密码立即失效。" class="drawer-alert" />
        <ElForm label-position="top" @submit.prevent>
          <ElFormItem label="核实方式" required>
            <ElSelect v-model="verifyBy" placeholder="选择核实方式">
              <ElOption v-for="o in verifyOptions" :key="o" :value="o" :label="o" />
            </ElSelect>
          </ElFormItem>
          <ElFormItem label="核实记录" required>
            <ElInput v-model="verifyNote" type="textarea" :rows="2" maxlength="200" placeholder="例如：11/20 回拨 138****6021，本人确认" />
          </ElFormItem>
        </ElForm>
      </template>
      <template v-else>
        <p class="form-hint">临时密码只显示这一次，请通过已核实的渠道发给客户，并提醒客户登录后修改。</p>
        <div class="temp-password">
          <span class="mono-value">{{ tempPassword }}</span>
          <ElButton :icon="CopyDocument" @click="copy(tempPassword)">复制</ElButton>
        </div>
        <p class="form-hint">操作人 {{ operator.name }} · 核实方式 {{ verifyBy }}</p>
      </template>
      <template #footer>
        <template v-if="!tempPassword">
          <ElButton @click="resetOpen = false">取消</ElButton>
          <ElButton type="danger" :disabled="!verifyBy || !verifyNote.trim()" @click="resetPassword">确认重置</ElButton>
        </template>
        <ElButton v-else type="primary" @click="resetOpen = false">完成</ElButton>
      </template>
    </ElDialog>
  </template>

  <template v-else>
    <div class="page-head"><h1>租户</h1></div>
    <section class="details-card tenant-search">
      <h2>查找租户</h2>
      <p class="card-desc">输入完整的主账号名或注册手机号，手机号可以不带 +86。</p>
      <form class="tenant-search-row" @submit.prevent="search">
        <ElInput v-model="query" size="large" :prefix-icon="Search" clearable placeholder="例如 chenxl-studio 或 13800136021" @clear="searched = false" />
        <ElButton type="primary" size="large" native-type="submit">查找</ElButton>
      </form>

      <a v-if="result" class="tenant-result" :href="`#tenants/${result.id}`">
        <div>
          <strong>{{ result.name }}</strong>
          <span class="mono">{{ result.account }}</span> · <span>{{ result.phone }}</span>
        </div>
        <span>余额 {{ formatMoney(tenantBalance(result.id)) }}</span>
      </a>
      <ElEmpty v-else-if="searched" description="没有找到租户，请确认主账号名或手机号是否完整" :image-size="64" />

      <div class="tenant-recent">
        <span>演示账号</span>
        <button v-for="t in recent" :key="t.id" type="button" class="chip" @click="query = t.account; search()">{{ t.account }}</button>
      </div>
    </section>
  </template>
</template>
