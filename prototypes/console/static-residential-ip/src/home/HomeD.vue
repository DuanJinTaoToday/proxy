<script setup lang="ts">
// 方案 D 首屏以下：参照 Shopify 旗舰视觉语言重构。
// 五幕不同构图交替：双大画幅视觉展板（上图下文）→ 深海蓝通栏多层终端舞台 → 深松绿暗色 Bento 数据画板 → 极简 FAQ → 翠绿光穹收尾。
// 全部数据来自控制台原型同一份演示状态并标为示例；不放无法核实的性能数字和客户评价。
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElCollapse, ElCollapseItem, ElMessage } from 'element-plus'
import { Bell, CopyDocument, Download, Promotion } from '@element-plus/icons-vue'
import LiveTerminal from './LiveTerminal.vue'
import { brandName, faqs, sample, telegram } from './home-demo'
import { products, staticCountries } from './home-products'
import { dayMs, formatExpiry, remainingLabel, statusOf } from '../demo'
import { dailyUsage, ips, subAccounts, trafficBalanceGb, usageUpdatedAt } from '../state'

// ── 1. 选型：精简卡片定位，视觉主体占 65% ────────────────────────
const copy = {
  dynamic: {
    desc: '数据采集、广告核对与价格监控。按 GB 计费，流量长期有效，用完为止。',
    link: '查看动态价格',
  },
  static: {
    desc: '跨境店铺与社媒矩阵长期稳定登录。独享或共享，到期前续费保留原 IP。',
    link: '查看静态价格',
  },
} as const
const columns = computed(() => products.value.map(p => ({ ...p, ...copy[p.key] })))

// 动态同心光环轨道上的 5 个示例出口节点
const orbitNodes = [
  { sid: 'a7k2', city: '洛杉矶', code: 'US', ip: '73.41.208.16', ms: 312, x: '18%', y: '24%' },
  { sid: 'q9m1', city: '伦敦', code: 'GB', ip: '86.12.77.204', ms: 268, x: '78%', y: '20%' },
  { sid: 'z3p8', city: '东京', code: 'JP', ip: '126.80.4.91', ms: 187, x: '84%', y: '56%' },
  { sid: 'w2c6', city: '法兰克福', code: 'DE', ip: '91.64.120.38', ms: 233, x: '22%', y: '68%' },
  { sid: 'h5d0', city: '圣保罗', code: 'BR', ip: '189.6.33.150', ms: 401, x: '66%', y: '76%' },
]
const activeOrbit = ref(0)
const currentOrbit = computed(() => orbitNodes[activeOrbit.value])

// 静态示意：左侧地区轨 + 中右悬浮纯白保号卡片
const staticRegions = [
  { code: 'US', label: '美国' },
  { code: 'JP', label: '日本' },
  { code: 'KR', label: '韩国', active: true },
  { code: 'GB', label: '英国' },
  { code: 'DE', label: '德国' },
]
const renewTarget = computed(() => ips.value.find(ip => statusOf(ip) === 'expiring') ?? ips.value[0])
const day = (t: number) => formatExpiry(t).slice(0, 10)

// ── 2. 接入：通栏深海蓝多层终端舞台 ────────────────────────────
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

// ── 3. 管理：暗色 Bento 可视化数据画板 ────────────────────────
const maxDay = Math.max(...dailyUsage.map(d => d.gb))
const weekGb = dailyUsage.reduce((s, d) => s + d.gb, 0)
const gb = (n: number) => n.toFixed(2)

let timer = 0
onMounted(() => { timer = window.setInterval(() => { activeOrbit.value = (activeOrbit.value + 1) % orbitNodes.length }, 2500) })
onBeforeUnmount(() => clearInterval(timer))

const openFaq = ref(['0'])
</script>

<template>
  <div class="d-page">
    <!-- 1 选型：上图下文大画幅沉浸展板 -->
    <section id="products" class="d-sec choose">
      <div class="wrap">
        <h2 class="statement">需要轮换出口，选动态。<span>需要固定身份，选静态。</span></h2>
        <div class="showcase-grid">
          <article v-for="c in columns" :key="c.key" class="stage-card" :class="c.key">
            <!-- 动态卡片主视觉：同心发光轨道 + 悬浮出口胶囊 -->
            <div v-if="c.key === 'dynamic'" class="visual-stage orbit-stage" aria-label="出口轮换示意">
              <div class="ring ring-1" aria-hidden="true" />
              <div class="ring ring-2" aria-hidden="true" />
              <div class="ring ring-3" aria-hidden="true" />
              <div class="orbit-core">
                <span>SOCKS5</span>
                <small>住宅网关</small>
              </div>
              <div
                v-for="(node, idx) in orbitNodes"
                :key="node.sid"
                class="orbit-pill"
                :class="{ active: idx === activeOrbit }"
                :style="{ left: node.x, top: node.y }"
              >
                <i />
                <span>{{ node.city }}</span>
              </div>
              <div class="stage-bar">
                <code>{{ sample.username }}<b>-sid-{{ currentOrbit.sid }}</b></code>
                <span class="stage-live">→ {{ currentOrbit.city }}</span>
              </div>
            </div>

            <!-- 静态卡片主视觉：深底地区轨 + 悬浮高对比纯白保号卡片 -->
            <div v-else class="visual-stage static-stage" aria-label="保号续费示意">
              <div class="region-rail" aria-hidden="true">
                <span v-for="r in staticRegions" :key="r.code" :class="{ active: r.active }">
                  <b>{{ r.code }}</b>
                  <small>{{ r.label }}</small>
                </span>
              </div>
              <div class="white-ip-card">
                <div class="wic-head">
                  <span class="wic-region">{{ renewTarget.region }}</span>
                  <span class="wic-warn">{{ remainingLabel(renewTarget) }}</span>
                </div>
                <code class="wic-addr">{{ renewTarget.address }}:{{ renewTarget.port }}</code>
                <p class="wic-sub">独享静态住宅 · 续费后原 IP 不变</p>
                <div class="wic-btn">保号续费 30 天</div>
                <div class="wic-dates">
                  <span>原到期 {{ day(renewTarget.expiry) }}</span>
                  <b>→ {{ day(renewTarget.expiry + 30 * dayMs) }}</b>
                </div>
              </div>
            </div>

            <!-- 底部极简编辑体文案与起价 -->
            <div class="stage-copy">
              <div class="stage-title-row">
                <h3>{{ c.name }}</h3>
                <span v-if="!c.onSale" class="closed-tag">暂未开放购买</span>
              </div>
              <p class="stage-desc">{{ c.desc }}</p>
              <div class="stage-footer">
                <div class="stage-price">
                  <template v-if="c.onSale && c.price">
                    <strong>{{ c.price }}</strong><span>{{ c.priceUnit }}</span>
                  </template>
                  <span v-else class="soon">开售后公布报价</span>
                </div>
                <a v-if="c.onSale" class="stage-link" :href="c.href">{{ c.link }} →</a>
                <a v-else class="stage-link muted" href="#faq">了解产品 →</a>
              </div>
            </div>
          </article>
        </div>
        <p class="footnote">静态 IP 目前可选 {{ staticCountries }} 个国家和地区，价格与库存以购买页实时报价为准。两种代理共用一个账户与钱包。</p>
      </div>
    </section>

    <!-- 2 接入：通栏深海蓝多层终端舞台（打破左文右图） -->
    <section id="integration" class="d-sec band-navy">
      <div class="wrap">
        <div class="section-head">
          <div class="head-copy">
            <h2>买完，复制一行配置就能用。</h2>
            <p class="sub">在控制台选定代理子账号，复制网关地址与账密，填入任意支持 SOCKS5 的浏览器、工具或代码。</p>
          </div>
          <div class="dev-tabs" role="tablist" aria-label="代码示例切换">
            <button v-for="t in tabs" :key="t" type="button" :class="{ active: t === tab }" @click="tab = t">{{ t }}</button>
            <button v-if="tab !== live" type="button" class="copy-pill" @click="copyCode"><CopyDocument />复制</button>
          </div>
        </div>

        <div class="dev-stage">
          <!-- 悬浮毛玻璃特性胶囊 -->
          <div class="dev-callouts">
            <span class="callout-pill"><i /><code>主账号名.子账号名</code> 自动轮换出口</span>
            <span class="callout-pill"><i /><code>-sid-会话标识</code> 同一会话沿用出口</span>
          </div>

          <!-- 主终端窗口 + 右下错位叠加的实时响应浮窗 -->
          <div class="dev-windows">
            <div class="dev-main">
              <div class="win-bar">
                <span class="win-dots" aria-hidden="true"><i /><i /><i /></span>
                <span class="win-title">{{ tab === live ? 'proxy-gateway — socks5h' : `example.${tab.toLowerCase()}` }}</span>
                <span class="win-tag">示例账号</span>
              </div>
              <div class="panel-body">
                <LiveTerminal v-if="tab === live" embedded />
                <pre v-else><code>{{ code }}</code></pre>
              </div>
            </div>

            <aside class="dev-float" aria-label="当前出口响应示例">
              <div class="float-head">
                <span class="float-dot" />
                <span>出口响应 · 实时路由</span>
                <em>{{ currentOrbit.ms }} ms</em>
              </div>
              <pre class="float-json">{
  "ip": "{{ currentOrbit.ip }}",
  "city": "{{ currentOrbit.city }}",
  "country": "{{ currentOrbit.code }}",
  "type": "residential",
  "session": "sid-{{ currentOrbit.sid }}"
}</pre>
              <div class="float-foot">
                <span>协议 SOCKS5</span>
                <span>1:1 按总字节计量</span>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </section>

    <!-- 3 查账与管理：深松绿暗色 Bento 可视化数据画板 -->
    <section id="manage" class="d-sec band-pine">
      <div class="wrap">
        <div class="section-head">
          <div class="head-copy">
            <h2>每 1 GB 用在哪，都查得到。</h2>
            <p class="sub">按总字节 1:1 扣减（1 GB = 10⁹ 字节）。控制台按天统计用量、按业务拆分子账号，随时导出 CSV 对账。</p>
          </div>
          <span class="sample-pill">控制台示例数据 · 更新于 {{ formatExpiry(usageUpdatedAt) }}</span>
        </div>

        <div class="bento">
          <!-- 大卡：可用流量 + 近 7 天发光柱状图 -->
          <article class="bento-card bento-main">
            <div class="bento-top">
              <div>
                <span class="bento-label">可用流量余额</span>
                <strong class="bento-hero-num">{{ gb(trafficBalanceGb) }}<em> GB</em></strong>
              </div>
              <div class="bento-kpi-right">
                <span class="bento-label">近 7 天累计扣减</span>
                <strong>{{ gb(weekGb) }} GB</strong>
              </div>
            </div>
            <div class="dark-chart" aria-label="近 7 天流量柱状图">
              <div v-for="d in dailyUsage" :key="d.day" class="bar-col">
                <span class="bar-val">{{ d.gb.toFixed(1) }}</span>
                <div class="bar-track">
                  <i :style="{ height: `${Math.max(12, (d.gb / maxDay) * 100)}%` }" />
                </div>
                <span class="bar-day">{{ d.day.slice(5) }}</span>
              </div>
            </div>
            <div class="bento-bottom">
              <span>按总字节 1:1 扣减 · 流量长期有效</span>
              <span class="csv-pill"><Download />支持导出 CSV</span>
            </div>
          </article>

          <!-- 右上卡：子账号用量分账 -->
          <article class="bento-card bento-subs">
            <div class="bento-card-head">
              <h3>按业务分子账号</h3>
              <span>最多 10 个</span>
            </div>
            <ul class="sub-list">
              <li v-for="a in subAccounts" :key="a.name">
                <div class="sub-row-top">
                  <div class="sub-id">
                    <code>{{ a.name }}</code>
                    <small>{{ a.remark || '未备注' }}</small>
                  </div>
                  <span class="sub-usage" :class="{ warn: a.limitGb && a.usedGb >= a.limitGb, off: !a.active }">
                    {{ gb(a.usedGb) }} / {{ a.limitGb ? `${a.limitGb} GB` : '不限' }}
                  </span>
                </div>
                <div class="sub-bar">
                  <i
                    :class="{ full: a.limitGb && a.usedGb >= a.limitGb, off: !a.active }"
                    :style="{ width: `${a.limitGb ? Math.min(100, (a.usedGb / a.limitGb) * 100) : 56}%` }"
                  />
                </div>
              </li>
            </ul>
          </article>

          <!-- 右下卡：余额与到期主动提醒 -->
          <article class="bento-card bento-alerts">
            <div class="bento-card-head">
              <h3>余额与到期提醒</h3>
              <span><Bell />控制台通知</span>
            </div>
            <div class="alert-stack">
              <div class="alert-item">
                <span class="alert-dot amber" />
                <div>
                  <strong>低流量阈值提醒</strong>
                  <p>可用流量低于你设定的 GB 阈值时，触发一次提醒。</p>
                </div>
              </div>
              <div class="alert-item">
                <span class="alert-dot cyan" />
                <div>
                  <strong>静态 IP 到期前 5 天提醒</strong>
                  <p>到期前续费保留原 IP，支持单个或批量顺延。</p>
                </div>
              </div>
            </div>
          </article>
        </div>
      </div>
    </section>

    <!-- 4 FAQ -->
    <section id="faq" class="d-sec band-dark">
      <div class="wrap faq-grid">
        <div class="copy">
          <h2>常见问题</h2>
          <p class="note">还有其他疑问？直接通过 Telegram 联系客服。</p>
        </div>
        <ElCollapse v-model="openFaq" class="faq">
          <ElCollapseItem v-for="(f, i) in faqs" :key="f.q" :name="String(i)" :title="f.q">{{ f.a }}</ElCollapseItem>
        </ElCollapse>
      </div>
    </section>

    <!-- 5 收尾：大圆角翠绿光穹 + 极简三步 -->
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
          <p>为出海团队提供真实住宅代理网络。</p>
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
  --bg: #020808; --navy: #020b1c; --pine: #031914; --line: rgba(255, 255, 255, .09);
  --text: #f3f7f6; --muted: rgba(255, 255, 255, .64); --faint: rgba(255, 255, 255, .42);
  --cyan: #7fd4f5; --emerald: #6ee7b7;
  background: var(--bg); color: var(--text);
}
.wrap { max-width: 1240px; margin: 0 auto; padding: 0 24px; }
.d-sec { position: relative; padding: 120px 0; }
h2 { font-size: clamp(34px, 4.2vw, 58px); font-weight: 450; line-height: 1.12; letter-spacing: -.03em; color: #fff; margin: 0; }
.sub { margin-top: 16px; max-width: 34em; color: var(--muted); font-size: 16.5px; line-height: 1.65; }
.note { color: var(--muted); line-height: 1.75; font-size: 15px; margin-top: 16px; }
.pill { display: inline-flex; align-items: center; justify-content: center; gap: 8px; height: 54px; padding: 0 32px; border-radius: 9999px; font-size: 16px; text-decoration: none; white-space: nowrap; transition: background .2s, border-color .2s, transform .15s; }
.pill:hover { transform: translateY(-1px); }
.pill.primary { background: #fff; color: #050909; font-weight: 600; }
.pill.primary:hover { background: #e6eae9; }
.pill.ghost { border: 1px solid rgba(255, 255, 255, .28); background: rgba(255, 255, 255, .04); color: #fff; font-weight: 500; }
.pill.ghost:hover { border-color: rgba(255, 255, 255, .65); background: rgba(255, 255, 255, .1); }
.pill svg { width: 16px; height: 16px; }

/* ── 1. 选型：上图下文大画幅沉浸展板 ─────────────────────────── */
.choose { padding: 112px 0 132px; }
.statement { max-width: 15em; margin-bottom: 60px; }
.statement span { display: block; color: var(--faint); }
.showcase-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 28px; }
.stage-card { display: flex; flex-direction: column; border-radius: 28px; overflow: hidden; background: #070d0c; border: 1px solid rgba(255, 255, 255, .08); }
.visual-stage { position: relative; height: 350px; overflow: hidden; display: flex; align-items: center; justify-content: center; }

/* 动态卡视觉：同心光环 + 悬浮出口胶囊 */
.orbit-stage { background: radial-gradient(75% 75% at 50% 48%, rgba(28, 128, 92, .32) 0%, #071411 62%, #050c0a 100%); }
.ring { position: absolute; border-radius: 50%; border: 1px solid rgba(130, 235, 192, .16); pointer-events: none; }
.ring-1 { width: 148px; height: 148px; border-color: rgba(130, 235, 192, .32); box-shadow: 0 0 40px rgba(82, 224, 168, .14) inset; }
.ring-2 { width: 256px; height: 256px; }
.ring-3 { width: 376px; height: 376px; border-color: rgba(130, 235, 192, .09); }
.orbit-core { position: relative; z-index: 2; width: 92px; height: 92px; border-radius: 24px; background: linear-gradient(145deg, rgba(130, 235, 192, .22), rgba(8, 22, 18, .9)); border: 1px solid rgba(130, 235, 192, .42); box-shadow: 0 20px 50px rgba(0, 0, 0, .65), 0 0 36px rgba(95, 227, 176, .22); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 2px; }
.orbit-core span { font: 600 13px ui-monospace, Consolas, monospace; color: #fff; letter-spacing: .04em; }
.orbit-core small { font-size: 11px; color: var(--emerald); }
.orbit-pill { position: absolute; transform: translate(-50%, -50%); z-index: 3; display: inline-flex; align-items: center; gap: 7px; padding: 7px 14px; border-radius: 999px; background: rgba(6, 16, 14, .78); border: 1px solid rgba(255, 255, 255, .14); color: rgba(255, 255, 255, .65); font-size: 12.5px; backdrop-filter: blur(10px); transition: all .45s cubic-bezier(.22, 1, .36, 1); white-space: nowrap; }
.orbit-pill i { width: 6px; height: 6px; border-radius: 50%; background: rgba(255, 255, 255, .28); transition: background .4s, box-shadow .4s; }
.orbit-pill.active { background: #fff; color: #050909; border-color: #fff; font-weight: 600; transform: translate(-50%, -50%) scale(1.08); box-shadow: 0 14px 32px rgba(0, 0, 0, .55), 0 0 24px rgba(130, 235, 192, .35); }
.orbit-pill.active i { background: #10b981; box-shadow: 0 0 0 4px rgba(16, 185, 129, .22); }
.stage-bar { position: absolute; inset: auto 18px 16px; z-index: 4; display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 9px 14px; border-radius: 12px; background: rgba(4, 10, 9, .82); border: 1px solid rgba(255, 255, 255, .1); backdrop-filter: blur(12px); font-size: 12.5px; }
.stage-bar code { font-family: ui-monospace, Consolas, monospace; color: rgba(255, 255, 255, .74); background: none; padding: 0; }
.stage-bar code b { color: var(--emerald); font-weight: 600; }
.stage-live { color: #fff; font-weight: 500; white-space: nowrap; }

/* 静态卡视觉：深青底 + 左侧地区轨 + 悬浮高对比纯白卡 */
.static-stage { background: radial-gradient(78% 78% at 72% 38%, rgba(44, 146, 196, .28) 0%, #07141b 60%, #050a0f 100%); gap: 22px; padding: 24px 28px; }
.region-rail { display: flex; flex-direction: column; gap: 8px; }
.region-rail span { width: 56px; padding: 8px 0; border-radius: 14px; background: rgba(255, 255, 255, .05); border: 1px solid rgba(255, 255, 255, .08); display: flex; flex-direction: column; align-items: center; gap: 1px; color: rgba(255, 255, 255, .45); transition: all .25s; }
.region-rail span b { font-size: 12px; font-weight: 600; letter-spacing: .04em; }
.region-rail span small { font-size: 10px; }
.region-rail span.active { background: rgba(255, 255, 255, .94); color: #050909; border-color: #fff; box-shadow: 0 12px 28px rgba(0, 0, 0, .5); transform: scale(1.06); }
.white-ip-card { width: 296px; padding: 22px 22px 20px; border-radius: 20px; background: #fff; color: #091311; box-shadow: 0 32px 70px -20px rgba(0, 0, 0, .85), 0 0 0 1px rgba(255, 255, 255, .3); display: grid; gap: 10px; }
.wic-head { display: flex; align-items: center; justify-content: space-between; font-size: 12.5px; }
.wic-region { font-weight: 600; color: #3d524c; }
.wic-warn { padding: 3px 9px; border-radius: 99px; background: #fef3c7; color: #92400e; font-weight: 600; font-size: 11.5px; }
.wic-addr { font: 600 17px ui-monospace, Consolas, monospace; color: #050909; background: none; padding: 0; letter-spacing: -.01em; }
.wic-sub { font-size: 12.5px; color: #5c706a; margin: 0; }
.wic-btn { margin-top: 4px; height: 40px; border-radius: 999px; background: #0a1f1c; color: #fff; font-size: 13.5px; font-weight: 600; display: flex; align-items: center; justify-content: center; }
.wic-dates { display: flex; justify-content: space-between; font-size: 11.5px; color: #64748b; padding-top: 4px; border-top: 1px solid #eef2f0; }
.wic-dates b { color: #0f766e; font-weight: 600; }

/* 卡片底部精简文字区 */
.stage-copy { padding: 32px 34px 36px; display: flex; flex-direction: column; flex: 1; }
.stage-title-row { display: flex; align-items: center; gap: 12px; margin-bottom: 10px; }
.stage-title-row h3 { font-size: 28px; font-weight: 500; letter-spacing: -.02em; color: #fff; margin: 0; }
.closed-tag { font-size: 12px; font-weight: 500; color: #ffd28a; background: rgba(255, 196, 102, .12); padding: 4px 10px; border-radius: 99px; }
.stage-desc { color: var(--muted); font-size: 15.5px; line-height: 1.65; margin: 0 0 28px; }
.stage-footer { margin-top: auto; padding-top: 22px; border-top: 1px solid rgba(255, 255, 255, .08); display: flex; align-items: baseline; justify-content: space-between; gap: 16px; }
.stage-price strong { font-size: 38px; font-weight: 450; letter-spacing: -.03em; color: #fff; font-variant-numeric: tabular-nums; margin-right: 4px; }
.stage-price span { font-size: 14px; color: var(--muted); }
.stage-price .soon { font-size: 16px; }
.stage-link { color: #fff; text-decoration: none; font-size: 15px; font-weight: 500; border-bottom: 1px solid rgba(255, 255, 255, .36); padding-bottom: 3px; white-space: nowrap; transition: border-color .2s, color .2s; }
.dynamic .stage-link:hover { color: var(--emerald); border-color: var(--emerald); }
.static .stage-link:hover { color: var(--cyan); border-color: var(--cyan); }
.stage-link.muted { color: var(--muted); border-color: rgba(255, 255, 255, .18); }
.footnote { margin-top: 24px; font-size: 13px; color: var(--faint); }

/* ── 2. 接入：深海蓝通栏多层终端舞台 ─────────────────────────── */
.band-navy {
  background:
    radial-gradient(58% 60% at 75% 18%, rgba(36, 108, 232, .26), transparent 62%),
    radial-gradient(40% 45% at 15% 85%, rgba(56, 189, 248, .12), transparent 60%),
    var(--navy);
  border-radius: 40px 40px 0 0;
  padding: 124px 0 156px;
}
.section-head { display: flex; align-items: flex-end; justify-content: space-between; gap: 32px; flex-wrap: wrap; margin-bottom: 44px; }
.dev-tabs { display: inline-flex; align-items: center; gap: 4px; padding: 5px; border-radius: 999px; background: rgba(255, 255, 255, .05); border: 1px solid rgba(255, 255, 255, .1); }
.dev-tabs button { border: 0; background: none; color: rgba(255, 255, 255, .68); padding: 8px 16px; border-radius: 999px; cursor: pointer; font-size: 13.5px; font-weight: 500; transition: all .2s; white-space: nowrap; }
.dev-tabs button:hover { color: #fff; }
.dev-tabs button.active { background: #fff; color: #050909; font-weight: 600; }
.dev-tabs .copy-pill { display: inline-flex; align-items: center; gap: 5px; color: #93c5fd; }
.dev-tabs .copy-pill svg { width: 14px; height: 14px; }

.dev-stage {
  position: relative; padding: 28px 28px 34px; border-radius: 30px;
  background:
    radial-gradient(circle at 1px 1px, rgba(148, 190, 255, .11) 1px, transparent 0) 0 0 / 24px 24px,
    linear-gradient(160deg, rgba(18, 46, 102, .45), rgba(4, 12, 30, .75));
  border: 1px solid rgba(148, 190, 255, .16);
  box-shadow: 0 48px 100px -36px rgba(0, 0, 0, .85);
}
.dev-callouts { display: flex; gap: 12px; flex-wrap: wrap; margin-bottom: 22px; }
.callout-pill { display: inline-flex; align-items: center; gap: 8px; padding: 7px 14px; border-radius: 999px; background: rgba(10, 24, 54, .78); border: 1px solid rgba(148, 190, 255, .24); color: rgba(255, 255, 255, .82); font-size: 13px; backdrop-filter: blur(12px); }
.callout-pill i { width: 7px; height: 7px; border-radius: 50%; border: 1.5px solid #60a5fa; }
.callout-pill code { font: 500 12.5px ui-monospace, Consolas, monospace; color: #bfdbfe; background: none; padding: 0; }

.dev-windows { position: relative; display: grid; grid-template-columns: minmax(0, 1.55fr) minmax(0, .95fr); gap: 22px; align-items: end; }
.dev-main { border-radius: 18px; background: #040812; border: 1px solid rgba(255, 255, 255, .1); overflow: hidden; box-shadow: 0 32px 70px -24px rgba(0, 0, 0, .85); }
.win-bar { display: flex; align-items: center; gap: 12px; padding: 12px 16px; border-bottom: 1px solid rgba(255, 255, 255, .08); background: rgba(255, 255, 255, .02); }
.win-dots { display: inline-flex; gap: 6px; }
.win-dots i { width: 10px; height: 10px; border-radius: 50%; }
.win-dots i:nth-child(1) { background: #e0665c; }
.win-dots i:nth-child(2) { background: #e6b450; }
.win-dots i:nth-child(3) { background: #5fc77c; }
.win-title { font: 12px ui-monospace, Consolas, monospace; color: rgba(255, 255, 255, .52); }
.win-tag { margin-left: auto; font-size: 11px; color: #93c5fd; border: 1px solid rgba(147, 197, 253, .28); border-radius: 99px; padding: 2px 9px; }
.panel-body { height: 360px; display: flex; flex-direction: column; }
.panel-body :deep(.terminal) { flex: 1; display: flex; flex-direction: column; min-height: 0; }
.panel-body :deep(.term-body) { flex: 1; height: auto !important; min-height: 0; }
.panel-body pre { flex: 1; margin: 0; padding: 24px 26px; overflow: auto; font: 14px/1.8 ui-monospace, "Cascadia Code", Consolas, monospace; color: #dbe7f8; white-space: pre-wrap; word-break: break-all; }
.panel-body pre code { all: unset; }

/* 右侧悬浮响应窗口：制造多层 Z 轴景深 */
.dev-float { border-radius: 18px; background: rgba(7, 16, 36, .92); border: 1px solid rgba(147, 197, 253, .22); box-shadow: 0 36px 80px -24px rgba(0, 0, 0, .9); overflow: hidden; backdrop-filter: blur(16px); }
.float-head { display: flex; align-items: center; gap: 8px; padding: 13px 18px; border-bottom: 1px solid rgba(255, 255, 255, .08); font-size: 12.5px; color: rgba(255, 255, 255, .8); }
.float-dot { width: 8px; height: 8px; border-radius: 50%; background: #34d399; box-shadow: 0 0 10px #34d399; }
.float-head em { margin-left: auto; font-style: normal; font-family: ui-monospace, Consolas, monospace; color: #93c5fd; font-size: 12px; }
.float-json { margin: 0; padding: 20px 20px 22px; font: 13px/1.85 ui-monospace, Consolas, monospace; color: #a5f3fc; background: transparent; overflow: auto; }
.float-foot { display: flex; justify-content: space-between; padding: 12px 18px; border-top: 1px solid rgba(255, 255, 255, .08); font-size: 12px; color: rgba(255, 255, 255, .52); }

/* ── 3. 管理：深松绿暗色 Bento 数据画板 ──────────────────────── */
.band-pine {
  background: radial-gradient(60% 60% at 78% 18%, rgba(22, 134, 98, .24), transparent 64%), var(--pine);
  border-radius: 40px 40px 0 0;
  margin-top: -36px;
  padding: 124px 0 156px;
}
.sample-pill { font-size: 12.5px; color: rgba(255, 255, 255, .64); padding: 8px 16px; border-radius: 999px; border: 1px solid rgba(255, 255, 255, .12); background: rgba(0, 0, 0, .25); white-space: nowrap; }
.bento { display: grid; grid-template-columns: minmax(0, 1.32fr) minmax(0, 1fr); grid-template-rows: auto auto; gap: 22px; }
.bento-card { padding: 30px 32px; border-radius: 26px; background: rgba(5, 16, 13, .72); border: 1px solid rgba(255, 255, 255, .09); backdrop-filter: blur(12px); display: flex; flex-direction: column; }
.bento-main { grid-row: span 2; background: radial-gradient(85% 70% at 85% 0%, rgba(45, 194, 140, .14), rgba(5, 16, 13, .82) 62%); }
.bento-top { display: flex; justify-content: space-between; align-items: flex-start; gap: 20px; margin-bottom: 34px; }
.bento-label { display: block; font-size: 12.5px; color: var(--faint); margin-bottom: 6px; }
.bento-hero-num { font-size: clamp(38px, 4vw, 56px); font-weight: 450; letter-spacing: -.03em; color: #fff; font-variant-numeric: tabular-nums; line-height: 1; }
.bento-hero-num em { font-style: normal; font-size: 20px; color: var(--emerald); font-weight: 500; }
.bento-kpi-right { text-align: right; }
.bento-kpi-right strong { font-size: 24px; font-weight: 500; color: rgba(255, 255, 255, .88); font-variant-numeric: tabular-nums; }

.dark-chart { flex: 1; min-height: 210px; display: flex; align-items: flex-end; gap: 16px; padding: 20px 10px 8px; border-radius: 18px; background: rgba(0, 0, 0, .26); border: 1px solid rgba(255, 255, 255, .06); }
.bar-col { flex: 1; height: 100%; display: flex; flex-direction: column; align-items: center; gap: 8px; }
.bar-val { font: 11.5px ui-monospace, Consolas, monospace; color: rgba(255, 255, 255, .58); }
.bar-track { flex: 1; width: 100%; display: flex; align-items: flex-end; justify-content: center; }
.bar-track i { width: 100%; max-width: 40px; border-radius: 8px 8px 3px 3px; background: linear-gradient(180deg, #6ee7b7 0%, rgba(20, 148, 108, .55) 100%); box-shadow: 0 0 24px rgba(110, 231, 183, .18); transition: filter .2s; }
.bar-col:hover .bar-track i { filter: brightness(1.18); }
.bar-day { font-size: 11.5px; color: var(--faint); }
.bento-bottom { margin-top: 22px; display: flex; align-items: center; justify-content: space-between; font-size: 13px; color: var(--muted); }
.csv-pill { display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 999px; background: rgba(110, 231, 183, .12); color: var(--emerald); font-weight: 500; font-size: 12.5px; }
.csv-pill svg { width: 14px; height: 14px; }

.bento-card-head { display: flex; align-items: center; justify-content: space-between; margin-bottom: 20px; }
.bento-card-head h3 { font-size: 20px; font-weight: 500; color: #fff; margin: 0; }
.bento-card-head span { display: inline-flex; align-items: center; gap: 5px; font-size: 12px; color: var(--faint); }
.bento-card-head svg { width: 14px; height: 14px; }

.sub-list { list-style: none; margin: 0; padding: 0; display: grid; gap: 16px; }
.sub-row-top { display: flex; align-items: baseline; justify-content: space-between; gap: 12px; margin-bottom: 7px; }
.sub-id { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
.sub-id code { font: 500 13.5px ui-monospace, Consolas, monospace; color: #fff; background: none; padding: 0; }
.sub-id small { font-size: 12px; color: var(--faint); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.sub-usage { font: 12.5px ui-monospace, Consolas, monospace; color: rgba(255, 255, 255, .75); white-space: nowrap; }
.sub-usage.warn { color: #fcd34d; }
.sub-usage.off { color: var(--faint); }
.sub-bar { height: 6px; border-radius: 99px; background: rgba(255, 255, 255, .08); overflow: hidden; }
.sub-bar i { display: block; height: 100%; border-radius: inherit; background: linear-gradient(90deg, #10b981, #6ee7b7); }
.sub-bar i.full { background: linear-gradient(90deg, #d97706, #fcd34d); }
.sub-bar i.off { background: rgba(255, 255, 255, .22); }

.alert-stack { display: grid; gap: 12px; }
.alert-item { display: flex; align-items: flex-start; gap: 12px; padding: 14px 16px; border-radius: 16px; background: rgba(255, 255, 255, .04); border: 1px solid rgba(255, 255, 255, .07); }
.alert-dot { flex: none; width: 9px; height: 9px; border-radius: 50%; margin-top: 6px; }
.alert-dot.amber { background: #fbbf24; box-shadow: 0 0 12px rgba(251, 191, 36, .6); }
.alert-dot.cyan { background: #38bdf8; box-shadow: 0 0 12px rgba(56, 189, 248, .6); }
.alert-item strong { display: block; font-size: 14.5px; font-weight: 500; color: #fff; margin-bottom: 3px; }
.alert-item p { margin: 0; font-size: 13px; color: var(--muted); line-height: 1.55; }

/* ── 4. FAQ ──────────────────────────────────────────────────── */
.band-dark { background: var(--bg); border-radius: 40px 40px 0 0; margin-top: -36px; padding: 116px 0 120px; }
.faq-grid { display: grid; grid-template-columns: minmax(0, .75fr) minmax(0, 1.25fr); gap: 64px; }
.faq { --el-collapse-header-font-size: 16.5px; --el-collapse-header-height: 68px; --el-collapse-border-color: var(--line); --el-collapse-header-bg-color: transparent; --el-collapse-content-bg-color: transparent; --el-collapse-header-text-color: #fff; border-top: 0; border-bottom-color: var(--line); }
.faq :deep(.el-collapse-item__header) { font-weight: 500; }
.faq :deep(.el-collapse-item__arrow) { color: rgba(255, 255, 255, .55); }
.faq :deep(.el-collapse-item__content) { color: var(--muted); line-height: 1.85; max-width: 44em; font-size: 14.5px; }

/* ── 5. 收尾 & 纯黑页脚 ─────────────────────────────────────── */
.closing { background: radial-gradient(74% 130% at 22% 0%, #125241 0%, #051b15 46%, #020808 100%); border-radius: 40px 40px 0 0; padding: 120px 0 128px; }
.closing h2 { font-size: clamp(38px, 5vw, 70px); font-weight: 450; }
.closing h2 span { color: var(--emerald); }
.steps { list-style: none; padding: 0; margin: 52px 0 48px; max-width: 960px; display: grid; grid-template-columns: repeat(3, 1fr); }
.steps li { padding-right: 32px; display: grid; gap: 8px; }
.steps li + li { padding-left: 32px; border-left: 1px solid rgba(255, 255, 255, .12); }
.steps span { font-size: 13px; font-weight: 500; color: rgba(255, 255, 255, .44); letter-spacing: .12em; }
.steps strong { font-size: 20px; font-weight: 500; color: #fff; }
.steps p { color: var(--muted); font-size: 14.5px; margin: 0; }
.closing-ctas { display: flex; gap: 16px; align-items: center; flex-wrap: wrap; }

.home-footer { background: #000; color: #a1a1aa; padding: 76px 0 38px; border-top: 1px solid rgba(255, 255, 255, .08); }
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
  .band-navy, .band-pine, .band-dark, .closing { border-radius: 26px 26px 0 0; padding: 84px 0 100px; }
  .choose { padding: 72px 0 84px; }
  .statement { margin-bottom: 36px; }
  .showcase-grid, .dev-windows, .bento, .faq-grid { grid-template-columns: 1fr; }
  .bento-main { grid-row: auto; }
  .visual-stage { height: 310px; }
  .static-stage { padding: 18px; gap: 14px; }
  .region-rail { display: none; }
  .white-ip-card { width: 100%; max-width: 310px; }
  .stage-copy { padding: 26px 22px 28px; }
  .dev-stage { padding: 16px; border-radius: 22px; }
  .dev-tabs { overflow-x: auto; max-width: 100%; }
  .panel-body { height: 330px; }
  .bento-card { padding: 24px 20px; }
  .steps { grid-template-columns: 1fr; gap: 22px; }
  .steps li + li { padding-left: 0; border-left: 0; }
  .footer-grid { grid-template-columns: 1fr 1fr; }
  .footer-bottom { flex-direction: column; gap: 8px; }
}
</style>
