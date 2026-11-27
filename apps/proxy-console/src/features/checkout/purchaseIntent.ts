import type { LocationQuery, LocationQueryRaw, LocationQueryValue } from 'vue-router'
import { maxStaticPurchaseLines, maxStaticPurchaseQuantity, type StaticCatalogFilter } from '@/api/staticPricing'

const sharedPurposeIds = new Set([8, 9, 10])
const positiveIntPattern = /^[1-9]\d*$/
const nonNegativeIntPattern = /^(?:0|[1-9]\d*)$/
const termPattern = /^[A-Za-z0-9_-]{1,32}$/
const packageIdPattern = /^[^\s]{1,128}$/
const maxQuantityBigInt = BigInt(maxStaticPurchaseQuantity)

export const staticIntentQueryKeys = ['shareMode', 'purpose', 'term', 'items', 'prices'] as const
export const dynamicIntentQueryKeys = ['package', 'price', 'version'] as const

export interface StaticPurchaseSnapshot {
  unitMinor?: bigint
  available?: number
}

export interface StaticPurchaseIntentLine {
  nodeId: string
  quantity: number
  snapshotUnitMinor?: bigint
  snapshotAvailable?: number
}

export interface StaticPurchaseIntent {
  filter: StaticCatalogFilter
  term: string
  lines: StaticPurchaseIntentLine[]
  key: string
}

export type ParsedStaticPurchaseIntent =
  | { kind: 'none' }
  | { kind: 'invalid', key: string }
  | { kind: 'valid', key: string, intent: StaticPurchaseIntent }

export interface StaticIntentDraftLine {
  nodeId: string
  quantity: number
  unitMinor: bigint
  available: number
}

export interface DynamicPurchaseIntent {
  packageId: string
  snapshotPriceMinor?: bigint
  snapshotVersion?: string
  key: string
}

export type ParsedDynamicPurchaseIntent =
  | { kind: 'none' }
  | { kind: 'invalid', key: string }
  | { kind: 'valid', key: string, intent: DynamicPurchaseIntent }

function readSingleQuery(value: LocationQueryValue | LocationQueryValue[] | undefined) {
  if (value === undefined) return { present: false as const }
  if (Array.isArray(value) || value === null) return { present: true as const, valid: false as const }
  return { present: true as const, valid: true as const, value }
}

function queryKeySignature(query: LocationQuery, keys: readonly string[]) {
  return JSON.stringify(keys.map(key => query[key] ?? null))
}

function hasAnyQueryKey(query: LocationQuery, keys: readonly string[]) {
  return keys.some(key => query[key] !== undefined)
}

export function stripQueryKeys(query: LocationQuery, keys: readonly string[]) {
  const keySet = new Set(keys)
  const nextQuery: LocationQueryRaw = {}
  let changed = false
  for (const [key, value] of Object.entries(query)) {
    if (keySet.has(key)) {
      changed = true
      continue
    }
    if (value !== undefined) nextQuery[key] = value
  }
  return { nextQuery, changed }
}

export function buildStaticPurchaseIntentQuery(input: {
  shareMode: 'exclusive' | 'shared'
  purposeId?: number
  term: string
  lines: StaticIntentDraftLine[]
}): LocationQueryRaw {
  const query: LocationQueryRaw = {
    shareMode: input.shareMode,
    term: input.term || undefined,
    items: input.lines.map(item => `${item.nodeId}:${item.quantity}`).join(','),
    prices: input.lines.map(item => `${item.nodeId}:${item.unitMinor.toString()}:${item.available}`).join(','),
  }
  if (input.shareMode === 'shared' && input.purposeId) {
    query.purpose = String(input.purposeId)
  }
  return query
}

function parseStaticPriceSnapshots(raw: string, allowedNodeIds: ReadonlySet<string>) {
  const snapshots = new Map<string, StaticPurchaseSnapshot>()
  if (!raw) return snapshots
  for (const part of raw.split(',')) {
    const segments = part.split(':')
    if (segments.length !== 2 && segments.length !== 3) continue
    const [nodeId, unitMinorStr, availableStr] = segments
    if (!nodeId || !unitMinorStr || !allowedNodeIds.has(nodeId) || snapshots.has(nodeId)) continue
    if (!positiveIntPattern.test(nodeId) || !positiveIntPattern.test(unitMinorStr)) continue
    let available: number | undefined
    if (availableStr !== undefined) {
      if (!nonNegativeIntPattern.test(availableStr)) continue
      const availableBig = BigInt(availableStr)
      if (availableBig > maxQuantityBigInt) continue
      available = Number(availableBig)
    }
    snapshots.set(nodeId, { unitMinor: BigInt(unitMinorStr), available })
  }
  return snapshots
}

export function parseStaticPurchaseIntent(query: LocationQuery): ParsedStaticPurchaseIntent {
  if (!hasAnyQueryKey(query, staticIntentQueryKeys)) return { kind: 'none' }
  const key = queryKeySignature(query, staticIntentQueryKeys)

  const shareModeRaw = readSingleQuery(query.shareMode)
  if (!shareModeRaw.present || !shareModeRaw.valid || (shareModeRaw.value !== 'exclusive' && shareModeRaw.value !== 'shared')) {
    return { kind: 'invalid', key }
  }
  const shareMode = shareModeRaw.value

  const purposeRaw = readSingleQuery(query.purpose)
  let filter: StaticCatalogFilter
  if (shareMode === 'exclusive') {
    if (purposeRaw.present && (!purposeRaw.valid || purposeRaw.value !== '')) {
      return { kind: 'invalid', key }
    }
    filter = { shareMode: 'exclusive' }
  } else {
    let purposeId: number | undefined
    if (purposeRaw.present) {
      if (!purposeRaw.valid) return { kind: 'invalid', key }
      if (purposeRaw.value !== '') {
        if (!nonNegativeIntPattern.test(purposeRaw.value)) return { kind: 'invalid', key }
        const parsedPurpose = Number(purposeRaw.value)
        if (parsedPurpose !== 0 && !sharedPurposeIds.has(parsedPurpose)) return { kind: 'invalid', key }
        purposeId = parsedPurpose || undefined
      }
    }
    filter = { shareMode: 'shared', purposeId }
  }

  const termRaw = readSingleQuery(query.term)
  if (!termRaw.present || !termRaw.valid || !termPattern.test(termRaw.value)) {
    return { kind: 'invalid', key }
  }

  const itemsRaw = readSingleQuery(query.items)
  if (!itemsRaw.present || !itemsRaw.valid || !itemsRaw.value) {
    return { kind: 'invalid', key }
  }

  const parts = itemsRaw.value.split(',')
  if (parts.length < 1 || parts.length > maxStaticPurchaseLines) {
    return { kind: 'invalid', key }
  }

  const seenNodeIds = new Set<string>()
  const parsedLines: StaticPurchaseIntentLine[] = []
  let totalQuantity = 0n

  for (const part of parts) {
    const segments = part.split(':')
    if (segments.length !== 2) return { kind: 'invalid', key }
    const [nodeId, qtyStr] = segments
    if (!nodeId || !qtyStr || !positiveIntPattern.test(nodeId) || !positiveIntPattern.test(qtyStr)) {
      return { kind: 'invalid', key }
    }
    if (seenNodeIds.has(nodeId)) return { kind: 'invalid', key }
    const qtyBig = BigInt(qtyStr)
    if (qtyBig < 1n || qtyBig > maxQuantityBigInt) return { kind: 'invalid', key }
    totalQuantity += qtyBig
    if (totalQuantity > maxQuantityBigInt) return { kind: 'invalid', key }
    seenNodeIds.add(nodeId)
    parsedLines.push({ nodeId, quantity: Number(qtyBig) })
  }

  const pricesRaw = readSingleQuery(query.prices)
  if (pricesRaw.present && pricesRaw.valid && pricesRaw.value) {
    const snapshots = parseStaticPriceSnapshots(pricesRaw.value, seenNodeIds)
    for (const line of parsedLines) {
      const snapshot = snapshots.get(line.nodeId)
      if (snapshot) {
        line.snapshotUnitMinor = snapshot.unitMinor
        line.snapshotAvailable = snapshot.available
      }
    }
  }

  return {
    kind: 'valid',
    key,
    intent: {
      filter,
      term: termRaw.value,
      lines: parsedLines,
      key,
    },
  }
}

export function buildDynamicPurchaseIntentQuery(tier: {
  id: string
  priceMinor: bigint
  version: string
}): LocationQueryRaw {
  return {
    package: tier.id,
    price: tier.priceMinor.toString(),
    version: tier.version,
  }
}

export function parseDynamicPurchaseIntent(query: LocationQuery): ParsedDynamicPurchaseIntent {
  if (!hasAnyQueryKey(query, dynamicIntentQueryKeys)) return { kind: 'none' }
  const key = queryKeySignature(query, dynamicIntentQueryKeys)

  const packageRaw = readSingleQuery(query.package)
  if (!packageRaw.present || !packageRaw.valid || !packageIdPattern.test(packageRaw.value)) {
    return { kind: 'invalid', key }
  }

  const priceRaw = readSingleQuery(query.price)
  const snapshotPriceMinor = priceRaw.present && priceRaw.valid && positiveIntPattern.test(priceRaw.value)
    ? BigInt(priceRaw.value)
    : undefined

  const versionRaw = readSingleQuery(query.version)
  const snapshotVersion = versionRaw.present && versionRaw.valid && versionRaw.value.trim().length > 0
    ? versionRaw.value
    : undefined

  return {
    kind: 'valid',
    key,
    intent: {
      packageId: packageRaw.value,
      snapshotPriceMinor,
      snapshotVersion,
      key,
    },
  }
}
