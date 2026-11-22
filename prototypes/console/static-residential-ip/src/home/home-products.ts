// 商品卡的数据：起价与开售状态直接读控制台原型的共用状态，运营后台调价、下架档位或关闭开售后首页随之变化。
import { computed } from 'vue'
import { supplierNodes } from '../catalog'
import { productOnSale, trafficPackages, type SaleProduct } from '../state'

const yuan = (cents: number) => `¥${(cents / 100).toFixed(2).replace(/\.00$/, '')}`

const dynamicFrom = computed(() => {
  const listed = trafficPackages.value.filter(p => p.listed)
  if (!listed.length) return undefined
  return Math.min(...listed.map(p => p.cents / p.gb))
})

const staticFrom = computed(() => {
  const prices = supplierNodes.flatMap(node => [...node.exclusive.prices, ...node.shared.prices]).filter(p => p.code === 'yp1' && p.yuan > 0)
  return Math.min(...prices.map(p => p.yuan * 100))
})

export const staticCountries = new Set(supplierNodes.map(node => node.country)).size

export interface HomeProduct {
  key: SaleProduct
  name: string
  unit: string
  tagline: string
  bullets: string[]
  price?: string
  priceUnit: string
  onSale: boolean
  href: string
}

export const products = computed<HomeProduct[]>(() => [
  {
    key: 'dynamic',
    name: '动态住宅代理',
    unit: '按 GB 计费',
    tagline: '真人住宅网络轮换出口，适合采集、验证和大规模并发。',
    bullets: ['9000 万+ 住宅 IP 轮换出口', '流量长期有效，用完为止', '最多 10 个子账号，独立账密与用量上限', '会话保持，同一会话尽量沿用出口'],
    price: dynamicFrom.value === undefined ? undefined : yuan(dynamicFrom.value),
    priceUnit: '/GB 起',
    onSale: productOnSale.value.dynamic && dynamicFrom.value !== undefined,
    href: '/#dynamic/pricing',
  },
  {
    key: 'static',
    name: '静态住宅 IP',
    unit: '按 IP 计费',
    tagline: '固定的真人住宅 IP，适合店铺、社媒等需要长期稳定身份的账号。',
    bullets: ['独享与共享两种类型', `${staticCountries} 个国家和地区可选`, '到期前续费，保留原 IP', '单条复制或批量导出 TXT'],
    price: yuan(staticFrom.value),
    priceUnit: '/IP·30 天起',
    onSale: productOnSale.value.static,
    href: '/#static/pricing',
  },
])

export const primaryHref = computed(() => products.value.find(p => p.onSale)?.href ?? '#products')

export type SaleScenario = 'both' | 'dynamic' | 'static'
export const saleScenario = computed<SaleScenario>({
  get: () => productOnSale.value.dynamic && productOnSale.value.static ? 'both' : productOnSale.value.static ? 'static' : 'dynamic',
  set: value => { productOnSale.value = { dynamic: value !== 'static', static: value !== 'dynamic' } },
})
