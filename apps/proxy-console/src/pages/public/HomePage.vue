<script setup lang="ts">
import { HomeHero, HomeProducts, HomeSections } from '@/features/home'
import '@/styles/homepage.css'
</script>

<template>
  <div id="top" class="homepage">
    <HomeHero />
    <HomeProducts />
    <HomeSections />
  </div>
</template>
