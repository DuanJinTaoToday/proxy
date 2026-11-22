<script setup lang="ts">
// 官网首页原型：组合方案 D 与三个单独方案 A/B/C，通过 ?variant=D|A|B|C&hero=map|globe 切换，首屏以下区块共用。
// D 参照 Shopify 首页重新组织：A 的地图首屏、B 的商品分屏（双色面板）、C 的实时终端（固定高度的接入面板），A/B/C 不受影响。
import { ref, watch } from 'vue'
import { Promotion } from '@element-plus/icons-vue'
import HeroA from './HeroA.vue'
import HeroB from './HeroB.vue'
import HeroC from './HeroC.vue'
import HeroD from './HeroD.vue'
import HomeD from './HomeD.vue'
import HomeHeader from './HomeHeader.vue'
import HomeSections from './HomeSections.vue'
import PrototypeSwitcher from './PrototypeSwitcher.vue'
import { telegram } from './home-demo'

const variants = [
  { key: 'D', name: '组合方案' },
  { key: 'A', name: '数据冲击' },
  { key: 'B', name: '双商品分屏' },
  { key: 'C', name: '实时控制台' },
]
const params = new URLSearchParams(location.search)
const variant = ref(variants.some(v => v.key === params.get('variant')) ? params.get('variant')! : 'D')
const hero = ref<'map' | 'globe'>(params.get('hero') === 'globe' ? 'globe' : 'map')

watch([variant, hero], ([v, h]) => {
  const next = new URLSearchParams(location.search)
  next.set('variant', v)
  next.set('hero', h)
  history.replaceState(null, '', `${location.pathname}?${next}${location.hash}`)
}, { immediate: true })
</script>

<template>
  <div id="top" class="home">
    <HomeHeader :dark="variant === 'D'" />
    <template v-if="variant === 'D'">
      <HeroD :key="`D-${hero}`" :hero="hero" />
      <HomeD />
    </template>
    <template v-else>
      <HeroA v-if="variant === 'A'" :key="`A-${hero}`" :hero="hero" />
      <HeroB v-else-if="variant === 'B'" />
      <HeroC v-else :key="`C-${hero}`" :hero="hero" />
      <HomeSections :key="variant" :show-products="variant !== 'B'" />
    </template>
    <a class="tg-float" :href="telegram.url" target="_blank" rel="noopener" aria-label="Telegram 客服"><Promotion /><span>Telegram 客服</span></a>
    <PrototypeSwitcher v-model:variant="variant" v-model:hero="hero" :variants="variants" />
  </div>
</template>
