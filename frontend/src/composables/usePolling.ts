import { onBeforeUnmount } from 'vue'

export function usePolling(
  fetchFn: (background?: boolean) => Promise<void>,
  shouldContinue: () => boolean,
  intervalMs = 10000,
) {
  let timer: ReturnType<typeof setTimeout> | null = null
  let inFlight = false

  async function safeFetch(background = false) {
    if (inFlight) return
    inFlight = true
    await fetchFn(background)
    inFlight = false
  }

  function schedule() {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
    if (!shouldContinue()) return
    timer = setTimeout(() => {
      if (document.visibilityState === 'visible') {
        safeFetch(true).finally(schedule)
      } else {
        schedule()
      }
    }, intervalMs)
  }

  onBeforeUnmount(() => {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
  })

  return { safeFetch, schedule }
}
