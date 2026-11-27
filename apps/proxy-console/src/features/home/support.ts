const env = import.meta.env

function text(value: unknown) {
  if (typeof value !== 'string') return undefined
  return value.trim() || undefined
}

function contactURL(value: unknown) {
  const raw = text(value)
  if (!raw) return undefined
  try {
    const url = new URL(raw)
    return url.username || url.password || url.port ? undefined : url
  } catch {
    return undefined
  }
}

const telegramUsername = /^[A-Za-z][A-Za-z0-9_]{3,31}$/
const telegramPhone = /^\+[1-9][0-9]{6,14}$/
const telegramReserved = new Set(['joinchat', 'addlist', 'addstickers', 'addemoji', 'addtheme', 'setlanguage', 'share', 'proxy', 'socks', 'login', 'confirmphone', 'invoice', 'boost', 'contact'])

function telegramTarget(url: URL) {
  if (url.protocol === 'tg:' && url.hostname === 'resolve' && !url.pathname) {
    return url.searchParams.get('domain')
  }
  if (url.protocol !== 'https:') return undefined
  if (url.hostname === 't.me' || url.hostname === 'telegram.me') {
    return /^\/([^/]+)\/?$/.exec(url.pathname)?.[1]
  }
  if (url.pathname === '/') return /^([a-z0-9_]+)\.t\.me$/.exec(url.hostname)?.[1]
}

function parseTelegram(value: unknown) {
  const url = contactURL(value)
  if (!url) return undefined
  const target = telegramTarget(url)
  if (!target || telegramReserved.has(target.toLowerCase())) return undefined
  if (telegramUsername.test(target) || telegramPhone.test(target)) return `https://t.me/${target}`
}

const qqPaths = new Map([
  ['qm.qq.com', /^\/(q\/[A-Za-z0-9_-]+|cgi-bin\/qm\/qr)$/],
  ['wpa.qq.com', /^\/msgrd$/],
  ['wpa1.qq.com', /^\/[A-Za-z0-9_-]+$/],
])

function parseQq(value: unknown) {
  const url = contactURL(value)
  if (url?.protocol !== 'https:' || !qqPaths.get(url.hostname)?.test(url.pathname)) return undefined
  return url.href
}

function parsePhone(value: unknown) {
  const display = text(value)
  if (!display || !/^\+?[\d ()-]+$/.test(display)) return undefined
  const digits = display.replace(/\D/g, '')
  if (digits.length < 5 || digits.length > 20) return undefined
  return { display, value: `${display.startsWith('+') ? '+' : ''}${digits}` }
}

export type SupportChannel = 'chat' | 'telegram' | 'qq' | 'phone'
export type PhoneContact = NonNullable<ReturnType<typeof parsePhone>>

// The live-chat entry stays inactive until the provider path passes acceptance.
export const supportConfig = {
  chat: undefined,
  telegram: parseTelegram(env.VITE_SUPPORT_TELEGRAM_URL),
  qq: parseQq(text(env.VITE_SUPPORT_QQ_URL) ?? env.VITE_SUPPORT_QQ),
  phone: parsePhone(env.VITE_SUPPORT_PHONE),
} satisfies Record<SupportChannel, string | PhoneContact | undefined>

export const supportChannels: SupportChannel[] = ['chat', 'telegram', 'qq', 'phone']
export const hasSupportContact = supportChannels.some(channel => supportConfig[channel])
