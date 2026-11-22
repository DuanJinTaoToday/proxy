const shanghai = 'Asia/Shanghai'
const isoBusinessDate = new Intl.DateTimeFormat('sv-SE', {
  timeZone: shanghai, year: 'numeric', month: '2-digit', day: '2-digit',
})
const dayMilliseconds = 86_400_000

export function shanghaiToday() {
  return isoBusinessDate.format(new Date())
}

export function validBusinessDate(date: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(date) || date.startsWith('0000')) return false
  const ordinal = dayNumber(date)
  return Number.isFinite(ordinal) && dayFromNumber(ordinal) === date
}

export function dayNumber(date: string) {
  return Date.parse(date + 'T00:00:00Z') / dayMilliseconds
}

export function dayFromNumber(ordinal: number) {
  return new Date(ordinal * dayMilliseconds).toISOString().slice(0, 10)
}

export function shanghaiDateTime(date: Date, locale: string, style: 'localized' | 'compact' = 'localized') {
  if (style === 'compact' && locale === 'zh-CN') {
    return new Intl.DateTimeFormat(locale, {
      timeZone: shanghai, year: 'numeric', month: '2-digit', day: '2-digit',
      hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
    }).format(date)
  }
  return new Intl.DateTimeFormat(locale, { dateStyle: 'medium', timeStyle: 'short', timeZone: shanghai }).format(date)
}
