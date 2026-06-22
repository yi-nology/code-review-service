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
          <el-button @click="fetchSettings" :loading="loading">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </header>

      <!-- 当前配置状态卡片 -->
      <div class="status-card" v-if="form.provider">
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
          <el-button type="primary" @click="showConfigDialog = true">
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
      <div class="empty-state" v-else>
        <div class="empty-icon">🤖</div>
        <h3>尚未配置模型</h3>
        <p>选择一个服务商开始配置</p>
        <el-button type="primary" @click="showConfigDialog = true">
          <el-icon><Plus /></el-icon>
          立即配置
        </el-button>
      </div>

      <!-- 配置对话框 -->
      <el-dialog
        v-model="showConfigDialog"
        title="模型配置"
        width="900px"
        :close-on-click-modal="false"
        destroy-on-close
      >
        <div class="config-dialog-content">
          <!-- 左侧：预设选择 -->
          <div class="preset-panel">
            <div class="panel-header">
              <strong>选择服务商</strong>
              <el-input
                v-model="searchKeyword"
                placeholder="搜索..."
                size="small"
                clearable
                prefix-icon="Search"
              />
            </div>
            <div class="preset-list">
              <div
                v-for="preset in filteredPresets"
                :key="preset.id"
                class="preset-item"
                :class="{ active: selectedPreset?.id === preset.id }"
                @click="selectPreset(preset)"
              >
                <div class="preset-icon" :class="preset.icon">
                  {{ preset.icon.charAt(0).toUpperCase() }}
                </div>
                <div class="preset-info">
                  <strong>{{ preset.display_name }}</strong>
                  <span class="preset-meta">
                    {{ preset.models.length }} 模型
                    <span v-if="preset.supports_thinking" class="thinking-badge">Plan</span>
                  </span>
                </div>
              </div>

              <!-- 自定义配置 -->
              <div
                class="preset-item custom"
                :class="{ active: isCustomMode }"
                @click="selectCustom"
              >
                <div class="preset-icon custom">+</div>
                <div class="preset-info">
                  <strong>自定义配置</strong>
                  <span class="preset-meta">手动输入</span>
                </div>
              </div>
            </div>
          </div>

          <!-- 右侧：配置表单 -->
          <div class="config-panel">
            <template v-if="selectedPreset || isCustomMode">
              <el-form :model="editForm" label-width="100px" label-position="top">
                <!-- 基础配置 -->
                <div class="config-section">
                  <h4>基础配置</h4>

                  <el-form-item label="接口地址" required>
                    <el-input v-model="editForm.url" placeholder="https://api.example.com/v1" />
                  </el-form-item>

                  <el-form-item label="模型" required>
                    <el-select v-model="editForm.model" filterable allow-create style="width: 100%">
                      <el-option
                        v-for="m in currentModels"
                        :key="m.id"
                        :label="m.display_name"
                        :value="m.id"
                      />
                    </el-select>
                  </el-form-item>

                  <el-form-item label="API Key" required>
                    <el-input
                      v-model="editForm.auth_token"
                      type="password"
                      placeholder="输入 API Key"
                      show-password
                    />
                  </el-form-item>

                  <el-form-item label="请求协议">
                    <el-radio-group v-model="editForm.use_anthropic">
                      <el-radio :value="false">OpenAI 兼容</el-radio>
                      <el-radio :value="true">Anthropic</el-radio>
                    </el-radio-group>
                  </el-form-item>
                </div>

                <!-- 高级配置 -->
                <div class="config-section">
                  <h4>高级配置</h4>

                  <el-form-item label="推理模式">
                    <el-select v-model="editForm.reasoning_mode" style="width: 100%">
                      <el-option label="禁用" value="disabled" />
                      <el-option label="轻量 (2K tokens)" value="light" />
                      <el-option label="中等 (8K tokens)" value="medium" />
                      <el-option label="充分 (32K tokens)" value="full" />
                      <el-option label="自定义" value="custom" />
                    </el-select>
                  </el-form-item>

                  <el-form-item v-if="editForm.reasoning_mode === 'custom'" label="推理预算">
                    <el-input-number
                      v-model="editForm.reasoning_budget"
                      :min="1000"
                      :max="100000"
                      :step="1000"
                      style="width: 100%"
                    />
                  </el-form-item>

                  <el-form-item label="输出语言">
                    <el-select v-model="editForm.language" style="width: 100%">
                      <el-option label="中文" value="Chinese" />
                      <el-option label="English" value="English" />
                    </el-select>
                  </el-form-item>
                </div>
              </el-form>
            </template>

            <template v-else>
              <div class="empty-config">
                <div class="empty-icon">👈</div>
                <p>请从左侧选择一个服务商</p>
              </div>
            </template>
          </div>
        </div>

        <template #footer>
          <div class="dialog-footer">
            <el-button @click="showConfigDialog = false">取消</el-button>
            <el-button @click="handleTestLLM" :loading="testingLLM">
              <el-icon><Connection /></el-icon>
              测试连接
            </el-button>
            <el-button type="primary" @click="handleSaveConfig" :loading="saving">
              <el-icon><Check /></el-icon>
              保存配置
            </el-button>
          </div>
        </template>
      </el-dialog>

      <!-- 其他配置 Tab -->
      <el-card class="other-config-card">
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
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, provide, onMounted, computed } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Refresh, Edit, Connection, Plus, Check, Search } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ExecutionParamsTab from '@/components/model-settings/ExecutionParamsTab.vue'
import ReviewFocusTab from '@/components/model-settings/ReviewFocusTab.vue'
import { modelSettingsKey, type ModelSettingsContext } from '@/components/model-settings/context'
import { getOCRSettings, updateOCRSettings, testLLM, listLLMProviders } from '@/api/config'
import type { OCRSettings, LLMProvider } from '@/types/config'

// 预设类型定义
interface ProviderPreset {
  id: string
  display_name: string
  category: string
  type: string
  base_url: string
  protocol: string
  supports_thinking: boolean
  models: ModelOption[]
  default_model: string
  icon: string
  region: string
  requires_key: boolean
}

interface ModelOption {
  id: string
  display_name: string
  max_tokens: number
  supports_thinking: boolean
}

// 硬编码预设列表
const allPresets: ProviderPreset[] = [
  // Coding Plan
  {
    id: 'zhipu-coding-plan', display_name: '智谱 AI (Coding Plan)', category: 'coding_plan',
    type: 'openai_compatible', base_url: 'https://open.bigmodel.cn/api/paas/v4', protocol: 'openai',
    supports_thinking: true, icon: 'zhipu', region: 'cn', requires_key: true, default_model: 'glm-5.1',
    models: [
      { id: 'glm-5.1', display_name: 'GLM-5.1', max_tokens: 4096, supports_thinking: true },
      { id: 'glm-5.2', display_name: 'GLM-5.2', max_tokens: 4096, supports_thinking: true },
      { id: 'glm-4v-plus', display_name: 'GLM-4V-Plus', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'volcengine-coding-plan', display_name: '火山引擎 (Coding Plan)', category: 'coding_plan',
    type: 'openai_compatible', base_url: 'https://ark.cn-beijing.volces.com/api/v3', protocol: 'openai',
    supports_thinking: true, icon: 'volcengine', region: 'cn', requires_key: true, default_model: 'doubao-seed-2-0-pro',
    models: [
      { id: 'doubao-seed-2-0-pro', display_name: '豆包 Seed 2.0 Pro', max_tokens: 4096, supports_thinking: true },
      { id: 'doubao-seed-2-0-lite', display_name: '豆包 Seed 2.0 Lite', max_tokens: 4096, supports_thinking: false },
      { id: 'doubao-seed-2-0-code', display_name: '豆包 Seed 2.0 Code', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'mimo-coding-plan', display_name: '小米 MiMo (Coding Plan)', category: 'coding_plan',
    type: 'openai_compatible', base_url: 'https://token-plan-cn.xiaomimimo.com/v1', protocol: 'openai',
    supports_thinking: true, icon: 'mimo', region: 'cn', requires_key: true, default_model: 'mimo-v2.5-pro',
    models: [
      { id: 'mimo-v2.5', display_name: 'MiMo V2.5', max_tokens: 4096, supports_thinking: true },
      { id: 'mimo-v2.5-pro', display_name: 'MiMo V2.5 Pro', max_tokens: 4096, supports_thinking: true },
      { id: 'mimo-v2.5-lite', display_name: 'MiMo V2.5 Lite', max_tokens: 4096, supports_thinking: false },
    ],
  },
  {
    id: 'minimax-coding-plan', display_name: 'MiniMax (Coding Plan)', category: 'coding_plan',
    type: 'openai_compatible', base_url: 'https://api.minimax.chat/v1', protocol: 'openai',
    supports_thinking: false, icon: 'minimax', region: 'cn', requires_key: true, default_model: 'MiniMax-M2.5',
    models: [
      { id: 'MiniMax-M2.5', display_name: 'MiniMax M2.5', max_tokens: 4096, supports_thinking: false },
      { id: 'MiniMax-M2.7-highspeed', display_name: 'MiniMax M2.7 Highspeed', max_tokens: 4096, supports_thinking: false },
    ],
  },
  {
    id: 'kimi-coding-plan', display_name: 'Kimi (Coding Plan)', category: 'coding_plan',
    type: 'openai_compatible', base_url: 'https://api.moonshot.cn/v1', protocol: 'openai',
    supports_thinking: false, icon: 'kimi', region: 'cn', requires_key: true, default_model: 'moonshot-v1-32k',
    models: [
      { id: 'moonshot-v1-8k', display_name: 'Moonshot V1 8K', max_tokens: 4096, supports_thinking: false },
      { id: 'moonshot-v1-32k', display_name: 'Moonshot V1 32K', max_tokens: 4096, supports_thinking: false },
      { id: 'moonshot-v1-128k', display_name: 'Moonshot V1 128K', max_tokens: 4096, supports_thinking: false },
    ],
  },
  // Direct API
  {
    id: 'openai', display_name: 'OpenAI', category: 'direct',
    type: 'openai_compatible', base_url: 'https://api.openai.com/v1', protocol: 'openai',
    supports_thinking: true, icon: 'openai', region: 'global', requires_key: true, default_model: 'gpt-5.2',
    models: [
      { id: 'gpt-5.2', display_name: 'GPT-5.2', max_tokens: 4096, supports_thinking: true },
      { id: 'gpt-4o', display_name: 'GPT-4o', max_tokens: 4096, supports_thinking: false },
      { id: 'o3', display_name: 'o3', max_tokens: 4096, supports_thinking: true },
      { id: 'o3-mini', display_name: 'o3 Mini', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'anthropic', display_name: 'Anthropic', category: 'direct',
    type: 'anthropic', base_url: 'https://api.anthropic.com', protocol: 'anthropic',
    supports_thinking: true, icon: 'anthropic', region: 'global', requires_key: true, default_model: 'claude-sonnet-4-5-20250929',
    models: [
      { id: 'claude-sonnet-4-5-20250929', display_name: 'Claude Sonnet 4.5', max_tokens: 4096, supports_thinking: true },
      { id: 'claude-opus-4-5-20250929', display_name: 'Claude Opus 4.5', max_tokens: 4096, supports_thinking: true },
      { id: 'claude-haiku-4-20250514', display_name: 'Claude Haiku 4', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'deepseek', display_name: 'DeepSeek', category: 'direct',
    type: 'openai_compatible', base_url: 'https://api.deepseek.com', protocol: 'openai',
    supports_thinking: true, icon: 'deepseek', region: 'cn', requires_key: true, default_model: 'deepseek-v4-pro',
    models: [
      { id: 'deepseek-v4-pro', display_name: 'DeepSeek V4 Pro', max_tokens: 4096, supports_thinking: true },
      { id: 'deepseek-v4-flash', display_name: 'DeepSeek V4 Flash', max_tokens: 4096, supports_thinking: false },
      { id: 'deepseek-reasoner', display_name: 'DeepSeek Reasoner', max_tokens: 4096, supports_thinking: true },
    ],
  },
  {
    id: 'dashscope', display_name: '阿里百炼', category: 'direct',
    type: 'openai_compatible', base_url: 'https://dashscope.aliyuncs.com/compatible-mode/v1', protocol: 'openai',
    supports_thinking: true, icon: 'dashscope', region: 'cn', requires_key: true, default_model: 'qwen-plus',
    models: [
      { id: 'qwen-max', display_name: 'Qwen Max', max_tokens: 4096, supports_thinking: true },
      { id: 'qwen-plus', display_name: 'Qwen Plus', max_tokens: 4096, supports_thinking: true },
      { id: 'qwen-turbo', display_name: 'Qwen Turbo', max_tokens: 4096, supports_thinking: false },
      { id: 'qwen-coder-plus', display_name: 'Qwen Coder Plus', max_tokens: 4096, supports_thinking: true },
    ],
  },
]

// 状态
const loading = ref(false)
const saving = ref(false)
const testingLLM = ref(false)
const activeTab = ref('execution')
const showConfigDialog = ref(false)
const searchKeyword = ref('')
const selectedPreset = ref<ProviderPreset | null>(null)
const isCustomMode = ref(false)

// 表单数据
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
  tools_path: '',
  telemetry_enabled: false,
  telemetry_exporter: '',
  telemetry_otlp_endpoint: '',
  telemetry_content_logging: false,
  reasoning_mode: 'light',
  reasoning_budget: 0,
  context_window: 0,
  max_output_tokens: 0,
})

// 编辑表单
const editForm = reactive({
  url: '',
  model: '',
  auth_token: '',
  use_anthropic: false,
  reasoning_mode: 'light' as 'disabled' | 'light' | 'medium' | 'full' | 'custom',
  reasoning_budget: 0,
  language: 'Chinese',
})

// 过滤后的预设
const filteredPresets = computed(() => {
  if (!searchKeyword.value) return allPresets
  const kw = searchKeyword.value.toLowerCase()
  return allPresets.filter(p =>
    p.display_name.toLowerCase().includes(kw) ||
    p.models.some(m => m.display_name.toLowerCase().includes(kw))
  )
})

// 当前模型列表
const currentModels = computed(() => {
  if (selectedPreset.value) {
    return selectedPreset.value.models
  }
  return []
})

// 获取预设名称
function getPresetName(provider: string) {
  const preset = allPresets.find(p => p.id === provider || p.id.includes(provider))
  return preset?.display_name || provider
}

// 选择预设
function selectPreset(preset: ProviderPreset) {
  selectedPreset.value = preset
  isCustomMode.value = false
  editForm.url = preset.base_url
  editForm.model = preset.default_model
  editForm.use_anthropic = preset.protocol === 'anthropic'
}

// 选择自定义
function selectCustom() {
  selectedPreset.value = null
  isCustomMode.value = true
  editForm.url = ''
  editForm.model = ''
  editForm.use_anthropic = false
}

// 保存配置
async function handleSaveConfig() {
  if (!editForm.url || !editForm.model) {
    ElMessage.warning('请填写接口地址和模型')
    return
  }

  saving.value = true
  try {
    const settings: OCRSettings = {
      ...form,
      enabled: true,
      provider: selectedPreset.value?.id || 'custom',
      url: editForm.url,
      model: editForm.model,
      auth_token: editForm.auth_token || form.auth_token,
      use_anthropic: editForm.use_anthropic,
      reasoning_mode: editForm.reasoning_mode,
      reasoning_budget: editForm.reasoning_budget,
      language: editForm.language,
    }

    await updateOCRSettings(settings)
    Object.assign(form, settings)
    showConfigDialog.value = false
    ElMessage.success('配置已保存')
  } catch (e: any) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    saving.value = false
  }
}

// 测试 LLM
async function handleTestLLM() {
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

// 获取设置
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

// 提供上下文
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

onMounted(fetchSettings)
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

/* 状态卡片 */
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

.status-icon.zhipu { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.status-icon.volcengine { background: linear-gradient(135deg, #f59e0b, #d97706); }
.status-icon.mimo { background: linear-gradient(135deg, #10b981, #059669); }
.status-icon.minimax { background: linear-gradient(135deg, #ec4899, #db2777); }
.status-icon.kimi { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.status-icon.openai { background: linear-gradient(135deg, #10b981, #059669); }
.status-icon.anthropic { background: linear-gradient(135deg, #d97706, #b45309); }
.status-icon.deepseek { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.status-icon.dashscope { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.status-icon.zhipu-coding-plan { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.status-icon.volcengine-coding-plan { background: linear-gradient(135deg, #f59e0b, #d97706); }
.status-icon.mimo-coding-plan { background: linear-gradient(135deg, #10b981, #059669); }
.status-icon.minimax-coding-plan { background: linear-gradient(135deg, #ec4899, #db2777); }
.status-icon.kimi-coding-plan { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }

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

/* 空状态 */
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

/* 配置对话框 */
.config-dialog-content {
  display: flex;
  gap: 24px;
  min-height: 500px;
}

.preset-panel {
  width: 280px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
}

.panel-header {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.panel-header strong {
  font-size: 14px;
  color: var(--app-text);
}

.preset-list {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.preset-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.preset-item:hover {
  border-color: var(--app-primary);
  background: var(--app-bg-hover);
}

.preset-item.active {
  border-color: var(--app-primary);
  background: var(--app-primary-bg);
}

.preset-item.custom {
  border-style: dashed;
}

.preset-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 14px;
  color: white;
  flex-shrink: 0;
}

.preset-icon.custom {
  background: var(--app-border);
  color: var(--app-text);
}

.preset-info {
  flex: 1;
  min-width: 0;
}

.preset-info strong {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.preset-meta {
  font-size: 11px;
  color: var(--app-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.thinking-badge {
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
  color: white;
  font-size: 9px;
  padding: 2px 5px;
  border-radius: 3px;
  font-weight: 600;
}

.config-panel {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
}

.config-section {
  margin-bottom: 24px;
}

.config-section h4 {
  margin: 0 0 16px;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
  padding-bottom: 8px;
  border-bottom: 1px solid var(--app-border);
}

.empty-config {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--app-muted);
}

.empty-config .empty-icon {
  font-size: 48px;
  margin-bottom: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

/* 其他配置卡片 */
.other-config-card {
  margin-top: 24px;
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

  .config-dialog-content {
    flex-direction: column;
  }

  .preset-panel {
    width: 100%;
  }
}
</style>
