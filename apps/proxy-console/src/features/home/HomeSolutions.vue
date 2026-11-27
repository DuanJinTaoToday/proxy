<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Check, DataAnalysis, Lock, Right, Share, ShoppingCart } from '@element-plus/icons-vue'

const { t, locale } = useI18n()

const scenarios = [
  { id: 'commerce', icon: ShoppingCart, category: 'exclusive' },
  { id: 'brand', icon: Lock, category: 'dynamic' },
  { id: 'social', icon: Share, category: 'exclusive' },
  { id: 'research', icon: DataAnalysis, category: 'dynamic' },
] as const
const points = ['p1', 'p2', 'p3'] as const
</script>

<template>
  <section id="solutions" class="sec sec-light">
    <div class="wrap">
      <header class="sec-head">
        <h2>{{ t('marketing.solutions.title') }}</h2>
        <p class="sec-sub">{{ t('marketing.solutions.sub') }}</p>
      </header>

      <ul class="columns" role="list">
        <li v-for="item in scenarios" :key="item.id" class="col">
          <span class="col__icon" aria-hidden="true">
            <component :is="item.icon" />
          </span>
          <h3>{{ t(`marketing.solutions.${item.id}.title`) }}</h3>
          <p class="col__desc">{{ t(`marketing.solutions.${item.id}.desc`) }}</p>
          <ul class="col__points" role="list">
            <li v-for="point in points" :key="point">
              <span class="col__check" aria-hidden="true"><Check /></span>
              <span>{{ t(`marketing.solutions.${item.id}.points.${point}`) }}</span>
            </li>
          </ul>
          <RouterLink class="col__more" :to="{ path: `/${locale}/pricing`, query: { category: item.category } }">
            {{ t('marketing.solutions.learnMore') }}<Right aria-hidden="true" />
          </RouterLink>
        </li>
      </ul>
    </div>
  </section>
</template>

<style scoped>
.columns {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 0;
  margin: var(--home-block-gap) 0 0;
  padding: 0;
  list-style: none;
}
.col {
  display: flex;
  flex-direction: column;
  padding: var(--space-2) var(--space-5) var(--space-3);
  border-inline-start: var(--border-width) solid var(--home-border-subtle);
}
.col__icon {
  display: inline-grid;
  place-items: center;
  width: var(--home-sol-icon-size);
  height: var(--home-sol-icon-size);
  border-radius: var(--radius-card);
  background: var(--home-brand-soft);
  color: var(--home-brand);
}
.col__icon svg { width: var(--space-5); height: var(--space-5); }

h3 {
  position: relative;
  margin-top: var(--space-5);
  font-size: var(--home-fs-card-title);
  font-weight: var(--font-weight-bold);
  color: var(--home-text-light);
}
h3::before {
  content: '';
  position: absolute;
  inset-inline-start: calc(var(--space-5) * -1 - var(--border-width));
  inset-block-start: var(--home-accent-bar-offset);
  width: var(--home-accent-bar-width);
  height: var(--home-accent-bar-height);
  border-radius: var(--home-accent-bar-radius);
  background: var(--home-brand);
}

.col__desc {
  margin-top: var(--space-3);
  font-size: var(--home-fs-small);
  line-height: 1.75;
  color: var(--home-muted-light);
}
.col__points {
  display: grid;
  gap: var(--space-2);
  margin: var(--space-5) 0 0;
  padding: 0;
  list-style: none;
}
.col__points li {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--home-fs-small);
  color: var(--home-text-light);
}
.col__check {
  display: inline-grid;
  place-items: center;
  flex: none;
  width: var(--home-check-size);
  height: var(--home-check-size);
  border-radius: var(--home-pill);
  background: var(--home-brand);
  color: var(--home-on-brand);
}
.col__check svg { width: var(--home-check-glyph); height: var(--home-check-glyph); }

.col__more {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
  margin-top: auto;
  padding-top: var(--space-5);
  font-size: var(--home-fs-small);
  font-weight: var(--font-weight-semibold);
  color: var(--home-brand);
}
.col__more svg { width: var(--home-icon-sm); height: var(--home-icon-sm); transition: transform .2s var(--home-ease); }
.col__more:hover svg { transform: translateX(4px); }

@media (max-width: 1024px) {
  .columns { grid-template-columns: repeat(2, minmax(0, 1fr)); row-gap: var(--space-6); }
}
@media (max-width: 600px) {
  .columns { grid-template-columns: minmax(0, 1fr); row-gap: var(--space-6); }
  .col { padding-inline: var(--space-5) 0; }
}
</style>
