export const bytesPerGB = 1_000_000_000n
const maxInt64 = 9_223_372_036_854_775_807n

export function parseLimitGB(value: string) {
  const parts = /^(\d{1,10})(?:\.(\d{1,9}))?$/.exec(value.trim())
  if (!parts) return
  const bytes = BigInt(parts[1]) * bytesPerGB + BigInt((parts[2] ?? '').padEnd(9, '0'))
  if (bytes <= maxInt64) return bytes
}

export function formatGB(bytes: bigint, locale: string, useGrouping = true) {
  const format = new Intl.NumberFormat(locale, { useGrouping })
  const wholeGB = bytes / bytesPerGB
  const whole = format.format(bytes < 0n && wholeGB === 0n ? -0 : wholeGB)
  const fraction = ((bytes < 0n ? -bytes : bytes) % bytesPerGB).toString().padStart(9, '0').replace(/0+$/, '')
  return fraction ? `${whole}.${fraction}` : whole
}
