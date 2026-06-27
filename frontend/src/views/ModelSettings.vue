<template>
  <AppLayout>
    <div class="ms-page">
      <header class="ms-header">
        <div>
          <div class="eyebrow">MODEL SETTINGS</div>
          <h1>模型配置</h1>
          <p>配置代码审查使用的 AI 模型与执行参数</p>
        </div>
        <div class="ms-actions">
          <el-button @click="syncProviders" :loading="syncingProviders">
            <el-icon><Connection /></el-icon>
            <span>同步 Provider</span>
          </el-button>
          <el-button @click="fetchSettings" :loading="loading">
            <el-icon><Refresh /></el-icon>
            <span>刷新</span>
          </el-button>
        </div>
      </header>

      <!-- 状态条 -->
      <section class="status-strip" :class="{ off: !form.enabled }">
        <div class="ss-icon" :class="currentIcon">{{ currentInitial }}</div>
        <div class="ss-main">
          <div class="ss-title">
            <strong>{{ currentDisplayName }}</strong>
            <el-tag :type="form.enabled ? 'success' : 'info'" size="small" effect="light">
              {{ form.enabled ? '已启用' : '未启用' }}
            </el-tag>
            <el-tag size="small" effect="plain" :type="form.use_anthropic ? 'warning' : 'info'">
              {{ form.use_anthropic ? 'Anthropic' : 'OpenAI 兼容' }}
            </el-tag>
            <el-tag v-if="currentRegion" size="small" effect="plain">{{ currentRegion === 'cn' ? '国内' : '海外' }}</el-tag>
          </div>
          <div class="ss-meta">
            <span class="mono">{{ form.model || '未设置模型' }}</span>
            <span class="sep">·</span>
            <span>{{ reasoningLabel }}</span>
            <span class="sep">·</span>
            <span>{{ form.language === 'Chinese' ? '中文' : 'English' }}</span>
            <span class="sep">·</span>
            <span class="mono url">{{ form.url || '—' }}</span>
          </div>
        </div>
        <div class="ss-controls">
          <label class="enable-toggle">
            <span>启用</span>
            <el-switch v-model="form.enabled" />
          </label>
          <el-button @click="handleTestLLM" :loading="testingLLM">
            <el-icon><Connection /></el-icon>
            <span>测试连接</span>
          </el-button>
        </div>
      </section>

      <section v-if="!form.provider" class="empty-banner">
        <el-icon><InfoFilled /></el-icon>
        <span>尚未选择服务商，请在下方选择或使用自定义配置。</span>
      </section>

      <!-- 主配置卡片 -->
      <section class="config-card">
        <!-- 服务商 -->
        <div class="block">
          <div class="block-head">
            <h3>服务商</h3>
            <el-input
              v-model="providerQuery"
              placeholder="搜索服务商…"
              clearable
              size="small"
              class="provider-search"
            >
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>
          </div>
          <div class="provider-chips">
            <button
              v-for="p in filteredProviders"
              :key="p.name"
              type="button"
              class="pchip"
              :class="{ active: isProviderSelected(p) }"
              @click="selectProvider(p)"
            >
              <span class="pchip-icon" :class="iconOf(p.name)">{{ initialOf(p.name) }}</span>
              <span class="pchip-body">
                <strong>{{ displayOf(p.name) }}</strong>
                <small>
                  {{ p.models.length ? `${p.models.length} 个推荐模型` : (p.base_url ? '手动填写模型' : '自定义') }}
                </small>
              </span>
              <el-icon v-if="isProviderSelected(p)" class="pchip-check"><Check /></el-icon>
            </button>
            <button type="button" class="pchip custom" :class="{ active: isCustomSelected }" @click="selectCustom">
              <span class="pchip-icon custom">+</span>
              <span class="pchip-body">
                <strong>自定义</strong>
                <small>手动填写全部信息</small>
              </span>
            </button>
          </div>
        </div>

        <el-divider />

        <!-- 连接 & 模型 -->
        <div class="grid-2">
          <div class="block">
            <h3 class="block-title">连接</h3>
            <el-form label-position="top" :model="form">
              <el-form-item label="接口地址" required>
                <el-input v-model="form.url" placeholder="https://api.example.com/v1" />
              </el-form-item>
              <el-form-item label="API Key">
                <el-input
                  v-model="authToken"
                  type="password"
                  show-password
                  placeholder="••••••••（留空保持不变）"
                />
              </el-form-item>
              <el-form-item label="请求协议">
                <el-radio-group v-model="form.use_anthropic">
                  <el-radio :value="false">OpenAI 兼容</el-radio>
                  <el-radio :value="true">Anthropic</el-radio>
                </el-radio-group>
              </el-form-item>
            </el-form>
          </div>

          <div class="block">
            <h3 class="block-title">模型</h3>
            <el-form label-position="top" :model="form">
              <el-form-item label="模型名称" required>
                <el-select
                  v-model="form.model"
                  filterable
                  allow-create
                  default-first-option
                  :placeholder="currentModels.length ? '选择或输入模型名称' : '输入模型名称（如 gpt-4o）'"
                  style="width: 100%"
                >
                  <el-option v-for="m in currentModels" :key="m" :label="m" :value="m" />
                </el-select>
                <div class="tip">模型版本更新频繁，请以服务商文档为准；可直接输入任意模型名。</div>
              </el-form-item>
              <div class="grid-2 inner">
                <el-form-item label="推理模式">
                  <el-select v-model="form.reasoning_mode" style="width: 100%">
                    <el-option label="禁用" value="disabled" />
                    <el-option label="轻量 (2K)" value="light" />
                    <el-option label="中等 (8K)" value="medium" />
                    <el-option label="充分 (32K)" value="full" />
                    <el-option label="自定义" value="custom" />
                  </el-select>
                </el-form-item>
                <el-form-item v-if="form.reasoning_mode === 'custom'" label="推理预算 (tokens)">
                  <el-input-number
                    v-model="form.reasoning_budget"
                    :min="1000"
                    :max="100000"
                    :step="1000"
                    style="width: 100%"
                  />
                </el-form-item>
              </div>
            </el-form>
          </div>
        </div>

        <el-divider />

        <!-- 输出 & 执行 -->
        <div class="grid-2">
          <div class="block">
            <h3 class="block-title">输出</h3>
            <el-form label-position="top" :model="form">
              <div class="grid-2 inner">
                <el-form-item label="输出语言">
                  <el-select v-model="form.language" style="width: 100%">
                    <el-option label="中文" value="Chinese" />
                    <el-option label="English" value="English" />
                  </el-select>
                </el-form-item>
                <el-form-item label="输出受众">
                  <el-segmented
                    v-model="form.audience"
                    :options="[
                      { label: '人工可读', value: 'human' },
                      { label: 'Agent 精简', value: 'agent' },
                    ]"
                  />
                </el-form-item>
              </div>
              <el-form-item label="默认严重级别">
                <el-select v-model="form.default_severity" style="width: 100%">
                  <el-option label="严重 Critical" value="critical" />
                  <el-option label="高危 Major" value="major" />
                  <el-option label="中危 Minor" value="minor" />
                  <el-option label="低危 Info" value="info" />
                </el-select>
              </el-form-item>
            </el-form>
          </div>

          <div class="block">
            <h3 class="block-title">执行</h3>
            <el-form label-position="top" :model="form">
              <div class="grid-3 inner">
                <el-form-item label="模型并发">
                  <el-input-number v-model="form.concurrency" :min="1" :max="32" style="width: 100%" />
                </el-form-item>
                <el-form-item label="Git 并发">
                  <el-input-number v-model="form.max_git_procs" :min="1" :max="64" style="width: 100%" />
                </el-form-item>
                <el-form-item label="超时(分)">
                  <el-input-number v-model="form.timeout_minutes" :min="1" :max="120" style="width: 100%" />
                </el-form-item>
              </div>
            </el-form>
          </div>
        </div>

        <div class="save-bar">
          <span class="save-hint">修改后请点击保存，配置将立即生效。</span>
          <div class="save-buttons">
            <el-button @click="handleTestLLM" :loading="testingLLM">
              <el-icon><Connection /></el-icon>
              <span>测试连接</span>
            </el-button>
            <el-button type="primary" @click="handleSave" :loading="saving">
              <el-icon><Check /></el-icon>
              <span>保存配置</span>
            </el-button>
          </div>
        </div>
      </section>

      <!-- 高级配置 -->
      <section class="advanced">
        <button type="button" class="adv-toggle" @click="showAdvanced = !showAdvanced">
          <div class="adv-toggle-left">
            <span class="adv-title">高级配置</span>
            <small>OCR 二进制 / 工具 / 遥测 / Viewer / 审查背景</small>
          </div>
          <el-icon class="adv-arrow"><ArrowDown v-if="!showAdvanced" /><ArrowUp v-else /></el-icon>
        </button>
        <el-collapse-transition>
          <div v-show="showAdvanced" class="adv-body">
            <el-tabs v-model="activeTab" class="adv-tabs">
              <el-tab-pane label="执行参数" name="execution">
                <ExecutionParamsTab />
              </el-tab-pane>
              <el-tab-pane label="审查背景" name="review">
                <ReviewFocusTab />
              </el-tab-pane>
            </el-tabs>
          </div>
        </el-collapse-transition>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, provide, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Refresh, Connection, Search, Check, ArrowDown, ArrowUp, InfoFilled } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ExecutionParamsTab from '@/components/model-settings/ExecutionParamsTab.vue'
import ReviewFocusTab from '@/components/model-settings/ReviewFocusTab.vue'
import { modelSettingsKey, type ModelSettingsContext } from '@/components/model-settings/context'
import { getOCRSettings, listLLMProviders, updateOCRSettings, testLLM } from '@/api/config'
import type { LLMProvider, OCRSettings } from '@/types/config'

// 服务商显示元数据：以 OCR 二进制返回的 provider name 为键
const providerMeta: Record<string, { display: string; icon: string; region: string }> = {
  openai: { display: 'OpenAI', icon: 'openai', region: 'global' },
  anthropic: { display: 'Anthropic', icon: 'anthropic', region: 'global' },
  deepseek: { display: 'DeepSeek', icon: 'deepseek', region: 'cn' },
  dashscope: { display: '阿里百炼 DashScope', icon: 'dashscope', region: 'cn' },
  'dashscope-tokenplan': { display: '阿里百炼 (Token Plan)', icon: 'dashscope', region: 'cn' },
  'z-ai': { display: '智谱 AI', icon: 'zhipu', region: 'cn' },
  volcengine: { display: '火山引擎 (豆包)', icon: 'volcengine', region: 'cn' },
  mimo: { display: '小米 MiMo', icon: 'mimo', region: 'cn' },
  minimax: { display: 'MiniMax', icon: 'minimax', region: 'cn' },
  kimi: { display: 'Moonshot Kimi', icon: 'kimi', region: 'cn' },
  'baidu-qianfan': { display: '百度千帆', icon: 'baidu', region: 'cn' },
  'hy-tokenplan': { display: '腾讯混元 (Token Plan)', icon: 'hunyuan', region: 'cn' },
  'tencent-tokenhub': { display: '腾讯云 (TokenHub)', icon: 'tencent', region: 'cn' },
}

interface ProviderOption {
  name: string
  protocol: string
  base_url: string
  models: string[]
}

function metaOf(name: string) {
  return providerMeta[name] || { display: name, icon: name, region: '' }
}
function displayOf(name: string) {
  return metaOf(name).display
}
function iconOf(name: string) {
  return metaOf(name).icon
}
function initialOf(name: string) {
  return displayOf(name).charAt(0).toUpperCase()
}

const loading = ref(false)
const saving = ref(false)
const testingLLM = ref(false)
const syncingProviders = ref(false)
const showAdvanced = ref(false)
const activeTab = ref('execution')
const providerQuery = ref('')
const authToken = ref('')
const extraBodyText = ref('')

const providers = ref<LLMProvider[]>([])

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

const providerOptions = computed<ProviderOption[]>(() =>
  providers.value.map((p) => ({
    name: p.name,
    protocol: p.protocol,
    base_url: p.base_url || '',
    models: p.default_model || [],
  })),
)

const filteredProviders = computed(() => {
  const kw = providerQuery.value.trim().toLowerCase()
  if (!kw) return providerOptions.value
  return providerOptions.value.filter(
    (p) =>
      p.name.toLowerCase().includes(kw) ||
      displayOf(p.name).toLowerCase().includes(kw) ||
      p.models.some((m) => m.toLowerCase().includes(kw)),
  )
})

const currentModels = computed(() => {
  const matched = providerOptions.value.find((p) => p.name === form.provider)
  return matched?.models || []
})

const isCustomSelected = computed(
  () => form.provider === 'custom' || (!!form.provider && !providerOptions.value.some((p) => p.name === form.provider)),
)

function isProviderSelected(p: ProviderOption) {
  return form.provider === p.name
}

const currentIcon = computed(() => iconOf(form.provider || 'custom'))
const currentDisplayName = computed(() => (form.provider ? displayOf(form.provider) : '未配置'))
const currentInitial = computed(() => currentDisplayName.value.charAt(0))
const currentRegion = computed(() => (form.provider ? metaOf(form.provider).region : ''))

const reasoningLabel = computed(() => {
  switch (form.reasoning_mode) {
    case 'disabled':
      return '推理关闭'
    case 'light':
      return '推理·轻量'
    case 'medium':
      return '推理·中等'
    case 'full':
      return '推理·充分'
    case 'custom':
      return `推理·${form.reasoning_budget || 0}`
    default:
      return '推理·未知'
  }
})

function selectProvider(p: ProviderOption) {
  form.provider = p.name
  if (p.base_url) form.url = p.base_url
  form.use_anthropic = p.protocol === 'anthropic'
  if (p.models.length && (!form.model || !p.models.includes(form.model))) {
    form.model = p.models[0]
  }
}

function selectCustom() {
  form.provider = 'custom'
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

async function syncProviders(silent = false) {
  syncingProviders.value = true
  try {
    providers.value = await listLLMProviders()
    if (!silent) ElMessage.success(`已同步 ${providers.value.length} 个 Provider`)
  } catch (e: any) {
    if (!silent) ElMessage.error(e?.message || '同步 Provider 失败')
  } finally {
    syncingProviders.value = false
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

const ctx: ModelSettingsContext = {
  form,
  saving,
  testingLLM,
  extraBodyText,
  save: handleSave,
  testLLM: handleTestLLM,
  syncExtraBody,
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
.ms-page {
  width: min(1180px, 100%);
  margin: 0 auto;
  padding: 30px 32px 56px;
}

.ms-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 22px;
}

.ms-header h1 {
  margin: 4px 0 6px;
  font-size: 26px;
  color: var(--app-text);
}

.ms-header p {
  margin: 0;
  font-size: 13px;
  color: var(--app-muted);
}

.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}

.ms-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

/* 状态条 */
.status-strip {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 20px;
  background: linear-gradient(135deg, #eff4ff 0%, #e8effd 100%);
  border: 1px solid #cdddff;
  border-radius: 14px;
  margin-bottom: 20px;
}

.status-strip.off {
  background: var(--app-surface);
  border-color: var(--app-border);
}

.ss-icon {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 19px;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
  background: var(--app-border);
}

.ss-main {
  flex: 1;
  min-width: 0;
}

.ss-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 5px;
}

.ss-title strong {
  font-size: 16px;
  color: var(--app-text);
}

.ss-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: var(--app-muted);
  flex-wrap: wrap;
  min-width: 0;
}

.ss-meta .mono {
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
}

.ss-meta .url {
  color: var(--app-primary);
  word-break: break-all;
}

.sep {
  color: #c4ccd9;
}

.ss-controls {
  display: flex;
  align-items: center;
  gap: 14px;
  flex-shrink: 0;
}

.enable-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: var(--app-muted);
}

.empty-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 16px;
  margin-bottom: 16px;
  border-radius: 10px;
  background: #fff7e6;
  border: 1px solid #ffe3b3;
  color: #93601a;
  font-size: 13px;
}

/* 主卡片 */
.config-card {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 16px;
  padding: 24px 26px;
  box-shadow: var(--app-shadow);
}

.block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.block-head h3,
.block-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text);
}

.provider-search {
  max-width: 260px;
}

.provider-chips {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.pchip {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  border: 2px solid var(--app-border);
  border-radius: 12px;
  background: var(--app-surface);
  cursor: pointer;
  text-align: left;
  transition: all 0.18s ease;
  position: relative;
  font: inherit;
  color: var(--app-text);
}

.pchip:hover {
  border-color: var(--app-primary);
  transform: translateY(-1px);
}

.pchip.active {
  border-color: var(--app-primary);
  background: linear-gradient(135deg, rgba(37, 99, 235, 0.06), rgba(37, 99, 235, 0.02));
}

.pchip.custom {
  border-style: dashed;
}

.pchip-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 15px;
  font-weight: 700;
  color: #fff;
  flex-shrink: 0;
  background: var(--app-border);
}

.pchip-body {
  flex: 1;
  min-width: 0;
}

.pchip-body strong {
  display: block;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.pchip-body small {
  display: block;
  margin-top: 3px;
  font-size: 11px;
  color: var(--app-muted);
}

.pchip-check {
  position: absolute;
  top: 8px;
  right: 8px;
  color: var(--app-primary);
  font-size: 16px;
}

.grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}

.grid-2.inner {
  gap: 16px;
}

.grid-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.tip {
  font-size: 12px;
  color: var(--app-muted);
  margin-top: 4px;
}

:deep(.el-divider--horizontal) {
  margin: 22px 0;
}

.save-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 8px;
  padding-top: 18px;
  border-top: 1px solid var(--app-border);
}

.save-hint {
  font-size: 12px;
  color: var(--app-muted);
}

.save-buttons {
  display: flex;
  gap: 10px;
}

/* 高级配置 */
.advanced {
  margin-top: 22px;
}

.adv-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 14px 20px;
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: var(--app-surface);
  cursor: pointer;
  font: inherit;
  color: var(--app-text);
  transition: border-color 0.18s ease;
}

.adv-toggle:hover {
  border-color: var(--app-primary);
}

.adv-toggle-left {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.adv-title {
  font-size: 15px;
  font-weight: 600;
}

.adv-toggle-left small {
  font-size: 12px;
  color: var(--app-muted);
}

.adv-arrow {
  color: var(--app-muted);
}

.adv-body {
  margin-top: 12px;
  padding: 18px 22px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 14px;
}

/* 服务商图标配色（与 OCR 二进制返回的 name 对应） */
.ss-icon.openai,
.pchip-icon.openai { background: linear-gradient(135deg, #10a37f, #0d8a6a); }
.ss-icon.anthropic,
.pchip-icon.anthropic { background: linear-gradient(135deg, #d97706, #b45309); }
.ss-icon.deepseek,
.pchip-icon.deepseek { background: linear-gradient(135deg, #4d6bfe, #2b4fd6); }
.ss-icon.dashscope,
.pchip-icon.dashscope { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.ss-icon.zhipu,
.pchip-icon.zhipu { background: linear-gradient(135deg, #3b5bdb, #2b3fa8); }
.ss-icon.volcengine,
.pchip-icon.volcengine { background: linear-gradient(135deg, #f59e0b, #d97706); }
.ss-icon.mimo,
.pchip-icon.mimo { background: linear-gradient(135deg, #10b981, #059669); }
.ss-icon.minimax,
.pchip-icon.minimax { background: linear-gradient(135deg, #ec4899, #db2777); }
.ss-icon.kimi,
.pchip-icon.kimi { background: linear-gradient(135deg, #8b5cf6, #6d28d9); }
.ss-icon.baidu,
.pchip-icon.baidu { background: linear-gradient(135deg, #2932e1, #1b2bbd); }
.ss-icon.hunyuan,
.pchip-icon.hunyuan { background: linear-gradient(135deg, #0052d9, #003eaa); }
.ss-icon.tencent,
.pchip-icon.tencent { background: linear-gradient(135deg, #00a4ff, #0080d6); }
.pchip-icon.custom { background: var(--app-border); color: var(--app-muted); }

@media (max-width: 860px) {
  .ms-page {
    padding: 20px 16px 40px;
  }

  .grid-2 {
    grid-template-columns: 1fr;
  }

  .grid-3 {
    grid-template-columns: repeat(2, 1fr);
  }

  .status-strip {
    flex-wrap: wrap;
  }

  .ss-controls {
    width: 100%;
    justify-content: space-between;
  }
}

@media (max-width: 560px) {
  .grid-3 {
    grid-template-columns: 1fr;
  }

  .save-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .save-buttons {
    justify-content: flex-end;
  }
}
</style>
