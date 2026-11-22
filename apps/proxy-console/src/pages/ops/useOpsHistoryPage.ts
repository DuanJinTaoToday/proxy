import { computed, onScopeDispose, shallowRef } from 'vue'

export type OpsHistoryState<Row> =
  | { kind: 'loading' }
  | { kind: 'failed', error: unknown }
  | { kind: 'ready', rows: Row[], nextBeforeId?: string }

export function useOpsHistoryPage<Row>(fetchPage: (beforeId: string | undefined, signal: AbortSignal) => Promise<{
  rows: Row[], nextBeforeId?: string,
}>) {
  const state = shallowRef<OpsHistoryState<Row>>({ kind: 'loading' })
  const cursors = shallowRef<(string | undefined)[]>([undefined])
  const page = computed(() => cursors.value.length)
  let request: AbortController | undefined
  onScopeDispose(() => request?.abort())

  async function load() {
    request?.abort()
    const controller = new AbortController()
    request = controller
    state.value = { kind: 'loading' }
    try {
      const result = await fetchPage(cursors.value.at(-1), controller.signal)
      if (!controller.signal.aborted) state.value = { kind: 'ready', ...result }
    } catch (error) {
      if (!controller.signal.aborted) state.value = { kind: 'failed', error }
    }
  }

  function reset() {
    cursors.value = [undefined]
    void load()
  }

  function previous() {
    if (page.value > 1) {
      cursors.value = cursors.value.slice(0, -1)
      void load()
    }
  }

  function next() {
    if (state.value.kind === 'ready' && state.value.nextBeforeId) {
      cursors.value = [...cursors.value, state.value.nextBeforeId]
      void load()
    }
  }

  return { state, page, load, reset, previous, next }
}
