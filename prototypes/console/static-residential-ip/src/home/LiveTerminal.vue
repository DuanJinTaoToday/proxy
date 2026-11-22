<script setup lang="ts">
// 实时接入终端（来自方案 C）：循环演示 SOCKS5 连接、出口在各国之间轮换与按字节计量。
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { sample } from './home-demo'

withDefaults(defineProps<{ height?: number, embedded?: boolean }>(), { height: 300, embedded: false })

const exits = [
  { sid: 'a7k2', ip: '73.41.208.16', city: 'Los Angeles', country: 'US', label: '美国 洛杉矶', ms: 312, bytes: 18_432 },
  { sid: 'q9m1', ip: '86.12.77.204', city: 'London', country: 'GB', label: '英国 伦敦', ms: 268, bytes: 21_904 },
  { sid: 'z3p8', ip: '126.80.4.91', city: 'Tokyo', country: 'JP', label: '日本 东京', ms: 187, bytes: 16_220 },
  { sid: 'h5d0', ip: '189.6.33.150', city: 'São Paulo', country: 'BR', label: '巴西 圣保罗', ms: 401, bytes: 19_876 },
  { sid: 'w2c6', ip: '91.64.120.38', city: 'Frankfurt', country: 'DE', label: '德国 法兰克福', ms: 233, bytes: 17_548 },
]

interface Line { kind: 'cmd' | 'out' | 'ok', text: string }
const lines = ref<Line[]>([])
const typing = ref('')
const metered = ref(1_284_639_120)
const current = ref(exits[0])
const timers: number[] = []
let alive = true
const wait = (ms: number) => new Promise<void>(resolve => { timers.push(window.setTimeout(resolve, ms)) })

async function loop() {
  for (let i = 0; alive; i = (i + 1) % exits.length) {
    const e = exits[i]
    const cmd = `curl -x socks5h://${sample.username}-sid-${e.sid}:••••@${sample.host}:${sample.port} ipinfo.io`
    for (let c = 1; c <= cmd.length && alive; c += 2) { typing.value = cmd.slice(0, c); await wait(16) }
    typing.value = ''
    lines.value.push({ kind: 'cmd', text: cmd })
    await wait(420)
    lines.value.push({ kind: 'out', text: `{ "ip": "${e.ip}", "city": "${e.city}", "country": "${e.country}", "type": "residential" }` })
    current.value = e
    metered.value += e.bytes
    await wait(260)
    lines.value.push({ kind: 'ok', text: `✓ 出口 ${e.label} · ${e.ms} ms · 计量 ${e.bytes.toLocaleString('en-US')} 字节` })
    if (lines.value.length > 9) lines.value.splice(0, lines.value.length - 9)
    await wait(2200)
  }
}
onMounted(loop)
onBeforeUnmount(() => { alive = false; timers.forEach(clearTimeout) })
</script>

<template>
  <div class="terminal" :class="{ embedded }" aria-label="接入演示">
    <div v-if="!embedded" class="term-bar"><i /><i /><i /><span>proxy-demo — zsh</span></div>
    <div class="term-body" :style="{ height: `${height}px` }">
      <p v-for="(l, i) in lines" :key="i" :class="l.kind"><template v-if="l.kind === 'cmd'"><b>$</b> </template>{{ l.text }}</p>
      <p class="cmd"><b>$</b> {{ typing }}<span class="caret" /></p>
    </div>
    <div class="term-foot">
      <div><span>当前出口</span><strong>{{ current.label }}</strong></div>
      <div><span>出口 IP</span><strong class="mono">{{ current.ip }}</strong></div>
      <div><span>累计计量</span><strong class="mono">{{ metered.toLocaleString('en-US') }} B</strong></div>
    </div>
  </div>
</template>

<style scoped>
.terminal { border-radius: 16px; background: rgba(5, 14, 12, .88); border: 1px solid rgba(179, 240, 215, .2); box-shadow: 0 40px 80px -30px rgba(0, 0, 0, .8), 0 0 0 6px rgba(179, 240, 215, .04); overflow: hidden; backdrop-filter: blur(8px); color: #fff; }
.terminal.embedded { border: 0; border-radius: 0; box-shadow: none; background: transparent; backdrop-filter: none; }
.term-bar { display: flex; align-items: center; gap: 7px; padding: 12px 16px; border-bottom: 1px solid rgba(179, 240, 215, .1); }
.term-bar i { width: 11px; height: 11px; border-radius: 50%; background: #3b4d48; }
.term-bar i:first-child { background: #e0665c; } .term-bar i:nth-child(2) { background: #e6b450; } .term-bar i:nth-child(3) { background: #5fc77c; }
.term-bar span { margin-left: 10px; font-size: 12px; color: #6f8d84; }
.term-body { padding: 16px 18px; overflow: hidden; display: flex; flex-direction: column; justify-content: flex-end; gap: 6px; font: 12.5px/1.6 ui-monospace, "Cascadia Code", Consolas, monospace; }
.term-body p { word-break: break-all; }
.term-body .cmd { color: #d9e6e2; } .term-body b { color: var(--mint); }
.term-body .out { color: #8fb0a6; }
.term-body .ok { color: #5fe3b0; margin-bottom: 6px; }
.caret { display: inline-block; width: 7px; height: 14px; background: var(--mint); vertical-align: -2px; animation: blink 1s steps(1) infinite; }
@keyframes blink { 50% { opacity: 0 } }
.term-foot { display: grid; grid-template-columns: repeat(3, 1fr); border-top: 1px solid rgba(179, 240, 215, .1); }
.term-foot div { padding: 14px 18px; display: grid; gap: 4px; }
.term-foot div + div { border-left: 1px solid rgba(179, 240, 215, .1); }
.term-foot span { font-size: 12px; color: #6f8d84; }
.term-foot strong { color: #fff; font-size: 15px; }
.mono { font-family: ui-monospace, Consolas, monospace; }
</style>
