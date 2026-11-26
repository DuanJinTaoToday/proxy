import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

// 公开报价的失败原因：优先用本页的稳定错误码文案（如供应商预算或刷新失败），其余沿用通用错误。
export function usePricingError() {
  const { t, te } = useI18n()
  const errorMessage = useErrorMessage()
  return (error: unknown) => {
    const key = error instanceof ApiError ? `publicPricing.errors.${error.reason}` : ''
    return key && te(key) ? t(key) : errorMessage(error)
  }
}
