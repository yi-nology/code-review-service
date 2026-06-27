import { reactive, computed } from 'vue'

export function useFilters<T extends Record<string, unknown>>(
  defaults: T,
  onApply: () => void,
) {
  const filters = reactive({ ...defaults }) as T

  const hasFilters = computed(() =>
    Object.keys(defaults).some(key => filters[key as keyof T] !== defaults[key as keyof T]),
  )

  function applyFilters() {
    onApply()
  }

  function resetFilters() {
    Object.assign(filters, defaults)
    onApply()
  }

  return { filters, hasFilters, applyFilters, resetFilters }
}
