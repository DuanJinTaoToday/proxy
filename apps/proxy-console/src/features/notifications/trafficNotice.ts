import { ref } from 'vue'

export const trafficNoticeRevision = ref(0)

export function refreshTrafficNotice() {
  trafficNoticeRevision.value++
}
