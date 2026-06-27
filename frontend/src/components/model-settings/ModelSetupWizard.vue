<template>
  <div class="model-setup-wizard">
    <StepIndicator :steps="steps" :current-step="currentStep" />

    <!-- 步骤 1: 选择服务商 -->
    <div v-show="currentStep === 0" class="step-content">
      <div class="step-header">
        <h2>选择服务商</h2>
        <p>选择一个 AI 服务商开始配置，或使用自定义配置</p>
      </div>

      <el-input
        v-model="searchKeyword"
        placeholder="搜索服务商或模型..."
        clearable
        class="search-input"
      >
        <template #prefix>
          <el-icon><Search /></el-icon>
        </template>
      </el-input>

      <div class="provider-grid">
        <ProviderCard
          v-for="preset in filteredPresets"
          :key="preset.id"
          :provider="preset"
          :selected="selectedPreset?.id === preset.id && !isCustomMode"
          @click="selectPreset(preset)"
        />
        <ProviderCard
          :provider="customPreset"
          :selected="isCustomMode"
          :is-custom="true"
          @click="selectCustom"
        />
      </div>
    </div>

    <!-- 步骤 2: 基础配置 -->
    <div v-show="currentStep === 1" class="step-content">
      <div class="step-header">
        <h2>基础配置</h2>
        <p>配置连接信息和模型选择</p>
      </div>

      <div class="config-cards">
        <el-card class="config-card">
          <template #header>
            <div class="card-header">
              <el-icon><Connection /></el-icon>
              <span>连接信息</span>
            </div>
          </template>
          <el-form :model="editForm" label-position="top">
            <el-form-item label="接口地址" required>
              <el-input
                v-model="editForm.url"
                placeholder="https://api.example.com/v1"
              />
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
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>
            <div class="card-header">
              <el-icon><Cpu /></el-icon>
              <span>模型选择</span>
            </div>
          </template>
          <el-form :model="editForm" label-position="top">
            <el-form-item label="模型" required>
              <el-select
                v-model="editForm.model"
                filterable
                allow-create
                placeholder="选择或输入模型名称"
                style="width: 100%"
              >
                <el-option
                  v-for="m in currentModels"
                  :key="m.id"
                  :label="m.display_name"
                  :value="m.id"
                >
                  <span>{{ m.display_name }}</span>
                  <el-tag v-if="m.supports_thinking" type="success" size="small" style="margin-left: 8px">
                    推理
                  </el-tag>
                </el-option>
              </el-select>
            </el-form-item>
          </el-form>
        </el-card>
      </div>
    </div>

    <!-- 步骤 3: 高级配置 -->
    <div v-show="currentStep === 2" class="step-content">
      <div class="step-header">
        <h2>高级配置</h2>
        <p>配置推理模式、执行参数和审查重点</p>
      </div>

      <div class="config-cards">
        <el-card class="config-card">
          <template #header>
            <div class="card-header">
              <el-icon><Lightning /></el-icon>
              <span>推理配置</span>
            </div>
          </template>
          <el-form :model="editForm" label-position="top">
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
          </el-form>
        </el-card>

        <el-card class="config-card">
          <template #header>
            <div class="card-header">
              <el-icon><Setting /></el-icon>
              <span>执行参数</span>
            </div>
          </template>
          <el-form :model="editForm" label-position="top">
            <el-form-item label="输出受众">
              <el-segmented
                v-model="editForm.audience"
                :options="[
                  { label: '人工可读', value: 'human' },
                  { label: 'Agent 精简', value: 'agent' },
                ]"
              />
            </el-form-item>
            <el-form-item label="默认严重级别">
              <el-select v-model="editForm.default_severity" style="width: 100%">
                <el-option label="严重" value="critical" />
                <el-option label="高危" value="major" />
                <el-option label="中危" value="minor" />
                <el-option label="低危" value="info" />
              </el-select>
            </el-form-item>
            <div class="form-row">
              <el-form-item label="模型并发" class="form-col">
                <el-input-number v-model="editForm.concurrency" :min="1" :max="32" style="width: 100%" />
              </el-form-item>
              <el-form-item label="Git 并发" class="form-col">
                <el-input-number v-model="editForm.max_git_procs" :min="1" :max="64" style="width: 100%" />
              </el-form-item>
            </div>
            <el-form-item label="单文件超时（分钟）">
              <el-input-number v-model="editForm.timeout_minutes" :min="1" :max="120" style="width: 100%" />
            </el-form-item>
          </el-form>
        </el-card>

        <el-card class="config-card full-width">
          <template #header>
            <div class="card-header">
              <el-icon><Document /></el-icon>
              <span>审查重点</span>
            </div>
          </template>
          <el-form :model="editForm" label-position="top">
            <el-form-item label="审查背景">
              <el-input
                v-model="editForm.background"
                type="textarea"
                :rows="4"
                placeholder="默认的审查侧重点，例如：这是一个电商项目，重点关注支付流程的安全性和性能"
              />
              <div class="form-tip">当未配置代理团队时，使用此默认背景进行审查</div>
            </el-form-item>
          </el-form>
        </el-card>
      </div>
    </div>

    <!-- 步骤 4: 确认与测试 -->
    <div v-show="currentStep === 3" class="step-content">
      <div class="step-header">
        <h2>确认配置</h2>
        <p>检查配置信息，测试连接后保存</p>
      </div>

      <ConfigSummary
        :config="summaryConfig"
        :provider-name="selectedPreset?.display_name"
      />

      <el-card class="test-card">
        <template #header>
          <div class="card-header">
            <el-icon><Connection /></el-icon>
            <span>连接测试</span>
          </div>
        </template>
        <div class="test-content">
          <p>在保存配置前，建议先测试连接是否正常</p>
          <el-button
            type="primary"
            :loading="testingLlm"
            @click="$emit('test')"
          >
            <el-icon><Connection /></el-icon>
            测试连接
          </el-button>
        </div>
      </el-card>
    </div>

    <!-- 底部操作栏 -->
    <div class="wizard-footer">
      <el-button v-if="currentStep > 0" @click="prevStep">
        <el-icon><ArrowLeft /></el-icon>
        上一步
      </el-button>
      <div class="footer-spacer" />
      <el-button v-if="currentStep < 3" type="primary" @click="nextStep" :disabled="!canProceed">
        下一步
        <el-icon><ArrowRight /></el-icon>
      </el-button>
      <el-button
        v-if="currentStep === 3"
        type="primary"
        :loading="saving"
        @click="$emit('save')"
      >
        <el-icon><Check /></el-icon>
        保存配置
      </el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch } from 'vue'
import {
  Search,
  Connection,
  Cpu,
  Lightning,
  Setting,
  Document,
  ArrowLeft,
  ArrowRight,
  Check,
} from '@element-plus/icons-vue'
import StepIndicator from './wizard/StepIndicator.vue'
import ProviderCard from './wizard/ProviderCard.vue'
import ConfigSummary from './wizard/ConfigSummary.vue'
import type { OCRSettings, ProviderPreset } from '@/types/config'

const props = defineProps<{
  presets: ProviderPreset[]
  saving: boolean
  testingLlm: boolean
  initialConfig?: OCRSettings
}>()

const emit = defineEmits<{
  save: []
  test: []
}>()

const steps = [
  { label: '选择服务商' },
  { label: '基础配置' },
  { label: '高级配置' },
  { label: '确认保存' },
]

const currentStep = ref(0)
const searchKeyword = ref('')
const selectedPreset = ref<ProviderPreset | null>(null)
const isCustomMode = ref(false)

const customPreset: ProviderPreset = {
  id: 'custom',
  display_name: '自定义配置',
  category: 'custom',
  type: 'openai_compatible',
  base_url: '',
  protocol: 'openai',
  supports_thinking: false,
  models: [],
  default_model: '',
  icon: 'custom',
  region: '',
  requires_key: true,
}

const editForm = reactive({
  url: '',
  model: '',
  auth_token: '',
  use_anthropic: false,
  reasoning_mode: 'light' as 'disabled' | 'light' | 'medium' | 'full' | 'custom',
  reasoning_budget: 0,
  language: 'Chinese',
  audience: 'human' as 'human' | 'agent',
  default_severity: 'major' as 'critical' | 'major' | 'minor' | 'info',
  concurrency: 2,
  max_git_procs: 4,
  timeout_minutes: 10,
  background: '',
})

const filteredPresets = computed(() => {
  if (!searchKeyword.value) return props.presets
  const kw = searchKeyword.value.toLowerCase()
  return props.presets.filter(
    (p) =>
      p.display_name.toLowerCase().includes(kw) ||
      p.models.some((m) => m.display_name.toLowerCase().includes(kw))
  )
})

const currentModels = computed(() => {
  if (selectedPreset.value) {
    return selectedPreset.value.models
  }
  return []
})

const canProceed = computed(() => {
  if (currentStep.value === 0) {
    return selectedPreset.value !== null || isCustomMode.value
  }
  if (currentStep.value === 1) {
    return editForm.url && editForm.model
  }
  return true
})

const summaryConfig = computed<OCRSettings>(() => ({
  enabled: true,
  url: editForm.url,
  model: editForm.model,
  auth_token: editForm.auth_token,
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
  binary: 'ocr',
  max_tools: 0,
}))

function selectPreset(preset: ProviderPreset) {
  selectedPreset.value = preset
  isCustomMode.value = false
  editForm.url = preset.base_url
  editForm.model = preset.default_model
  editForm.use_anthropic = preset.protocol === 'anthropic'
}

function selectCustom() {
  selectedPreset.value = null
  isCustomMode.value = true
  editForm.url = ''
  editForm.model = ''
  editForm.use_anthropic = false
}

function nextStep() {
  if (currentStep.value < 3 && canProceed.value) {
    currentStep.value++
  }
}

function prevStep() {
  if (currentStep.value > 0) {
    currentStep.value--
  }
}

function getEditForm() {
  return { ...editForm }
}

function loadConfig(config: OCRSettings) {
  editForm.url = config.url || ''
  editForm.model = config.model || ''
  editForm.auth_token = ''
  editForm.use_anthropic = config.use_anthropic || false
  editForm.reasoning_mode = (config.reasoning_mode as any) || 'light'
  editForm.reasoning_budget = config.reasoning_budget || 0
  editForm.language = config.language || 'Chinese'
  editForm.audience = (config.audience as any) || 'human'
  editForm.default_severity = (config.default_severity as any) || 'major'
  editForm.concurrency = config.concurrency || 2
  editForm.max_git_procs = config.max_git_procs || 4
  editForm.timeout_minutes = config.timeout_minutes || 10
  editForm.background = config.background || ''

  const preset = props.presets.find((p) => p.id === config.provider)
  if (preset) {
    selectedPreset.value = preset
    isCustomMode.value = false
  } else {
    selectedPreset.value = null
    isCustomMode.value = true
  }
}

watch(
  () => props.initialConfig,
  (config) => {
    if (config) {
      loadConfig(config)
    }
  },
  { immediate: true }
)

defineExpose({
  getEditForm,
  loadConfig,
  selectedPreset,
  isCustomMode,
})
</script>

<style scoped>
.model-setup-wizard {
  max-width: 960px;
  margin: 0 auto;
}

.step-content {
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from {
    opacity: 0;
    transform: translateY(10px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.step-header {
  text-align: center;
  margin-bottom: 32px;
}

.step-header h2 {
  margin: 0 0 8px;
  font-size: 24px;
  font-weight: 700;
  color: var(--app-text);
}

.step-header p {
  margin: 0;
  font-size: 14px;
  color: var(--app-muted);
}

.search-input {
  max-width: 400px;
  margin: 0 auto 24px;
}

.provider-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.config-cards {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 20px;
}

.config-card {
  border-radius: 14px;
}

.config-card.full-width {
  grid-column: 1 / -1;
}

.card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text);
}

.card-header .el-icon {
  color: var(--app-primary);
}

.form-row {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16px;
}

.form-tip {
  font-size: 12px;
  color: var(--app-muted);
  margin-top: 4px;
}

.test-card {
  margin-top: 24px;
  border-radius: 14px;
}

.test-content {
  text-align: center;
  padding: 20px 0;
}

.test-content p {
  margin: 0 0 16px;
  color: var(--app-muted);
}

.wizard-footer {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 40px;
  padding-top: 24px;
  border-top: 1px solid var(--app-border);
}

.footer-spacer {
  flex: 1;
}

@media (max-width: 768px) {
  .provider-grid {
    grid-template-columns: 1fr;
  }

  .config-cards {
    grid-template-columns: 1fr;
  }

  .form-row {
    grid-template-columns: 1fr;
  }

  .step-header h2 {
    font-size: 20px;
  }
}
</style>
