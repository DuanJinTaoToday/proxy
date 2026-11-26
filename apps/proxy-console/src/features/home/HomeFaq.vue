<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const items = ['mainland', 'plans', 'protocol', 'pricing', 'support'] as const
</script>

<template>
  <section id="faq" class="sec sec-light-alt">
    <div class="wrap faq">
      <header class="sec-head">
        <h2>{{ t('marketing.faq.title') }}</h2>
      </header>
      <div class="faq__list">
        <details v-for="(item, index) in items" :key="item" :open="index === 0">
          <summary>{{ t(`marketing.faq.items.${item}.q`) }}</summary>
          <p>{{ t(`marketing.faq.items.${item}.a`) }}</p>
        </details>
      </div>
    </div>
  </section>
</template>

<style scoped>
.faq__list {
  display: grid;
  gap: var(--space-3);
  max-width: var(--home-faq-max-width);
  margin: var(--home-block-gap) auto 0;
}
details {
  border: var(--border-width) solid var(--home-border-light);
  border-radius: var(--home-radius-md);
  background: var(--home-light);
  transition: border-color .2s, box-shadow .25s var(--home-ease);
}
details[open] {
  border-color: var(--home-brand-soft-border);
  box-shadow: var(--home-card-shadow);
}
summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: var(--space-5) var(--space-6);
  font-size: var(--font-size-lg);
  font-weight: var(--font-weight-semibold);
  color: var(--home-text-light);
  cursor: pointer;
  list-style: none;
}
summary::-webkit-details-marker { display: none; }
summary::after {
  content: '';
  flex: none;
  width: var(--home-faq-arrow);
  height: var(--home-faq-arrow);
  border-right: var(--home-accent-bar-radius) solid var(--home-faint-light);
  border-bottom: var(--home-accent-bar-radius) solid var(--home-faint-light);
  transform: rotate(45deg);
  transition: transform .25s var(--home-ease), border-color .2s;
}
details[open] summary::after {
  transform: rotate(-135deg);
  border-color: var(--home-brand);
}
summary:hover { color: var(--home-brand); }
details p {
  padding: 0 var(--space-6) var(--space-5);
  font-size: var(--home-fs-body);
  line-height: var(--home-lh-body);
  color: var(--home-muted-light);
}
@media (max-width: 560px) {
  summary { padding: var(--space-4) var(--space-4); font-size: var(--home-fs-body); }
  details p { padding: 0 var(--space-4) var(--space-4); }
}
</style>
