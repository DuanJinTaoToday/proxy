import type { FlagCode } from '@/shared/ui/FlagIcon.vue'

const regionMatchers: [RegExp, FlagCode][] = [
  [/美国|united states|\busa?\b|america/i, 'us'],
  [/英国|united kingdom|\buk\b|britain|england/i, 'gb'],
  [/日本|japan|\bjp\b/i, 'jp'],
  [/德国|germany|\bde\b/i, 'de'],
  [/法国|france|\bfr\b/i, 'fr'],
  [/加拿大|canada|\bca\b/i, 'ca'],
  [/意大利|italy|\bit\b/i, 'it'],
  [/西班牙|spain|\bes\b/i, 'es'],
  [/新加坡|singapore|\bsg\b/i, 'sg'],
  [/巴西|brazil|\bbr\b/i, 'br'],
  [/韩国|korea|\bkr\b/i, 'kr'],
  [/马来|malaysia|\bmy\b/i, 'my'],
  [/泰国|thailand|\bth\b/i, 'th'],
  [/越南|vietnam|\bvn\b/i, 'vn'],
  [/印尼|印度尼西亚|indonesia|\bid\b/i, 'id'],
  [/澳|australia|\bau\b/i, 'au'],
  [/香港|hong kong|\bhk\b/i, 'hk'],
  [/台湾|taiwan|\btw\b/i, 'tw'],
  [/阿联酋|迪拜|uae|dubai/i, 'ae'],
  [/菲律宾|philippines|\bph\b/i, 'ph'],
]

export function flagForRegion(region: string): FlagCode {
  for (const [pattern, code] of regionMatchers) {
    if (pattern.test(region)) return code
  }
  return 'other'
}
