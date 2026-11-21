import { request, requestText } from './client'

interface DailyUsageWire {
  date: string
  billable_bytes: string
  updated_at?: string
}

export interface DailyUsageDay {
  date: string
  bytes: bigint
  updatedAt: string | undefined
}

export interface DailyUsageFilter {
  start: string
  end: string
  clientId?: string
}

const dailyPath = '/v1/tenant/usage/daily'

function usageQuery({ start, end, clientId }: DailyUsageFilter) {
  return { start_date: start, end_date: end, client_id: clientId }
}

export async function getDailyUsage(filter: DailyUsageFilter, signal: AbortSignal): Promise<DailyUsageDay[]> {
  const wire = await request<{ days?: DailyUsageWire[] }>(dailyPath, { query: usageQuery(filter), signal })
  return (wire.days ?? []).map(day => ({ date: day.date, bytes: BigInt(day.billable_bytes), updatedAt: day.updated_at }))
}

export function exportDailyUsage(filter: DailyUsageFilter, signal: AbortSignal) {
  return requestText(dailyPath + '.csv', { query: usageQuery(filter), signal }, 'text/csv')
}
