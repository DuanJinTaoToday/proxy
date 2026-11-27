export const UUID_PATTERN = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/

export type RateParseError = 'empty' | 'format' | 'precision' | 'range'

export type RateParseResult =
  | { ok: true, bps: number }
  | { ok: false, error: RateParseError }

export function isValidCustomerKey(value: string): boolean {
  return UUID_PATTERN.test(value)
}

export function parsePercentToBps(raw: string): RateParseResult {
  const trimmed = raw.trim()
  if (trimmed === '') return { ok: false, error: 'empty' }
  if (!/^\d+(?:\.\d+)?$/.test(trimmed)) return { ok: false, error: 'format' }

  const [integerPart, fractionPart = ''] = trimmed.split('.')
  if (fractionPart.length > 2) return { ok: false, error: 'precision' }

  const normalizedInt = integerPart.replace(/^0+(?=\d)/, '')
  if (normalizedInt.length > 3) return { ok: false, error: 'range' }

  const bps = Number(normalizedInt) * 100 + Number(fractionPart.padEnd(2, '0'))
  if (!Number.isInteger(bps) || bps < 0 || bps > 10_000) return { ok: false, error: 'range' }
  return { ok: true, bps }
}

export function formatBpsInput(bps: number): string {
  const whole = Math.trunc(bps / 100)
  const fraction = Math.abs(bps % 100)
  if (fraction === 0) return String(whole)
  return `${whole}.${String(fraction).padStart(2, '0').replace(/0+$/, '')}`
}

export function formatBpsPercent(bps: number): string {
  return `${formatBpsInput(bps)}%`
}
