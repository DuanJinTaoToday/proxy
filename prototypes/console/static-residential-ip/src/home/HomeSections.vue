<script setup lang="ts">
// 首屏以下的共用区块，三个方案共用，只有方案 B 因首屏已展示商品而隐藏商品区。
import { computed, ref } from 'vue'
import { ElCollapse, ElCollapseItem, ElMessage } from 'element-plus'
import { CopyDocument, Promotion } from '@element-plus/icons-vue'
import HeroCanvas from './HeroCanvas.vue'
import ProductCard from './ProductCard.vue'
import { advantages, brandName, coverage, faqs, sample, steps, telegram, testimonials, useCases } from './home-demo'
import { primaryHref, products } from './home-products'

defineProps<{ showProducts: boolean }>()

const proxyUrl = `socks5h://${sample.username}:${sample.password}@${sample.host}:${sample.port}`
const snippets: Record<string, string> = {
  cURL: `curl -x ${proxyUrl} https://ipinfo.io`,
  Python: `import requests

proxy = "${proxyUrl}"
resp = requests.get("https://ipinfo.io", proxies={"http": proxy, "https": proxy})
print(resp.json())`,
  'Node.js': `import axios from 'axios'
import { SocksProxyAgent } from 'socks-proxy-agent'

const agent = new SocksProxyAgent('${proxyUrl}')
const { data } = await axios.get('https://ipinfo.io', { httpsAgent: agent })
console.log(data)`,
  Go: `dialer, _ := proxy.SOCKS5("tcp", "${sample.host}:${sample.port}",
    &proxy.Auth{User: "${sample.username}", Password: "${sample.password}"}, proxy.Direct)
client := &http.Client{Transport: &http.Transport{Dial: dialer.Dial}}
resp, _ := client.Get("https://ipinfo.io")`,
}
const langs = Object.keys(snippets)
const lang = ref(langs[0])
const code = computed(() => snippets[lang.value])
async function copy() {
  await navigator.clipboard.writeText(code.value)
  ElMessage.success('已复制示例代码')
}
const openFaq = ref(['0'])
</script>

<template>
  <section v-if="showProducts" id="products" class="block">
    <div class="wrap">
      <header class="block-head">
        <p class="kicker">代理产品</p>
        <h2>两种住宅代理，覆盖出海常见场景</h2>
        <p>动态住宅按流量计费、出口持续轮换；静态住宅 IP 独享固定、按 IP 租用。注册一个账户即可同时使用。</p>
      </header>
      <div class="product-grid">
        <ProductCard v-for="p in products" :key="p.key" :product="p" />
      </div>
    </div>
  </section>
  <span v-else id="products" />

  <section class="block coverage">
    <div class="wrap coverage-grid">
      <div>
        <p class="kicker">全球覆盖</p>
        <h2>195+ 国家和地区的真实住宅网络</h2>
        <p class="sub">出口来自真实家庭宽带，按国家和城市就近调度。热门市场节点充足，持续扩充中。</p>
        <ul class="region-list">
          <li v-for="r in coverage" :key="r.region">
            <div class="region-row"><span>{{ r.region }}</span><strong>{{ r.count }}</strong></div>
            <div class="bar"><i :style="{ width: `${r.share * 3}%` }" /></div>
          </li>
        </ul>
      </div>
      <div class="coverage-map"><HeroCanvas mode="map" theme="light" /></div>
    </div>
  </section>

  <section class="block wash">
    <div class="wrap">
      <header class="block-head">
        <p class="kicker">使用场景</p>
        <h2>出海团队都在用它做什么</h2>
      </header>
      <div class="case-grid">
        <article v-for="c in useCases" :key="c.title" class="case">
          <span class="case-icon"><component :is="c.icon" /></span>
          <h3>{{ c.title }}</h3>
          <p>{{ c.body }}</p>
        </article>
      </div>
    </div>
  </section>

  <section id="integration" class="block dark-block">
    <div class="wrap integration-grid">
      <div>
        <p class="kicker">快速接入</p>
        <h2>标准 SOCKS5，几行代码即可接入</h2>
        <p class="sub">任何支持 SOCKS5 的工具、浏览器或代码库都能直接使用。用户名格式为：</p>
        <p class="format"><code>主账号名.子账号名</code><code class="opt">-sid-会话标识</code></p>
        <p class="sub small">追加会话标识可让同一会话的连接尽量沿用同一出口；不加则每次自动轮换。示例使用演示账号，真实网关与账密登录后在控制台查看。</p>
      </div>
      <div class="code-panel">
        <div class="code-tabs">
          <button v-for="l in langs" :key="l" type="button" :class="{ active: l === lang }" @click="lang = l">{{ l }}</button>
          <button type="button" class="copy" @click="copy"><CopyDocument />复制</button>
        </div>
        <pre><code>{{ code }}</code></pre>
      </div>
    </div>
  </section>

  <section class="block">
    <div class="wrap">
      <header class="block-head">
        <p class="kicker">为什么选择 {{ brandName }}</p>
        <h2>计费透明，用得放心</h2>
      </header>
      <div class="adv-grid">
        <article v-for="(a, i) in advantages" :key="a.title" class="adv">
          <span class="adv-no">0{{ i + 1 }}</span>
          <h3>{{ a.title }}</h3>
          <p>{{ a.body }}</p>
        </article>
      </div>
    </div>
  </section>

  <section class="block wash">
    <div class="wrap">
      <header class="block-head">
        <p class="kicker">开始使用</p>
        <h2>三步开通，当天即可使用</h2>
      </header>
      <ol class="steps">
        <li v-for="(s, i) in steps" :key="s.title">
          <span class="step-no">{{ i + 1 }}</span>
          <h3>{{ s.title }}</h3>
          <p>{{ s.body }}</p>
          <a v-if="i === 0" class="cta primary" href="/">免费注册</a>
          <a v-else-if="i === 1" class="cta outline-dark" :href="telegram.url" target="_blank" rel="noopener"><Promotion />{{ telegram.handle }}</a>
          <a v-else class="cta outline-dark" :href="primaryHref">查看价格</a>
        </li>
      </ol>
    </div>
  </section>

  <section class="block">
    <div class="wrap">
      <header class="block-head">
        <p class="kicker">客户评价</p>
        <h2>来自出海团队的真实反馈</h2>
      </header>
      <div class="quote-grid">
        <figure v-for="t in testimonials" :key="t.name" class="quote">
          <blockquote>“{{ t.quote }}”</blockquote>
          <figcaption><span class="avatar-dot">{{ t.name.slice(0, 1) }}</span><div><strong>{{ t.name }}</strong><small>{{ t.role }}</small></div></figcaption>
        </figure>
      </div>
    </div>
  </section>

  <section id="faq" class="block wash">
    <div class="wrap faq-grid">
      <header class="block-head left">
        <p class="kicker">常见问题</p>
        <h2>您可能想知道</h2>
        <p>没找到答案？通过 Telegram 联系客服。</p>
        <a class="cta outline-dark" :href="telegram.url" target="_blank" rel="noopener"><Promotion />联系客服</a>
      </header>
      <ElCollapse v-model="openFaq" class="faq">
        <ElCollapseItem v-for="(f, i) in faqs" :key="f.q" :name="String(i)" :title="f.q">{{ f.a }}</ElCollapseItem>
      </ElCollapse>
    </div>
  </section>

  <section class="closing">
    <div class="wrap">
      <h2>准备好开始了吗？</h2>
      <p>注册账号，联系客服充值，几分钟内接入全球住宅网络。</p>
      <div class="closing-ctas">
        <a class="cta primary lg" href="/">免费注册</a>
        <a class="cta outline lg" :href="primaryHref">查看价格</a>
      </div>
    </div>
  </section>

  <footer class="home-footer">
    <div class="wrap footer-grid">
      <div class="footer-brand">
        <strong>{{ brandName }}</strong>
        <p>为出海团队提供稳定的全球住宅代理。</p>
        <a :href="telegram.url" target="_blank" rel="noopener"><Promotion />Telegram {{ telegram.handle }}</a>
      </div>
      <div><h4>代理产品</h4><a href="/#dynamic/pricing">动态住宅代理</a><a href="/#static/pricing">静态住宅 IP</a></div>
      <div><h4>资源</h4><a href="#integration">SOCKS5 快速接入</a><a href="#faq">常见问题</a></div>
      <div><h4>账户</h4><a href="/">登录</a><a href="/">注册</a></div>
      <div><h4>条款</h4><a href="#top">服务条款</a><a href="#top">隐私政策</a></div>
    </div>
    <div class="wrap footer-bottom">
      <span>© 2026 {{ brandName }}</span>
      <span>所有代理产品均不支持在中国大陆网络环境下使用。</span>
    </div>
  </footer>
</template>

<style scoped>
.wrap { max-width: 1240px; margin: 0 auto; padding: 0 24px; }
.block { padding: 104px 0; background: #fff; }
.block.wash { background: var(--canvas); }
.block-head { max-width: 720px; margin: 0 auto 52px; text-align: center; display: grid; gap: 12px; }
.block-head.left { text-align: left; margin: 0; align-content: start; justify-items: start; }
.kicker { color: var(--teal); font-weight: 700; font-size: 13px; letter-spacing: .12em; }
h2 { font-size: clamp(28px, 3vw, 40px); font-weight: 800; line-height: 1.25; }
.block-head > p:last-child, .sub { color: var(--muted); line-height: 1.75; font-size: 16px; }
.sub.small { font-size: 14px; }
.product-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 24px; }

.coverage { overflow: hidden; }
.coverage-grid { display: grid; grid-template-columns: 0.8fr 1.2fr; gap: 48px; align-items: center; }
.coverage-grid h2 { margin: 12px 0 14px; }
.region-list { list-style: none; padding: 0; margin: 32px 0 0; display: grid; gap: 16px; }
.region-row { display: flex; justify-content: space-between; margin-bottom: 6px; }
.region-row strong { color: var(--teal); }
.bar { height: 6px; border-radius: 99px; background: var(--teal-soft); overflow: hidden; }
.bar i { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, var(--teal), #3fc596); }
.coverage-map { position: relative; height: 420px; }

.case-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; }
.case { padding: 28px; border-radius: 18px; background: #fff; border: 1px solid var(--line); display: grid; gap: 10px; transition: border-color .2s, transform .2s; }
.case:hover { border-color: #9fd8c4; transform: translateY(-3px); }
.case-icon { width: 44px; height: 44px; border-radius: 12px; display: grid; place-items: center; background: var(--teal-soft); color: var(--teal); font-size: 22px; }
.case h3 { font-size: 18px; }
.case p { color: var(--muted); line-height: 1.7; }

.dark-block { background: linear-gradient(160deg, #0b1f1b, #142522 60%, #0e2a22); color: #e8f3ef; }
.dark-block .kicker { color: var(--mint); }
.dark-block .sub { color: #a9c1ba; }
.integration-grid { display: grid; grid-template-columns: 0.85fr 1.15fr; gap: 56px; align-items: center; }
.integration-grid h2 { margin: 12px 0 16px; }
.format { margin: 14px 0; display: flex; flex-wrap: wrap; gap: 6px; }
.format code { font: 15px ui-monospace, Consolas, monospace; padding: 6px 10px; border-radius: 8px; background: rgba(179, 240, 215, .12); color: var(--mint); }
.format code.opt { background: transparent; border: 1px dashed rgba(179, 240, 215, .35); color: #a9c1ba; }
.code-panel { border-radius: 16px; background: #061210; border: 1px solid rgba(179, 240, 215, .16); overflow: hidden; }
.code-tabs { display: flex; gap: 4px; padding: 10px; border-bottom: 1px solid rgba(179, 240, 215, .1); }
.code-tabs button { border: 0; background: none; color: #8fb0a6; padding: 7px 14px; border-radius: 8px; cursor: pointer; }
.code-tabs button.active { background: rgba(179, 240, 215, .12); color: var(--mint); }
.code-tabs .copy { margin-left: auto; display: inline-flex; align-items: center; gap: 6px; }
pre { margin: 0; padding: 22px 24px; min-height: 210px; overflow-x: auto; font: 13.5px/1.75 ui-monospace, "Cascadia Code", Consolas, monospace; color: #d9e6e2; }
pre code { all: unset; white-space: pre; }

.adv-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px; }
.adv { padding: 32px; border-radius: 18px; background: linear-gradient(180deg, var(--teal-soft), #fff 70%); border: 1px solid var(--line); display: grid; gap: 12px; }
.adv-no { font-size: 40px; font-weight: 800; color: #9fd8c4; }
.adv h3 { font-size: 20px; }
.adv p { color: var(--muted); line-height: 1.75; }

.steps { list-style: none; padding: 0; margin: 0; display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px; counter-reset: s; }
.steps li { position: relative; padding: 32px; border-radius: 18px; background: #fff; border: 1px solid var(--line); display: grid; gap: 12px; align-content: start; justify-items: start; }
.steps li:not(:last-child)::after { content: ''; position: absolute; top: 52px; right: -24px; width: 24px; border-top: 2px dashed #9fd8c4; }
.step-no { width: 40px; height: 40px; border-radius: 50%; display: grid; place-items: center; background: var(--shell); color: var(--mint); font-weight: 800; }
.steps h3 { font-size: 20px; }
.steps p { color: var(--muted); line-height: 1.7; }

.quote-grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 20px; }
.quote { margin: 0; padding: 28px; border-radius: 18px; border: 1px solid var(--line); display: flex; flex-direction: column; gap: 20px; }
.quote blockquote { margin: 0; line-height: 1.8; color: var(--ink); flex: 1; }
.quote figcaption { display: flex; gap: 12px; align-items: center; }
.quote figcaption div { display: grid; }
.quote small { color: var(--muted); }
.avatar-dot { width: 38px; height: 38px; border-radius: 50%; display: grid; place-items: center; background: var(--shell); color: var(--mint); font-weight: 700; }

.faq-grid { display: grid; grid-template-columns: 0.7fr 1.3fr; gap: 56px; }
.faq { --el-collapse-header-font-size: 16px; --el-collapse-content-font-size: 14px; --el-collapse-header-height: 60px; border-top: 0; }
.faq :deep(.el-collapse-item__header) { font-weight: 650; background: transparent; }
.faq :deep(.el-collapse-item__wrap) { background: transparent; }
.faq :deep(.el-collapse-item__content) { color: var(--muted); line-height: 1.8; }

.closing { padding: 96px 0; text-align: center; color: #fff; background: radial-gradient(70% 120% at 50% 0%, #1f6a54, #0c201c 70%); }
.closing h2 { font-size: clamp(30px, 3.4vw, 44px); }
.closing p { color: #b7cdc6; margin: 14px 0 32px; }
.closing-ctas { display: flex; gap: 14px; justify-content: center; }

.home-footer { background: #0a1916; color: #9fb3ad; padding: 64px 0 28px; }
.footer-grid { display: grid; grid-template-columns: 1.6fr repeat(4, 1fr); gap: 32px; }
.footer-grid h4 { margin: 0 0 14px; color: #e8f3ef; font-size: 14px; }
.footer-grid a { display: flex; align-items: center; gap: 6px; color: inherit; text-decoration: none; margin-bottom: 10px; }
.footer-grid a:hover { color: var(--mint); }
.footer-brand strong { font-size: 20px; color: #fff; }
.footer-brand p { margin: 10px 0 16px; }
.footer-bottom { margin-top: 40px; padding-top: 20px; border-top: 1px solid rgba(179, 240, 215, .1); display: flex; justify-content: space-between; font-size: 12px; color: #6f8d84; }

@media (max-width: 960px) {
  .product-grid, .coverage-grid, .integration-grid, .faq-grid { grid-template-columns: 1fr; }
  .case-grid, .adv-grid, .steps, .quote-grid { grid-template-columns: 1fr; }
  .steps li::after { display: none; }
  .footer-grid { grid-template-columns: 1fr 1fr; }
  .footer-bottom { flex-direction: column; gap: 8px; }
  .coverage-map { height: 240px; }
}
</style>
