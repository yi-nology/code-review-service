<script setup lang="ts">
/**
 * ModelProviderSection — 服务商选择 + 连接 + 模型 + 推理 + 输出语言。
 * 从原 ModelSettings.vue 抽取，通过 useSettingsForm() 读写共享表单状态。
 * 不含保存按钮 —— 由页面级 sticky 保存条统一触发。
 */
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { Connection, Search, Check, InfoFilled } from '@element-plus/icons-vue'
import { useSettingsForm } from './context'
import { listLLMProviders } from '@/api/config'
import type { LLMProvider } from '@/types/config'

const { form, authToken, testingLLM, testLLM } = useSettingsForm()

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
  // OCR v1.7.7+ 新增 EdenAI（聚合 provider）。
  edenai: { display: 'Eden AI', icon: 'edenai', region: 'global' },
  // OCR v1.7.11+ 新增 Ollama Cloud、智谱 coding plan。
  'ollama-cloud': { display: 'Ollama Cloud', icon: 'ollama', region: 'global' },
  'z-ai-coding': { display: '智谱 AI (Coding Plan)', icon: 'zhipu', region: 'cn' },
  // OCR v1.7.12+ 新增 LiteLLM gateway（统一接 100+ 模型）。
  litellm: { display: 'LiteLLM Gateway', icon: 'litellm', region: 'self' },
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

const syncingProviders = ref(false)
const providerQuery = ref('')
const providers = ref<LLMProvider[]>([])

// 常见模型的 context window 映射 (2026年数据)
const MODEL_CONTEXT_WINDOWS: Record<string, number> = {
  // OpenAI GPT-5 系列
  'gpt-5.5': 400000,
  'gpt-5.4': 1048576,        // 272K standard, 1.05M opt-in
  'gpt-5.4-pro': 1048576,
  'gpt-5.2': 400000,
  'gpt-5': 400000,
  'gpt-5-mini': 400000,
  'gpt-5-nano': 400000,
  'gpt-5.3-codex': 400000,
  // OpenAI GPT-4 系列
  'gpt-4.1': 1000000,
  'gpt-4o': 128000,
  'gpt-4o-mini': 128000,
  'gpt-4-turbo': 128000,
  'gpt-4': 8192,
  'gpt-3.5-turbo': 16385,
  // OpenAI o 系列
  'o1': 200000,
  'o1-mini': 128000,
  'o1-preview': 128000,
  'o3': 200000,
  'o3-mini': 200000,
  'o4-mini': 200000,
  // Anthropic Claude 4 系列
  'claude-opus-4': 200000,
  'claude-opus-4.6': 200000,
  'claude-sonnet-4': 200000,
  'claude-sonnet-4.6': 200000,
  'claude-haiku-4': 200000,
  'claude-haiku-4.5': 200000,
  // Anthropic Claude 3 系列
  'claude-3-opus': 200000,
  'claude-3-sonnet': 200000,
  'claude-3-haiku': 200000,
  'claude-3.5-sonnet': 200000,
  'claude-3.5-haiku': 200000,
  // DeepSeek
  'deepseek-v4-pro': 128000,
  'deepseek-v4-flash': 128000,
  'deepseek-v3.2': 128000,
  'deepseek-v3.1': 128000,
  'deepseek-chat': 128000,
  'deepseek-coder': 128000,
  'deepseek-reasoner': 128000,
  'deepseek-r1': 128000,
  // Google Gemini
  'gemini-3.1-pro': 1000000,
  'gemini-3-flash': 1000000,
  'gemini-2.5-pro': 1000000,
  'gemini-2.5-flash': 1000000,
  'gemini-pro': 1000000,
  'gemini-flash': 1000000,
  // Qwen 通义千问
  'qwen3.5': 1000000,        // 256K standard, 1M via Plus API
  'qwen3': 256000,
  'qwen3-235b': 1000000,
  'qwen3-coder': 1000000,
  'qwen-coder-plus': 131072,
  'qwen-turbo': 1000000,
  'qwen-plus': 131072,
  'qwen-max': 32768,
  'qwen-long': 10000000,
  'qwen2.5': 131072,
  'qwen2': 32768,
  // 智谱 GLM
  'glm-5.2': 128000,
  'glm-5.1': 128000,
  'glm-4v-plus': 131072,
  'glm-4': 128000,
  'glm-4-flash': 128000,
  'glm-4v': 131072,
  // 豆包
  'doubao-seed-2-0-pro': 256000,
  'doubao-seed-2-0-lite': 256000,
  'doubao-seed-2-0-code': 256000,
  'doubao-pro': 128000,
  'doubao-lite': 128000,
  // MiMo 小米
  'mimo-v2.5-pro': 1048576,
  'mimo-v2.5': 1048576,
  'mimo-v2.5-lite': 1048576,
  'mimo': 1048576,           // 1M context
  // MiniMax
  'minimax-m2.7-highspeed': 1000000,
  'minimax-m2.5': 1000000,
  // Moonshot Kimi
  'moonshot-v1-8k': 8192,
  'moonshot-v1-32k': 32768,
  'moonshot-v1-128k': 131072,
  'kimi': 131072,
  // Mistral
  'mistral-large-3': 256000,
  'mistral-large': 256000,
  'mistral-medium-3': 131000,
  'mistral-medium': 131000,
  'mistral-small': 256000,
  'ministral': 256000,
  'magistral': 128000,
  'mixtral': 32768,
  'mistral': 32768,
  // Meta Llama
  'llama-4-maverick': 1000000,
  'llama-4-scout': 10000000,   // 10M context!
  'llama-4': 1000000,
  'llama-3': 8192,
  'llama-2': 4096,
  // Nvidia Nemotron
  'nemotron-3-ultra': 1000000,
  'nemotron-3-super': 1000000,
  'nemotron-3-nano': 1000000,
  'nemotron': 1000000,
  // 其他小模型
  'yi': 4096,
  'chatglm': 8192,
  'codellama': 16384,
}

// 获取当前模型的 context window
const currentContextWindow = computed(() => {
  const model = form.model?.toLowerCase() || ''
  // 精确匹配
  if (MODEL_CONTEXT_WINDOWS[model]) return MODEL_CONTEXT_WINDOWS[model]
  // 模糊匹配：模型名包含某关键字即命中（长键优先，避免短键误匹配）
  const fuzzyKeys = Object.keys(MODEL_CONTEXT_WINDOWS).sort((a, b) => b.length - a.length)
  for (const key of fuzzyKeys) {
    if (key && model.includes(key)) return MODEL_CONTEXT_WINDOWS[key]
  }
  // 默认 128K（大多数现代模型）
  return 128000
})

// 根据 context window 动态计算推理模式选项
const reasoningModeOptions = computed(() => {
  const ctx = currentContextWindow.value
  // 按比例计算：轻量 ~2%, 中等 ~12%, 充分 ~50%；
  // 上限刻意调高，确保 128K/200K/256K 等常见模型之间能看到差异
  const light = Math.min(8192, Math.max(1024, Math.round(ctx * 0.02)))
  const medium = Math.min(32768, Math.max(4096, Math.round(ctx * 0.12)))
  const full = Math.min(131072, Math.max(8192, Math.round(ctx * 0.5)))

  return [
    { label: '禁用', value: 'disabled' },
    { label: `轻量 (${formatTokens(light)})`, value: 'light', budget: light },
    { label: `中等 (${formatTokens(medium)})`, value: 'medium', budget: medium },
    { label: `充分 (${formatTokens(full)})`, value: 'full', budget: full },
    { label: '自定义', value: 'custom' },
  ]
})

function formatTokens(n: number): string {
  if (n >= 1000000) return `${(n / 1000000).toFixed(0)}M`
  if (n >= 1000) return `${(n / 1000).toFixed(0)}K`
  return `${n}`
}

// 当用户选择预设推理模式时，同步写入 reasoning_budget。
function onReasoningModeChange(mode: string) {
  form.reasoning_mode = mode as any
  if (mode !== 'custom') {
    const opt = reasoningModeOptions.value.find((o) => o.value === mode)
    form.reasoning_budget = (opt as any)?.budget || 0
  }
}

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

onMounted(() => {
  syncProviders(true)
})

defineExpose({ syncProviders })
</script>

<template>
  <section v-if="!form.provider" class="empty-banner">
    <el-icon><InfoFilled /></el-icon>
    <span>尚未选择服务商，请在下方选择或使用自定义配置。</span>
  </section>

  <!-- 服务商 -->
  <div class="block">
    <div class="block-head">
      <h3>服务商</h3>
      <div class="head-actions">
        <el-button size="small" @click="syncProviders()" :loading="syncingProviders">
          <el-icon><Connection /></el-icon>
          <span>同步</span>
        </el-button>
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
        <el-form-item label="LLM 协议 (OCR v1.7.10+)">
          <el-select v-model="form.protocol" placeholder="自动（由上方协议推断）" clearable style="width: 100%">
            <el-option label="自动（由请求协议推断，兼容旧版）" value="" />
            <el-option label="anthropic" value="anthropic" />
            <el-option label="openai (Chat Completions)" value="openai" />
            <el-option label="openai-responses (GPT-5.x / o-series)" value="openai-responses" />
          </el-select>
          <div class="form-hint">设为「自动」时由「请求协议」推断；GPT-5.x / o-series 等走 Responses API 的模型必须选 openai-responses。</div>
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
            <el-select :model-value="form.reasoning_mode" style="width: 100%" @change="onReasoningModeChange">
              <el-option
                v-for="opt in reasoningModeOptions"
                :key="opt.value"
                :label="opt.label"
                :value="opt.value"
              />
            </el-select>
            <div class="tip">基于当前模型 {{ form.model || '未选择' }} ({{ formatTokens(currentContextWindow) }} 上下文) 计算</div>
          </el-form-item>
          <el-form-item v-if="form.reasoning_mode === 'custom'" label="推理预算 (tokens)">
            <el-input-number
              v-model="form.reasoning_budget"
              :min="1000"
              :max="currentContextWindow"
              :step="1000"
              style="width: 100%"
            />
          </el-form-item>
        </div>
      </el-form>
    </div>
  </div>

  <el-divider />

  <!-- 输出语言 -->
  <div class="grid-2">
    <div class="block">
      <h3 class="block-title">输出</h3>
      <el-form label-position="top" :model="form">
        <el-form-item label="输出语言">
          <el-select v-model="form.language" style="width: 100%">
            <el-option label="中文" value="Chinese" />
            <el-option label="English" value="English" />
          </el-select>
        </el-form-item>
      </el-form>
    </div>
    <div class="block block-spacer" />
  </div>

  <div class="section-test">
    <el-button @click="testLLM" :loading="testingLLM">
      <el-icon><Connection /></el-icon>
      <span>测试连接</span>
    </el-button>
  </div>
</template>

<style scoped>
.block { margin-bottom: 0; }

.block-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.block-head h3,
.block-title {
  margin: 0 0 12px;
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text);
}

.provider-search {
  max-width: 220px;
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

.tip {
  font-size: 12px;
  color: var(--app-muted);
  margin-top: 4px;
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

.block-spacer {
  /* 占位让输出语言与连接/模型两列对齐 */
}

.section-test {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}

:deep(.el-divider--horizontal) {
  margin: 22px 0;
}

/* 服务商图标配色（与 OCR 二进制返回的 name 对应） */
.pchip-icon.openai { background: linear-gradient(135deg, #10a37f, #0d8a6a); }
.pchip-icon.anthropic { background: linear-gradient(135deg, #d97706, #b45309); }
.pchip-icon.deepseek { background: linear-gradient(135deg, #4d6bfe, #2b4fd6); }
.pchip-icon.dashscope { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.pchip-icon.zhipu { background: linear-gradient(135deg, #3b5bdb, #2b3fa8); }
.pchip-icon.volcengine { background: linear-gradient(135deg, #f59e0b, #d97706); }
.pchip-icon.mimo { background: linear-gradient(135deg, #10b981, #059669); }
.pchip-icon.minimax { background: linear-gradient(135deg, #ec4899, #db2777); }
.pchip-icon.kimi { background: linear-gradient(135deg, #8b5cf6, #6d28d9); }
.pchip-icon.baidu { background: linear-gradient(135deg, #2932e1, #1b2bbd); }
.pchip-icon.hunyuan { background: linear-gradient(135deg, #0052d9, #003eaa); }
.pchip-icon.tencent { background: linear-gradient(135deg, #00a4ff, #0080d6); }
/* OCR v1.7.7+ EdenAI */
.pchip-icon.edenai { background: linear-gradient(135deg, #14b8a6, #0d9488); }
/* OCR v1.7.11+ Ollama Cloud */
.pchip-icon.ollama { background: linear-gradient(135deg, #6b7280, #4b5563); }
/* OCR v1.7.12+ LiteLLM gateway（自部署） */
.pchip-icon.litellm { background: linear-gradient(135deg, #34d399, #10b981); }
.pchip-icon.custom { background: var(--app-border); color: var(--app-muted); }

@media (max-width: 860px) {
  .grid-2 {
    grid-template-columns: 1fr;
  }
}
</style>
