import type { TrafficPackage } from '@/api/products'
import { bytesPerGB } from '@/shared/lib/trafficUnits'

export function unitPrice(tier: TrafficPackage) {
  return (tier.priceMinor * bytesPerGB + tier.capacityBytes / 2n) / tier.capacityBytes
}
