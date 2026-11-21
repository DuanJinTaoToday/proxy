const shanghai = 'Asia/Shanghai'
const isoBusinessDate = new Intl.DateTimeFormat('sv-SE', {
  timeZone: shanghai, year: 'numeric', month: '2-digit', day: '2-digit',
})

export const usageWeek = () => {
  const today = new Date(isoBusinessDate.format(new Date()) + 'T00:00:00Z')
  return Array.from({ length: 7 }, (_, index) => {
    const date = new Date(today)
    date.setUTCDate(today.getUTCDate() + index - 6)
    return date.toISOString().slice(0, 10)
  })
}

export function shanghaiDateTime(date: Date, locale: string) {
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short', timeZone: shanghai }).format(date)
}

export function formatYuan(minor: bigint, locale: string) {
  const major = minor / 100n
  const format = new Intl.NumberFormat(locale, { style: 'currency', currency: 'CNY', currencyDisplay: 'narrowSymbol' })
  const absoluteMinor = minor < 0n ? -minor : minor
  return format.formatToParts(minor < 0n && major === 0n ? -0 : major)
    .map(part => part.type === 'fraction' ? (absoluteMinor % 100n).toString().padStart(2, '0') : part.value)
    .join('')
}
