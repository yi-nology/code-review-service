import { inject } from 'vue'
import type { InjectionKey, Ref } from 'vue'
import type { OCRSettings, LLMProvider, CustomProvider } from '@/types/config'

export interface SettingsFormContext {
  form: OCRSettings
  saving: Ref<boolean>
  testingLLM: Ref<boolean>
  extraBodyText: Ref<string>
  save: () => void
  testLLM: () => void
  syncExtraBody: (showError?: boolean) => boolean
  selectedProvider: Ref<LLMProvider | CustomProvider | null>
  selectedProviderType: Ref<'builtin' | 'custom' | null>
  selectProvider: (provider: LLMProvider | CustomProvider, type: 'builtin' | 'custom') => void
}

export const settingsFormKey: InjectionKey<SettingsFormContext> = Symbol('settingsForm')

export function useSettingsForm(): SettingsFormContext {
  const ctx = inject(settingsFormKey)
  if (!ctx) throw new Error('useSettingsForm must be called within a settings provider')
  return ctx
}
