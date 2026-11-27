<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { ChatDotRound, Phone, Promotion } from '@element-plus/icons-vue'
import { supportChannels, supportConfig, type PhoneContact, type SupportChannel } from './support'
import { useCopy } from './useCopy'

// dock：官网右侧客服坞；list：官网页脚联系栏；panel：用户控制台客服弹窗（浅色）。
const props = withDefaults(defineProps<{ variant?: 'dock' | 'list' | 'panel' }>(), { variant: 'dock' })
const { t } = useI18n()
const { copy } = useCopy()

const qqGlyphPath = 'M12 2.2c-3.7 0-6.2 2.9-6.2 7 0 .6.1 1.1.2 1.6-1.1 1.4-2 3.4-2 4.8 0 .9.5 1.4 1.1 1.3.4-.1.9-.5 1.3-1.1.4 1.2 1.1 2.2 2 2.9-.8.3-1.4.9-1.4 1.5 0 .9 1.5 1.5 3.6 1.5.7 0 1.1-.1 1.4-.2.3.1.7.2 1.4.2 2.1 0 3.6-.6 3.6-1.5 0-.6-.6-1.2-1.4-1.5.9-.7 1.6-1.7 2-2.9.4.6.9 1 1.3 1.1.6.1 1.1-.4 1.1-1.3 0-1.4-.9-3.4-2-4.8.1-.5.2-1 .2-1.6 0-4.1-2.5-7-6.2-7Zm-2.3 6.2a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm4.6 0a1 1 0 1 1 0 2 1 1 0 0 1 0-2Zm-2.3 3.2c1.4 0 2.6.5 2.6 1 0 .6-1.2 1.2-2.6 1.2s-2.6-.6-2.6-1.2c0-.5 1.2-1 2.6-1Z'

const icons = { chat: ChatDotRound, telegram: Promotion, qq: undefined, phone: Phone }
const entries = computed(() => supportChannels
  .map((channel) => {
    const value = supportConfig[channel]
    return { channel, icon: icons[channel], href: typeof value === 'string' ? value : undefined, phone: typeof value === 'object' ? value : undefined }
  })
  .filter(entry => props.variant === 'dock' || entry.href || entry.phone))

const label = (channel: SupportChannel) => t(`marketing.support.${channel}`)
function copyPhone(phone: PhoneContact) {
  void copy('phone', phone.value, t('marketing.support.copied', { name: label('phone') }))
}
function notifySoon() {
  ElMessage({ message: t('marketing.support.soon'), type: 'info', offset: 80 })
}
</script>

<template>
  <nav class="support" :class="`support--${variant}`" :aria-label="t('marketing.support.title')">
    <template v-for="entry in entries" :key="entry.channel">
      <a v-if="entry.href" class="support__item" :data-channel="entry.channel" :href="entry.href" target="_blank" rel="noopener noreferrer">
        <component :is="entry.icon" v-if="entry.icon" class="support__icon" aria-hidden="true" />
        <svg v-else class="support__icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path :d="qqGlyphPath" />
        </svg>
        <span class="support__label">{{ label(entry.channel) }}</span>
        <span v-if="variant !== 'dock'" class="support__value">{{ t('marketing.support.open') }}</span>
        <span class="support__sr">{{ t('marketing.support.newTab') }}</span>
      </a>
      <button v-else-if="entry.phone" type="button" class="support__item" :data-channel="entry.channel" :title="t('marketing.support.copyHint')" @click="copyPhone(entry.phone)">
        <Phone class="support__icon" aria-hidden="true" />
        <span class="support__label">{{ label(entry.channel) }}</span>
        <span v-if="variant !== 'dock'" class="support__value mono">{{ entry.phone.display }}</span>
        <span v-else class="support__tip mono">{{ entry.phone.display }}</span>
      </button>
      <button v-else type="button" class="support__item support__item--off" :data-channel="entry.channel" data-state="soon" @click="notifySoon">
        <component :is="entry.icon" v-if="entry.icon" class="support__icon" aria-hidden="true" />
        <svg v-else class="support__icon" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path :d="qqGlyphPath" />
        </svg>
        <span class="support__label">{{ label(entry.channel) }}</span>
        <span class="support__tip">{{ t('marketing.support.soon') }}</span>
      </button>
    </template>
  </nav>
</template>

<style scoped>
.support__item { position: relative; display: flex; align-items: center; border: 0; background: transparent; color: var(--home-on-dark); font-size: var(--home-fs-micro); cursor: pointer; transition: background .2s, color .2s; }
.support__item--off { color: var(--home-on-dark-faint); }
.support__icon { flex: none; width: var(--space-5); height: var(--space-5); }
.support__sr { position: absolute; width: 1px; height: 1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }

.support--dock { position: fixed; z-index: 20; inset-inline-end: var(--space-3); inset-block-start: 50%; display: flex; flex-direction: column; gap: var(--space-1); padding: var(--space-2); border: var(--border-width) solid var(--home-line-dark-strong); border-radius: var(--home-radius-lg); background: var(--home-header-background); box-shadow: var(--home-dock-shadow); backdrop-filter: blur(var(--home-header-blur)); transform: translateY(-50%); }
.support--dock .support__item { flex-direction: column; justify-content: center; gap: var(--home-dock-item-gap); width: var(--home-dock-item-size); min-height: var(--home-dock-item-size); border-radius: var(--home-radius-md); }
.support--dock .support__item:not(.support__item--off):hover { background: var(--home-brand); color: var(--home-on-brand); }
.support--dock .support__tip { position: absolute; inset-inline-end: calc(100% + var(--space-3)); inset-block-start: 50%; padding: var(--space-2) var(--space-3); border: var(--border-width) solid var(--home-line-dark-strong); border-radius: var(--home-radius-sm); background: var(--home-surface-2); color: var(--home-on-dark); font-size: var(--home-fs-small); white-space: nowrap; opacity: 0; pointer-events: none; transform: translate(6px, -50%); transition: opacity .2s, transform .2s var(--home-ease); }
.support--dock .support__item:hover .support__tip, .support--dock .support__item:focus-visible .support__tip { opacity: 1; transform: translate(0, -50%); }

.support--list { display: grid; gap: var(--space-1); }
.support--list .support__item { gap: var(--space-3); padding: var(--space-2) var(--space-3); margin-inline: calc(var(--space-3) * -1); border-radius: var(--home-radius-sm); font-size: var(--home-fs-small); text-align: start; }
.support--list .support__item:hover { background: var(--home-line-dark); }
.support--list .support__label { color: var(--home-on-dark-muted); min-width: 4.5em; }
.support--list .support__value { color: var(--home-on-dark); }

/* 控制台浅色面板：沿用页脚联系栏的行结构，颜色改用控制台语义 token。 */
.support--panel { display: grid; gap: var(--space-2); }
.support--panel .support__item { gap: var(--space-3); min-height: var(--control-height); padding: var(--space-2) var(--space-3); border: var(--border-width) solid var(--color-border); border-radius: var(--radius-control); color: var(--color-text); font-size: var(--font-size-md); text-align: start; text-decoration: none; }
.support--panel .support__item:hover { border-color: var(--color-primary-border); background: var(--color-primary-soft); }
.support--panel .support__icon { color: var(--color-primary); }
.support--panel .support__label { min-width: 5em; color: var(--color-text-secondary); }
.support--panel .support__value { margin-inline-start: auto; color: var(--color-text); font-weight: var(--font-weight-medium); }

@media (max-width: 767px) {
  .support--dock { inset: auto 0 0; display: grid; grid-template-columns: repeat(4, 1fr); gap: 0; padding: var(--space-1) var(--space-2) calc(var(--space-1) + env(safe-area-inset-bottom)); border-width: var(--border-width) 0 0; border-radius: 0; transform: none; }
  .support--dock .support__item { width: auto; }
  .support--dock .support__tip { display: none; }
}
</style>
