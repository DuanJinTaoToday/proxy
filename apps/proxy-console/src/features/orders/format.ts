import { useI18n } from 'vue-i18n'
import type { OrderItem, OrderSummary } from '@/api/orders'
import { formatGB } from '@/shared/lib/trafficUnits'
import { shanghaiDateTime } from '@/shared/lib/businessDates'
import { formatYuan } from '@/shared/lib/money'

const statusTypes = {
  pending: 'info',
  succeeded: 'success',
  partial: 'warning',
  failed: 'danger',
  unknown: 'info',
} as const

const maxOrderId = 9_223_372_036_854_775_807n

export function readOrderSearch(value: unknown) {
  if (typeof value !== 'string') return
  const digits = value.trim().replace(/^#/, '')
  if (/^\d{1,19}$/.test(digits)) return digits
}

export function readOrderId(value: unknown) {
  const digits = readOrderSearch(value)
  if (!digits) return
  const id = BigInt(digits)
  if (id > 0n && id <= maxOrderId) return id.toString()
}

export function useOrderFormatting() {
  const { t, locale } = useI18n()
  const money = (minor: bigint) => formatYuan(minor, locale.value)
  const dateTime = (date: Date) => shanghaiDateTime(date, locale.value, 'compact')
  const capacity = (bytes: bigint) => t('orders.gb', { value: formatGB(bytes, locale.value) })
  function itemDescription(item: OrderItem) {
    if (item.capacityBytes > 0n) return t('orders.trafficPackage', { capacity: capacity(item.capacityBytes) })
    return item.specification || t('orders.unavailable')
  }

  function delivery(order: OrderSummary) {
    if (order.kind === 'traffic_package') return capacity(order.deliveredBytes)
    if (order.kind === 'unknown') return t('orders.unavailable')
    return t('orders.ipCount', { count: new Intl.NumberFormat(locale.value).format(order.deliveredCount) })
  }

  return { money, dateTime, itemDescription, delivery, statusTypes }
}
