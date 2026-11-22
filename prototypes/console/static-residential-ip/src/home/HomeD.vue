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

    <!-- 2 买完怎么接入：深海蓝叠层段落 -->
    <section id="integration" class="d-sec band-navy">
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
        <div class="panel-stage">
          <div class="panel">
            <div class="panel-tabs">
              <span class="win-dots" aria-hidden="true"><i /><i /><i /></span>
              <button v-for="t in tabs" :key="t" type="button" :class="{ active: t === tab }" @click="tab = t">{{ t }}</button>
              <button v-if="tab !== live" type="button" class="copy-btn" @click="copyCode"><CopyDocument />复制</button>
            </div>
            <div class="panel-body">
              <LiveTerminal v-if="tab === live" embedded />
              <pre v-else><code>{{ code }}</code></pre>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- 3 用了以后怎么查账和管理：深松绿叠层段落 + 暗色画框承托控制台局部 -->
    <section id="manage" class="d-sec band-pine">
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
        <div class="console-frame">
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
      </div>
    </section>

    <section id="faq" class="d-sec band-dark">
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

    <!-- 收尾：深绿光晕收口 + 纯白胶囊主按钮 -->
    <section class="d-sec closing">
      <div class="wrap">
        <h2>今天注册，<span>当天接入。</span></h2>
        <ol class="steps">
          <li><span>01</span><strong>注册账号</strong><p>手机号注册，开通即有控制台。</p></li>
          <li><span>02</span><strong>联系客服充值</strong><p>Telegram {{ telegram.handle }}</p></li>
          <li><span>03</span><strong>购买并接入</strong><p>选好代理，复制配置开始使用。</p></li>
        </ol>
        <div class="closing-ctas">
          <a class="pill primary" href="/">免费注册</a>
          <a class="pill ghost" :href="telegram.url" target="_blank" rel="noopener"><Promotion />联系客服</a>
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
  --bg: #020808; --navy: #020c1f; --pine: #041b16; --line: rgba(255, 255, 255, .09);
  --text: #f3f7f6; --muted: rgba(255, 255, 255, .64); --faint: rgba(255, 255, 255, .42); --cyan: #7fd4f5;
  background: var(--bg); color: var(--text);
}
.wrap { max-width: 1240px; margin: 0 auto; padding: 0 24px; }
.d-sec { position: relative; padding: 112px 0; }
.d-sec.band-navy {
  background: radial-gradient(62% 68% at 78% 18%, rgba(28, 92, 196, .22), transparent 64%), var(--navy);
  border-radius: 36px 36px 0 0;
  padding: 120px 0 144px;
}
.d-sec.band-pine {
  background: radial-gradient(58% 66% at 76% 24%, rgba(22, 134, 98, .2), transparent 64%), var(--pine);
  border-radius: 36px 36px 0 0;
  margin-top: -32px;
  padding: 120px 0 144px;
}
.d-sec.band-dark {
  background: var(--bg);
  border-radius: 36px 36px 0 0;
  margin-top: -32px;
  padding: 116px 0 120px;
}
.kicker { color: rgba(255, 255, 255, .52); font-weight: 500; font-size: 13px; letter-spacing: .14em; margin-bottom: 16px; }
h2 { font-size: clamp(32px, 3.8vw, 54px); font-weight: 500; line-height: 1.15; letter-spacing: -.025em; color: #fff; }
.note { color: var(--muted); line-height: 1.75; font-size: 14.5px; }
.pill { display: inline-flex; align-items: center; justify-content: center; gap: 8px; height: 52px; padding: 0 30px; border-radius: 9999px; font-size: 15.5px; text-decoration: none; white-space: nowrap; transition: background .2s, border-color .2s, transform .15s, color .2s; }
.pill:hover { transform: translateY(-1px); }
.pill.primary { background: #fff; color: #050909; font-weight: 600; }
.pill.primary:hover { background: #e4e8e7; }
.pill.ghost { border: 1px solid rgba(255, 255, 255, .28); background: rgba(255, 255, 255, .04); color: #fff; font-weight: 500; }
.pill.ghost:hover { border-color: rgba(255, 255, 255, .65); background: rgba(255, 255, 255, .09); }
.pill svg { width: 16px; height: 16px; }

/* 1 选型：两张独立沉浸式大圆角展板，无表格横线，微弱环境光区分产品 */
.choose { padding: 108px 0 120px; }
.statement { font-size: clamp(34px, 4vw, 58px); max-width: 15.5em; margin-bottom: 56px; font-weight: 500; }
.statement span { display: block; color: var(--faint); }
.compare { display: grid; grid-template-columns: 1fr 1fr; grid-template-rows: repeat(6, auto); gap: 0 24px; }
.col { display: grid; grid-row: span 6; grid-template-rows: subgrid; padding: 40px 42px 42px; min-width: 0; border-radius: 26px; border: 1px solid rgba(255, 255, 255, .08); }
.col.dynamic { background: radial-gradient(88% 68% at 92% 0%, rgba(56, 198, 144, .16) 0%, #091311 54%, #060b0a 100%); }
.col.static { background: radial-gradient(88% 68% at 92% 0%, rgba(76, 182, 232, .16) 0%, #081218 54%, #060a0e 100%); }
.col-head { display: flex; align-items: center; gap: 12px; padding-bottom: 24px; }
.col-head h3 { font-size: 28px; font-weight: 550; letter-spacing: -.015em; color: #fff; }
.closed-tag { font-size: 12px; font-weight: 500; color: #ffd28a; background: rgba(255, 196, 102, .12); padding: 4px 10px; border-radius: 99px; }
.field { padding: 14px 0; }
.label { display: block; font-size: 12px; color: var(--faint); letter-spacing: .06em; margin-bottom: 6px; }
.field p { line-height: 1.7; color: rgba(255, 255, 255, .82); font-size: 15px; }
.price { padding-top: 20px; }
.price p { color: var(--muted); }
.price strong { font-size: 46px; font-weight: 500; letter-spacing: -.03em; margin-right: 6px; font-variant-numeric: tabular-nums; color: #fff; }
.price .soon { font-size: 18px; padding: 12px 0; }
.entry { padding-top: 4px; padding-bottom: 8px; }
.entry-link { font-weight: 500; text-decoration: none; color: #fff; border-bottom: 1px solid rgba(255, 255, 255, .32); padding-bottom: 3px; transition: border-color .2s, color .2s; }
.dynamic .entry-link:hover { color: var(--mint); border-color: var(--mint); }
.static .entry-link:hover { color: var(--cyan); border-color: var(--cyan); }
.entry-link.muted { color: var(--muted); border-color: rgba(255, 255, 255, .18); }
.shot { margin: 24px 0 0; padding: 20px 22px 22px; border-radius: 18px; background: rgba(0, 0, 0, .42); border: 1px solid rgba(255, 255, 255, .07); display: flex; flex-direction: column; gap: 12px; }
.shot code { background: none; padding: 0; border-radius: 0; color: inherit; }
.shot figcaption { display: flex; justify-content: space-between; font-size: 12px; color: var(--muted); }
.shot figcaption em { font-style: normal; color: var(--faint); border: 1px solid rgba(255, 255, 255, .1); border-radius: 99px; padding: 1px 9px; }
.shot-user { font: 13px ui-monospace, Consolas, monospace; color: rgba(255, 255, 255, .74); }
.shot-user b { color: var(--mint); font-weight: 600; }
.sessions { list-style: none; margin: 0; padding: 0; display: grid; gap: 8px; }
.sessions li { display: grid; grid-template-columns: 88px 20px 1fr 64px; align-items: center; padding: 9px 12px; border-radius: 10px; background: rgba(255, 255, 255, .03); color: var(--muted); font-size: 13px; transition: background .4s, color .4s; }
.sessions li.now { background: rgba(125, 232, 190, .12); color: #fff; }
.sessions code { font: 12.5px ui-monospace, Consolas, monospace; }
.sessions small { font-size: 11px; color: var(--mint); text-align: right; }
.ip-card { display: flex; justify-content: space-between; align-items: center; gap: 12px; padding: 12px 14px; border-radius: 12px; background: rgba(255, 255, 255, .04); }
.ip-main { display: grid; gap: 2px; min-width: 0; }
.ip-main code { font: 500 13.5px ui-monospace, Consolas, monospace; color: #fff; }
.ip-main span { font-size: 12px; color: var(--muted); }
.ip-warn { font-size: 12px; font-weight: 500; color: #ffd28a; background: rgba(255, 196, 102, .12); padding: 3px 10px; border-radius: 99px; white-space: nowrap; }
.renew { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; font-size: 13px; color: var(--muted); }
.renew-tag { color: #04151e; background: #fff; font-weight: 600; padding: 4px 12px; border-radius: 999px; }
.renew i { font-style: normal; margin: 0 6px; }
.renew b { color: var(--cyan); font-weight: 500; }
.renew-note { font-size: 12.5px; color: var(--cyan); }
.footnote { margin-top: 24px; font-size: 13px; color: var(--faint); }

/* 2 接入、3 管理：左文右图 */
.split { display: grid; grid-template-columns: minmax(0, .82fr) minmax(0, 1.18fr); gap: 68px; align-items: center; }
.copy h2 { margin-bottom: 24px; }
.how { margin: 0 0 22px; padding-left: 20px; display: grid; gap: 10px; color: rgba(255, 255, 255, .82); line-height: 1.65; font-size: 15px; }
.how li::marker { color: #8ab4ff; font-weight: 600; }
.format { margin: 0 0 16px; display: flex; flex-wrap: wrap; gap: 8px; }
.format code { font: 13.5px ui-monospace, Consolas, monospace; padding: 7px 12px; border-radius: 10px; background: rgba(138, 180, 255, .12); color: #d6e4ff; border: 1px solid rgba(138, 180, 255, .2); }
.format code.opt { background: transparent; border: 1px dashed rgba(255, 255, 255, .26); color: var(--muted); }

/* 终端外层舞台：柔和蓝光 + 玻璃底托增强景深 */
.panel-stage { padding: 14px; border-radius: 26px; background: linear-gradient(145deg, rgba(120, 168, 255, .12), rgba(255, 255, 255, .02)); border: 1px solid rgba(154, 192, 255, .14); box-shadow: 0 44px 90px -36px rgba(0, 0, 0, .85); }
.panel { border-radius: 16px; background: #040914; border: 1px solid rgba(255, 255, 255, .08); overflow: hidden; }
.panel-tabs { display: flex; align-items: center; gap: 4px; padding: 10px 12px; border-bottom: 1px solid rgba(255, 255, 255, .08); background: rgba(255, 255, 255, .02); }
.win-dots { display: inline-flex; gap: 6px; margin-right: 10px; padding-left: 4px; }
.win-dots i { width: 10px; height: 10px; border-radius: 50%; }
.win-dots i:nth-child(1) { background: #e0665c; }
.win-dots i:nth-child(2) { background: #e6b450; }
.win-dots i:nth-child(3) { background: #5fc77c; }
.panel-tabs button { border: 0; background: none; color: var(--muted); padding: 6px 13px; border-radius: 999px; cursor: pointer; white-space: nowrap; font-size: 13px; transition: background .2s, color .2s; }
.panel-tabs button:hover { color: #fff; }
.panel-tabs button.active { background: rgba(255, 255, 255, .12); color: #fff; }
.panel-tabs .copy-btn { margin-left: auto; display: inline-flex; align-items: center; gap: 6px; }
.panel-body { height: 360px; display: flex; flex-direction: column; }
.panel-body :deep(.terminal) { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.panel-body :deep(.term-body) { flex: 1; height: auto !important; min-height: 0; }
pre { flex: 1; margin: 0; padding: 22px 24px; overflow: auto; font: 13.5px/1.75 ui-monospace, "Cascadia Code", Consolas, monospace; color: #dbe7f8; white-space: pre-wrap; word-break: break-all; }
pre code { all: unset; }

.points { list-style: none; margin: 0; padding: 0; display: grid; gap: 22px; }
.points li { color: var(--muted); line-height: 1.75; padding-left: 18px; border-left: 1.5px solid rgba(125, 232, 190, .35); font-size: 14.5px; }
.points strong { display: block; color: #fff; font-size: 16.5px; font-weight: 550; margin-bottom: 4px; }

/* 控制台画框：用深绿磨砂底托柔和承托浅色真实后台界面 */
.console-frame { padding: 16px; border-radius: 26px; background: linear-gradient(155deg, rgba(125, 232, 190, .14), rgba(255, 255, 255, .03)); border: 1px solid rgba(255, 255, 255, .1); box-shadow: 0 44px 90px -36px rgba(0, 0, 0, .85); }
.console { margin: 0; border-radius: 16px; overflow: hidden; background: #f4f6f5; color: var(--ink); box-shadow: 0 20px 44px -20px rgba(0, 0, 0, .65); }
.console-bar { display: flex; align-items: center; gap: 7px; padding: 11px 16px; background: #0b1614; color: #9fb3ad; font-size: 12px; }
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
.faq { --el-collapse-header-font-size: 16.5px; --el-collapse-header-height: 66px; --el-collapse-border-color: var(--line); --el-collapse-header-bg-color: transparent; --el-collapse-content-bg-color: transparent; --el-collapse-header-text-color: #fff; border-top: 0; border-bottom-color: var(--line); }
.faq :deep(.el-collapse-item__header) { font-weight: 500; }
.faq :deep(.el-collapse-item__arrow) { color: rgba(255, 255, 255, .6); }
.faq :deep(.el-collapse-item__content) { color: var(--muted); line-height: 1.85; max-width: 42em; font-size: 14.5px; }

/* 收尾：大圆角绿色环境光展板 */
.closing { background: radial-gradient(72% 130% at 18% 0%, #124d3d 0%, #051914 48%, #020808 100%); border-radius: 36px 36px 0 0; padding: 116px 0 124px; }
.closing h2 { font-size: clamp(36px, 4.6vw, 64px); font-weight: 500; }
.closing h2 span { color: var(--mint); }
.steps { list-style: none; padding: 0; margin: 48px 0 44px; max-width: 920px; display: grid; grid-template-columns: repeat(3, 1fr); }
.steps li { padding-right: 28px; display: grid; gap: 8px; }
.steps li + li { padding-left: 28px; border-left: 1px solid rgba(255, 255, 255, .12); }
.steps span { font-size: 13px; font-weight: 500; color: rgba(255, 255, 255, .48); letter-spacing: .12em; }
.steps strong { font-size: 19px; font-weight: 550; color: #fff; }
.steps p { color: var(--muted); font-size: 14.5px; }
.closing-ctas { display: flex; gap: 16px; align-items: center; flex-wrap: wrap; }

/* 页脚：纯黑底，干净中性灰 */
.home-footer { background: #000; color: #a1a1aa; padding: 72px 0 36px; border-top: 1px solid rgba(255, 255, 255, .08); }
.footer-grid { display: grid; grid-template-columns: 1.6fr repeat(4, 1fr); gap: 32px; }
.footer-grid h4 { margin: 0 0 16px; color: #fff; font-size: 14px; font-weight: 550; }
.footer-grid a { display: flex; align-items: center; gap: 6px; color: inherit; text-decoration: none; margin-bottom: 11px; font-size: 14px; transition: color .2s; }
.footer-grid a:hover { color: #fff; }
.footer-brand strong { font-size: 20px; font-weight: 600; color: #fff; }
.footer-brand p { margin: 10px 0 16px; }
.footer-bottom { margin-top: 48px; padding-top: 22px; border-top: 1px solid rgba(255, 255, 255, .08); display: flex; justify-content: space-between; font-size: 12px; color: #71717a; }

@media (max-width: 960px) {
  .wrap { padding: 0 18px; }
  .d-sec { padding: 76px 0; }
  .d-sec.band-navy, .d-sec.band-pine, .d-sec.band-dark, .closing { border-radius: 24px 24px 0 0; padding: 80px 0 96px; }
  .choose { padding: 64px 0 76px; }
  .statement { margin-bottom: 36px; }
  .compare { grid-template-columns: 1fr; grid-template-rows: none; gap: 20px; }
  .col { grid-row: auto; grid-template-rows: none; padding: 28px 22px; border-radius: 22px; }
  .price strong { font-size: 38px; }
  .sessions li { grid-template-columns: 78px 16px 1fr 56px; padding: 8px 10px; }
  .split, .faq-grid { grid-template-columns: 1fr; gap: 36px; }
  .panel-stage, .console-frame { padding: 10px; border-radius: 20px; }
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
