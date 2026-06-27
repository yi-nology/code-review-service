<template>
  <AppLayout>
    <div class="model-settings-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">MODEL SETTINGS</div>
          <h1>模型配置</h1>
          <p>配置代码审查使用的 AI 模型和参数</p>
        </div>
        <div class="header-actions">
          <el-button @click="syncProviders" :loading="syncingProviders">
            <el-icon><Connection /></el-icon>
            同步 OCR Provider
          </el-button>
          <el-button @click="fetchSettings" :loading="loading">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </header>

      <!-- 当前配置状态卡片 -->
      <div class="status-card" v-if="form.provider && !showWizard">
        <div class="status-icon" :class="form.provider">
          {{ form.provider.charAt(0).toUpperCase() }}
        </div>
        <div class="status-info">
          <div class="status-title">
            <strong>{{ getPresetName(form.provider) }}</strong>
            <el-tag :type="form.enabled ? 'success' : 'info'" size="small">
              {{ form.enabled ? '已启用' : '未启用' }}
            </el-tag>
          </div>
          <div class="status-meta">
            <span>{{ form.model }}</span>
            <span class="separator">·</span>
            <span>{{ form.reasoning_mode === 'disabled' ? '推理关闭' : '推理开启' }}</span>
            <span class="separator">·</span>
            <span>{{ form.language }}</span>
          </div>
        </div>
        <div class="status-actions">
          <el-button type="primary" @click="openWizard">
            <el-icon><Edit /></el-icon>
            修改配置
          </el-button>
          <el-button @click="handleTestLLM" :loading="testingLLM">
            <el-icon><Connection /></el-icon>
            测试连接
          </el-button>
        </div>
      </div>

      <!-- 未配置提示 -->
      <div class="empty-state" v-if="!form.provider && !showWizard">
        <div class="empty-icon">🤖</div>
        <h3>尚未配置模型</h3>
        <p>选择一个服务商开始配置</p>
        <el-button type="primary" @click="openWizard">
          <el-icon><Plus /></el-icon>
          立即配置
        </el-button>
      </div>

      <!-- 配置向导 -->
      <ModelSetupWizard
        v-if="showWizard"
        ref="wizardRef"
        :presets="syncedPresets"
        :saving="saving"
        :testing-llm="testingLLM"
        :initial-config="form"
        @save="handleSaveConfig"
        @test="handleTestLLM"
      />

      <!-- 快速配置入口（仅在显示状态卡片时） -->
      <div v-if="form.provider && !showWizard" class="quick-config-section">
        <div class="section-header">
          <h3>快速配置</h3>
          <el-button text type="primary" @click="showAdvanced = !showAdvanced">
            {{ showAdvanced ? '收起' : '展开' }}
            <el-icon><ArrowDown v-if="!showAdvanced" /><ArrowUp v-else /></el-icon>
          </el-button>
        </div>
        <el-collapse-transition>
          <div v-show="showAdvanced" class="advanced-config">
            <el-card class="config-card">
              <el-tabs v-model="activeTab">
                <el-tab-pane label="执行参数" name="execution">
                  <ExecutionParamsTab />
                </el-tab-pane>
                <el-tab-pane label="审查侧重点" name="review">
                  <ReviewFocusTab />
                </el-tab-pane>
              </el-tabs>
            </el-card>
          </div>
        </el-collapse-transition>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, provide, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Refresh, Edit, Connection, Plus, ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ExecutionParamsTab from '@/components/model-settings/ExecutionParamsTab.vue'
import ReviewFocusTab from '@/components/model-settings/ReviewFocusTab.vue'
import ModelSetupWizard from '@/components/model-settings/ModelSetupWizard.vue'
import { modelSettingsKey, type ModelSettingsContext } from '@/components/model-settings/context'
import { getOCRSettings, listLLMProviders, updateOCRSettings, testLLM } from '@/api/config'
import type { LLMProvider, OCRSettings, ProviderPreset } from '@/types/config'

const allPresets: ProviderPreset[] = [
  {
    id: 'zhipu-coding-plan',
    display_name: '智谱 AI (Coding Plan)',
    category: 'coding_plan',
    type: 'openai_compatible',
    base_url: 'https://open.bigmodel.cn/api/paas/v4',
    protocol: 'openai',
    supports_thinking: true,
    icon: 'zhipu',
    region: 'cn',
    requires_key: true,
    default_model: 'glm-5.1',
    models: [
      { id: 'glm-5.1', display_name: 'GLM-5.1', max_tokens: 4096, supports_thinking: true },
      { id: 'glm-5.2', display_name: 'GLM-5.2', max_tokens: 4096, supports_thinking: true },
      { id: 'glm-4v-plus', display_name: 'GLM-4V-Plus', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'volcengine-coding-plan',
    display_name: '火山引擎 (Coding Plan)',
    category: 'coding_plan',
    type: 'openai_compatible',
    base_url: 'https://ark.cn-beijing.volces.com/api/v3',
    protocol: 'openai',
    supports_thinking: true,
    icon: 'volcengine',
    region: 'cn',
    requires_key: true,
    default_model: 'doubao-seed-2-0-pro',
    models: [
      { id: 'doubao-seed-2-0-pro', display_name: '豆包 Seed 2.0 Pro', max_tokens: 4096, supports_thinking: true },
      { id: 'doubao-seed-2-0-lite', display_name: '豆包 Seed 2.0 Lite', max_tokens: 4096, supports_thinking: false },
      { id: 'doubao-seed-2-0-code', display_name: '豆包 Seed 2.0 Code', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'mimo-coding-plan',
    display_name: '小米 MiMo (Coding Plan)',
    category: 'coding_plan',
    type: 'openai_compatible',
    base_url: 'https://token-plan-cn.xiaomimimo.com/v1',
    protocol: 'openai',
    supports_thinking: true,
    icon: 'mimo',
    region: 'cn',
    requires_key: true,
    default_model: 'mimo-v2.5-pro',
    models: [
      { id: 'mimo-v2.5', display_name: 'MiMo V2.5', max_tokens: 4096, supports_thinking: true },
      { id: 'mimo-v2.5-pro', display_name: 'MiMo V2.5 Pro', max_tokens: 4096, supports_thinking: true },
      { id: 'mimo-v2.5-lite', display_name: 'MiMo V2.5 Lite', max_tokens: 4096, supports_thinking: false },
    ],
  },
  {
    id: 'minimax-coding-plan',
    display_name: 'MiniMax (Coding Plan)',
    category: 'coding_plan',
    type: 'openai_compatible',
    base_url: 'https://api.minimax.chat/v1',
    protocol: 'openai',
    supports_thinking: false,
    icon: 'minimax',
    region: 'cn',
    requires_key: true,
    default_model: 'MiniMax-M2.5',
    models: [
      { id: 'MiniMax-M2.5', display_name: 'MiniMax M2.5', max_tokens: 4096, supports_thinking: false },
      { id: 'MiniMax-M2.7-highspeed', display_name: 'MiniMax M2.7 Highspeed', max_tokens: 4096, supports_thinking: false },
    ],
  },
  {
    id: 'kimi-coding-plan',
    display_name: 'Kimi (Coding Plan)',
    category: 'coding_plan',
    type: 'openai_compatible',
    base_url: 'https://api.moonshot.cn/v1',
    protocol: 'openai',
    supports_thinking: false,
    icon: 'kimi',
    region: 'cn',
    requires_key: true,
    default_model: 'moonshot-v1-32k',
    models: [
      { id: 'moonshot-v1-8k', display_name: 'Moonshot V1 8K', max_tokens: 4096, supports_thinking: false },
      { id: 'moonshot-v1-32k', display_name: 'Moonshot V1 32K', max_tokens: 4096, supports_thinking: false },
      { id: 'moonshot-v1-128k', display_name: 'Moonshot V1 128K', max_tokens: 4096, supports_thinking: false },
    ],
  },
  {
    id: 'openai',
    display_name: 'OpenAI',
    category: 'direct',
    type: 'openai_compatible',
    base_url: 'https://api.openai.com/v1',
    protocol: 'openai',
    supports_thinking: true,
    icon: 'openai',
    region: 'global',
    requires_key: true,
    default_model: 'gpt-5.2',
    models: [
      { id: 'gpt-5.2', display_name: 'GPT-5.2', max_tokens: 4096, supports_thinking: true },
      { id: 'gpt-4o', display_name: 'GPT-4o', max_tokens: 4096, supports_thinking: false },
      { id: 'o3', display_name: 'o3', max_tokens: 4096, supports_thinking: true },
      { id: 'o3-mini', display_name: 'o3 Mini', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'anthropic',
    display_name: 'Anthropic',
    category: 'direct',
    type: 'anthropic',
    base_url: 'https://api.anthropic.com',
    protocol: 'anthropic',
    supports_thinking: true,
    icon: 'anthropic',
    region: 'global',
    requires_key: true,
    default_model: 'claude-sonnet-4-5-20250929',
    models: [
      { id: 'claude-sonnet-4-5-20250929', display_name: 'Claude Sonnet 4.5', max_tokens: 4096, supports_thinking: true },
      { id: 'claude-opus-4-5-20250929', display_name: 'Claude Opus 4.5', max_tokens: 4096, supports_thinking: true },
      { id: 'claude-haiku-4-20250514', display_name: 'Claude Haiku 4', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'deepseek',
    display_name: 'DeepSeek',
    category: 'direct',
    type: 'openai_compatible',
    base_url: 'https://api.deepseek.com',
    protocol: 'openai',
    supports_thinking: true,
    icon: 'deepseek',
    region: 'cn',
    requires_key: true,
    default_model: 'deepseek-v4-pro',
    models: [
      { id: 'deepseek-v4-pro', display_name: 'DeepSeek V4 Pro', max_tokens: 4096, supports_thinking: true },
      { id: 'deepseek-v4-flash', display_name: 'DeepSeek V4 Flash', max_tokens: 4096, supports_thinking: false },
      { id: 'deepseek-reasoner', display_name: 'DeepSeek Reasoner', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'dashscope',
    display_name: '阿里百炼 (Token Plan)',
    category: 'coding_plan',
    type: 'openai_compatible',
    base_url: 'https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1',
    protocol: 'openai',
    supports_thinking: true,
    icon: 'dashscope',
    region: 'cn',
    requires_key: true,
    default_model: 'qwen3.7-plus',
    models: [
      { id: 'qwen3.7-max', display_name: 'Qwen3.7 Max', max_tokens: 8192, supports_thinking: true },
      { id: 'qwen3.7-plus', display_name: 'Qwen3.7 Plus', max_tokens: 8192, supports_thinking: true },
      { id: 'qwen3.6-flash', display_name: 'Qwen3.6 Flash', max_tokens: 8192, supports_thinking: true },
      { id: 'qwen-max', display_name: 'Qwen Max', max_tokens: 8192, supports_thinking: true },
      { id: 'qwen-plus', display_name: 'Qwen Plus', max_tokens: 8192, supports_thinking: true },
      { id: 'qwen-turbo', display_name: 'Qwen Turbo', max_tokens: 8192, supports_thinking: false },
      { id: 'qwen-coder-plus', display_name: 'Qwen Coder Plus', max_tokens: 8192, supports_thinking: true },
      { id: 'qwen-long', display_name: 'Qwen Long', max_tokens: 8192, supports_thinking: false },
    ],
  },
]

const loading = ref(false)
const saving = ref(false)
const testingLLM = ref(false)
const syncingProviders = ref(false)
const showWizard = ref(false)
const showAdvanced = ref(false)
const activeTab = ref('execution')
const wizardRef = ref<InstanceType<typeof ModelSetupWizard> | null>(null)
const syncedPresets = ref<ProviderPreset[]>([...allPresets])

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

function getPresetName(provider: string) {
  const preset = syncedPresets.value.find((p) => p.id === provider || p.id.includes(provider))
  return preset?.display_name || provider
}

function openWizard() {
  showWizard.value = true
}

async function handleSaveConfig() {
  saving.value = true
  try {
    let settings: OCRSettings

    if (showWizard.value && wizardRef.value) {
      const editForm = wizardRef.value.getEditForm()

      if (!editForm.url || !editForm.model) {
        ElMessage.warning('请填写接口地址和模型')
        return
      }

      settings = {
        ...form,
        enabled: true,
        provider: wizardRef.value.selectedPreset?.id || 'custom',
        url: editForm.url,
        model: editForm.model,
        auth_token: editForm.auth_token || form.auth_token,
        use_anthropic: editForm.use_anthropic,
        reasoning_mode: editForm.reasoning_mode,
        reasoning_budget: editForm.reasoning_budget,
        language: editForm.language,
        audience: editForm.audience,
        default_severity: editForm.default_severity,
        concurrency: editForm.concurrency,
        max_git_procs: editForm.max_git_procs,
        timeout_minutes: editForm.timeout_minutes,
        background: editForm.background,
      }
    } else {
      settings = { ...form, enabled: true }
    }

    await updateOCRSettings(settings)
    Object.assign(form, settings)
    showWizard.value = false
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

async function handleTestLLM() {
  // 如果在向导模式中，先保存配置再测试
  if (showWizard.value && wizardRef.value) {
    const editForm = wizardRef.value.getEditForm()
    if (editForm.url && editForm.model) {
      try {
        const settings: OCRSettings = {
          ...form,
          enabled: true,
          provider: wizardRef.value.selectedPreset?.id || 'custom',
          url: editForm.url,
          model: editForm.model,
          auth_token: editForm.auth_token || form.auth_token,
          use_anthropic: editForm.use_anthropic,
          reasoning_mode: editForm.reasoning_mode,
          reasoning_budget: editForm.reasoning_budget,
          language: editForm.language,
          audience: editForm.audience,
          default_severity: editForm.default_severity,
          concurrency: editForm.concurrency,
          max_git_procs: editForm.max_git_procs,
          timeout_minutes: editForm.timeout_minutes,
          background: editForm.background,
        }
        await updateOCRSettings(settings)
        Object.assign(form, settings)
      } catch (e: any) {
        ElMessage.warning('保存配置失败，将测试当前已保存的配置')
      }
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

async function fetchSettings() {
  loading.value = true
  try {
    const settings = await getOCRSettings()
    Object.assign(form, settings)
    if (!form.binary) form.binary = 'ocr'
    if (!form.audience) form.audience = 'human'
    if (!form.default_severity) form.default_severity = 'major'
  } finally {
    loading.value = false
  }
}

async function syncProviders(silent = false) {
  syncingProviders.value = true
  try {
    const providers = await listLLMProviders()
    syncedPresets.value = mergeProviderPresets(allPresets, providers)
    if (!silent) ElMessage.success(`已同步 ${providers.length} 个 OCR Provider`)
  } catch (e: any) {
    if (!silent) ElMessage.error(e.message || '同步 OCR Provider 失败')
  } finally {
    syncingProviders.value = false
  }
}

function mergeProviderPresets(base: ProviderPreset[], providers: LLMProvider[]): ProviderPreset[] {
  const next = base.map((item) => ({ ...item, models: [...item.models] }))
  providers.forEach((provider) => {
    const models = (provider.default_model || []).map((model) => ({
      id: model,
      display_name: model,
      max_tokens: 4096,
      supports_thinking: /reason|thinking|o\d|glm|qwen|claude|deepseek/i.test(model),
    }))
    const existing = next.find((item) => item.id === provider.name || item.id.includes(provider.name))
    if (existing) {
      if (provider.base_url) existing.base_url = provider.base_url
      if (provider.protocol) {
        existing.protocol = provider.protocol
        existing.type = provider.protocol === 'anthropic' ? 'anthropic' : 'openai_compatible'
      }
      if (models.length) {
        existing.models = models
        existing.default_model = models[0].id
      }
      return
    }
    next.push({
      id: provider.name,
      display_name: provider.name,
      category: 'ocr',
      type: provider.protocol === 'anthropic' ? 'anthropic' : 'openai_compatible',
      base_url: provider.base_url || '',
      protocol: provider.protocol || 'openai',
      supports_thinking: models.some((model) => model.supports_thinking),
      models,
      default_model: models[0]?.id || '',
      icon: provider.name,
      region: 'global',
      requires_key: true,
    })
  })
  return next
}

const ctx: ModelSettingsContext = {
  form,
  saving,
  testingLLM,
  extraBodyText: ref(''),
  save: handleSaveConfig,
  testLLM: handleTestLLM,
  syncExtraBody: () => true,
  selectedProvider: ref(null),
  selectedProviderType: ref(null),
  selectProvider: () => {},
}
provide(modelSettingsKey, ctx)

onMounted(() => {
  fetchSettings()
  syncProviders(true)
})
</script>

<style scoped>
.model-settings-page {
  width: min(1200px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
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

.header-actions {
  display: flex;
  gap: 8px;
}

.status-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 20px 24px;
  background: linear-gradient(135deg, #f0f9ff 0%, #e0f2fe 100%);
  border: 1px solid #bae6fd;
  border-radius: 16px;
  margin-bottom: 24px;
}

.status-icon {
  width: 56px;
  height: 56px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  color: white;
  flex-shrink: 0;
}

.status-icon.zhipu,
.status-icon.zhipu-coding-plan {
  background: linear-gradient(135deg, #6366f1, #4f46e5);
}

.status-icon.volcengine,
.status-icon.volcengine-coding-plan {
  background: linear-gradient(135deg, #f59e0b, #d97706);
}

.status-icon.mimo,
.status-icon.mimo-coding-plan {
  background: linear-gradient(135deg, #10b981, #059669);
}

.status-icon.minimax,
.status-icon.minimax-coding-plan {
  background: linear-gradient(135deg, #ec4899, #db2777);
}

.status-icon.kimi,
.status-icon.kimi-coding-plan {
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
}

.status-icon.openai {
  background: linear-gradient(135deg, #10b981, #059669);
}

.status-icon.anthropic {
  background: linear-gradient(135deg, #d97706, #b45309);
}

.status-icon.deepseek {
  background: linear-gradient(135deg, #3b82f6, #2563eb);
}

.status-icon.dashscope {
  background: linear-gradient(135deg, #6366f1, #4f46e5);
}

.status-info {
  flex: 1;
  min-width: 0;
}

.status-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.status-title strong {
  font-size: 16px;
  color: var(--app-text);
}

.status-meta {
  font-size: 13px;
  color: var(--app-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.separator {
  color: #d1d5db;
}

.status-actions {
  display: flex;
  gap: 8px;
}

.empty-state {
  text-align: center;
  padding: 60px 40px;
  background: var(--app-bg);
  border: 2px dashed var(--app-border);
  border-radius: 16px;
  margin-bottom: 24px;
}

.empty-state .empty-icon {
  font-size: 48px;
  margin-bottom: 16px;
}

.empty-state h3 {
  margin: 0 0 8px;
  font-size: 18px;
  color: var(--app-text);
}

.empty-state p {
  margin: 0 0 20px;
  color: var(--app-muted);
  font-size: 14px;
}

.quick-config-section {
  margin-top: 32px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.section-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--app-text);
}

.advanced-config {
  margin-top: 8px;
}

.config-card {
  border-radius: 14px;
}

@media (max-width: 768px) {
  .model-settings-page {
    padding: 20px 16px;
  }

  .status-card {
    flex-direction: column;
    text-align: center;
  }

  .status-meta {
    justify-content: center;
    flex-wrap: wrap;
  }

  .status-actions {
    justify-content: center;
  }

  .page-header {
    flex-direction: column;
    gap: 16px;
  }

  .header-actions {
    width: 100%;
  }

  .header-actions .el-button {
    flex: 1;
  }
}
</style>
