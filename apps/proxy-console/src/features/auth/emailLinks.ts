import { onScopeDispose, ref } from 'vue'

// 邮件链接中的一次性令牌。路由守卫在页面渲染前把令牌从地址栏移出，暂存在本标签页的 sessionStorage，
// 避免令牌留在浏览历史、书签、分享链接或 Referer 中；刷新和切换语言仍可继续。成功或令牌失效后立即清除。
export type LinkPurpose = 'reset' | 'recover'

const storageKey = (purpose: LinkPurpose) => `proxy-console.link-token.${purpose}`

export function stashLinkToken(purpose: LinkPurpose, token: string) {
  try {
    if (token) sessionStorage.setItem(storageKey(purpose), token)
    else sessionStorage.removeItem(storageKey(purpose))
  } catch {
    // 存储不可用时，页面按链接无效处理并引导重新获取。
  }
}

export function readLinkToken(purpose: LinkPurpose) {
  try {
    return sessionStorage.getItem(storageKey(purpose)) ?? ''
  } catch {
    return ''
  }
}

export function dropLinkToken(purpose: LinkPurpose) {
  stashLinkToken(purpose, '')
}

// 发送邮件后的重发冷却，按秒倒计时。
export function useCooldown(seconds = 60) {
  const remaining = ref(0)
  let timer: ReturnType<typeof setInterval> | undefined
  const stop = () => {
    if (timer) clearInterval(timer)
    timer = undefined
  }
  function start() {
    stop()
    remaining.value = seconds
    timer = setInterval(() => {
      remaining.value -= 1
      if (remaining.value <= 0) stop()
    }, 1000)
  }
  onScopeDispose(stop)
  return { remaining, start }
}
