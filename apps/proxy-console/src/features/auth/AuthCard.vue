<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { Connection, Key, Lock, Wallet } from '@element-plus/icons-vue'
import BrandMark from '@/shared/ui/BrandMark.vue'

// 认证页面共用的双栏舞台卡片：左侧展示云网钴蓝品牌与核心能力，右侧承载表单；窄屏自动折叠左侧以保证移动端表单首屏直达。
defineProps<{ title: string, subtitle?: string }>()
const { t } = useI18n()

const highlights = [
  { key: 'network', icon: Connection },
  { key: 'access', icon: Key },
  { key: 'billing', icon: Wallet },
] as const
</script>

<template>
  <div class="auth-stage">
    <aside class="auth-aside" aria-hidden="true">
      <div class="auth-aside__glow" />
      <div class="auth-aside__grid" />

      <div class="auth-aside__top">
        <div class="auth-aside__brand">
          <BrandMark />
        </div>
        <p class="auth-aside__headline">{{ t('auth.showcase.headline') }}</p>
        <p class="auth-aside__lead">{{ t('auth.showcase.lead') }}</p>
      </div>

      <svg class="auth-aside__art" viewBox="0 0 360 128" fill="none">
        <path d="M24 98 Q120 18 214 68 T338 36" class="auth-aside__arc" stroke-width="1.5" />
        <path d="M56 108 Q166 44 282 88" class="auth-aside__arc-soft" stroke-width="1.2" stroke-dasharray="3 4" />
        <circle cx="24" cy="98" r="3.5" class="auth-aside__node" />
        <circle cx="214" cy="68" r="10" class="auth-aside__halo" />
        <circle cx="214" cy="68" r="4" class="auth-aside__node-bright" />
        <circle cx="338" cy="36" r="3.5" class="auth-aside__node" />
        <circle cx="282" cy="88" r="3" class="auth-aside__node" />
      </svg>

      <ul class="auth-aside__list">
        <li v-for="item in highlights" :key="item.key">
          <span class="auth-aside__icon"><component :is="item.icon" /></span>
          <div class="auth-aside__item-copy">
            <strong>{{ t(`auth.showcase.items.${item.key}.title`) }}</strong>
            <span class="auth-aside__item-desc">{{ t(`auth.showcase.items.${item.key}.desc`) }}</span>
          </div>
        </li>
      </ul>

      <p class="auth-aside__compliance">
        <Lock class="auth-aside__lock" />
        <span>{{ t('auth.showcase.compliance') }}</span>
      </p>
    </aside>

    <section class="auth-card">
      <header class="auth-card__head">
        <h1 class="auth-card__title">{{ title }}</h1>
        <p v-if="subtitle" class="auth-card__subtitle">{{ subtitle }}</p>
      </header>
      <div class="auth-card__body">
        <slot />
      </div>
      <p class="auth-card__mobile-compliance">
        <Lock class="auth-aside__lock" aria-hidden="true" />
        <span>{{ t('auth.showcase.compliance') }}</span>
      </p>
    </section>
  </div>
</template>

<style scoped>
.auth-stage {
  display: grid;
  grid-template-columns: minmax(0, 0.94fr) minmax(0, 1.06fr);
  width: 100%;
  max-width: var(--auth-stage-width);
  margin-inline: auto;
  overflow: hidden;
  border: var(--border-width) solid var(--color-border);
  border-radius: var(--auth-panel-radius);
  background: var(--color-surface);
  box-shadow: var(--auth-panel-shadow);
}

.auth-aside {
  --color-brand-accent: var(--auth-aside-accent);
  --brand-mark-size: var(--icon-size);
  --brand-wordmark-display: inline;
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: var(--space-5);
  padding: var(--space-6) var(--space-6) var(--space-5);
  overflow: hidden;
  background: linear-gradient(160deg, var(--auth-aside-surface) 0%, var(--auth-aside-bg) 72%);
  color: var(--auth-aside-text);
}

.auth-aside__glow {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(68% 58% at 88% 12%, var(--auth-aside-glow-1) 0%, transparent 72%),
    radial-gradient(52% 48% at 14% 88%, var(--auth-aside-glow-2) 0%, transparent 70%);
}

.auth-aside__grid {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background-image: radial-gradient(var(--auth-aside-grid) var(--border-width), transparent var(--border-width));
  background-size: var(--auth-grid-step) var(--auth-grid-step);
  -webkit-mask-image: linear-gradient(180deg, black 0%, transparent 86%);
  mask-image: linear-gradient(180deg, black 0%, transparent 86%);
}

.auth-aside__top,
.auth-aside__art,
.auth-aside__list,
.auth-aside__compliance {
  position: relative;
  z-index: 1;
}

.auth-aside__brand {
  --brand-mark-size: var(--auth-brand-size);
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  font-size: var(--font-size-xl);
  font-weight: var(--font-weight-bold);
  color: var(--auth-aside-text);
}

.auth-aside__headline {
  margin-top: var(--space-5);
  font-size: var(--font-size-title);
  font-weight: var(--font-weight-bold);
  line-height: var(--line-height-tight);
  letter-spacing: var(--brand-wordmark-tracking);
  color: var(--auth-aside-text);
  text-wrap: balance;
}

.auth-aside__lead {
  margin-top: var(--space-3);
  font-size: var(--font-size-sm);
  line-height: var(--line-height-normal);
  color: var(--auth-aside-muted);
}

.auth-aside__art {
  display: block;
  width: 100%;
  height: auto;
  margin-block: calc(var(--space-2) * -1);
}
.auth-aside__arc { stroke: var(--auth-aside-accent); }
.auth-aside__arc-soft { stroke: var(--auth-aside-border); }
.auth-aside__halo { fill: var(--auth-aside-glow-1); }
.auth-aside__node { fill: var(--auth-aside-accent); }
.auth-aside__node-bright { fill: var(--auth-aside-cyan); }

.auth-aside__list {
  display: grid;
  gap: var(--space-4);
  margin: 0;
  padding: 0;
  list-style: none;
}

.auth-aside__list li {
  display: flex;
  align-items: flex-start;
  gap: var(--space-3);
}

.auth-aside__icon {
  display: inline-grid;
  place-items: center;
  flex: none;
  width: var(--auth-icon-box);
  height: var(--auth-icon-box);
  border: var(--border-width) solid var(--auth-aside-pill-border);
  border-radius: var(--radius-control);
  background: var(--auth-aside-pill-bg);
  color: var(--auth-aside-cyan);
}
.auth-aside__icon svg { width: var(--icon-size); height: var(--icon-size); }

.auth-aside__item-copy strong {
  display: block;
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
  color: var(--auth-aside-text);
  line-height: var(--line-height-tight);
}

.auth-aside__item-desc {
  display: block;
  margin-top: var(--space-1);
  font-size: var(--font-size-xs);
  line-height: var(--line-height-normal);
  color: var(--auth-aside-muted);
}

.auth-aside__compliance {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  padding-top: var(--space-4);
  border-top: var(--border-width) solid var(--auth-aside-border);
  font-size: var(--font-size-xs);
  color: var(--auth-aside-faint);
}

.auth-aside__lock {
  width: var(--font-size-md);
  height: var(--font-size-md);
  flex: none;
  color: var(--auth-aside-accent);
}

.auth-card {
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding: var(--space-7) var(--space-6);
  background: var(--color-surface);
  min-width: 0;
}

.auth-card__head {
  margin-bottom: var(--space-5);
}

.auth-card__title {
  font-size: var(--font-size-title);
  font-weight: var(--font-weight-bold);
  line-height: var(--line-height-tight);
  letter-spacing: var(--brand-wordmark-tracking);
  color: var(--color-text);
  overflow-wrap: anywhere;
}

.auth-card__subtitle {
  margin-top: var(--space-2);
  font-size: var(--font-size-md);
  line-height: var(--line-height-normal);
  color: var(--color-text-secondary);
}

.auth-card__body {
  min-width: 0;
}

.auth-card :deep(.auth-card__alert) { margin-bottom: var(--space-4); }
.auth-card :deep(.auth-card__hint) {
  margin-left: var(--space-2);
  color: var(--color-text-secondary);
  font-size: var(--font-size-xs);
  font-weight: var(--font-weight-regular);
}
.auth-card :deep(.auth-card__submit) {
  width: 100%;
  height: var(--auth-submit-height);
  margin-top: var(--space-2);
  font-size: var(--font-size-md);
  font-weight: var(--font-weight-semibold);
}
.auth-card :deep(.auth-card__footer) {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  margin-top: var(--space-5);
  padding-top: var(--space-4);
  border-top: var(--border-width) solid var(--color-border);
  font-size: var(--font-size-sm);
  color: var(--color-text-secondary);
}

.auth-card :deep(.auth-card__footer-link) {
  padding-inline: var(--space-1);
}
.auth-card :deep(.auth-card__footer-switch) {
  display: inline-flex;
  align-items: center;
  gap: var(--space-1);
}

.auth-card__mobile-compliance {
  display: none;
}

@media (max-width: 959px) {
  .auth-stage {
    grid-template-columns: minmax(0, 1fr);
    max-width: var(--auth-card-width);
  }
  .auth-aside { display: none; }
  .auth-card { padding: var(--space-6) var(--space-5); }
  .auth-card__mobile-compliance {
    display: inline-flex;
    align-items: center;
    gap: var(--space-2);
    margin-top: var(--space-4);
    font-size: var(--font-size-xs);
    color: var(--color-text-secondary);
  }
}

@media (max-width: 480px) {
  .auth-card { padding: var(--space-5) var(--space-4); }
}
</style>
