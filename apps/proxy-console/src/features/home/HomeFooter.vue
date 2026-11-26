<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Lock } from '@element-plus/icons-vue'
import { hasSession } from '@/features/auth'
import BrandMark from '@/shared/ui/BrandMark.vue'
import { supportChannels, supportConfig } from './support'
import SupportChannels from './SupportChannels.vue'

const { t, locale } = useI18n()
const hasContact = computed(() => supportChannels.some(channel => supportConfig[channel]))
const products = [
  { id: 'dynamic', query: { category: 'dynamic' } },
  { id: 'exclusive', query: { category: 'exclusive' } },
  { id: 'shared', query: { category: 'shared' } },
] as const
</script>

<template>
  <footer class="footer home-footer sec-dark">
    <div class="wrap">
      <div class="cta">
        <div>
          <h2>{{ t('marketing.footer.ctaTitle') }}</h2>
          <p>{{ t('marketing.footer.ctaSub') }}</p>
        </div>
        <RouterLink v-if="hasSession()" class="home-btn home-btn--primary" to="/console">{{ t('home.console') }}</RouterLink>
        <RouterLink v-else class="home-btn home-btn--primary" :to="{ name: 'register', params: { locale } }">{{ t('marketing.hero.register') }}</RouterLink>
      </div>

      <div id="contact" class="footer__grid">
        <div class="footer__brand footer-brand">
          <RouterLink :to="`/${locale}/`" class="brand" :aria-label="t('app.name')">
            <BrandMark />
          </RouterLink>
          <p>{{ t('marketing.footer.tagline') }}</p>
        </div>
        <div>
          <h3>{{ t('marketing.footer.products') }}</h3>
          <ul>
            <li v-for="item in products" :key="item.id">
              <RouterLink :to="{ path: `/${locale}/pricing`, query: item.query }">{{ t(`marketing.pricing.${item.id}.name`) }}</RouterLink>
            </li>
          </ul>
        </div>
        <div>
          <h3>{{ t('marketing.footer.contact') }}</h3>
          <SupportChannels v-if="hasContact" variant="list" />
          <p v-else class="footer__soon">{{ t('marketing.support.soonAll') }}</p>
        </div>
      </div>

      <div class="footer__legal footer-bottom">
        <p class="compliance"><Lock aria-hidden="true" />{{ t('marketing.compliance') }}</p>
        <p>© {{ new Date().getFullYear() }} {{ t('marketing.brand') }}</p>
      </div>
    </div>
  </footer>
</template>

<style scoped>
.footer { padding: var(--home-section-y) 0 var(--space-5); border-top: var(--border-width) solid var(--home-line-dark); }
.cta { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: var(--space-5); padding: clamp(24px, 4vw, 48px); border: var(--border-width) solid var(--home-line-dark-strong); border-radius: var(--home-radius-lg); background: radial-gradient(70% 140% at 100% 0, var(--home-glow-primary), transparent 70%), var(--home-surface-1); }
.cta h2 { font-size: var(--home-fs-h2); line-height: 1.15; font-weight: var(--font-weight-bold); letter-spacing: -.025em; color: var(--home-on-dark); }
.cta p { margin-top: var(--space-2); color: var(--home-on-dark-muted); }
.footer__grid { display: grid; grid-template-columns: 1.4fr 1fr 1.2fr; gap: var(--space-6); margin-top: var(--home-block-gap); }
.brand {
  --brand-mark-size: 28px;
  --brand-wordmark-display: inline;
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--home-on-dark);
}
.footer__brand p { margin-top: var(--space-3); max-width: 24em; font-size: var(--home-fs-small); line-height: var(--home-lh-body); color: var(--home-on-dark-muted); }
h3 { margin-bottom: var(--space-4); font-size: var(--home-fs-small); font-weight: var(--font-weight-medium); letter-spacing: .06em; color: var(--home-on-dark-faint); }
ul { display: grid; gap: var(--space-3); margin: 0; padding: 0; list-style: none; font-size: var(--home-fs-small); }
ul a { color: var(--home-on-dark-muted); transition: color .2s; }
ul a:hover { color: var(--home-sky-300); }
.footer__soon { font-size: var(--home-fs-small); color: var(--home-on-dark-muted); }
.footer__legal { display: flex; flex-wrap: wrap; justify-content: space-between; gap: var(--space-3); margin-top: var(--home-block-gap); padding-top: var(--space-5); border-top: var(--border-width) solid var(--home-line-dark); font-size: var(--home-fs-micro); color: var(--home-on-dark-faint); }
.compliance { display: inline-flex; align-items: center; gap: var(--space-2); }
.compliance svg { width: var(--home-icon-sm); height: var(--home-icon-sm); color: var(--home-sky-400); }
@media (max-width: 767px) { .footer__grid { grid-template-columns: minmax(0, 1fr); } }
</style>
