import { onScopeDispose, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'

export function useCopy(resetMs = 2000) {
  const { t } = useI18n()
  const copiedKey = ref<string>()
  let timer: number | undefined

  async function copy(key: string, value: string, notice?: string) {
    try {
      await navigator.clipboard.writeText(value)
      copiedKey.value = key
      clearTimeout(timer)
      timer = window.setTimeout(() => { copiedKey.value = undefined }, resetMs)
      if (notice) ElMessage({ message: notice, type: 'info', offset: 80 })
    } catch {
      ElMessage.error(t('marketing.copy.failed'))
    }
  }

  onScopeDispose(() => clearTimeout(timer))
  return { copiedKey, copy }
}
