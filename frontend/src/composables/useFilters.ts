import { reactive } from 'vue'

export function useFilters<T extends Record<string, unknown>>(
  defaults: T,
  onApply: () => void,
) {
  const filters = reactive({ ...defaults }) as T

  function applyFilters() {
    onApply()
  }

  function resetFilters() {
    Object.assign(filters, defaults)
    onApply()
  }

  return { filters, applyFilters, resetFilters }
}
