export function readBusinessDate(value: unknown) {
  if (typeof value !== 'string' || !/^(?!0000)\d{4}-\d{2}-\d{2}$/.test(value)) return undefined
  const date = new Date(value)
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value ? value : undefined
}

export function readBusinessDateBounds(startQuery: unknown, endQuery: unknown) {
  const start = readBusinessDate(startQuery)
  const end = readBusinessDate(endQuery)
  return start && end && start > end ? {} : { start, end }
}
