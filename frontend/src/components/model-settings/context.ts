import { inject } from 'vue'
import type { InjectionKey, Ref } from 'vue'
import type { OCRSettings } from '@/types/config'

export interface ModelSettingsContext {
  form: OCRSettings
  saving: Ref<boolean>
  testingLLM: Ref<boolean>
  extraBodyText: Ref<string>
  save: () => void
  testLLM: () => void
  syncExtraBody: (showError?: boolean) => boolean
}

export const modelSettingsKey: InjectionKey<ModelSettingsContext> = Symbol('modelSettings')

export function useModelSettings(): ModelSettingsContext {
  const ctx = inject(modelSettingsKey)
  if (!ctx) throw new Error('useModelSettings must be called within ModelSettings')
  return ctx
}
