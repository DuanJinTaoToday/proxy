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
