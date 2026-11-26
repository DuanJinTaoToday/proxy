<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { CircleCheck, Right } from '@element-plus/icons-vue'
import AdvantageGlobe from './AdvantageGlobe.vue'

const { t, locale } = useI18n()
const router = useRouter()

const dynamicFeatures = ['uptime', 'extract', 'latency', 'session'] as const
// 按 #100 / #113 决策：首页先采用固定起售价吸引点击跳转公开报价页，开售前由产品方按实际目录调整。
const staticPlans = [
  { id: 'exclusive' as const, price: '¥16', unitKey: 'perIp', pricingCategory: 'exclusive', consolePath: '/console/static/pricing' },
  { id: 'shared' as const, price: '¥2', unitKey: 'perIp', pricingCategory: 'shared', consolePath: '/console/static/pricing' },
]

function openPricing(category: string) {
  void router.push({ path: `/${locale.value}/pricing`, query: { category } })
}
</script>

<template>
  <section id="products" class="sec sec-light">
    <div class="wrap">
      <header class="sec-head">
        <h2>{{ t('marketing.pricing.title') }}</h2>
        <p class="sec-sub">{{ t('marketing.pricing.sub') }}</p>
      </header>

      <div class="bento">
        <div class="bento__row bento__row--top">
          <article class="card card--advantage">
            <h3>{{ t('marketing.pricing.advantage.title') }}</h3>
            <p class="card__text">{{ t('marketing.pricing.advantage.desc') }}</p>
            <AdvantageGlobe />
          </article>

          <article class="card card--clickable" @click="openPricing('dynamic')">
            <div class="card__head">
              <div>
                <h3>
                  <RouterLink :to="{ path: `/${locale}/pricing`, query: { category: 'dynamic' } }" @click.stop>
                    {{ t('marketing.pricing.dynamic.name') }}
                  </RouterLink>
                </h3>
                <p class="card__desc">{{ t('marketing.pricing.dynamic.desc') }}</p>
              </div>
              <div class="price">
                <span class="price__tag">{{ t('marketing.pricing.fromTag') }}</span>
                <p class="price__value tabular"><strong>¥3.5</strong><span>{{ t('marketing.pricing.perGb') }}</span></p>
              </div>
            </div>

            <ul class="feature-strip" role="list">
              <li v-for="feat in dynamicFeatures" :key="feat">
                <CircleCheck class="feature-strip__icon" aria-hidden="true" />
                <span>{{ t(`marketing.pricing.dynamic.features.${feat}`) }}</span>
              </li>
            </ul>

            <div class="card__actions">
              <RouterLink class="start-btn" to="/console/dynamic/pricing" @click.stop>
                {{ t('marketing.pricing.start') }}
              </RouterLink>
              <RouterLink class="arrow-btn" :to="{ path: `/${locale}/pricing`, query: { category: 'dynamic' } }" :aria-label="t('marketing.pricing.cta')" @click.stop>
                <Right aria-hidden="true" />
              </RouterLink>
            </div>
          </article>
        </div>

        <div class="bento__row bento__row--bottom">
          <article v-for="plan in staticPlans" :key="plan.id" class="card card--clickable" @click="openPricing(plan.pricingCategory)">
            <div class="card__head">
              <div>
                <h3>
                  <RouterLink :to="{ path: `/${locale}/pricing`, query: { category: plan.pricingCategory } }" @click.stop>
                    {{ t(`marketing.pricing.${plan.id}.name`) }}
                  </RouterLink>
                </h3>
                <p class="card__desc">{{ t(`marketing.pricing.${plan.id}.desc`) }}</p>
              </div>
              <div class="price">
                <span class="price__tag">{{ t('marketing.pricing.fromTag') }}</span>
                <p class="price__value tabular"><strong>{{ plan.price }}</strong><span>{{ t(`marketing.pricing.${plan.unitKey}`) }}</span></p>
              </div>
            </div>

            <div class="card__actions">
              <RouterLink class="start-btn" :to="plan.consolePath" @click.stop>
                {{ t('marketing.pricing.start') }}
              </RouterLink>
              <RouterLink class="arrow-btn" :to="{ path: `/${locale}/pricing`, query: { category: plan.pricingCategory } }" :aria-label="t('marketing.pricing.cta')" @click.stop>
                <Right aria-hidden="true" />
              </RouterLink>
            </div>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.bento {
  display: grid;
  gap: var(--space-5);
  margin-top: var(--home-block-gap);
}
.bento__row { display: grid; gap: var(--space-5); }
.bento__row--top { grid-template-columns: minmax(0, 1fr) minmax(0, 2.05fr); }
.bento__row--bottom { grid-template-columns: repeat(2, minmax(0, 1fr)); }

.card {
  position: relative;
  display: flex;
  flex-direction: column;
  padding: var(--home-card-pad-y) var(--home-card-pad-x);
  border: var(--border-width) solid var(--home-border-subtle);
  border-radius: var(--home-radius-lg);
  background: var(--home-card-gray);
  transition: background .25s, border-color .25s, box-shadow .3s var(--home-ease), transform .3s var(--home-ease);
}
.card--clickable { cursor: pointer; }
.card--clickable:hover {
  background: var(--home-card-gray-hover);
  border-color: var(--home-brand-soft-border);
  box-shadow: var(--home-card-shadow-hover);
  transform: translateY(-2px);
}
.card--advantage {
  overflow: hidden;
  justify-content: flex-start;
  min-height: var(--home-adv-min-height);
  padding-bottom: var(--home-adv-pad-bottom);
}

.card__head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: var(--space-4);
}
h3 {
  font-size: var(--home-fs-card-title);
  line-height: 1.35;
  font-weight: var(--font-weight-bold);
  letter-spacing: -.01em;
  color: var(--home-text-light);
}
h3 a:hover { color: var(--home-brand); }
.card__text {
  position: relative;
  z-index: 1;
  margin-top: var(--space-3);
  font-size: var(--home-fs-note);
  line-height: 1.85;
  letter-spacing: .015em;
  color: var(--home-faint-light);
}
.card__desc {
  margin-top: var(--space-2);
  max-width: 34em;
  font-size: var(--home-fs-note);
  line-height: 1.75;
  letter-spacing: .01em;
  color: var(--home-faint-light);
}

.price {
  flex: none;
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: var(--space-1);
}
.price__tag {
  padding: var(--home-tag-pad-y) var(--home-tag-pad-x);
  border-radius: var(--radius-control);
  background: var(--home-brand-soft);
  color: var(--home-brand);
  font-size: var(--home-fs-micro);
  font-weight: var(--font-weight-semibold);
}
.price__value {
  margin: 0;
  white-space: nowrap;
  color: var(--home-price);
  line-height: 1.1;
}
.price__value strong {
  font-size: var(--home-fs-price);
  font-weight: var(--font-weight-bold);
  letter-spacing: -.02em;
}
.price__value span {
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-semibold);
}

.feature-strip {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: var(--space-3) var(--space-5);
  margin: var(--space-5) 0 0;
  padding: var(--space-4) var(--space-5);
  list-style: none;
  border: var(--border-width) solid var(--home-border-subtle);
  border-radius: var(--home-radius-md);
  background: var(--home-light);
}
.feature-strip li {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--home-fs-note);
  color: var(--home-text-light);
}
.feature-strip__icon {
  width: var(--icon-size);
  height: var(--icon-size);
  flex: none;
  color: var(--home-brand);
}

.card__actions {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  margin-top: auto;
  padding-top: var(--space-5);
}
.start-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: var(--home-btn-sm-height);
  padding: 0 var(--space-5);
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-radius-sm);
  background: var(--home-light);
  color: var(--home-text-light);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-medium);
  transition: background .2s, border-color .2s, color .2s;
}
.start-btn:hover {
  border-color: var(--home-brand);
  background: var(--home-brand);
  color: var(--home-on-brand);
}
.arrow-btn {
  display: inline-grid;
  place-items: center;
  width: var(--home-btn-circle);
  height: var(--home-btn-circle);
  border: var(--border-width) solid var(--home-brand-soft-border);
  border-radius: var(--home-pill);
  background: var(--home-light);
  color: var(--home-brand);
  transition: background .2s, color .2s, transform .2s var(--home-ease);
}
.arrow-btn svg { width: var(--icon-size); height: var(--icon-size); }
.card--clickable:hover .arrow-btn {
  background: var(--home-brand);
  color: var(--home-on-brand);
  transform: translateX(3px);
}

@media (max-width: 960px) {
  .bento__row--top,
  .bento__row--bottom { grid-template-columns: minmax(0, 1fr); }
  .card--advantage { min-height: var(--home-adv-min-height-mobile); padding-bottom: var(--home-adv-pad-bottom); }
}
@media (max-width: 560px) {
  .card { padding: var(--space-5) var(--space-4); }
  .card--advantage { padding-bottom: var(--home-adv-pad-bottom); }
  .card__head { flex-direction: column; }
  .price { align-items: baseline; flex-direction: row-reverse; }
  .feature-strip { grid-template-columns: minmax(0, 1fr); }
}
</style>
