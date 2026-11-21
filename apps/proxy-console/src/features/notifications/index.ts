import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

export { default as NotificationBell } from './NotificationBell.vue'
export { refreshTrafficNotice, trafficNoticeRevision } from './trafficNotice'
export const messages = { 'zh-CN': zhCN, en }
