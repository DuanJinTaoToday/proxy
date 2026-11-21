import type { TrafficPackage } from '@/api/products'
import { bytesPerGB } from '@/shared/lib/trafficUnits'

export function unitPrice(tier: TrafficPackage) {
  return (tier.priceMinor * bytesPerGB + tier.capacityBytes / 2n) / tier.capacityBytes
}

export function savingsPercent(tier: TrafficPackage, base: TrafficPackage) {
  const original = base.priceMinor * tier.capacityBytes
  const saving = original - tier.priceMinor * base.capacityBytes
  return saving > 0n ? (saving * 100n + original / 2n) / original : 0n
}
