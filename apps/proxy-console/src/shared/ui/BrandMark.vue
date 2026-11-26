<script setup lang="ts">
const markSrc = `${import.meta.env.BASE_URL}favicon.svg`
</script>

<template>
  <span class="brand-mark">
    <img
      :src="markSrc"
      alt=""
      aria-hidden="true"
      class="brand-mark__icon"
      width="32"
      height="32"
      decoding="async"
    >
    <span class="brand-mark__wordmark">Roc<span class="brand-mark__accent">Proxy</span></span>
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
  display: var(--brand-wordmark-display, inline);
  letter-spacing: var(--brand-wordmark-tracking);
  line-height: 1;
}
.brand-mark__accent {
  color: var(--color-brand-accent);
  font-weight: var(--font-weight-semibold);
}
</style>
