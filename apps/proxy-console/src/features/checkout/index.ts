import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

export {
  buildDynamicPurchaseIntentQuery,
  buildStaticPurchaseIntentQuery,
  dynamicIntentQueryKeys,
  parseDynamicPurchaseIntent,
  parseStaticPurchaseIntent,
  staticIntentQueryKeys,
  stripQueryKeys,
  type DynamicPurchaseIntent,
  type ParsedDynamicPurchaseIntent,
  type ParsedStaticPurchaseIntent,
  type StaticIntentDraftLine,
  type StaticPurchaseIntent,
  type StaticPurchaseIntentLine,
  type StaticPurchaseSnapshot,
} from './purchaseIntent'
export { useTrafficCheckout, type DynamicIntentNotice, type TrafficCheckoutOptions } from './useTrafficCheckout'
export const messages = { 'zh-CN': zhCN, en }
