import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import type { StaticIpExchangeStatus } from '@/api/staticIpExchanges'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

export const exchangeStatusTagType: Record<StaticIpExchangeStatus, 'info' | 'warning' | 'danger' | 'success'> = {
  pending: 'info',
  unknown: 'warning',
  failed: 'danger',
  activating: 'info',
  ready: 'success',
  support_required: 'warning',
}

export function endpoint({ ip, port }: { ip: string, port: number }) {
  return `${ip.includes(':') ? `[${ip}]` : ip}:${port}`
}

export function useStaticIpPresentation() {
  const { locale, t, te } = useI18n()
  const fallbackError = useErrorMessage()
  const dates = computed(() => new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Shanghai',
  }))
  return {
    formatDate: (date: Date) => dates.value.format(date),
    errorMessage: (error: unknown) => {
      if (error instanceof ApiError && te(`staticIps.apiErrors.${error.reason}`)) {
        return t(`staticIps.apiErrors.${error.reason}`)
      }
      return fallbackError(error)
    },
  }
}
