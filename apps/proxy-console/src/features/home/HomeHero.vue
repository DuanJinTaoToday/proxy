<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Lock, Right } from '@element-plus/icons-vue'
import { hasSession } from '@/features/auth'
import HomeMap from './HomeMap.vue'
import { heroExits, heroPhrases } from './world'

const { t, locale } = useI18n()
const phraseIndex = ref(0)
let timer: number | undefined

onMounted(() => {
  if (!matchMedia('(prefers-reduced-motion: reduce)').matches) {
    timer = window.setInterval(() => {
      phraseIndex.value = (phraseIndex.value + 1) % heroPhrases.length
    }, 2800)
  }
})
onUnmounted(() => clearInterval(timer))
</script>

<template>
  <section class="hero sec-dark">
    <HomeMap />
    <div class="hero__ambient" aria-hidden="true" />
    <div class="hero__shade" aria-hidden="true" />

    <div class="wrap hero__inner">
      <div class="hero__copy">
        <h1>
          <span class="hero__line">{{ t('marketing.hero.title') }}</span>
          <span class="hero__line rotator">
            <em v-for="phrase in heroPhrases" :key="phrase" class="rotator__size" aria-hidden="true">{{ t(`marketing.hero.phrases.${phrase}`) }}</em>
            <Transition name="swap">
              <em :key="phraseIndex">{{ t(`marketing.hero.phrases.${heroPhrases[phraseIndex]}`) }}</em>
            </Transition>
          </span>
        </h1>
        <p class="hero__lead">{{ t('marketing.hero.lead') }}</p>
        <div class="hero__actions">
          <RouterLink v-if="hasSession()" class="home-btn home-btn--primary" to="/console">{{ t('home.console') }}</RouterLink>
          <RouterLink v-else class="home-btn home-btn--primary" :to="{ name: 'register', params: { locale } }">{{ t('marketing.hero.register') }}</RouterLink>
          <RouterLink class="home-btn home-btn--outline" :to="`/${locale}/pricing`">
            {{ t('marketing.hero.pricing') }}<Right class="home-btn__icon" aria-hidden="true" />
          </RouterLink>
        </div>
        <p class="hero__compliance"><Lock aria-hidden="true" />{{ t('marketing.compliance') }}</p>
      </div>
    </div>

    <div class="wrap exit-strip">
      <span class="exit-strip__label">{{ t('marketing.hero.exampleExits') }}</span>
      <div class="exit-strip__rail">
        <div class="exit-strip__track">
          <span
            v-for="(city, i) in [...heroExits, ...heroExits]"
            :key="i"
            :aria-hidden="i >= heroExits.length"
          >
            <i aria-hidden="true" />{{ t(`marketing.hero.cities.${city}`) }}
          </span>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.hero {
  position: relative;
  display: flex;
  flex-direction: column;
  min-height: var(--home-hero-min-height);
  overflow: clip;
  background: var(--home-canvas);
}
.hero__ambient {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(56% 62% at 74% 38%, var(--home-glow-primary) 0%, transparent 72%),
    radial-gradient(38% 42% at 84% 20%, var(--home-glow-secondary) 0%, transparent 65%);
}
.hero__shade {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    linear-gradient(90deg, var(--home-hero-shade-start) 0%, var(--home-hero-shade-mid) 44%, var(--home-hero-shade-end) 80%),
    linear-gradient(0deg, var(--home-canvas) 0%, transparent 26%);
}

.hero__inner {
  position: relative;
  z-index: 1;
  flex: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  padding-top: calc(var(--home-header-height) + var(--home-hero-pad-top));
  padding-bottom: var(--home-hero-pad-bottom);
}
.hero__copy { max-width: var(--home-hero-copy-width); }

h1 {
  display: grid;
  font-size: var(--home-fs-display);
  line-height: var(--home-lh-display);
  font-weight: var(--font-weight-bold);
  letter-spacing: -.032em;
  color: var(--home-on-dark);
}
.hero__line { display: block; overflow-wrap: anywhere; }

.rotator {
  display: grid;
  min-height: 1.18em;
  overflow: hidden;
}
.rotator em {
  grid-area: 1 / 1;
  display: block;
  font-style: normal;
  background: linear-gradient(90deg, var(--home-sky-300) 0%, var(--home-sky-400) 52%, var(--home-cyan) 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.rotator__size { visibility: hidden; }
.swap-enter-active,
.swap-leave-active {
  transition: transform .52s var(--home-ease), opacity .42s ease;
}
.swap-enter-from { transform: translateY(85%); opacity: 0; }
.swap-leave-to { transform: translateY(-85%); opacity: 0; }

.hero__lead {
  margin-top: var(--space-5);
  max-width: 40em;
  font-size: var(--home-fs-lead);
  line-height: var(--home-lh-body);
  color: var(--home-on-dark-muted);
}
.hero__actions {
  display: flex;
  flex-wrap: wrap;
  gap: var(--space-3);
  margin-top: var(--space-6);
}
.hero__compliance {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  margin-top: var(--space-5);
  font-size: var(--home-fs-small);
  color: var(--home-on-dark-faint);
}
.hero__compliance svg { width: var(--home-icon-sm); height: var(--home-icon-sm); flex: none; color: var(--home-sky-400); }

.exit-strip {
  position: relative;
  z-index: 1;
  display: flex;
  align-items: center;
  gap: var(--space-4);
  padding-block: var(--space-4) var(--space-5);
  border-top: var(--border-width) solid var(--home-line-dark);
}
.exit-strip__label {
  flex: none;
  font-size: var(--home-fs-micro);
  letter-spacing: .08em;
  color: var(--home-on-dark-faint);
}
.exit-strip__rail {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  -webkit-mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent);
  mask-image: linear-gradient(90deg, transparent, #000 6%, #000 94%, transparent);
}
.exit-strip__track {
  display: flex;
  gap: var(--space-6);
  width: max-content;
  font-size: var(--home-fs-small);
  color: var(--home-on-dark-muted);
  animation: exit-marquee 44s linear infinite;
}
.exit-strip__track span {
  display: inline-flex;
  align-items: center;
  gap: var(--space-2);
  white-space: nowrap;
}
.exit-strip__track i {
  width: var(--home-dot-xs);
  height: var(--home-dot-xs);
  border-radius: var(--home-pill);
  background: var(--home-cyan);
  box-shadow: var(--home-exit-dot-shadow);
}
@keyframes exit-marquee { to { transform: translateX(-50%); } }

@media (prefers-reduced-motion: reduce) {
  .exit-strip__track { animation: none; }
  .exit-strip__track span[aria-hidden="true"] { display: none; }
}

@media (max-width: 767px) {
  .hero { min-height: var(--home-hero-min-height-mobile); }
  h1 { font-size: var(--home-fs-display-mobile); }
  .hero__actions .home-btn { flex: 1 1 100%; }
  .exit-strip { padding-block: var(--space-3) var(--space-4); gap: var(--space-3); }
}
</style>
