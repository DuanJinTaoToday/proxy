import { onScopeDispose, shallowRef, type ShallowRef } from 'vue'
import { getPublicCatalog, type TrafficPackage } from '@/api/products'
import { listStaticOffers, type StaticCatalogFilter, type StaticPurchaseOffer } from '@/api/staticPricing'

export type Loadable<T> =
  | { kind: 'loading' }
  | { kind: 'failed', error: unknown }
  | { kind: 'ready', data: T }

export interface DynamicPricing { onSale: boolean, packages: TrafficPackage[] }
export interface StaticPricing { open: boolean, quotedAt?: string, offers: StaticPurchaseOffer[] }

// 共享静态 IP 的用途与客户购买页一致：0 表示不指定用途。
export const sharedPurposes = [0, 8, 9, 10] as const
export type SharedPurpose = (typeof sharedPurposes)[number]
export type StaticCategory = 'exclusive' | `shared-${SharedPurpose}`

export const staticCategoryOf = (shareMode: 'exclusive' | 'shared', purpose: SharedPurpose = 0): StaticCategory =>
  shareMode === 'exclusive' ? 'exclusive' : `shared-${purpose}`

function filterOf(category: StaticCategory): StaticCatalogFilter {
  if (category === 'exclusive') return { shareMode: 'exclusive' }
  const purposeId = Number(category.slice('shared-'.length))
  return { shareMode: 'shared', purposeId: purposeId || undefined }
}

// 每个分类独立请求、独立失败与重试，某一类失败不影响其他分类展示。
export interface Resource<T> { state: ShallowRef<Loadable<T>>, load: () => Promise<void> }

function useLoadable<T>(fetcher: (signal: AbortSignal) => Promise<T>): Resource<T> {
  const state = shallowRef<Loadable<T>>({ kind: 'loading' })
  let controller: AbortController | undefined
  async function load() {
    controller?.abort()
    const current = controller = new AbortController()
    state.value = { kind: 'loading' }
    try {
      const data = await fetcher(current.signal)
      if (!current.signal.aborted) state.value = { kind: 'ready', data }
    } catch (error) {
      if (!current.signal.aborted) state.value = { kind: 'failed', error }
    }
  }
  onScopeDispose(() => controller?.abort())
  void load()
  return { state, load }
}

export function usePublicPricing() {
  // 动态与静态的开售开关相互独立：动态以商品接口的 open 为准，静态以各自报价接口返回的 open 为准。
  // 动态 open=true 但无在售档位时 canPurchase 为 false，此时应展示无可售流量包而非未开售，故不参与 onSale。
  const dynamic = useLoadable<DynamicPricing>(async signal => {
    const { dynamic: product } = await getPublicCatalog(signal)
    return { onSale: product.open, packages: product.packages }
  })
  const categories: StaticCategory[] = ['exclusive', ...sharedPurposes.map(purpose => staticCategoryOf('shared', purpose))]
  const statics = Object.fromEntries(categories.map(category => [
    category,
    useLoadable<StaticPricing>(signal => listStaticOffers(filterOf(category), signal, { anonymous: true })),
  ])) as Record<StaticCategory, Resource<StaticPricing>>
  return { dynamic, statics }
}
