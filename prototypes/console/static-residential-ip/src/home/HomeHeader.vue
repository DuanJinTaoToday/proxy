<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { ElButton, ElDropdown, ElDropdownItem, ElDropdownMenu, ElMessage } from 'element-plus'
import { ArrowDown, Bell, Switch } from '@element-plus/icons-vue'
import { brandName } from './home-demo'

/** dark：滚动后保持暗色玻璃，不变白底（方案 D 的全暗页面）。 */
const props = defineProps<{ dark?: boolean }>()

const solid = ref(false)
const onScroll = () => { solid.value = window.scrollY > 40 }
onMounted(() => { onScroll(); window.addEventListener('scroll', onScroll, { passive: true }) })
onBeforeUnmount(() => window.removeEventListener('scroll', onScroll))

const nav = [
  { label: '首页', href: '#top', active: true },
  { label: '代理产品', href: '#products' },
  { label: '价格', href: '/#dynamic/pricing' },
  { label: '使用教程', href: '#integration' },
]

const guestOnly = () => ElMessage.info('登录后查看通知')
const onLocale = (value: string) => { if (value === 'en') ElMessage.info('英文版将在正式实现时提供') }
</script>

<template>
  <header class="home-header" :class="{ solid: solid && !props.dark, glass: solid && props.dark, 'dark-mode': props.dark }">
    <div class="home-header-inner">
      <a class="home-brand" href="#top"><span class="brand-mark"><Switch /></span>{{ brandName }}</a>
      <nav class="home-nav" aria-label="官网导航">
        <a v-for="item in nav" :key="item.label" :href="item.href" :aria-current="item.active ? 'page' : undefined">{{ item.label }}</a>
      </nav>
      <div class="home-actions">
        <button v-if="!props.dark" class="icon-btn" type="button" aria-label="通知" @click="guestOnly"><Bell /></button>
        <ElDropdown trigger="click" @command="onLocale">
          <button class="locale-btn" type="button">简体中文 <ArrowDown /></button>
          <template #dropdown>
            <ElDropdownMenu>
              <ElDropdownItem command="zh-CN">简体中文</ElDropdownItem>
              <ElDropdownItem command="en">English</ElDropdownItem>
            </ElDropdownMenu>
          </template>
        </ElDropdown>
        <a class="login-link" href="/">登录</a>
        <ElButton type="primary" tag="a" href="/" class="register-btn">注册</ElButton>
      </div>
    </div>
  </header>
</template>

<style scoped>
.home-header { position: fixed; inset: 0 0 auto; z-index: 50; height: 68px; color: #e8f3ef; transition: background .25s, color .25s, box-shadow .25s; }
.home-header.dark-mode { height: 72px; color: #fff; }
.home-header.solid { background: rgba(255, 255, 255, .92); color: var(--ink); box-shadow: 0 1px 0 var(--line); backdrop-filter: blur(12px); }
.home-header-inner { max-width: 1240px; height: 100%; margin: 0 auto; padding: 0 24px; display: flex; align-items: center; gap: 40px; }
.home-brand { display: flex; align-items: center; gap: 10px; font-size: 21px; font-weight: 700; color: inherit; text-decoration: none; letter-spacing: -.01em; }
.home-nav { display: flex; gap: 28px; flex: 1; }
.home-nav a { color: inherit; opacity: .76; text-decoration: none; font-weight: 450; font-size: 14.5px; transition: opacity .2s, color .2s; }
.home-nav a:hover, .home-nav a[aria-current] { opacity: 1; }
.home-nav a[aria-current] { color: var(--mint); }
.dark-mode .home-nav a[aria-current] { color: #fff; font-weight: 500; }
.home-header.glass { background: rgba(2, 8, 8, .82); box-shadow: 0 1px 0 rgba(255, 255, 255, .08); backdrop-filter: blur(16px) saturate(1.2); }
.solid .home-nav a[aria-current] { color: var(--teal); }
.home-actions { display: flex; align-items: center; gap: 18px; }
.home-actions :deep(.el-dropdown) { color: inherit; }
.icon-btn, .locale-btn { display: inline-flex; align-items: center; gap: 6px; border: 0; background: none; color: inherit; opacity: .82; cursor: pointer; padding: 4px; font-size: 14px; transition: opacity .2s; }
.icon-btn:hover, .locale-btn:hover { opacity: 1; }
.icon-btn svg { width: 18px; height: 18px; }
.locale-btn svg { width: 12px; height: 12px; }
.login-link { color: inherit; opacity: .88; text-decoration: none; font-weight: 500; font-size: 14.5px; transition: opacity .2s; }
.login-link:hover { opacity: 1; }
.register-btn { --el-button-bg-color: var(--mint); --el-button-border-color: var(--mint); --el-button-text-color: var(--shell); --el-button-hover-bg-color: #cff7e6; --el-button-hover-border-color: #cff7e6; --el-button-hover-text-color: var(--shell); font-weight: 650; }
.dark-mode .register-btn { --el-button-bg-color: #fff; --el-button-border-color: #fff; --el-button-text-color: #050909; --el-button-hover-bg-color: #e4e8e7; --el-button-hover-border-color: #e4e8e7; --el-button-hover-text-color: #050909; border-radius: 9999px; height: 38px; padding: 0 22px; font-weight: 600; font-size: 14px; }
.solid .register-btn { --el-button-bg-color: var(--teal); --el-button-border-color: var(--teal); --el-button-text-color: #fff; --el-button-hover-bg-color: #177d63; --el-button-hover-border-color: #177d63; --el-button-hover-text-color: #fff; }
@media (max-width: 860px) { .home-nav, .icon-btn, .locale-btn { display: none; } .home-header-inner { justify-content: space-between; } }
</style>
