import type { TrafficPurchaseRequest } from '@/api/trafficPurchase'

export function pendingTrafficPurchase(tenantId: string) {
  const key = `proxy-console.traffic-purchase.${tenantId}`
  return {
    read(): TrafficPurchaseRequest | undefined {
      const payload: unknown = JSON.parse(sessionStorage.getItem(key) ?? 'null')
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
    },
    save(payload: TrafficPurchaseRequest) {
      sessionStorage.setItem(key, JSON.stringify(payload))
    },
    clear(payload: TrafficPurchaseRequest) {
      const saved = this.read()
      if (saved?.request_key === payload.request_key) sessionStorage.removeItem(key)
    },
  }
}
