import { reactive } from 'vue'

export interface PaginationState {
  page: number
  pageSize: number
  total: number
}

export function usePagination(fetchFn: () => void, defaultPageSize = 20) {
  const pagination = reactive<PaginationState>({
    page: 1,
    pageSize: defaultPageSize,
    total: 0,
  })

  function handlePageChange() {
    fetchFn()
  }

  function handlePageSizeChange() {
    pagination.page = 1
    fetchFn()
  }

  function resetPage() {
    pagination.page = 1
  }

  return { pagination, handlePageChange, handlePageSizeChange, resetPage }
}
