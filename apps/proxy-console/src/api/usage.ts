import { request, requestCSV } from './control'

export interface UsageFilter {
  start_date: string
  end_date: string
  client_id: string
}

export interface UsageDay {
  date: string
  billable_bytes: string
  updated_at?: string
}

export interface TrafficAlert {
  available_bytes?: string
  threshold_bytes?: string
  occurrence?: string
  triggered_at?: string
}

function dailyPath(filter: UsageFilter, extension = '') {
  return '/v1/tenant/usage/daily' + extension + '?' + new URLSearchParams({ ...filter })
}

export function listDailyUsage(token: string, filter: UsageFilter, signal: AbortSignal) {
  return request<{ days: UsageDay[] }>(dailyPath(filter), { token, signal })
}

export function exportDailyUsage(token: string, filter: UsageFilter, signal: AbortSignal) {
  return requestCSV(dailyPath(filter, '.csv'), { token, signal })
}

export function getTrafficAlert(token: string, signal: AbortSignal) {
  return request<TrafficAlert>('/v1/tenant/traffic-alert', { token, signal })
}

export function setTrafficAlert(token: string, threshold: string | null, signal: AbortSignal) {
  return request<TrafficAlert>('/v1/tenant/traffic-alert', {
    token, signal, method: 'PUT', body: { threshold_bytes: threshold },
  })
}
