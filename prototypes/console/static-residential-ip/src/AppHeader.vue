<script setup lang="ts">
import { ref } from 'vue'
import { ElDropdown, ElDropdownItem, ElDropdownMenu, ElMessage, ElMessageBox, ElPopover } from 'element-plus'
import { ArrowDown, Bell, Check, CircleCheck, Close, Switch } from '@element-plus/icons-vue'
import { accountProfile, markRead, notices, readNotices, relativeDay, unreadCount, type Notice } from './state'

type Locale = 'zh-CN' | 'en'
const locales: { value: Locale, label: string }[] = [{ value: 'zh-CN', label: '简体中文' }, { value: 'en', label: 'English' }]
const locale = ref<Locale>('zh-CN')
const bellOpen = ref(false)

function openNotice(notice: Notice) {
  markRead([notice.id])
  bellOpen.value = false
  location.hash = notice.to.slice(1)
}

function onLocale(value: Locale) {
  if (value === 'en') ElMessage.info('英文版文案尚未制作，原型保持中文')
}

async function onAccount(command: string) {
  if (command === 'logout') {
    const ok = await ElMessageBox.confirm('退出后需要重新登录才能进入控制台。', '退出登录？', { confirmButtonText: '退出登录', cancelButtonText: '取消', type: 'warning' }).then(() => true, () => false)
    if (ok) ElMessage.info('原型不接认证，正式版会回到登录页')
    return
  }
  location.hash = command
}
</script>

<template>
  <header class="topbar">
    <div class="brand"><span class="brand-mark"><Switch /></span><span>Proxy</span></div>
    <div class="top-tools">
      <ElPopover v-model:visible="bellOpen" placement="bottom-end" trigger="click" :width="360" popper-class="notice-popper" :show-arrow="false" :offset="10">
        <template #reference>
          <button class="bell" :class="{ active: bellOpen }" type="button" :aria-label="unreadCount ? `通知，${unreadCount} 条未读` : '通知'">
            <Bell /><span v-if="unreadCount" class="bell-dot" aria-hidden="true" />
          </button>
        </template>
        <div class="notice-head">
          <strong>通知</strong>
          <div class="notice-head-actions">
            <button v-if="unreadCount" type="button" class="notice-text-btn" @click="markRead(notices.map(n => n.id))"><Check />全部已读</button>
            <button type="button" class="notice-close" aria-label="关闭通知" @click="bellOpen = false"><Close /></button>
          </div>
        </div>
        <ul v-if="notices.length" class="notice-list">
          <li v-for="n in notices" :key="n.id">
            <button type="button" class="notice-item" :class="{ unread: !readNotices.has(n.id) }" @click="openNotice(n)">
              <span class="notice-dot" aria-hidden="true" />
              <span class="notice-body"><strong>{{ n.title }}</strong><span>{{ n.detail }}</span></span>
              <time class="notice-time">{{ relativeDay(n.at) }}</time>
            </button>
          </li>
        </ul>
        <div v-else class="notice-empty"><CircleCheck /><span>暂无通知</span></div>
      </ElPopover>

      <ElDropdown trigger="click" placement="bottom-end" @command="onLocale">
        <button type="button" class="top-item">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" aria-hidden="true"><circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c2.5 2.7 3.8 5.7 3.8 9s-1.3 6.3-3.8 9c-2.5-2.7-3.8-5.7-3.8-9S9.5 5.7 12 3z" /></svg>
          {{ locales.find(l => l.value === locale)?.label }}<ArrowDown class="chevron" />
        </button>
        <template #dropdown>
          <ElDropdownMenu>
            <ElDropdownItem v-for="l in locales" :key="l.value" :command="l.value" :class="{ 'is-current': l.value === locale }">
              <span class="menu-check"><Check v-if="l.value === locale" /></span>{{ l.label }}
            </ElDropdownItem>
          </ElDropdownMenu>
        </template>
      </ElDropdown>

      <ElDropdown trigger="click" placement="bottom-end" popper-class="account-popper" @command="onAccount">
        <button type="button" class="top-item top-account">{{ accountProfile.name }}<ArrowDown class="chevron" /></button>
        <template #dropdown>
          <div class="account-head">
            <strong>{{ accountProfile.name }}</strong>
            <span>{{ accountProfile.phone }}</span>
          </div>
          <ElDropdownMenu>
            <ElDropdownItem command="account/profile">账户信息</ElDropdownItem>
            <ElDropdownItem command="account/orders">订单记录</ElDropdownItem>
            <ElDropdownItem command="logout">退出登录</ElDropdownItem>
          </ElDropdownMenu>
        </template>
      </ElDropdown>
    </div>
  </header>
</template>
