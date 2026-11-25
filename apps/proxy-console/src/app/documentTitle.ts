import type { RouteLocationNormalizedLoaded } from 'vue-router'

export type EntryTitleKind = 'public' | 'console' | 'ops'

export function entryTitleKindOf(route: Pick<RouteLocationNormalizedLoaded, 'matched' | 'meta'>): EntryTitleKind {
  if (route.matched.some(record => record.meta.requiresOperator) || route.meta.operatorGuestOnly) return 'ops'
  if (route.matched.some(record => record.meta.requiresAuth)) return 'console'
  return 'public'
}

export function formatDocumentTitle(
  route: Pick<RouteLocationNormalizedLoaded, 'matched' | 'meta'>,
  translate: (key: string) => string,
): string {
  const parts: string[] = []
  const titleKey = route.meta.titleKey
  if (titleKey) parts.push(translate(titleKey))

  const entryKind = entryTitleKindOf(route)
  if (entryKind === 'console') parts.push(translate('app.consoleTitle'))
  else if (entryKind === 'ops') parts.push(translate('app.opsTitle'))

  parts.push(translate('app.name'))
  return parts.join(' · ')
}
