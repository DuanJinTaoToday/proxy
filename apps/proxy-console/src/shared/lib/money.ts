export function formatYuan(minor: bigint, locale: string) {
  const major = minor / 100n
  const format = new Intl.NumberFormat(locale, { style: 'currency', currency: 'CNY', currencyDisplay: 'narrowSymbol' })
  const absoluteMinor = minor < 0n ? -minor : minor
  return format.formatToParts(minor < 0n && major === 0n ? -0 : major)
    .map(part => part.type === 'fraction' ? (absoluteMinor % 100n).toString().padStart(2, '0') : part.value)
    .join('')
}
