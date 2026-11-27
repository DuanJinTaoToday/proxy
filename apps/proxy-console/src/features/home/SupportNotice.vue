<script setup lang="ts">
import { useI18n } from 'vue-i18n'

// 官网页脚与控制台客服弹窗共用的联系须知：仅作提示，不收集任何信息。颜色继承所在区域。
const { t } = useI18n()
const items = ['noticeVerify', 'noticeContinue', 'noticeCredentials'] as const
</script>

<template>
  <div class="support-notice" role="note" :aria-label="t('marketing.support.noticeTitle')">
    <p class="support-notice__title">{{ t('marketing.support.noticeTitle') }}</p>
    <ul>
      <li v-for="item in items" :key="item">{{ t(`marketing.support.${item}`) }}</li>
    </ul>
  </div>
</template>

<style scoped>
.support-notice { font-size: var(--font-size-xs); line-height: var(--line-height-normal); }
.support-notice__title { margin: 0 0 var(--space-1); font-weight: var(--font-weight-medium); }
.support-notice ul { display: grid; gap: var(--space-1); margin: 0; padding-inline-start: var(--space-4); }
</style>
