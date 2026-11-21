import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ApiError } from '@/api/client'
import { useErrorMessage } from '@/shared/lib/useErrorMessage'

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
