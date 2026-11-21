import type { TrafficPurchaseRequest } from '@/api/trafficPurchase'

export function pendingTrafficPurchase(tenantId: string) {
  const key = `proxy-console.traffic-purchase.${tenantId}`

  function read(): TrafficPurchaseRequest | undefined {
    const payload: unknown = JSON.parse(localStorage.getItem(key) ?? 'null')
    if (typeof payload === 'object' && payload !== null
      && 'package_id' in payload && typeof payload.package_id === 'string'
      && 'package_version' in payload && typeof payload.package_version === 'string'
      && 'request_key' in payload && typeof payload.request_key === 'string') {
      return {
        package_id: payload.package_id,
        package_version: payload.package_version,
        request_key: payload.request_key,
      }
    }
  }

  return {
    read,
    reserve(payload: TrafficPurchaseRequest) {
      return navigator.locks.request(key, () => {
        const existing = read()
        if (!existing) localStorage.setItem(key, JSON.stringify(payload))
        return existing
      })
    },
    clear(payload: TrafficPurchaseRequest) {
      return navigator.locks.request(key, () => {
        if (read()?.request_key === payload.request_key) localStorage.removeItem(key)
      })
    },
  }
}
