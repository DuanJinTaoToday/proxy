// 中国大陆手机号：与控制面一致，接受带或不带 +86、空格和连字符。

export function normalizePhone(value: string) {
  const digits = value.replace(/[\s-]/g, '').replace(/^\+?86/, '')
  return /^1\d{10}$/.test(digits) ? digits : ''
}

// 顶栏等处显示时隐藏中间四位，例如 +86 138 **** 6021。
export function maskPhone(phone: string) {
  const digits = normalizePhone(phone)
  return digits ? `+86 ${digits.slice(0, 3)} **** ${digits.slice(7)}` : ''
}

// 登录后回跳只接受本站路径，拒绝站外地址、协议相对地址和登录注册页本身。
export function safeRedirect(value: unknown, fallback = '/console') {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return fallback
  const url = new URL(value, window.location.origin)
  if (url.origin !== window.location.origin || /^\/(zh-CN|en)\/(login|register)\/?$/.test(url.pathname)) return fallback
  return url.pathname + url.search + url.hash
}
