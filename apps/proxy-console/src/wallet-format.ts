const integer = new Intl.NumberFormat('zh-CN')
const timestamp = new Intl.DateTimeFormat('zh-CN', {
  dateStyle: 'medium', timeStyle: 'medium', timeZone: 'Asia/Shanghai', hourCycle: 'h23',
})

export function money(value: string | bigint = '0') {
  const cents = BigInt(value)
  const abs = cents < 0n ? -cents : cents
  return `${cents < 0n ? '−' : ''}¥${integer.format(abs / 100n)}.${String(abs % 100n).padStart(2, '0')}`
}

export function flowAmount(bytes = '0', forInput = false) {
  const amount = BigInt(bytes)
  const abs = amount < 0n ? -amount : amount
  const fraction = String(abs % 1_000_000_000n).padStart(9, '0').replace(/0+$/, '')
  const whole = abs / 1_000_000_000n
  const value = (amount < 0n ? '−' : '') + (forInput ? String(whole) : integer.format(whole)) + (fraction ? '.' + fraction : '')
  return forInput ? value : value + ' GB'
}

export function dateTime(value: string) {
  return timestamp.format(new Date(value))
}

export const orderKinds: Record<string, string> = {
  traffic_package: '流量包', static_purchase: '静态 IP 购买', static_renewal: '静态 IP 续费',
}

export const orderStatuses: Record<string, string> = {
  pending: '处理中', succeeded: '成功', partial: '部分成功', failed: '失败',
}
