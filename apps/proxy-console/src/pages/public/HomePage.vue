<script setup lang="ts">
import { HomeCoverage, HomeFaq, HomeFooter, HomeHero, HomePricing, HomeSolutions, SupportChannels } from '@/features/home'
import '@/styles/homepage.css'
</script>

<template>
  <div id="top" class="homepage">
    <HomeHero />
    <HomePricing />
    <HomeCoverage />
    <HomeSolutions />
    <HomeFaq />
    <HomeFooter />
    <SupportChannels variant="dock" />
  </div>
</template>
