import { reactive } from 'vue'

export function usePagination(fetchFn: () => void, defaultPageSize = 20) {
  const pagination = reactive({
    page: 1,
    pageSize: defaultPageSize,
    total: 0,
  })

  function handlePageSizeChange() {
    pagination.page = 1
    fetchFn()
  }

  function resetPage() {
    pagination.page = 1
  }

  return { pagination, handlePageSizeChange, resetPage }
}
