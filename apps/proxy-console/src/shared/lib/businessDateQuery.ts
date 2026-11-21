import { validBusinessDate } from './businessDates'

function readBusinessDate(value: unknown) {
  return typeof value === 'string' && validBusinessDate(value) ? value : undefined
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
