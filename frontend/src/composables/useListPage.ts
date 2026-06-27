import { ref, reactive, computed, onMounted, type Ref } from 'vue'
import { usePagination } from './usePagination'
import { useFilters } from './useFilters'
import { usePolling } from './usePolling'

export interface ListPageOptions<T, F extends Record<string, unknown>> {
  fetchFn: (params: F & { page: number; page_size: number }) => Promise<{ items: T[]; total: number; [key: string]: unknown }>
  defaultFilters: F
  defaultPageSize?: number
  shouldPoll?: (result: unknown) => boolean
  pollInterval?: number
  immediate?: boolean
}

export function useListPage<T, F extends Record<string, unknown>>(options: ListPageOptions<T, F>) {
  const {
    fetchFn,
    defaultFilters,
    defaultPageSize = 20,
    shouldPoll,
    pollInterval = 10000,
    immediate = true,
  } = options

  const loading = ref(false)
  const items: Ref<T[]> = ref([])
  const extraData: Ref<Record<string, unknown>> = ref({})

  const { pagination, handlePageChange, handlePageSizeChange, resetPage } = usePagination(
    () => fetchItems(),
    defaultPageSize,
  )

  const { filters, hasFilters, applyFilters, resetFilters } = useFilters(
    defaultFilters,
    () => {
      resetPage()
      fetchItems()
    },
  )

  async function fetchItems(background = false) {
    if (!background) loading.value = true
    try {
      const params = {
        ...filters,
        page: pagination.page,
        page_size: pagination.pageSize,
      } as F & { page: number; page_size: number }
      const result = await fetchFn(params)
      items.value = result.items
      pagination.total = result.total

      const { items: _items, total: _total, ...rest } = result as { items: T[]; total: number; [key: string]: unknown }
      extraData.value = rest

      if (shouldPoll && shouldPoll(result)) {
        schedulePolling()
      }
    } finally {
      if (!background) loading.value = false
    }
  }

  const { safeFetch, schedule: schedulePolling } = shouldPoll
    ? usePolling(
        (bg) => fetchItems(bg),
        () => {
          const running = (extraData.value as Record<string, unknown>).running
          return typeof running === 'number' ? running > 0 : false
        },
        pollInterval,
      )
    : { safeFetch: () => Promise.resolve(), schedule: () => {} }

  function refresh() {
    fetchItems()
  }

  function refreshBackground() {
    fetchItems(true)
  }

  if (immediate) {
    onMounted(() => fetchItems())
  }

  return {
    loading,
    items,
    extraData,
    pagination,
    filters,
    hasFilters,
    handlePageChange,
    handlePageSizeChange,
    applyFilters,
    resetFilters,
    refresh,
    refreshBackground,
    fetchItems,
  }
}
