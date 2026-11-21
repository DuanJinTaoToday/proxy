<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { ElAlert, ElButton, ElTabPane, ElTabs } from 'element-plus'
import DynamicPricing from './DynamicPricing.vue'
import DynamicSetup from './DynamicSetup.vue'
import DynamicStats from './DynamicStats.vue'
import { gatewayHost, gatewayPort, primaryAccount } from './state'

type Tab = 'setup' | 'pricing' | 'usage' | 'guide'
const tabs: Tab[] = ['setup', 'pricing', 'usage', 'guide']
const fromHash = (): Tab => tabs.find(t => location.hash.endsWith(`/${t}`)) ?? 'setup'
const tab = ref<Tab>(fromHash())
watch(tab, value => { history.replaceState(null, '', `#dynamic/${value}`) })
const onHash = () => { tab.value = fromHash() }
window.addEventListener('hashchange', onHash)
onBeforeUnmount(() => window.removeEventListener('hashchange', onHash))

const user = `${primaryAccount}.shop01`
const python = computed(() => [
  'import requests',
  '',
  `proxy = "socks5h://${user}:你的密码@${gatewayHost}:${gatewayPort}"`,
  'resp = requests.get("https://api.ipify.org", proxies={"http": proxy, "https": proxy}, timeout=30)',
  'print(resp.text)',
].join('\n'))
</script>

<template>
  <div class="page-head">
    <h1>动态住宅代理-GB</h1>
    <ElButton type="primary" @click="tab = 'pricing'">购买流量</ElButton>
  </div>

  <ElTabs v-model="tab" class="page-tabs">
    <ElTabPane label="代理设置" name="setup"><DynamicSetup /></ElTabPane>
    <ElTabPane label="价格" name="pricing"><DynamicPricing @purchased="tab = 'setup'" /></ElTabPane>
    <ElTabPane label="数据统计" name="usage" lazy><DynamicStats /></ElTabPane>
    <ElTabPane label="使用教程" name="guide">
      <section class="details-card guide-card">
        <h2>通过 SOCKS5 接入动态住宅代理</h2>
        <ElAlert type="warning" :closable="false" show-icon title="所有代理产品均不支持在中国大陆网络环境下使用。" class="guide-alert" />
        <ol class="steps">
          <li><strong>购买流量</strong><p>在「价格」页签购买流量包。流量长期有效，所有子账号共用，可用流量大于 0 才能建立新连接。</p></li>
          <li><strong>创建子账号</strong><p>在左侧「代理子账号」创建账号并设置密码，可以为不同项目分开设置用量上限。</p></li>
          <li>
            <strong>获取接入信息</strong>
            <p>在「代理设置」选择已启用的子账号，复制网关地址、端口和账密。用户名格式为 <code>{{ primaryAccount }}.子账号名</code>。</p>
          </li>
          <li>
            <strong>按需保持会话</strong>
            <p>在用户名后追加 <code>-sid-会话标识</code>（1–32 位小写字母或数字），同一会话的新连接会尽量使用同一出口。不追加时使用子账号的默认会话。重连时出口 IP 仍可能变化。</p>
          </li>
          <li>
            <strong>在程序中使用</strong>
            <p>建议使用 <code>socks5h</code>，让域名在代理端解析。</p>
            <div class="code-block"><div class="code-head"><span>Python</span></div><pre><code>{{ python }}</code></pre></div>
          </li>
          <li><strong>查看用量</strong><p>在「数据统计」按天查看实际扣减的流量，可按子账号筛选并导出 CSV。扣减按实际传输字节计算，1 GB = 10⁹ 字节。</p></li>
        </ol>
        <ElButton @click="tab = 'setup'">去代理设置</ElButton>
      </section>
    </ElTabPane>
  </ElTabs>
</template>
