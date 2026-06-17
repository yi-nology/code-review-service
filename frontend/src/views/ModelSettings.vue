<template>
  <AppLayout>
    <div class="model-settings-page">
      <header class="page-header">
        <div class="eyebrow">MODEL SETTINGS</div>
        <h1>审查引擎配置</h1>
        <p>管理 Open Code Review 的模型连接、CLI 执行方式与默认审查行为。</p>
      </header>

      <el-card v-loading="loading">
        <el-tabs v-model="activeTab" class="settings-tabs">
          <el-tab-pane label="模型连接" name="connection">
            <ModelConnectionTab />
          </el-tab-pane>
          <el-tab-pane label="执行参数" name="execution">
            <ExecutionParamsTab />
          </el-tab-pane>
          <el-tab-pane label="审查侧重点" name="review">
            <ReviewFocusTab />
          </el-tab-pane>
        </el-tabs>
      </el-card>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, provide, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import ModelConnectionTab from '@/components/model-settings/ModelConnectionTab.vue'
import ExecutionParamsTab from '@/components/model-settings/ExecutionParamsTab.vue'
import ReviewFocusTab from '@/components/model-settings/ReviewFocusTab.vue'
import { modelSettingsKey, type ModelSettingsContext } from '@/components/model-settings/context'
import { getOCRSettings, updateOCRSettings, testLLM } from '@/api/config'
import type { OCRSettings } from '@/types/config'

const loading = ref(false)
const saving = ref(false)
const testingLLM = ref(false)
const extraBodyText = ref('')
const activeTab = ref('connection')

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
  language: 'Chinese',
  concurrency: 2,
  max_git_procs: 4,
  timeout_minutes: 10,
  max_tools: 0,
  background: '',
})

function syncExtraBody(showError = true) {
  const text = extraBodyText.value.trim()
  if (!text) {
    form.extra_body = ''
    return true
  }
  try {
    JSON.parse(text)
    form.extra_body = text
    return true
  } catch {
    if (showError) ElMessage.error('额外请求体必须是合法 JSON')
    return false
  }
}

async function handleSave() {
  if (!syncExtraBody()) return
  saving.value = true
  try {
    await updateOCRSettings(form)
    ElMessage.success('配置已保存')
  } finally {
    saving.value = false
  }
}

async function handleTestLLM() {
  testingLLM.value = true
  try {
    const result = await testLLM()
    if (result.success) {
      ElMessage.success(result.message || 'LLM 连接测试成功')
    } else {
      ElMessage.error(result.message || 'LLM 连接测试失败')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '测试失败')
  } finally {
    testingLLM.value = false
  }
}

async function fetchSettings() {
  loading.value = true
  try {
    const settings = await getOCRSettings()
    Object.assign(form, settings)
    if (!form.binary) form.binary = 'ocr'
    if (!form.audience) form.audience = 'human'
    if (!form.default_severity) form.default_severity = 'major'
    extraBodyText.value = typeof settings.extra_body === 'string'
      ? settings.extra_body
      : settings.extra_body
        ? JSON.stringify(settings.extra_body, null, 2)
        : ''
  } finally {
    loading.value = false
  }
}

const ctx: ModelSettingsContext = {
  form,
  saving,
  testingLLM,
  extraBodyText,
  save: handleSave,
  testLLM: handleTestLLM,
  syncExtraBody,
}
provide(modelSettingsKey, ctx)

onMounted(fetchSettings)
</script>

<style scoped>
.model-settings-page {
  width: min(960px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 4px 0 8px;
  color: var(--app-text);
  font-size: 28px;
}

.page-header p {
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}

.settings-tabs :deep(.el-tabs__content) {
  padding-top: 12px;
}

@media (max-width: 720px) {
  .model-settings-page {
    padding: 22px 14px 36px;
  }

  .page-header h1 {
    font-size: 24px;
  }

  :deep(.el-form-item) {
    display: block;
  }

  :deep(.el-form-item__label) {
    display: block;
    width: auto !important;
    margin-bottom: 6px;
    text-align: left;
  }

  :deep(.el-form-item__content) {
    margin-left: 0 !important;
  }
}
</style>
