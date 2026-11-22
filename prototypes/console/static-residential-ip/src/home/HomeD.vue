<script setup lang="ts">
// 方案 D 首屏以下：在同一块连续深色底上，按「选哪种 → 买完怎么接入 → 用了怎么查账和管理」讲三件事。
// 标题和正文与首屏共用一条左边线（1240 内容宽、24 内边距），只有接入和收尾两段换底色。
// 产品画面使用控制台原型的同一份演示数据并标为示例；不放无法核实的性能数字和客户评价。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElCollapse, ElCollapseItem, ElMessage } from 'element-plus'
import { CopyDocument, Download, Promotion } from '@element-plus/icons-vue'
import LiveTerminal from './LiveTerminal.vue'
import { brandName, faqs, sample, telegram } from './home-demo'
import { products, staticCountries } from './home-products'
import { dayMs, formatExpiry, remainingLabel, statusOf } from '../demo'
import { dailyUsage, ips, subAccounts, trafficBalanceGb, usageUpdatedAt } from '../state'

// ── 选型：两列字段位置一致 ──────────────────────────────
const copy = {
  dynamic: { fit: '数据采集、广告核对、价格监控等需要不断换出口的任务。', billing: '按 GB 计费，流量长期有效，用完为止。', link: '查看动态价格' },
  static: { fit: '店铺、社媒账号等需要长期固定同一个身份的业务。', billing: '按 IP 和租期计费，独享或共享，到期前可续费。', link: '查看静态价格' },
} as const
const columns = computed(() => products.value.map(p => ({ ...p, ...copy[p.key] })))

// 动态示意：同一个子账号换会话标识，出口随之轮换。
const sessions = [
  { sid: 'a7k2', exit: '美国 洛杉矶' }, { sid: 'q9m1', exit: '英国 伦敦' }, { sid: 'z3p8', exit: '日本 东京' },
  { sid: 'h5d0', exit: '巴西 圣保罗' }, { sid: 'w2c6', exit: '德国 法兰克福' },
]
const sessionStart = ref(0)
const recentSessions = computed(() => [0, 1, 2].map(i => sessions[(sessionStart.value - i + sessions.length * 3) % sessions.length]))

// 静态示意：「我的 IP」里一条即将到期的 IP，续费后原 IP 不变、到期顺延。
const renewTarget = computed(() => ips.value.find(ip => statusOf(ip) === 'expiring') ?? ips.value[0])
const day = (t: number) => formatExpiry(t).slice(0, 10)

// ── 接入：面板高度固定，页签切换不跳动 ───────────────────────
const proxyUrl = `socks5h://${sample.username}:${sample.password}@${sample.host}:${sample.port}`
const snippets: Record<string, string> = {
  cURL: `curl -x ${proxyUrl} \\
  https://ipinfo.io`,
  Python: `import requests

proxy = "${proxyUrl}"
resp = requests.get("https://ipinfo.io",
                    proxies={"http": proxy, "https": proxy})
print(resp.json())`,
  'Node.js': `import axios from 'axios'
import { SocksProxyAgent } from 'socks-proxy-agent'

const agent = new SocksProxyAgent('${proxyUrl}')
const { data } = await axios.get('https://ipinfo.io', { httpsAgent: agent })
console.log(data)`,
  Go: `dialer, _ := proxy.SOCKS5("tcp", "${sample.host}:${sample.port}",
    &proxy.Auth{User: "${sample.username}", Password: "${sample.password}"},
    proxy.Direct)
client := &http.Client{Transport: &http.Transport{Dial: dialer.Dial}}
resp, _ := client.Get("https://ipinfo.io")`,
}
const live = '实时演示'
const tabs = [live, ...Object.keys(snippets)]
const tab = ref(live)
const code = computed(() => snippets[tab.value] ?? '')
async function copyCode() {
  await navigator.clipboard.writeText(code.value)
  ElMessage.success('已复制示例代码')
}

// ── 管理：控制台局部，数据与客户控制台原型一致 ─────────────────
const maxDay = Math.max(...dailyUsage.map(d => d.gb))
const weekGb = dailyUsage.reduce((s, d) => s + d.gb, 0)
const gb = (n: number) => n.toFixed(2)

let timer = 0
onMounted(() => { timer = window.setInterval(() => { sessionStart.value = (sessionStart.value + 1) % sessions.length }, 2600) })
onBeforeUnmount(() => clearInterval(timer))

const openFaq = ref(['0'])
</script>

<template>
  <div class="d-page">
    <!-- 1 选哪种：一句过渡接一组双栏对比 -->
    <section id="products" class="d-sec choose">
      <div class="wrap">
        <h2 class="statement">需要轮换出口，选动态。<span>需要保留同一个 IP，选静态。</span></h2>
        <div class="compare">
          <article v-for="c in columns" :key="c.key" class="col" :class="c.key">
            <header class="col-head">
              <h3>{{ c.name }}</h3>
              <span v-if="!c.onSale" class="closed-tag">暂未开放购买</span>
            </header>
            <div class="field"><span class="label">适用场景</span><p>{{ c.fit }}</p></div>
            <div class="field"><span class="label">计费单位</span><p>{{ c.billing }}</p></div>
            <div class="field price">
              <span class="label">起价</span>
              <p v-if="c.onSale && c.price"><strong>{{ c.price }}</strong>{{ c.priceUnit }}</p>
              <p v-else class="soon">开售后公布报价</p>
            </div>
            <div class="field entry">
              <a v-if="c.onSale" class="entry-link" :href="c.href">{{ c.link }} →</a>
              <a v-else class="entry-link muted" href="#faq">了解产品 →</a>
            </div>
            <figure class="shot" :aria-label="c.key === 'dynamic' ? '出口轮换示例' : '保号续费示例'">
              <figcaption><span>{{ c.key === 'dynamic' ? '出口轮换' : '保号续费' }}</span><em>示例</em></figcaption>
              <template v-if="c.key === 'dynamic'">
                <p class="shot-user">{{ sample.username }}<b>-sid-{{ recentSessions[0].sid }}</b></p>
                <ul class="sessions">
                  <li v-for="(s, i) in recentSessions" :key="i" :class="{ now: i === 0 }">
                    <code>sid-{{ s.sid }}</code><span>→</span><strong>{{ s.exit }}</strong><small>{{ i === 0 ? '当前出口' : '' }}</small>
                  </li>
                </ul>
              </template>
              <template v-else>
                <div class="ip-card">
                  <div class="ip-main"><code>{{ renewTarget.address }}:{{ renewTarget.port }}</code><span>{{ renewTarget.region }}</span></div>
                  <span class="ip-warn">{{ remainingLabel(renewTarget) }}</span>
                </div>
                <div class="renew">
                  <span class="renew-tag">续费 30 天</span>
                  <span>到期 {{ day(renewTarget.expiry) }}<i>→</i><b>{{ day(renewTarget.expiry + 30 * dayMs) }}</b></span>
                </div>
                <p class="renew-note">续费成功后仍是 {{ renewTarget.address }}</p>
              </template>
            </figure>
          </article>
        </div>
        <p class="footnote">静态 IP 目前可选 {{ staticCountries }} 个国家和地区，价格和库存以购买页实时报价为准。两种代理共用一个账户和钱包。</p>
      </div>
    </section>

    <!-- 2 买完怎么接入：换底色的重点段落 -->
    <section id="integration" class="d-sec band">
      <div class="wrap split">
        <div class="copy">
          <p class="kicker">接入</p>
          <h2>买完，复制一行配置就能用。</h2>
          <ol class="how">
            <li>在控制台选一个代理子账号。</li>
            <li>复制网关地址、端口和账密。</li>
            <li>填进任何支持 SOCKS5 的工具、浏览器或代码。</li>
          </ol>
          <p class="format"><code>主账号名.子账号名</code><code class="opt">-sid-会话标识</code></p>
          <p class="note">加上会话标识，同一会话尽量沿用同一出口；不加则自动轮换。右侧为示例账号，真实网关和账密登录后在控制台查看。</p>
        </div>
        <div class="panel">
          <div class="panel-tabs">
            <button v-for="t in tabs" :key="t" type="button" :class="{ active: t === tab }" @click="tab = t">{{ t }}</button>
            <button v-if="tab !== live" type="button" class="copy-btn" @click="copyCode"><CopyDocument />复制</button>
          </div>
          <div class="panel-body">
            <LiveTerminal v-if="tab === live" embedded />
            <pre v-else><code>{{ code }}</code></pre>
          </div>
        </div>
      </div>
    </section>

    <!-- 3 用了以后怎么查账和管理：控制台局部 -->
    <section id="manage" class="d-sec">
      <div class="wrap split">
        <div class="copy">
          <p class="kicker">查账与管理</p>
          <h2>每 1 GB 用在哪，控制台里都查得到。</h2>
          <ul class="points">
            <li><strong>按天看用量</strong>按总字节 1:1 扣减，1 GB = 10⁹ 字节。可按子账号筛选，导出 CSV。</li>
            <li><strong>按业务分子账号</strong>最多 10 个，各自账密、备注和累计用量上限，共用账户余额。</li>
            <li><strong>余额和到期提醒</strong>流量低于你设的阈值、静态 IP 到期前 5 天，控制台各提醒一次。</li>
          </ul>
        </div>
        <figure class="console" aria-label="控制台示例">
          <div class="console-bar"><i /><i /><i /><span>控制台 · 动态住宅代理-GB · 数据统计</span><em>示例数据</em></div>
          <div class="console-body">
            <div class="kpis">
              <div><span>可用流量</span><strong>{{ gb(trafficBalanceGb) }} GB</strong></div>
              <div><span>近 7 天扣减</span><strong>{{ gb(weekGb) }} GB</strong></div>
              <div><span>数据更新</span><strong class="small">{{ formatExpiry(usageUpdatedAt) }}</strong></div>
            </div>
            <div class="chart">
              <div v-for="d in dailyUsage" :key="d.day" class="col-bar">
                <i :style="{ height: `${(d.gb / maxDay) * 100}%` }" :title="`${d.gb} GB`" />
                <span>{{ d.day.slice(5) }}</span>
              </div>
            </div>
            <div class="subs-head"><strong>代理子账号</strong><span class="csv"><Download />导出 CSV</span></div>
            <table class="subs">
              <thead><tr><th>子账号</th><th>备注</th><th>状态</th><th>累计用量 / 上限</th></tr></thead>
              <tbody>
                <tr v-for="a in subAccounts" :key="a.name">
                  <td><code>{{ a.name }}</code></td>
                  <td>{{ a.remark || '—' }}</td>
                  <td><span class="state" :class="{ off: !a.active }">{{ a.active ? '启用' : '已禁用' }}</span></td>
                  <td>
                    <div class="usage">
                      <span class="meter"><i :class="{ full: a.limitGb && a.usedGb >= a.limitGb }" :style="{ width: `${a.limitGb ? Math.min(100, (a.usedGb / a.limitGb) * 100) : 100}%`, opacity: a.limitGb ? 1 : .35 }" /></span>
                      <small>{{ gb(a.usedGb) }} / {{ a.limitGb ? `${a.limitGb} GB` : '不限' }}</small>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </figure>
      </div>
    </section>

    <section id="faq" class="d-sec">
      <div class="wrap faq-grid">
        <div class="copy">
          <p class="kicker">常见问题</p>
          <h2>您可能想知道</h2>
          <p class="note">没找到答案？通过 Telegram 联系客服。</p>
        </div>
        <ElCollapse v-model="openFaq" class="faq">
          <ElCollapseItem v-for="(f, i) in faqs" :key="f.q" :name="String(i)" :title="f.q">{{ f.a }}</ElCollapseItem>
        </ElCollapse>
      </div>
    </section>

    <!-- 收尾：第二处换底色 -->
    <section class="d-sec closing">
      <div class="wrap">
        <h2>今天注册，<span>当天接入。</span></h2>
        <ol class="steps">
          <li><span>01</span><strong>注册账号</strong><p>手机号注册，开通即有控制台。</p></li>
          <li><span>02</span><strong>联系客服充值</strong><p>Telegram {{ telegram.handle }}</p></li>
          <li><span>03</span><strong>购买并接入</strong><p>选好代理，复制配置开始使用。</p></li>
        </ol>
        <div class="closing-ctas">
          <a class="cta primary lg" href="/">免费注册</a>
          <a class="text-link" :href="telegram.url" target="_blank" rel="noopener"><Promotion />联系客服</a>
        </div>
      </div>
    </section>

    <footer class="home-footer">
      <div class="wrap footer-grid">
        <div class="footer-brand">
          <strong>{{ brandName }}</strong>
          <p>为出海团队提供住宅代理。</p>
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
  </div>
</template>

<style scoped>
.d-page {
  --bg: #050e0c; --band: #0b1a17; --surface: #0e1f1b; --line: rgba(179, 240, 215, .11);
  --text: #e8f3ef; --muted: #93aca5; --faint: #6f8d84; --cyan: #8ee3f5;
  background: var(--bg); color: var(--text);
}
.wrap { max-width: 1240px; margin: 0 auto; padding: 0 24px; }
.d-sec { padding: 104px 0; }
.d-sec.band { background: var(--band); border-block: 1px solid var(--line); }
.kicker { color: var(--mint); font-weight: 700; font-size: 13px; letter-spacing: .12em; margin-bottom: 14px; }
h2 { font-size: clamp(30px, 3.4vw, 48px); font-weight: 850; line-height: 1.18; letter-spacing: -.015em; color: #fff; }
.note { color: var(--muted); line-height: 1.75; font-size: 14px; }
.cta.primary { background: var(--mint); color: var(--shell); }
.cta.primary:hover { background: #d4fbea; }
.text-link { display: inline-flex; align-items: center; gap: 6px; color: var(--text); font-weight: 650; text-decoration: none; border-bottom: 1px solid rgba(232, 243, 239, .4); padding-bottom: 2px; }
.text-link:hover { color: var(--mint); border-color: var(--mint); }

/* 1 选型：两列共享底色，字段用 subgrid 逐行对齐 */
.choose { padding-top: 32px; }
.statement { font-size: clamp(30px, 3.6vw, 52px); max-width: 16em; margin-bottom: 48px; }
.statement span { display: block; color: var(--faint); }
.compare { display: grid; grid-template-columns: 1fr 1fr; grid-template-rows: repeat(6, auto); border-radius: 22px; background: var(--surface); border: 1px solid var(--line); }
.col { display: grid; grid-row: span 6; grid-template-rows: subgrid; padding: 36px 40px 40px; min-width: 0; }
.col + .col { border-left: 1px solid var(--line); }
.col-head { display: flex; align-items: center; gap: 12px; padding-bottom: 22px; }
.col-head h3 { font-size: 26px; font-weight: 800; color: #fff; }
.closed-tag { font-size: 12px; font-weight: 650; color: #ffd28a; background: rgba(255, 196, 102, .12); padding: 4px 10px; border-radius: 99px; }
.field { padding: 16px 0; border-top: 1px solid var(--line); }
.label { display: block; font-size: 12px; color: var(--faint); letter-spacing: .06em; margin-bottom: 6px; }
.field p { line-height: 1.7; color: #d2e2dd; }
.price p { color: var(--muted); }
.price strong { font-size: 44px; font-weight: 850; letter-spacing: -.02em; margin-right: 6px; }
.dynamic .price strong { color: var(--mint); }
.static .price strong { color: var(--cyan); }
.price .soon { font-size: 18px; padding: 12px 0; }
.entry { padding-top: 2px; border-top: 0; }
.entry-link { font-weight: 650; text-decoration: none; color: var(--mint); }
.static .entry-link { color: var(--cyan); }
.entry-link.muted { color: var(--muted); }
.entry-link:hover { text-decoration: underline; }
.shot { margin: 24px 0 0; padding: 18px 20px 20px; border-radius: 16px; background: #071411; border: 1px solid var(--line); display: flex; flex-direction: column; gap: 12px; }
.shot code { background: none; padding: 0; border-radius: 0; color: inherit; }
.shot figcaption { display: flex; justify-content: space-between; font-size: 12px; color: var(--muted); }
.shot figcaption em { font-style: normal; color: var(--faint); border: 1px solid var(--line); border-radius: 99px; padding: 1px 8px; }
.shot-user { font: 13px ui-monospace, Consolas, monospace; color: #bcd0ca; }
.shot-user b { color: var(--mint); font-weight: 600; }
.sessions { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
.sessions li { display: grid; grid-template-columns: 88px 20px 1fr 64px; align-items: center; padding: 9px 12px; border-radius: 10px; background: rgba(255, 255, 255, .03); color: var(--muted); font-size: 13px; transition: background .4s, color .4s; }
.sessions li.now { background: rgba(95, 227, 176, .12); color: #fff; }
.sessions code { font: 12.5px ui-monospace, Consolas, monospace; }
.sessions small { font-size: 11px; color: var(--mint); text-align: right; }
.ip-card { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 12px 14px; border-radius: 10px; background: rgba(255, 255, 255, .04); }
.ip-main { display: grid; gap: 2px; min-width: 0; }
.ip-main code { font: 600 13.5px ui-monospace, Consolas, monospace; color: #fff; }
.ip-main span { font-size: 12px; color: var(--muted); }
.ip-warn { font-size: 12px; font-weight: 650; color: #ffd28a; background: rgba(255, 196, 102, .12); padding: 3px 9px; border-radius: 99px; white-space: nowrap; }
.renew { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; font-size: 13px; color: var(--muted); }
.renew-tag { color: #05222b; background: var(--cyan); font-weight: 650; padding: 4px 10px; border-radius: 8px; }
.renew i { font-style: normal; margin: 0 6px; }
.renew b { color: var(--cyan); }
.renew-note { font-size: 12.5px; color: var(--cyan); }
.footnote { margin-top: 18px; font-size: 13px; color: var(--faint); }

/* 2 接入、3 管理：左文右图 */
.split { display: grid; grid-template-columns: minmax(0, .82fr) minmax(0, 1.18fr); gap: 64px; align-items: center; }
.copy h2 { margin-bottom: 22px; }
.how { margin: 0 0 18px; padding-left: 20px; display: grid; gap: 8px; color: #d2e2dd; line-height: 1.6; }
.how li::marker { color: var(--mint); font-weight: 700; }
.format { margin: 0 0 14px; display: flex; flex-wrap: wrap; gap: 6px; }
.format code { font: 14px ui-monospace, Consolas, monospace; padding: 6px 10px; border-radius: 8px; background: rgba(179, 240, 215, .12); color: var(--mint); }
.format code.opt { background: transparent; border: 1px dashed rgba(179, 240, 215, .35); color: var(--muted); }
.panel { border-radius: 18px; background: #061210; border: 1px solid rgba(179, 240, 215, .16); overflow: hidden; box-shadow: 0 40px 80px -40px rgba(0, 0, 0, .8); }
.panel-tabs { display: flex; gap: 4px; padding: 10px; border-bottom: 1px solid var(--line); }
.panel-tabs button { border: 0; background: none; color: var(--muted); padding: 7px 14px; border-radius: 8px; cursor: pointer; white-space: nowrap; }
.panel-tabs button.active { background: rgba(179, 240, 215, .12); color: var(--mint); }
.panel-tabs .copy-btn { margin-left: auto; display: inline-flex; align-items: center; gap: 6px; }
.panel-body { height: 360px; display: flex; flex-direction: column; }
.panel-body :deep(.terminal) { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.panel-body :deep(.term-body) { flex: 1; height: auto !important; min-height: 0; }
pre { flex: 1; margin: 0; padding: 22px 24px; overflow: auto; font: 13.5px/1.75 ui-monospace, "Cascadia Code", Consolas, monospace; color: #d9e6e2; white-space: pre-wrap; word-break: break-all; }
pre code { all: unset; }

.points { list-style: none; margin: 0; padding: 0; display: grid; gap: 18px; }
.points li { color: var(--muted); line-height: 1.7; padding-left: 16px; border-left: 2px solid rgba(95, 227, 176, .35); }
.points strong { display: block; color: #fff; font-size: 16px; margin-bottom: 2px; }
/* 控制台局部：沿用真实控制台的浅色界面，和深色页面形成明确的画面边界 */
.console { margin: 0; border-radius: 16px; overflow: hidden; background: #f4f6f5; color: var(--ink); box-shadow: 0 40px 90px -40px rgba(0, 0, 0, .9), 0 0 0 1px rgba(179, 240, 215, .15); }
.console-bar { display: flex; align-items: center; gap: 7px; padding: 11px 16px; background: var(--shell); color: #9fb3ad; font-size: 12px; }
.console-bar i { width: 10px; height: 10px; border-radius: 50%; background: #3b4d48; }
.console-bar span { margin-left: 8px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.console-bar em { margin-left: auto; flex: none; font-style: normal; color: var(--mint); border: 1px solid rgba(179, 240, 215, .3); border-radius: 99px; padding: 1px 8px; }
.console-body { padding: 18px; display: grid; gap: 14px; }
.kpis { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
.kpis div { background: #fff; border: 1px solid #dee5e1; border-radius: 10px; padding: 12px 14px; display: grid; gap: 4px; }
.kpis span { font-size: 12px; color: #556562; }
.kpis strong { font-size: 20px; color: var(--ink); }
.kpis strong.small { font-size: 13px; font-weight: 600; padding-top: 6px; }
.chart { height: 110px; display: flex; align-items: flex-end; gap: 10px; padding: 12px 14px 0; background: #fff; border: 1px solid #dee5e1; border-radius: 10px; }
.col-bar { flex: 1; height: 100%; display: flex; flex-direction: column; justify-content: flex-end; align-items: center; gap: 4px; padding-bottom: 6px; }
.col-bar i { width: 100%; max-width: 34px; border-radius: 5px 5px 2px 2px; background: var(--teal); }
.col-bar span { font-size: 10.5px; color: #556562; }
.subs-head { display: flex; justify-content: space-between; align-items: center; font-size: 13px; }
.csv { display: inline-flex; align-items: center; gap: 4px; color: var(--teal); font-weight: 600; font-size: 12px; }
.subs { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid #dee5e1; font-size: 12.5px; }
.subs th { text-align: left; font-weight: 600; color: #556562; background: #f7faf8; padding: 8px 12px; }
.subs td { padding: 9px 12px; border-top: 1px solid #eef2f0; white-space: nowrap; }
.subs code { font: 12.5px ui-monospace, Consolas, monospace; background: none; padding: 0; color: var(--ink); }
.state { font-size: 11.5px; font-weight: 600; color: var(--teal); background: var(--teal-soft); padding: 2px 8px; border-radius: 99px; }
.state.off { color: #556562; background: #eef2f0; }
.usage { display: flex; align-items: center; gap: 8px; }
.meter { width: 90px; height: 6px; border-radius: 99px; background: #e4ece8; overflow: hidden; }
.meter i { display: block; height: 100%; border-radius: inherit; background: var(--teal); }
.meter i.full { background: #c4781e; }
.usage small { color: #556562; }

/* FAQ */
.faq-grid { display: grid; grid-template-columns: minmax(0, .82fr) minmax(0, 1.18fr); gap: 64px; }
.faq { --el-collapse-header-font-size: 16px; --el-collapse-header-height: 62px; --el-collapse-border-color: var(--line); --el-collapse-header-bg-color: transparent; --el-collapse-content-bg-color: transparent; --el-collapse-header-text-color: #fff; border-top: 0; border-bottom-color: var(--line); }
.faq :deep(.el-collapse-item__header) { font-weight: 650; }
.faq :deep(.el-collapse-item__arrow) { color: var(--mint); }
.faq :deep(.el-collapse-item__content) { color: #bcd0ca; line-height: 1.85; max-width: 42em; }

/* 收尾 */
.closing { background: radial-gradient(70% 140% at 15% 0%, #1b5a48, var(--band) 65%); border-top: 1px solid var(--line); }
.closing h2 { font-size: clamp(34px, 4.4vw, 60px); }
.closing h2 span { color: var(--mint); }
.steps { list-style: none; padding: 0; margin: 44px 0 40px; max-width: 900px; display: grid; grid-template-columns: repeat(3, 1fr); }
.steps li { padding-right: 28px; display: grid; gap: 6px; }
.steps li + li { padding-left: 28px; border-left: 1px solid rgba(179, 240, 215, .18); }
.steps span { font-size: 13px; font-weight: 700; color: var(--mint); letter-spacing: .1em; }
.steps strong { font-size: 19px; color: #fff; }
.steps p { color: var(--muted); }
.closing-ctas { display: flex; gap: 28px; align-items: center; }

.home-footer { color: var(--muted); padding: 56px 0 28px; }
.footer-grid { display: grid; grid-template-columns: 1.6fr repeat(4, 1fr); gap: 32px; }
.footer-grid h4 { margin: 0 0 14px; color: var(--text); font-size: 14px; }
.footer-grid a { display: flex; align-items: center; gap: 6px; color: inherit; text-decoration: none; margin-bottom: 10px; }
.footer-grid a:hover { color: var(--mint); }
.footer-brand strong { font-size: 20px; color: #fff; }
.footer-brand p { margin: 10px 0 16px; }
.footer-bottom { margin-top: 40px; padding-top: 20px; border-top: 1px solid var(--line); display: flex; justify-content: space-between; font-size: 12px; color: var(--faint); }

@media (max-width: 960px) {
  .wrap { padding: 0 18px; }
  .d-sec { padding: 72px 0; }
  .choose { padding-top: 28px; }
  .statement { margin-bottom: 32px; }
  .compare { grid-template-columns: 1fr; grid-template-rows: none; }
  .col { grid-row: auto; grid-template-rows: none; padding: 28px 22px; }
  .col + .col { border-left: 0; border-top: 1px solid var(--line); }
  .price strong { font-size: 38px; }
  .sessions li { grid-template-columns: 78px 16px 1fr 56px; padding: 8px 10px; }
  .split, .faq-grid { grid-template-columns: 1fr; gap: 36px; }
  .panel-tabs { overflow-x: auto; }
  .panel-body { height: 340px; }
  .console-body { padding: 12px; }
  .kpis { grid-template-columns: 1fr 1fr; }
  .kpis div:last-child { grid-column: span 2; }
  .subs th:nth-child(2), .subs td:nth-child(2) { display: none; }
  .subs th, .subs td { padding: 8px; }
  .meter { width: 48px; }
  .steps { grid-template-columns: 1fr; gap: 22px; }
  .steps li + li { padding-left: 0; border-left: 0; }
  .footer-grid { grid-template-columns: 1fr 1fr; }
  .footer-bottom { flex-direction: column; gap: 8px; }
}
</style>
