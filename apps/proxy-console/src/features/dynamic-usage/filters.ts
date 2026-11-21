import type { LocationQuery } from 'vue-router'
import type { DailyUsageFilter } from '@/api/usage'
import { dayFromNumber, dayNumber, shanghaiToday, validBusinessDate } from '@/shared/lib/businessDates'

type RecentDays = '7' | '30'
export type UsageRange = { kind: RecentDays } | { kind: 'custom', start: string, end: string }

export function customRange(start: unknown, end: unknown): UsageRange | undefined {
  if (typeof start !== 'string' || typeof end !== 'string') return
  if (validBusinessDate(start) && validBusinessDate(end) && start <= end) return { kind: 'custom', start, end }
}

export function selectedRange(query: LocationQuery): UsageRange {
  if (query.range === 'custom') {
    const range = customRange(query.startDate, query.endDate)
    if (range) return range
  }
  return { kind: query.range === '30' ? '30' : '7' }
}

export function selectedAccount(query: LocationQuery) {
  const { subaccountId } = query
  if (typeof subaccountId === 'string' && /^[\da-f]{8}(?:-[\da-f]{4}){3}-[\da-f]{12}$/i.test(subaccountId)) {
    return subaccountId.toLowerCase()
  }
}

export function usageFilter(range: UsageRange, clientId?: string): DailyUsageFilter {
  if (range.kind === 'custom') return { start: range.start, end: range.end, clientId }
  const end = shanghaiToday()
  return { start: dayFromNumber(dayNumber(end) - Number(range.kind) + 1), end, clientId }
}
