import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { getOCRSettings, updateOCRSettings, testLLM } from '@/api/config'
import type { OCRSettings } from '@/types/config'

export function useOcrSettingsForm() {
  const loading = ref(false)
  const saving = ref(false)
  const testingLLM = ref(false)
  const authToken = ref('')
  const extraBodyText = ref('')

  const form = reactive<OCRSettings>({
    enabled: false,
    binary: 'ocr',
    audience: 'human',
    default_severity: 'major',
    provider: '',
    url: '',
    model: '',
    use_anthropic: false,
    auth_token: '',
    auth_header: 'Authorization',
    extra_headers: '',
    language: 'Chinese',
    concurrency: 2,
    max_git_procs: 4,
    timeout_minutes: 10,
    max_tools: 0,
    background: '',
    tools_path: '',
    tools_config: '',
    telemetry_enabled: false,
    telemetry_exporter: '',
    telemetry_otlp_endpoint: '',
    telemetry_content_logging: false,
    reasoning_mode: 'light',
    reasoning_budget: 0,
    context_window: 0,
    max_output_tokens: 0,
  })

  async function fetchSettings() {
    loading.value = true
    try {
      const settings = await getOCRSettings()
      Object.assign(form, settings)
      if (!form.binary) form.binary = 'ocr'
      if (!form.audience) form.audience = 'human'
      if (!form.default_severity) form.default_severity = 'major'
      if (!form.reasoning_mode) form.reasoning_mode = 'light'
      authToken.value = ''
      extraBodyText.value = form.extra_body ? JSON.stringify(form.extra_body, null, 2) : ''
    } catch (e: any) {
      ElMessage.error(e?.message || '加载配置失败')
    } finally {
      loading.value = false
    }
  }

  async function handleSave() {
    if (!form.url || !form.model) {
      ElMessage.warning('请填写接口地址和模型')
      return
    }
    saving.value = true
    try {
      const settings: OCRSettings = { ...form, auth_token: authToken.value || form.auth_token }
      const saved = await updateOCRSettings(settings)
      Object.assign(form, saved)
      authToken.value = ''
      ElMessage.success('配置已保存')
    } catch (e: any) {
      ElMessage.error(e?.message || '保存失败')
    } finally {
      saving.value = false
    }
  }

  async function handleTestLLM() {
    if (form.url && form.model) {
      try {
        const settings: OCRSettings = { ...form, auth_token: authToken.value || form.auth_token }
        const saved = await updateOCRSettings(settings)
        Object.assign(form, saved)
        authToken.value = ''
      } catch {
        ElMessage.warning('保存配置失败，将测试已保存的配置')
      }
    }
    testingLLM.value = true
    try {
      const result = await testLLM()
      if (result.success) {
        ElMessage.success(result.message || '连接测试成功')
      } else {
        ElMessage.error(result.message || '连接测试失败')
      }
    } catch (e: any) {
      ElMessage.error(e?.message || '测试失败')
    } finally {
      testingLLM.value = false
    }
  }

  function syncExtraBody(showError = false): boolean {
    const text = extraBodyText.value.trim()
    if (!text) {
      form.extra_body = undefined
      return true
    }
    try {
      const parsed = JSON.parse(text)
      if (typeof parsed !== 'object' || Array.isArray(parsed) || parsed === null) {
        throw new Error('必须是 JSON 对象')
      }
      form.extra_body = parsed
      return true
    } catch (e: any) {
      if (showError) ElMessage.error(`额外请求体格式错误：${e.message || e}`)
      return false
    }
  }

  return {
    form,
    loading,
    saving,
    testingLLM,
    authToken,
    extraBodyText,
    fetchSettings,
    handleSave,
    handleTestLLM,
    syncExtraBody,
  }
}
