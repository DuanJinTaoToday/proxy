// 客服联系方式由 #112 的正式参数通过环境变量注入。未配置的渠道不渲染外链，也不写死占位号码。
const env = import.meta.env

const text = (value: unknown) => (typeof value === 'string' && value.trim() ? value.trim() : undefined)
const link = (value: unknown) => {
  const url = text(value)
  return url && /^(https?:\/\/|tg:\/\/|tencent:\/\/)/i.test(url) ? url : undefined
}

export type SupportChannel = 'chat' | 'telegram' | 'qq' | 'phone'

// 按产品决策：QQ 与 Telegram 为外链，电话为点击复制。
export const supportConfig = {
  chat: link(env.VITE_SUPPORT_CHAT_URL),
  telegram: link(env.VITE_SUPPORT_TELEGRAM_URL),
  qq: link(env.VITE_SUPPORT_QQ_URL ?? env.VITE_SUPPORT_QQ),
  phone: text(env.VITE_SUPPORT_PHONE),
} satisfies Record<SupportChannel, string | undefined>

export const supportChannels: SupportChannel[] = ['chat', 'telegram', 'qq', 'phone']
export const isCopyChannel = (channel: SupportChannel) => channel === 'phone'
