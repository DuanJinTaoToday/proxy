import { dayFromNumber, dayNumber, shanghaiToday } from '@/shared/lib/businessDates'

export const usageWeek = () => {
  const today = dayNumber(shanghaiToday())
  return Array.from({ length: 7 }, (_, index) => dayFromNumber(today + index - 6))
}
