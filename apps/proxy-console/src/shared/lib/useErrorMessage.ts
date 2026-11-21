import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'

// 把请求错误翻译为当前语言的提示：优先按控制面错误码，其次区分网络失败，其余显示状态码。
export function useErrorMessage() {
  const { t, te } = useI18n()
  return (error: unknown) => {
    if (error instanceof ApiError) {
      if (te(`errors.${error.reason}`)) return t(`errors.${error.reason}`)
      if (error.reason === 'NETWORK') return t('errors.network')
      return t('errors.unknown', { status: error.status })
    }
    return t('errors.unknown', { status: '—' })
  }
}
