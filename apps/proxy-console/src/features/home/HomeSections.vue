<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElButton, ElCollapse, ElCollapseItem, ElMessage } from 'element-plus'
import { Bell, CopyDocument, Download } from '@element-plus/icons-vue'
import { hasSession } from '@/features/auth'
import LinkButton from '@/shared/ui/LinkButton.vue'
import { codeTabs, dailyUsage, exits, faqTopics, sample, snippets, subAccounts, type CodeTab } from './demo'

const { t, locale, n } = useI18n()
const selected = ref<CodeTab>('demo')
const code = computed(() => selected.value === 'demo' ? '' : snippets[selected.value])
const openFaq = ref(['expiry'])
const activeExit = ref(0)
const metered = ref(1_284_639_120)
const exit = computed(() => exits[activeExit.value])
const exitCity = computed(() => t(`marketing.cities.${exit.value.city}`))
const response = computed(() => JSON.stringify({
  ip: exit.value.ip,
  city: exitCity.value,
  country: exit.value.country,
  type: 'residential',
  session: `sid-${exit.value.sid}`,
}, null, 2))
const shortResponse = computed(() => JSON.stringify({ ip: exit.value.ip, city: exitCity.value, country: exit.value.country }))
const terminalCommand = computed(() => `curl -x socks5h://${sample.username}-sid-${exit.value.sid}:••••@${sample.host}:${sample.port} ipinfo.io`)
const maxDay = Math.max(...dailyUsage.map(day => day.gb))
const weekGb = dailyUsage.reduce((sum, day) => sum + day.gb, 0)
const gb = (value: number) => n(value, { minimumFractionDigits: 2, maximumFractionDigits: 2 })

let rotation: number | undefined
onMounted(() => {
  if (!matchMedia('(prefers-reduced-motion: reduce)').matches) {
    rotation = window.setInterval(() => {
      activeExit.value = (activeExit.value + 1) % exits.length
      metered.value += exit.value.bytes
    }, 2500)
  }
})
onUnmounted(() => clearInterval(rotation))

async function copyCode() {
  try {
    await navigator.clipboard.writeText(code.value)
    ElMessage.success(t('marketing.copied'))
  } catch {
    ElMessage.error(t('marketing.copyFailed'))
  }
}
</script>

<template>
  <section id="integration" class="d-sec band-navy">
    <div class="wrap">
      <div class="section-head">
        <div class="head-copy">
          <h2>{{ t('marketing.setupTitle') }}</h2>
          <p class="sub">{{ t('marketing.setupDesc') }}</p>
        </div>
        <div class="dev-tabs" role="group" :aria-label="t('marketing.codeChoice')">
          <ElButton v-for="tab in codeTabs" :key="tab" :class="{ active: tab === selected }" :aria-pressed="tab === selected" @click="selected = tab">
            {{ tab === 'demo' ? t('marketing.demo') : tab }}
          </ElButton>
          <ElButton v-if="selected !== 'demo'" class="copy-pill" :icon="CopyDocument" @click="copyCode">{{ t('marketing.copy') }}</ElButton>
        </div>
      </div>
      <div class="dev-stage">
        <div class="dev-callouts">
          <span class="callout-pill"><i /><code>{{ t('marketing.username') }}</code> {{ t('marketing.autoRotate') }}</span>
          <span class="callout-pill"><i /><code>{{ t('marketing.session') }}</code> {{ t('marketing.sameExit') }}</span>
        </div>
        <div class="dev-windows">
          <div class="dev-main">
            <div class="win-bar">
              <span class="win-dots" aria-hidden="true"><i /><i /><i /></span>
              <span class="win-title">{{ selected === 'demo' ? 'proxy-gateway / socks5h' : selected }}</span>
              <span class="win-tag">{{ t('marketing.sampleAccount') }}</span>
            </div>
            <div class="panel-body">
              <div v-if="selected === 'demo'" class="terminal">
                <div class="term-body">
                  <p class="term-command"><b>$</b> {{ terminalCommand }}</p>
                  <p>{{ shortResponse }}</p>
                  <p class="term-result">✓ {{ exitCity }} · {{ exit.ms }} ms · {{ n(exit.bytes) }} B</p>
                  <p class="term-command"><b>$</b> <span class="caret" /></p>
                </div>
                <div class="term-foot">
                  <div><span>{{ t('marketing.currentExit') }}</span><strong>{{ exitCity }}</strong></div>
                  <div><span>{{ t('marketing.exitIp') }}</span><strong class="mono">{{ exit.ip }}</strong></div>
                  <div><span>{{ t('marketing.totalMetered') }}</span><strong class="mono">{{ n(metered) }} B</strong></div>
                </div>
              </div>
              <pre v-else><code>{{ code }}</code></pre>
            </div>
          </div>
          <aside class="dev-float" :aria-label="t('marketing.sampleResponse')">
            <div class="float-head"><span class="float-dot" /><span>{{ t('marketing.sampleResponse') }}</span><em>{{ exit.ms }} ms</em></div>
            <pre class="float-json">{{ response }}</pre>
            <div class="float-foot"><span>{{ t('marketing.protocol') }}</span><span>{{ t('marketing.metered') }}</span></div>
          </aside>
        </div>
      </div>
    </div>
  </section>

  <section id="manage" class="d-sec band-pine">
    <div class="wrap">
      <div class="section-head">
        <div class="head-copy">
          <h2>{{ t('marketing.usageTitle') }}</h2>
          <p class="sub">{{ t('marketing.usageDesc') }}</p>
        </div>
        <span class="sample-pill">{{ t('marketing.sampleUsage') }}</span>
      </div>
      <div class="bento">
        <article class="bento-card bento-main">
          <div class="bento-top">
            <div><span class="bento-label">{{ t('marketing.balance') }}</span><strong class="bento-hero-num">{{ gb(120.76) }}<em> GB</em></strong></div>
            <div class="bento-kpi-right"><span class="bento-label">{{ t('marketing.week') }}</span><strong>{{ gb(weekGb) }} GB</strong></div>
          </div>
          <div class="dark-chart" role="group" :aria-label="t('marketing.chart')">
            <div v-for="day in dailyUsage" :key="day.day" class="bar-col">
              <span class="bar-val">{{ day.gb.toFixed(1) }}</span>
              <div class="bar-track" aria-hidden="true"><i :style="{ height: `${day.gb / maxDay * 100}%` }" /></div>
              <span class="bar-day">{{ day.day }}</span>
            </div>
          </div>
          <div class="bento-bottom">
            <span>{{ t('marketing.noExpiry') }}</span>
            <span class="csv-pill"><Download aria-hidden="true" />{{ t('marketing.csv') }}</span>
          </div>
        </article>
        <article class="bento-card bento-subs">
          <div class="bento-card-head"><h3>{{ t('marketing.splitSubs') }}</h3><span>{{ t('marketing.subLimit') }}</span></div>
          <ul class="sub-list">
            <li v-for="account in subAccounts" :key="account.name">
              <div class="sub-row-top">
                <div class="sub-id"><code>{{ account.name }}</code></div>
                <span class="sub-usage" :class="{ warn: account.usedGb >= account.limitGb }">{{ gb(account.usedGb) }} / {{ account.limitGb }} GB</span>
              </div>
              <div class="sub-bar" aria-hidden="true"><i :class="{ full: account.usedGb >= account.limitGb }" :style="{ width: `${Math.min(100, account.usedGb / account.limitGb * 100)}%` }" /></div>
            </li>
          </ul>
        </article>
        <article class="bento-card bento-alerts">
          <div class="bento-card-head"><h3>{{ t('marketing.reminders') }}</h3><span><Bell aria-hidden="true" />{{ t('marketing.notifications') }}</span></div>
          <div class="alert-stack">
            <div class="alert-item"><span class="alert-dot amber" /><div><strong>{{ t('marketing.low') }}</strong><p>{{ t('marketing.lowDesc') }}</p></div></div>
            <div class="alert-item"><span class="alert-dot cyan" /><div><strong>{{ t('marketing.expiryAlert') }}</strong><p>{{ t('marketing.expiryDesc') }}</p></div></div>
          </div>
        </article>
      </div>
    </div>
  </section>

  <section id="faq" class="d-sec band-dark">
    <div class="wrap faq-grid">
      <div><h2>{{ t('marketing.faq') }}</h2><p class="note">{{ t('marketing.faqIntro') }}</p></div>
      <ElCollapse v-model="openFaq" class="faq">
        <ElCollapseItem v-for="topic in faqTopics" :key="topic" :name="topic" :title="t(`marketing.faqs.${topic}.q`)">
          <p class="faq-answer">{{ t(`marketing.faqs.${topic}.a`) }}</p>
        </ElCollapseItem>
      </ElCollapse>
    </div>
  </section>

  <section class="d-sec closing">
    <div class="wrap">
      <h2>{{ t('marketing.closeFirst') }} <span>{{ t('marketing.closeSecond') }}</span></h2>
      <ol class="steps">
        <li><span>01</span><strong>{{ t('marketing.stepRegister') }}</strong><p>{{ t('marketing.stepRegisterBody') }}</p></li>
        <li><span>02</span><strong>{{ t('marketing.stepTopUp') }}</strong><p>{{ t('marketing.stepTopUpBody') }}</p></li>
        <li><span>03</span><strong>{{ t('marketing.stepConnect') }}</strong><p>{{ t('marketing.stepConnectBody') }}</p></li>
      </ol>
      <div class="closing-ctas">
        <LinkButton v-if="hasSession()" class="pill primary" to="/console">{{ t('home.console') }}</LinkButton>
        <LinkButton v-else class="pill primary" :to="{ name: 'register', params: { locale } }">{{ t('marketing.freeRegister') }}</LinkButton>
        <a class="pill ghost" href="#integration">{{ t('marketing.learnSetup') }}</a>
      </div>
    </div>
  </section>

  <footer class="home-footer">
    <div class="wrap footer-grid">
      <div class="footer-brand"><strong>{{ t('app.name') }}</strong><p>{{ t('marketing.brandDesc') }}</p></div>
      <div><h3>{{ t('marketing.products') }}</h3><a href="#dynamic-product">{{ t('marketing.dynamic') }}</a><a href="#static-product">{{ t('marketing.static') }}</a></div>
      <div><h3>{{ t('marketing.resources') }}</h3><a href="#integration">{{ t('marketing.guide') }}</a><a href="#faq">{{ t('marketing.faq') }}</a></div>
      <div>
        <h3>{{ t('marketing.account') }}</h3>
        <RouterLink v-if="hasSession()" to="/console">{{ t('home.console') }}</RouterLink>
        <template v-else>
          <RouterLink :to="{ name: 'login', params: { locale } }">{{ t('header.login') }}</RouterLink>
          <RouterLink :to="{ name: 'register', params: { locale } }">{{ t('header.register') }}</RouterLink>
        </template>
      </div>
    </div>
    <div class="wrap footer-bottom"><span>© {{ new Date().getFullYear() }} {{ t('app.name') }}</span><span>{{ t('marketing.mainland') }}</span></div>
  </footer>
</template>

<style scoped src="./sections.css"></style>
