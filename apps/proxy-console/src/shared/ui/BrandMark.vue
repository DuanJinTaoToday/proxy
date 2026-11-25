<script setup lang="ts">
const props = withDefaults(defineProps<{
  surface?: 'light' | 'dark'
  wordmarkClass?: string
}>(), {
  surface: 'light',
  wordmarkClass: '',
})

const markSrc = `${import.meta.env.BASE_URL}favicon.svg`
</script>

<template>
  <span class="brand-mark" :class="`brand-mark--${props.surface}`">
    <img
      :src="markSrc"
      alt=""
      aria-hidden="true"
      class="brand-mark__icon"
      width="32"
      height="32"
      decoding="async"
    >
    <span class="brand-mark__wordmark" :class="props.wordmarkClass">Roc<span class="brand-mark__accent">Proxy</span></span>
  </span>
</template>

<style scoped>
.brand-mark {
  display: inline-flex;
  align-items: center;
  gap: inherit;
  min-width: 0;
  line-height: 1;
}
.brand-mark__icon {
  display: block;
  flex: none;
  width: var(--brand-mark-size);
  height: var(--brand-mark-size);
}
.brand-mark__wordmark {
  letter-spacing: var(--brand-wordmark-tracking);
  line-height: 1;
}
.brand-mark__accent {
  font-weight: var(--font-weight-semibold);
}
.brand-mark--light .brand-mark__accent {
  color: var(--color-brand-accent);
}
.brand-mark--dark .brand-mark__accent {
  color: var(--color-brand-accent-dark);
}
</style>
