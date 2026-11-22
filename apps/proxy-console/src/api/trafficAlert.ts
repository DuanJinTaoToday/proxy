import { request } from './client'

interface TrafficAlertWire {
  available_bytes?: string
  threshold_bytes?: string
  occurrence?: string
  triggered_at?: string
}

const endpoint = '/v1/tenant/traffic-alert'

function readAlert(wire: TrafficAlertWire) {
  const triggeredAt = wire.triggered_at ? new Date(wire.triggered_at) : undefined
  return {
    availableBytes: BigInt(wire.available_bytes ?? 0),
    thresholdBytes: wire.threshold_bytes === undefined ? undefined : BigInt(wire.threshold_bytes),
    occurrence: wire.occurrence === undefined ? undefined : BigInt(wire.occurrence).toString(),
    triggeredAt: triggeredAt && Number.isFinite(triggeredAt.getTime()) ? triggeredAt : undefined,
  }
}

export async function getTrafficAlert(signal: AbortSignal) {
  return readAlert(await request<TrafficAlertWire>(endpoint, { signal }))
}

export async function setTrafficAlert(thresholdBytes: bigint | undefined) {
  const body = { threshold_bytes: thresholdBytes?.toString() ?? null }
  return readAlert(await request<TrafficAlertWire>(endpoint, { method: 'PUT', body }))
}
