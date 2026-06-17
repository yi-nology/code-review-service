<template>
  <AppLayout>
    <div class="model-settings-page">
      <header class="page-header">
        <div class="eyebrow">MODEL SETTINGS</div>
        <h1>审查引擎配置</h1>
        <p>管理 Open Code Review 的模型连接、CLI 执行方式与默认审查行为。</p>
      </header>

      <el-card v-loading="loading">
        <el-form :model="form" label-width="120px" :disabled="saving">
          <el-divider content-position="left">模型连接</el-divider>

          <el-form-item label="启用 OCR">
            <el-switch v-model="form.enabled" />
          </el-form-item>

          <el-form-item label="模型供应商">
            <el-select v-model="form.provider" @change="handleProviderChange">
              <el-option label="阿里云百炼 · 千问" value="qwen" />
              <el-option label="智谱 AI · GLM" value="zhipu" />
              <el-option label="MiniMax" value="minimax" />
              <el-option label="小米 · MiMo" value="mimo" />
              <el-option label="DeepSeek" value="deepseek" />
              <el-option label="本地 Ollama" value="ollama" />
              <el-option label="自定义兼容接口" value="custom" />
            </el-select>
          </el-form-item>

          <el-form-item label="模型接口地址">
            <el-input v-model="form.url" placeholder="http://127.0.0.1:11434/v1/chat/completions" />
          </el-form-item>

          <el-form-item label="模型名称">
            <el-input v-model="form.model" placeholder="qwen3.5-plus" />
          </el-form-item>

          <el-form-item label="请求协议">
            <el-select v-model="form.use_anthropic">
              <el-option label="OpenAI 兼容" :value="false" />
              <el-option label="Anthropic" :value="true" />
            </el-select>
          </el-form-item>

          <el-form-item label="Token">
            <el-input v-model="form.auth_token" type="password" placeholder="留空则保持现有 Token" />
          </el-form-item>

          <el-form-item label="Token 请求头">
            <el-input v-model="form.auth_header" placeholder="Authorization" />
          </el-form-item>

          <el-form-item label="输出语言">
            <el-select v-model="form.language">
              <el-option label="中文" value="Chinese" />
              <el-option label="English" value="English" />
            </el-select>
          </el-form-item>

          <el-divider content-position="left">执行参数</el-divider>

          <el-form-item label="OCR 二进制">
            <el-input v-model="form.binary" placeholder="ocr 或 /usr/local/bin/ocr" />
            <div class="form-tip">只配置可执行文件路径，具体 review 参数由服务端按白名单拼装。</div>
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

          <el-form-item label="默认严重级别">
            <el-select v-model="form.default_severity">
              <el-option label="Critical" value="critical" />
              <el-option label="Major" value="major" />
              <el-option label="Minor" value="minor" />
              <el-option label="Info" value="info" />
            </el-select>
            <div class="form-tip">当 OCR 输出未携带严重级别时使用该值。</div>
          </el-form-item>

          <el-form-item label="模型并发数">
            <el-input-number v-model="form.concurrency" :min="1" :max="32" />
          </el-form-item>

          <el-form-item label="Git 并发数">
            <el-input-number v-model="form.max_git_procs" :min="1" :max="64" />
          </el-form-item>

          <el-form-item label="单文件超时（分钟）">
            <el-input-number v-model="form.timeout_minutes" :min="1" :max="120" />
          </el-form-item>

          <el-form-item label="最大工具轮次">
            <el-input-number v-model="form.max_tools" :min="0" :max="100" />
            <div class="form-tip">0 表示使用 OCR 内置默认值</div>
          </el-form-item>

          <el-form-item label="额外请求体">
            <el-input
              v-model="extraBodyText"
              type="textarea"
              :rows="4"
              placeholder='{"temperature":0.2}'
              @blur="handleExtraBodyBlur"
            />
            <div class="form-tip">可选。必须是 JSON 对象字符串，会透传给 OCR_LLM_EXTRA_BODY。</div>
          </el-form-item>

          <el-divider content-position="left">默认审查侧重点</el-divider>

          <el-form-item label="审查背景">
            <el-input
              v-model="form.background"
              type="textarea"
              :rows="4"
              placeholder="默认的审查侧重点，例如：这是一个电商项目，重点关注支付流程的安全性和性能"
            />
            <div class="form-tip">当未配置代理团队时，使用此默认背景进行审查。配置代理后，各代理使用各自的审查侧重点。</div>
          </el-form-item>

          <el-divider content-position="left">执行预览</el-divider>

          <div class="command-preview">
            <div class="preview-title">服务端将按受控参数组装命令</div>
            <code>{{ commandPreview }}</code>
          </div>

          <el-form-item>
            <el-button type="primary" @click="handleSave" :loading="saving">保存配置</el-button>
            <el-button @click="handleTestLLM" :loading="testingLLM">测试 LLM 连接</el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getOCRSettings, updateOCRSettings, testLLM } from '@/api/config'
import type { OCRSettings } from '@/types/config'

const loading = ref(false)
const saving = ref(false)
const testingLLM = ref(false)
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
  language: 'Chinese',
  concurrency: 2,
  max_git_procs: 4,
  timeout_minutes: 10,
  max_tools: 0,
  background: '',
})

const commandPreview = computed(() => {
  const args = [
    form.binary || 'ocr',
    'review',
    '--repo <workspace>',
    '--from <base>',
    '--to <head>',
    '--format json',
    `--concurrency ${form.concurrency || 2}`,
    `--max-git-procs ${form.max_git_procs || 4}`,
    `--timeout ${form.timeout_minutes || 10}`,
    `--audience ${form.audience || 'human'}`,
  ]
  if (form.max_tools && form.max_tools > 0) {
    args.push(`--max-tools ${form.max_tools}`)
  }
  if (form.background) {
    args.push('--background <configured background>')
  }
  return args.join(' ')
})

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

function handleExtraBodyBlur() {
  syncExtraBody(false)
}

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

function handleProviderChange(provider: string) {
  const presets: Record<string, { url: string; model: string }> = {
    qwen: { url: 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions', model: 'qwen-plus' },
    zhipu: { url: 'https://open.bigmodel.cn/api/paas/v4/chat/completions', model: 'glm-4' },
    minimax: { url: 'https://api.minimax.chat/v1/text/chatcompletion_v2', model: 'abab6.5-chat' },
    mimo: { url: 'https://token-plan-cn.xiaomimimo.com/v1/chat/completions', model: 'mimo-v2.5-pro' },
    deepseek: { url: 'https://api.deepseek.com/v1/chat/completions', model: 'deepseek-coder' },
    ollama: { url: 'http://127.0.0.1:11434/v1/chat/completions', model: 'qwen2.5-coder:7b' },
  }
  const preset = presets[provider]
  if (preset) {
    form.url = preset.url
    form.model = preset.model
  }
}

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

.form-tip {
  font-size: 12px;
  color: #999;
  margin-top: 4px;
}

.command-preview {
  margin: 0 0 22px 120px;
  padding: 12px 14px;
  border: 1px solid #dcdfe6;
  border-radius: 8px;
  background: #f7f9fc;
}

.preview-title {
  margin-bottom: 8px;
  color: #606266;
  font-size: 12px;
}

.command-preview code {
  display: block;
  white-space: pre-wrap;
  word-break: break-word;
  color: #303133;
  font-size: 12px;
  line-height: 1.6;
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

  .command-preview {
    margin-left: 0;
  }
}
</style>
