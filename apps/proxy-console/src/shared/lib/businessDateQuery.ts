function readBusinessDate(value: unknown) {
  if (typeof value !== 'string' || !/^(?!0000)\d{4}-\d{2}-\d{2}$/.test(value)) return undefined
  const date = new Date(value)
  return Number.isFinite(date.getTime()) && date.toISOString().slice(0, 10) === value ? value : undefined
}

type BusinessDateBounds = { start?: string, end?: string }

export function readBusinessDateBounds(startQuery: unknown, endQuery: unknown): BusinessDateBounds {
  const start = readBusinessDate(startQuery)
  const end = readBusinessDate(endQuery)
  return start && end && start > end ? {} : { start, end }
}

export function updateBusinessDateBounds(bounds: BusinessDateBounds, field: keyof BusinessDateBounds, value: unknown): BusinessDateBounds {
  const date = readBusinessDate(value)
  const { start, end } = bounds
  if (field === 'start') return { start: date, end: date && end && date > end ? undefined : end }
  return { start: date && start && date < start ? undefined : start, end: date }
}
