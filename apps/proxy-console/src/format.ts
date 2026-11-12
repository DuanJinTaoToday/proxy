const shanghai = 'Asia/Shanghai'
const shanghaiDate = new Intl.DateTimeFormat('sv-SE', { timeZone: shanghai })
const shanghaiTime = new Intl.DateTimeFormat('sv-SE', {
  timeZone: shanghai,
  dateStyle: 'short',
  timeStyle: 'medium',
})
const integers = new Intl.NumberFormat('zh-CN')
const bytesPerMB = 1_048_576n

export function bytes(value?: string) {
  return BigInt(value ?? '0')
}

export function formatMB(value: bigint) {
  const negative = value < 0n
  const magnitude = negative ? -value : value
  const hundredths = (magnitude * 100n + bytesPerMB / 2n) / bytesPerMB
  if (magnitude > 0n && hundredths === 0n) return negative ? '−<0.01' : '<0.01'
  return `${negative ? '−' : ''}${integers.format(hundredths / 100n)}.${String(hundredths % 100n).padStart(2, '0')}`
}

export function formatTime(value?: string) {
  return value ? shanghaiTime.format(new Date(value)) : '—'
}

export function defaultDates() {
  const now = Date.now()
  return { start: shanghaiDate.format(now - 6 * 86_400_000), end: shanghaiDate.format(now) }
}

export function validDates(start: string, end: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(start) || !/^\d{4}-\d{2}-\d{2}$/.test(end)) return false
  const first = Date.parse(start)
  const last = Date.parse(end)
  return !start.startsWith('0000') && last >= first && last - first < 93 * 86_400_000
}
