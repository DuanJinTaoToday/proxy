<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { ApiError } from './api/control'
import { canManageBilling, findWalletTenant, getWallet, quoteRecharge, rechargeWallet } from './api/wallet'
import type { Recharge, RechargeQuote, WalletTenant } from './api/wallet'
import WalletRecords from './WalletRecords.vue'
import OperatorProducts from './OperatorProducts.vue'
import { dateTime, money } from './wallet-format'

const props = defineProps<{ token: string; operatorName: string }>()
const emit = defineEmits<{ expired: [] }>()
const pendingKey = 'proxy-console.pending-recharge.' + props.operatorName
type Confirmation = RechargeQuote & Recharge
const restored = readPending()
const tenant = ref<WalletTenant | null>(restored?.tenant ?? null)
const query = ref('')
const searching = ref(false)
const searchError = ref('')
const permitted = ref(false)
const amount = ref('')
const reason = ref('')
const confirmation = ref<Confirmation | null>(restored)
const quoting = ref(false)
const submitting = ref(false)
const needsRefresh = ref(false)
const uncertainMessage = '充值结果待确认。请重试本次请求，避免创建一笔新的充值。'
const rechargeError = ref(restored ? uncertainMessage : '')
const success = ref('')
const revision = ref(0)

let searchController: AbortController | undefined
onUnmounted(() => searchController?.abort())

function readPending(): Confirmation | null {
  try {
    return JSON.parse(sessionStorage.getItem(pendingKey) ?? 'null')
  } catch {
    return null
  }
}

watch(() => props.token, async (token, _, onCleanup) => {
  const abort = new AbortController()
  onCleanup(() => abort.abort())
  try {
    await canManageBilling(token, abort.signal)
    if (!abort.signal.aborted) permitted.value = true
  } catch (error) {
    if (!abort.signal.aborted && error instanceof ApiError && error.status === 401) emit('expired')
  }
}, { immediate: true })

async function findTenant() {
  if (!query.value.trim() || searching.value || confirmation.value || quoting.value || submitting.value) return
  const abort = new AbortController()
  searchController = abort
  searching.value = true
  searchError.value = ''
  tenant.value = null
  success.value = ''
  rechargeError.value = ''
  needsRefresh.value = false
  amount.value = ''
  reason.value = ''
  try {
    const found = await findWalletTenant(props.token, query.value.trim(), abort.signal)
    if (!abort.signal.aborted) tenant.value = found
  } catch (error) {
    if (abort.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else if (error instanceof ApiError && error.status === 403) searchError.value = '当前账号没有查看租户钱包的权限。'
    else if (error instanceof ApiError && error.status === 404) searchError.value = '未找到租户，请核对主账号名或手机号。'
    else searchError.value = '查询失败，请重试。'
  } finally {
    if (!abort.signal.aborted) searching.value = false
  }
}

async function prepareRecharge() {
  const selected = tenant.value
  if (!selected || confirmation.value || needsRefresh.value || quoting.value) return
  success.value = ''
  rechargeError.value = ''
  const value = amount.value.trim()
  if (!/^\d{1,17}(\.\d{1,2})?$/.test(value) || !reason.value.trim()) {
    rechargeError.value = '请输入金额（最多两位小数）和充值原因。'
    return
  }
  const [yuan = '0', fraction = ''] = value.split('.')
  const cents = BigInt(yuan) * 100n + BigInt(fraction.padEnd(2, '0'))
  if (cents <= 0n || cents > 9_223_372_036_854_775_807n) {
    rechargeError.value = '充值金额超出可用范围。'
    return
  }
  const abort = new AbortController()
  searchController = abort
  const requestKey = crypto.randomUUID()
  quoting.value = true
  try {
    const quote = await quoteRecharge(props.token, selected.tenant_id, {
      request_key: requestKey, amount_minor: cents.toString(), reason: reason.value.trim(),
    }, abort.signal)
    if (!abort.signal.aborted) {
      confirmation.value = { ...quote, request_key: requestKey }
      tenant.value = quote.tenant
    }
  } catch (error) {
    if (abort.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else if (error instanceof ApiError && error.reason === 'WALLET_BALANCE_LIMIT') rechargeError.value = '预计到账后余额超出可用范围。'
    else rechargeError.value = '获取确认信息失败，请检查金额和操作权限后重试。'
  } finally {
    if (!abort.signal.aborted) quoting.value = false
  }
}

async function refreshTenant() {
  const selected = tenant.value
  if (!selected || searching.value) return
  const abort = new AbortController()
  searchController = abort
  searching.value = true
  rechargeError.value = ''
  try {
    const updated = await getWallet(props.token, abort.signal, selected.tenant_id)
    if (abort.signal.aborted) return
    tenant.value = updated
    revision.value++
    needsRefresh.value = false
  } catch (error) {
    if (abort.signal.aborted) return
    if (error instanceof ApiError && error.status === 401) emit('expired')
    else rechargeError.value = '余额刷新失败，请重试。'
  } finally {
    if (!abort.signal.aborted) searching.value = false
  }
}

function cancelConfirmation() {
  confirmation.value = null
  rechargeError.value = ''
}

async function submitRecharge() {
  const selected = tenant.value
  const request = confirmation.value
  if (!selected || !request || submitting.value) return
  submitting.value = true
  rechargeError.value = ''
  try {
    sessionStorage.setItem(pendingKey, JSON.stringify(request))
  } catch {
    // The current page can still retry the same request if browser storage is full.
  }
  try {
    const entry = await rechargeWallet(props.token, selected.tenant_id, {
      request_key: request.request_key, confirmation_token: request.confirmation_token,
    })
    sessionStorage.removeItem(pendingKey)
    confirmation.value = null
    success.value = `充值已到账。流水 #${entry.id}，到账时间 ${dateTime(entry.created_at)}。`
    tenant.value = { ...selected, available_minor: entry.after_minor ?? '0' }
    amount.value = ''
    reason.value = ''
    needsRefresh.value = true
    await refreshTenant()
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      emit('expired')
    } else if (error instanceof ApiError && [400, 404, 409].includes(error.status)) {
      sessionStorage.removeItem(pendingKey)
      confirmation.value = null
      if (error.reason === 'WALLET_BALANCE_CHANGED' || error.reason === 'WALLET_QUOTE_EXPIRED') {
        needsRefresh.value = true
        rechargeError.value = '余额已变动或确认信息已过期。请刷新余额，再核对并提交。'
      } else rechargeError.value = '充值未提交，请核对金额、原因和操作权限。'
    } else {
      rechargeError.value = uncertainMessage
    }
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="page-heading"><h1>租户钱包</h1></div>
  <form class="panel filters wallet-filters" @submit.prevent="findTenant">
    <label>主账号名或手机号
      <input v-model="query" required maxlength="128" placeholder="输入完整主账号名或手机号">
    </label>
    <button type="submit" :disabled="searching || !!confirmation || quoting || submitting">{{ searching ? '正在查询…' : '查找租户' }}</button>
    <p v-if="searchError" class="error" role="alert">{{ searchError }}</p>
  </form>

  <OperatorProducts v-if="permitted" :token="token" @expired="emit('expired')" />

  <template v-if="tenant">
    <section class="panel wallet-summary" aria-labelledby="tenant-heading">
      <div class="section-head"><h2 id="tenant-heading">{{ tenant.name }}</h2></div>
      <dl class="record-details">
        <div><dt>主账号名</dt><dd>{{ tenant.primary_account_name }}</dd></div>
        <div><dt>当前钱包余额</dt><dd>{{ money(tenant.available_minor) }}</dd></div>
      </dl>
    </section>

    <section v-if="permitted" class="panel" aria-labelledby="recharge-heading">
      <div class="section-head"><h2 id="recharge-heading">钱包充值</h2></div>
      <form v-if="!confirmation" class="filters wallet-filters" @submit.prevent="prepareRecharge">
        <label>充值金额（人民币元）
          <input v-model="amount" inputmode="decimal" maxlength="20" placeholder="0.00" required>
        </label>
        <label class="recharge-reason">充值原因
          <input v-model="reason" maxlength="500" required>
        </label>
        <button v-if="needsRefresh" type="button" class="secondary" :disabled="searching || submitting" @click="refreshTenant">刷新余额</button>
        <button v-else type="submit" :disabled="quoting || submitting">{{ quoting ? '正在获取确认信息…' : '核对充值' }}</button>
      </form>
      <form v-else class="recharge-confirm" @submit.prevent="submitRecharge">
        <h3>确认充值信息</h3>
        <dl class="record-details">
          <div><dt>租户名</dt><dd>{{ tenant.name }}</dd></div>
          <div><dt>主账号名</dt><dd>{{ tenant.primary_account_name }}</dd></div>
          <div><dt>充值金额</dt><dd>{{ money(confirmation.amount_minor) }}</dd></div>
          <div><dt>预计到账后余额</dt><dd>{{ money(confirmation.expected_after_minor) }}</dd></div>
          <div><dt>充值原因</dt><dd>{{ confirmation.reason }}</dd></div>
        </dl>
        <div class="confirm-actions">
          <button type="button" class="secondary" :disabled="submitting || !!rechargeError" @click="cancelConfirmation">返回修改</button>
          <button type="submit" :disabled="submitting">{{ submitting ? '正在提交…' : rechargeError ? '重试本次充值' : '确认充值' }}</button>
        </div>
      </form>
      <p v-if="rechargeError" class="form-notice error" role="alert">{{ rechargeError }}</p>
      <p v-if="success" class="form-notice" role="status">{{ success }}</p>
    </section>
    <WalletRecords :key="tenant.tenant_id" :token="token" :tenant-id="tenant.tenant_id" :revision="revision" @expired="emit('expired')" />
  </template>
</template>
