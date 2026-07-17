import { ref, reactive, computed, onMounted, type Ref } from 'vue'
import { usePagination } from './usePagination'
import { useFilters } from './useFilters'
import { usePolling } from './usePolling'

export interface ListPageOptions<T, F extends Record<string, unknown>> {
  fetchFn: (params: F & { page: number; page_size: number; sort_by?: string; sort_order?: string }) => Promise<{ items: T[]; total: number; [key: string]: unknown }>
  defaultFilters: F
  defaultPageSize?: number
  shouldPoll?: (result: unknown) => boolean
  pollInterval?: number
  immediate?: boolean
  initialSort?: { by: string; order: 'asc' | 'desc' }
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

  const sort = reactive<{ by: string; order: 'asc' | 'desc' }>(
    options.initialSort ? { ...options.initialSort } : { by: '', order: 'desc' },
  )

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
        ...(sort.by ? { sort_by: sort.by, sort_order: sort.order } : {}),
      } as F & { page: number; page_size: number; sort_by?: string; sort_order?: string }
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

  // 服务端排序：变更排序时回到第 1 页并重新查询。by 置空则回退到后端默认排序。
  function applySort(by: string, order: 'asc' | 'desc') {
    sort.by = by
    sort.order = order
    resetPage()
    fetchItems()
  }

  function clearSort() {
    sort.by = ''
    resetPage()
    fetchItems()
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
    sort,
    hasFilters,
    handlePageChange,
    handlePageSizeChange,
    applyFilters,
    resetFilters,
    applySort,
    clearSort,
    refresh,
    refreshBackground,
    fetchItems,
  }
}
