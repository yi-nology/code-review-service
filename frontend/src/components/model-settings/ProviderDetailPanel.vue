<template>
  <div class="provider-detail">
    <!-- Empty state -->
    <el-empty v-if="!provider" description="请从左侧选择一个供应商" :image-size="80" class="empty-state" />

    <!-- Built-in provider detail -->
    <template v-else-if="type === 'builtin'">
      <el-card class="detail-card">
        <template #header>
          <div class="card-header">
            <div class="header-title">
              <span class="provider-icon builtin">
                <el-icon><Connection /></el-icon>
              </span>
              <div>
                <strong>{{ provider.name }}</strong>
                <span class="header-meta">{{ builtinProvider?.protocol || 'openai' }} 协议</span>
              </div>
            </div>
            <el-tag type="success" size="small">内置</el-tag>
          </div>
        </template>

        <el-form :model="form" label-width="100px" :disabled="saving">
          <el-form-item label="接口地址">
            <el-input v-model="form.url" placeholder="http://127.0.0.1:11434/v1/chat/completions" />
          </el-form-item>

          <el-form-item label="模型名称">
            <el-select v-model="form.model" filterable allow-create placeholder="选择或输入模型">
              <el-option
                v-for="m in currentModelList"
                :key="m"
                :label="m"
                :value="m"
              />
            </el-select>
          </el-form-item>

          <el-form-item label="请求协议">
            <el-select v-model="form.use_anthropic">
              <el-option label="OpenAI 兼容" :value="false" />
              <el-option label="Anthropic" :value="true" />
            </el-select>
          </el-form-item>

          <el-form-item label="Token">
            <el-input v-model="form.auth_token" type="password" placeholder="留空则保持现有 Token" show-password />
          </el-form-item>

          <el-form-item label="Token 请求头">
            <el-input v-model="form.auth_header" placeholder="Authorization" />
          </el-form-item>

          <el-form-item label="额外请求头">
            <el-input
              v-model="form.extra_headers"
              type="textarea"
              :rows="2"
              placeholder='X-Org-ID=org-123,X-Trace-ID=review-service'
            />
            <div class="form-hint">可选。逗号分隔的 key=value，会透传给 OCR_LLM_EXTRA_HEADERS。</div>
          </el-form-item>

          <el-form-item label="输出语言">
            <el-select v-model="form.language">
              <el-option label="中文" value="Chinese" />
              <el-option label="English" value="English" />
            </el-select>
          </el-form-item>

          <el-form-item label="推理模式">
            <el-select v-model="form.reasoning_mode" placeholder="选择推理模式">
              <el-option label="禁用" value="disabled" />
              <el-option label="轻量 (2K tokens)" value="light" />
              <el-option label="中等 (8K tokens)" value="medium" />
              <el-option label="充分 (32K tokens)" value="full" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="form.reasoning_mode === 'custom'" label="推理预算">
            <el-input-number
              v-model="form.reasoning_budget"
              :min="1000"
              :max="100000"
              :step="1000"
              placeholder="自定义 token 预算"
            />
            <span class="form-hint">tokens</span>
          </el-form-item>

          <el-divider content-position="left">上下文窗口</el-divider>

          <el-form-item label="上下文窗口">
            <el-select v-model="contextWindowOption" placeholder="选择上下文窗口大小">
              <el-option label="自动 (模型默认)" value="auto" />
              <el-option label="32K tokens" :value="32000" />
              <el-option label="64K tokens" :value="64000" />
              <el-option label="128K tokens" :value="128000" />
              <el-option label="256K tokens" :value="256000" />
              <el-option label="512K tokens" :value="512000" />
              <el-option label="1M tokens" :value="1000000" />
              <el-option label="2M tokens" :value="2000000" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="contextWindowOption === 'custom'" label="自定义窗口">
            <el-input-number
              v-model="form.context_window"
              :min="4096"
              :max="10000000"
              :step="8192"
              placeholder="自定义上下文窗口大小"
            />
            <span class="form-hint">tokens</span>
          </el-form-item>

          <el-form-item label="最大输出">
            <el-input-number
              v-model="form.max_output_tokens"
              :min="1024"
              :max="100000"
              :step="1024"
              placeholder="最大输出 token 数"
            />
            <span class="form-hint">tokens (0=模型默认)</span>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" @click="save" :loading="saving">保存配置</el-button>
            <el-button @click="testLLM" :loading="testingLLM">测试连接</el-button>
            <el-button @click="fetchVersion" :loading="loadingVersion">查看版本</el-button>
          </el-form-item>
        </el-form>
      </el-card>
    </template>

    <!-- Custom provider detail -->
    <template v-else>
      <el-card class="detail-card">
        <template #header>
          <div class="card-header">
            <div class="header-title">
              <span class="provider-icon custom">
                <el-icon><Setting /></el-icon>
              </span>
              <div>
                <strong>{{ customProvider?.name || '新建供应商' }}</strong>
                <span class="header-meta">{{ customProvider ? '自定义供应商' : '创建新的自定义供应商' }}</span>
              </div>
            </div>
            <el-switch
              v-if="customProvider"
              v-model="cpForm.enabled"
              inline-prompt
              active-text="启用"
              inactive-text="禁用"
              @change="handleSaveCustom"
            />
          </div>
        </template>

        <el-form :model="cpForm" label-width="100px" :disabled="saving">
          <el-form-item label="名称" required>
            <el-input v-model="cpForm.name" placeholder="my-provider" />
          </el-form-item>

          <el-form-item label="协议" required>
            <el-select v-model="cpForm.protocol">
              <el-option label="OpenAI 兼容" value="openai" />
              <el-option label="Anthropic" value="anthropic" />
            </el-select>
          </el-form-item>

          <el-form-item label="接口地址">
            <el-input v-model="cpForm.url" placeholder="https://api.example.com/v1/chat/completions" />
          </el-form-item>

          <el-form-item label="API Key">
            <el-input v-model="cpForm.api_key" type="password" :placeholder="customProvider ? '留空则不修改' : '请输入 API Key'" show-password />
          </el-form-item>

          <el-form-item label="模型名称">
            <el-input v-model="cpForm.model" placeholder="model-name" />
          </el-form-item>

          <el-form-item label="Token 请求头">
            <el-select v-model="cpForm.auth_header" clearable placeholder="默认">
              <el-option label="Authorization" value="authorization" />
              <el-option label="X-API-Key" value="x-api-key" />
            </el-select>
          </el-form-item>

          <el-form-item label="额外请求体">
            <el-input v-model="cpForm.extra_body" type="textarea" :rows="2" placeholder='可选 JSON 对象' />
          </el-form-item>

          <el-divider content-position="left">推理模式</el-divider>

          <el-form-item label="推理模式">
            <el-select v-model="form.reasoning_mode" placeholder="选择推理模式">
              <el-option label="禁用" value="disabled" />
              <el-option label="轻量 (2K tokens)" value="light" />
              <el-option label="中等 (8K tokens)" value="medium" />
              <el-option label="充分 (32K tokens)" value="full" />
              <el-option label="自定义" value="custom" />
            </el-select>
          </el-form-item>

          <el-form-item v-if="form.reasoning_mode === 'custom'" label="推理预算">
            <el-input-number
              v-model="form.reasoning_budget"
              :min="1000"
              :max="100000"
              :step="1000"
              placeholder="自定义 token 预算"
            />
            <span class="form-hint">tokens</span>
          </el-form-item>

          <el-form-item>
            <el-button type="primary" @click="handleSaveCustom" :loading="saving">
              {{ customProvider ? '保存修改' : '创建供应商' }}
            </el-button>
            <el-button v-if="customProvider" @click="testLLM" :loading="testingLLM">测试连接</el-button>
            <el-popconfirm
              v-if="customProvider"
              title="确认删除该供应商？"
              @confirm="handleDeleteCustom"
            >
              <template #reference>
                <el-button type="danger">删除供应商</el-button>
              </template>
            </el-popconfirm>
          </el-form-item>
        </el-form>
      </el-card>
    </template>

    <!-- OCR Version Dialog -->
    <el-dialog v-model="showVersion" title="OCR 版本信息" width="400px">
      <el-skeleton v-if="loadingVersion" :rows="3" animated />
      <el-descriptions :column="1" border v-else-if="ocrVersion">
        <el-descriptions-item label="版本">{{ ocrVersion.version }}</el-descriptions-item>
        <el-descriptions-item label="Commit" v-if="ocrVersion.commit">{{ ocrVersion.commit }}</el-descriptions-item>
        <el-descriptions-item label="构建时间" v-if="ocrVersion.build_time">{{ ocrVersion.build_time }}</el-descriptions-item>
      </el-descriptions>
      <el-empty v-else description="获取版本信息失败" :image-size="60" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Connection, Setting } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { useModelSettings } from './context'
import { getOCRVersion, createCustomProvider, updateCustomProvider, deleteCustomProvider } from '@/api/config'
import type { LLMProvider, CustomProvider, OCRVersion } from '@/types/config'

const props = defineProps<{
  provider: LLMProvider | CustomProvider | null
  type: 'builtin' | 'custom' | null
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
  (e: 'delete'): void
}>()

const { form, saving, testingLLM, save, testLLM } = useModelSettings()

const ocrVersion = ref<OCRVersion | null>(null)
const showVersion = ref(false)
const loadingVersion = ref(false)

const builtinProvider = computed(() => (props.type === 'builtin' ? props.provider as LLMProvider : null))
const customProvider = computed(() => (props.type === 'custom' ? props.provider as CustomProvider : null))

const cpForm = ref({
  name: '',
  protocol: 'openai',
  url: '',
  api_key: '',
  model: '',
  auth_header: '',
  extra_body: '',
  enabled: true,
})

// 上下文窗口预设选项
const contextWindowPresets = [32000, 64000, 128000, 256000, 512000, 1000000, 2000000]

// 获取当前 Provider 的模型列表（优先使用后端返回的 default_model）
const currentModelList = computed(() => {
  if (props.type === 'builtin' && props.provider) {
    const bp = props.provider as LLMProvider
    return bp.default_model || []
  }
  return []
})

// 上下文窗口选择器
const contextWindowOption = computed<number | string>({
  get() {
    const val = form.context_window
    if (!val || val === 0) return 'auto'
    if (contextWindowPresets.includes(val)) return val
    return 'custom'
  },
  set(v) {
    if (v === 'auto') {
      form.context_window = 0
    } else if (typeof v === 'number') {
      form.context_window = v
    }
    // 'custom' 不修改当前值，让用户手动输入
  },
})

watch(
  () => props.provider,
  (p) => {
    if (!p) return
    if (props.type === 'builtin') {
      const bp = p as LLMProvider
      form.url = bp.base_url || ''
      form.model = bp.default_model?.[0] || ''
      form.use_anthropic = bp.protocol === 'anthropic'
      form.provider = bp.name
    } else if (props.type === 'custom') {
      const cp = p as CustomProvider
      form.url = cp.url || ''
      form.model = cp.model || ''
      form.use_anthropic = cp.protocol === 'anthropic'
      form.provider = 'custom:' + cp.name
      cpForm.value = {
        name: cp.name,
        protocol: cp.protocol,
        url: cp.url || '',
        api_key: '',
        model: cp.model || '',
        auth_header: cp.auth_header || '',
        extra_body: cp.extra_body || '',
        enabled: cp.enabled,
      }
    }
  },
  { immediate: true }
)

async function fetchVersion() {
  loadingVersion.value = true
  showVersion.value = true
  try {
    ocrVersion.value = await getOCRVersion()
  } catch {
    ocrVersion.value = null
  } finally {
    loadingVersion.value = false
  }
}

async function handleSaveCustom() {
  if (!cpForm.value.name.trim()) {
    ElMessage.warning('名称不能为空')
    return
  }
  saving.value = true
  try {
    if (customProvider.value) {
      await updateCustomProvider(customProvider.value.id, cpForm.value)
      ElMessage.success('更新成功')
    } else {
      await createCustomProvider(cpForm.value)
      ElMessage.success('创建成功')
    }
    emit('refresh')
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    saving.value = false
  }
}

async function handleDeleteCustom() {
  if (!customProvider.value) return
  try {
    await deleteCustomProvider(customProvider.value.id)
    ElMessage.success('已删除')
    emit('delete')
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}
</script>

<style scoped>
.provider-detail {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.empty-state {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
}

.detail-card {
  height: 100%;
}

.detail-card :deep(.el-card__body) {
  overflow-y: auto;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.header-title strong {
  font-size: 14px;
  color: var(--app-text);
}

.header-meta {
  display: block;
  font-size: 12px;
  color: var(--app-muted);
}

.provider-icon {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.provider-icon.builtin {
  background: linear-gradient(135deg, #3b82f6, #1d4ed8);
  color: white;
}

.provider-icon.custom {
  background: linear-gradient(135deg, #8b5cf6, #6d28d9);
  color: white;
}

.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: var(--app-muted);
}
</style>
