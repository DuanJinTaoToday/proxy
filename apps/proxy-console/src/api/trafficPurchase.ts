import { request } from './client'

export interface TrafficPurchaseRequest {
  package_id: string
  package_version: string
  request_key: string
}

interface OrderWire {
  id: string
  status: string
  kind: string
  debited_minor: string
  delivered_bytes: string
}

export async function purchaseTraffic(body: TrafficPurchaseRequest) {
  const order = await request<OrderWire>('/v1/tenant/orders/traffic-packages', { body })
  if (order.kind !== 'traffic_package' || order.status !== 'succeeded' || !order.id) {
    throw new Error('Missing settled traffic order')
  }
  return {
    id: String(order.id),
    debitedMinor: BigInt(order.debited_minor),
    deliveredBytes: BigInt(order.delivered_bytes),
  }
}
