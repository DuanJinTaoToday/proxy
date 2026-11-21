<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { ElAlert, ElButton, ElCollapse, ElCollapseItem, ElInput, ElInputNumber, ElMessage, ElOption, ElRadioButton, ElRadioGroup, ElSelect } from 'element-plus'
import { CopyDocument, Hide, View } from '@element-plus/icons-vue'
import { gatewayHost, gatewayPort, socksUsername, subAccounts, trafficBalanceGb, trafficThresholdGb } from './state'

const sidPattern = /^[a-z0-9]{1,32}$/
const enabled = computed(() => subAccounts.value.filter(a => a.active))
const accountName = ref(enabled.value[0]?.name ?? '')
const account = computed(() => enabled.value.find(a => a.name === accountName.value))
const mode = ref<'default' | 'sticky'>('default')
const sid = ref('')
const sidtime = ref('')
const reveal = ref(false)
const threshold = ref(trafficThresholdGb.value)

watch(mode, value => { if (value === 'sticky' && !sid.value) sid.value = Math.random().toString(36).slice(2, 10) })

const sidError = computed(() => mode.value === 'sticky' && !sidPattern.test(sid.value) ? '会话标识为 1–32 位小写字母或数字' : '')
const username = computed(() => {
  if (!account.value) return ''
  let name = socksUsername(account.value.name)
  if (mode.value === 'sticky' && !sidError.value) {
    name += `-sid-${sid.value}`
    if (sidtime.value.trim()) name += `-sidtime-${sidtime.value.trim()}`
  }
  return name
})
const password = computed(() => account.value?.password ?? '')
const connection = computed(() => `socks5h://${username.value}:${password.value}@${gatewayHost}:${gatewayPort}`)
const curl = computed(() => `curl -x "${connection.value}" https://api.ipify.org`)
const fields = computed(() => [
  { label: '网关地址', value: gatewayHost },
  { label: '端口', value: String(gatewayPort) },
  { label: '用户名', value: username.value },
  { label: '密码', value: password.value, secret: true },
])

async function copy(text: string, label: string) {
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success(`已复制${label}`)
  } catch {
    ElMessage.error('复制失败，请检查浏览器剪贴板权限')
  }
}

function saveThreshold() {
  trafficThresholdGb.value = threshold.value
  ElMessage.success(`提醒阈值已设为 ${threshold.value} GB`)
}

const gb = (value: number) => `${value.toLocaleString('zh-CN', { minimumFractionDigits: 2, maximumFractionDigits: 2 })} GB`
</script>

<template>
  <div class="dyn-setup">
    <section class="details-card">
      <div class="section-head">
        <h2>SOCKS5 接入信息</h2>
        <a href="#sub-accounts" class="reminder-link">管理子账号</a>
      </div>

      <template v-if="enabled.length">
        <div class="setup-row">
          <span id="sub-label">子账号</span>
          <ElSelect v-model="accountName" class="setup-select" aria-labelledby="sub-label">
            <ElOption v-for="a in enabled" :key="a.name" :value="a.name" :label="a.remark ? `${a.name} · ${a.remark}` : a.name" />
          </ElSelect>
        </div>
        <div class="setup-row">
          <span id="mode-label">会话</span>
          <ElRadioGroup v-model="mode" aria-labelledby="mode-label">
            <ElRadioButton value="default">默认会话</ElRadioButton>
            <ElRadioButton value="sticky">指定会话标识</ElRadioButton>
          </ElRadioGroup>
        </div>
        <div v-if="mode === 'sticky'" class="setup-row setup-sub">
          <span />
          <div class="setup-sid">
            <ElInput v-model="sid" maxlength="32" placeholder="1–32 位小写字母或数字" class="sid-input">
              <template #prepend>sid</template>
            </ElInput>
            <p :class="sidError ? 'form-error' : 'form-hint'">{{ sidError || '同一会话标识的新连接尽量使用同一出口，重连时出口 IP 仍可能变化。' }}</p>
            <ElCollapse class="sidtime-collapse">
              <ElCollapseItem title="会话时长（有限支持）" name="sidtime">
                <ElInput v-model="sidtime" maxlength="8" placeholder="留空使用上游默认" class="sid-input">
                  <template #prepend>sidtime</template>
                </ElInput>
                <p class="form-hint">原样透传给上游，实际时长、到期处理和出口 IP 由上游决定。</p>
              </ElCollapseItem>
            </ElCollapse>
          </div>
        </div>

        <dl class="credential-list">
          <div v-for="f in fields" :key="f.label">
            <dt>{{ f.label }}</dt>
            <dd>
              <span class="mono-value">{{ f.secret && !reveal ? '••••••••' : f.value }}</span>
              <ElButton v-if="f.secret" link :icon="reveal ? Hide : View" :aria-label="reveal ? '隐藏密码' : '显示密码'" @click="reveal = !reveal" />
              <ElButton link :icon="CopyDocument" :aria-label="`复制${f.label}`" @click="copy(f.value, f.label)" />
            </dd>
          </div>
        </dl>

        <div class="code-block">
          <div class="code-head"><span>连接串</span><ElButton link type="primary" :icon="CopyDocument" @click="copy(connection, '连接串')">复制</ElButton></div>
          <code>{{ reveal ? connection : connection.replace(password, '••••••••') }}</code>
        </div>
        <div class="code-block">
          <div class="code-head"><span>测试命令</span><ElButton link type="primary" :icon="CopyDocument" @click="copy(curl, '测试命令')">复制</ElButton></div>
          <code>{{ reveal ? curl : curl.replace(password, '••••••••') }}</code>
        </div>
      </template>

      <ElAlert v-else type="info" :closable="false" show-icon title="还没有启用的子账号">
        <p>动态住宅代理通过子账号接入，请先创建或启用一个子账号。</p>
        <ElButton type="primary" size="small" tag="a" href="#sub-accounts" class="alert-action">去创建子账号</ElButton>
      </ElAlert>
    </section>

    <aside class="dyn-side">
      <section class="stat-card">
        <header><span>可用流量</span></header>
        <p class="stat-value" :class="{ bad: trafficBalanceGb < 0 }">{{ gb(trafficBalanceGb) }}</p>
        <footer>
          <span>流量长期有效，余额大于 0 才能建立新连接</span>
          <div class="stat-actions"><ElButton size="small" type="primary" tag="a" href="#dynamic/pricing">购买流量</ElButton></div>
        </footer>
      </section>

      <section class="details-card">
        <h2 class="side-title">低余额提醒</h2>
        <p class="form-hint">可用流量降到阈值或以下时，在控制台提醒一次。回到阈值以上后，下次降低会再次提醒。</p>
        <div class="threshold-row">
          <ElInputNumber v-model="threshold" :min="0" :step="5" :precision="0" />
          <span class="form-unit">GB</span>
          <ElButton :disabled="threshold === trafficThresholdGb" @click="saveThreshold">保存</ElButton>
        </div>
      </section>
    </aside>
  </div>
</template>
