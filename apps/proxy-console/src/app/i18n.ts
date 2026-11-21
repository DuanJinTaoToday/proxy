import { createI18n } from 'vue-i18n'
import dayjs from 'dayjs'
import 'dayjs/locale/zh-cn'
import zhCnElement from 'element-plus/es/locale/lang/zh-cn'
import enElement from 'element-plus/es/locale/lang/en'
import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'
import { messages as authMessages } from '@/features/auth'
import { messages as dynamicProxyMessages } from '@/features/dynamic-proxy'
import { messages as checkoutMessages } from '@/features/checkout'
import { messages as dashboardMessages } from '@/features/dashboard/messages'
import { messages as notificationMessages } from '@/features/notifications'

export const locales = ['zh-CN', 'en'] as const
export type Locale = (typeof locales)[number]
export const defaultLocale: Locale = 'zh-CN'

const preferenceKey = 'proxy-console.locale'
const elementLocales = { 'zh-CN': zhCnElement, en: enElement }
const dayjsLocales = { 'zh-CN': 'zh-cn', en: 'en' }

export const isLocale = (value: unknown): value is Locale => locales.includes(value as Locale)

type Messages = Record<string, unknown>
type MessageSchema = typeof zhCN

// 应用层文案加上各业务模块入口导出的翻译，模块各自维护自己的命名空间。
const moduleMessages: Record<Locale, Messages>[] = [authMessages, dynamicProxyMessages, dashboardMessages, notificationMessages, checkoutMessages]
function messagesFor(locale: Locale, base: MessageSchema) {
  return moduleMessages.reduce<Messages>((all, module) => ({ ...all, ...module[locale] }), base) as MessageSchema
}

export const i18n = createI18n({
  legacy: false,
  locale: defaultLocale,
  fallbackLocale: defaultLocale,
  messages: { 'zh-CN': messagesFor('zh-CN', zhCN), en: messagesFor('en', en) },
})

export const elementLocaleOf = (locale: string) => elementLocales[isLocale(locale) ? locale : defaultLocale]

// 应用、Element Plus 和日期控件共用同一 locale，并同步文档语言。
export function applyLocale(locale: Locale) {
  i18n.global.locale.value = locale
  dayjs.locale(dayjsLocales[locale])
  document.documentElement.lang = locale
}

// 控制台语言来自浏览器记住的偏好，缺省和无效值回到中文。
export function preferredLocale(): Locale {
  const saved = localStorage.getItem(preferenceKey)
  return isLocale(saved) ? saved : defaultLocale
}

export function rememberLocale(locale: Locale) {
  localStorage.setItem(preferenceKey, locale)
}
