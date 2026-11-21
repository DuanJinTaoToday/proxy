import { ref, watch, type Ref } from 'vue'

// 客户控制台和运营后台在不同标签页打开时共用这份演示数据：写入 localStorage，另一个标签页通过 storage 事件同步。
const prefix = 'proxy-prototype:v1:'

export function persisted<T>(key: string, initial: () => T): Ref<T> {
  const raw = localStorage.getItem(prefix + key)
  const state = ref(raw ? JSON.parse(raw) : initial()) as Ref<T>
  watch(state, value => localStorage.setItem(prefix + key, JSON.stringify(value)), { deep: true, immediate: true })
  window.addEventListener('storage', event => {
    if (event.key === prefix + key && event.newValue) state.value = JSON.parse(event.newValue)
  })
  return state
}

export function resetDemoData() {
  Object.keys(localStorage).filter(key => key.startsWith(prefix)).forEach(key => localStorage.removeItem(key))
  location.reload()
}
