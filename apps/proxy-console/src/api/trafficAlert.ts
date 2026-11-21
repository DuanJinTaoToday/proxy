import { request } from './client'

interface TrafficAlertWire {
  available_bytes?: string
  threshold_bytes?: string
  occurrence?: string
}

const endpoint = '/v1/tenant/traffic-alert'

function readAlert(wire: TrafficAlertWire) {
  return {
    availableBytes: BigInt(wire.available_bytes ?? 0),
    thresholdBytes: wire.threshold_bytes === undefined ? undefined : BigInt(wire.threshold_bytes),
    occurrence: wire.occurrence === undefined ? undefined : BigInt(wire.occurrence).toString(),
  }
}

export async function getTrafficAlert(signal: AbortSignal) {
  return readAlert(await request<TrafficAlertWire>(endpoint, { signal }))
}

export async function setTrafficAlert(thresholdBytes: bigint | undefined) {
  const body = { threshold_bytes: thresholdBytes?.toString() ?? null }
  return readAlert(await request<TrafficAlertWire>(endpoint, { method: 'PUT', body }))
}
