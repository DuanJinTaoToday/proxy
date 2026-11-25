// 邮箱只做基础格式检查，最终以控制面校验为准。
// 收件地址：只去掉首尾空白、保留大小写，用于发送验证码和恢复邮件，邮件按客户核实的原样地址投递。
export function emailAddress(value: string) {
  return value.trim()
}

// 账号标识：与控制面一致转为小写，用于登录、注册、会话和本地数据的账号键。
export function normalizeEmail(value: string) {
  return emailAddress(value).toLowerCase()
}

export function isEmail(value: string) {
  const email = emailAddress(value)
  return email.length <= 254 && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
}

// 与控制面一致：至少 12 个字符（按字符而非 UTF-16 单元计），最多 1024 字节。
export function passwordProblem(value: string): 'tooShort' | 'tooLong' | undefined {
  if ([...value].length < 12) return 'tooShort'
  if (new TextEncoder().encode(value).length > 1024) return 'tooLong'
  return undefined
}

// 认证流程自身的页面不作为回跳目标。
const authPages = /^\/(zh-CN|en)\/(login|register|forgot-password|reset-password|recover-email)\/?$/

// 登录后回跳只接受本站路径，拒绝站外地址、协议相对地址和认证页面本身。
export function safeRedirect(value: unknown, fallback = '/console') {
  if (typeof value !== 'string' || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) return fallback
  const url = new URL(value, window.location.origin)
  if (url.origin !== window.location.origin || authPages.test(url.pathname)) return fallback
  return url.pathname + url.search + url.hash
}

// 在认证页面之间切换时只带上已通过校验的回跳地址。
export function redirectQuery(value: unknown): Record<string, string> {
  return typeof value === 'string' && safeRedirect(value, '') ? { redirect: value } : {}
}
