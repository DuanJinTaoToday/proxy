// 客服联系方式由 #112 的正式参数在构建时注入。未配置或不合规的渠道不渲染外链，也不写死占位号码。
const env = import.meta.env

const text = (value: unknown) => (typeof value === 'string' && value.trim() ? value.trim() : undefined)

// 只接受指定服务域名上的 HTTPS 地址：不带账号口令、不改端口，统一输出浏览器可直接打开的规范地址。
function httpsOn(value: string | undefined, hosts: readonly string[]) {
  let url: URL
  try { url = new URL(value ?? '') } catch { return undefined }
  if (url.protocol !== 'https:' || url.username || url.password || url.port) return undefined
  return hosts.includes(url.hostname) ? url : undefined
}

// Telegram 统一输出 https://t.me/<用户名>；官方 tg://resolve?domain=<用户名> 换成等价的 HTTPS 地址。
// t.me 上的保留路径会触发加群、代理设置等客户端动作，不作为客服账号。
const telegramName = /^[A-Za-z][A-Za-z0-9_]{3,31}$/
const telegramReserved = new Set(['joinchat', 'addlist', 'addstickers', 'addemoji', 'addtheme', 'setlanguage', 'share', 'proxy', 'socks', 'login', 'confirmphone', 'invoice', 'boost', 'contact'])
export function parseTelegram(value: unknown) {
  const raw = text(value)
  const name = raw && /^tg:\/\/resolve\?/i.test(raw)
    ? new URLSearchParams(raw.slice(raw.indexOf('?') + 1)).get('domain')
    : /^\/([^/]+)\/?$/.exec(httpsOn(raw, ['t.me', 'telegram.me'])?.pathname ?? '')?.[1]
  return name && telegramName.test(name) && !telegramReserved.has(name.toLowerCase()) ? `https://t.me/${name}` : undefined
}

// QQ 只接受腾讯官方的 HTTPS 名片或会话链接：qm.qq.com 的名片分享（/q/…、/cgi-bin/qm/qr）
// 与 wpa.qq.com 的临时会话跳转（/msgrd）。tencent:// 和 mqqwpa:// 只能由已安装的 QQ 客户端处理，浏览器无法打开，不予接受。
const qqPaths: Record<string, RegExp> = { 'qm.qq.com': /^\/(q\/[A-Za-z0-9]+|cgi-bin\/qm\/qr)$/, 'wpa.qq.com': /^\/msgrd$/ }
export function parseQq(value: unknown) {
  const url = httpsOn(text(value), Object.keys(qqPaths))
  return url && qqPaths[url.hostname]?.test(url.pathname) ? url.href : undefined
}

// 电话保留配置的展示文本，复制时只复制可拨号的号码（可带国际区号 +）。
export function parsePhone(value: unknown) {
  const display = text(value)
  if (!display || !/^\+?[\d ()-]+$/.test(display)) return undefined
  const digits = display.replace(/\D/g, '')
  if (digits.length < 5 || digits.length > 20) return undefined
  return { display, value: `${display.startsWith('+') ? '+' : ''}${digits}` }
}

export type SupportChannel = 'chat' | 'telegram' | 'qq' | 'phone'
export type PhoneContact = NonNullable<ReturnType<typeof parsePhone>>

// 按产品决策：QQ 与 Telegram 为外链，电话为点击复制。
// 在线客服暂无经过校验的接入方式，保留“即将开通”入口，不读取 VITE_SUPPORT_CHAT_URL，避免客户被引向未校验的聊天页。
export const supportConfig = {
  chat: undefined,
  telegram: parseTelegram(env.VITE_SUPPORT_TELEGRAM_URL),
  qq: parseQq(text(env.VITE_SUPPORT_QQ_URL) ?? env.VITE_SUPPORT_QQ),
  phone: parsePhone(env.VITE_SUPPORT_PHONE),
} satisfies Record<SupportChannel, string | PhoneContact | undefined>

export const supportChannels: SupportChannel[] = ['chat', 'telegram', 'qq', 'phone']
export const hasSupportContact = supportChannels.some(channel => supportConfig[channel])
