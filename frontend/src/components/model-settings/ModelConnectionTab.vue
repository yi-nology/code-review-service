<template>
  <el-form :model="form" label-width="120px" :disabled="saving">
    <el-form-item label="启用 OCR">
      <el-switch v-model="form.enabled" />
    </el-form-item>

    <el-form-item label="模型供应商">
      <el-select v-model="form.provider" @change="handleProviderChange" filterable>
        <el-option
          v-for="p in builtInProviders"
          :key="p.name"
          :label="p.name"
          :value="p.name"
        />
        <el-option
          v-for="p in customProviders"
          :key="'custom-' + p.name"
          :label="p.name + ' (自定义)'"
          :value="'custom:' + p.name"
        />
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

    <el-form-item>
      <el-button type="primary" @click="save" :loading="saving">保存配置</el-button>
      <el-button @click="testLLM" :loading="testingLLM">测试 LLM 连接</el-button>
      <el-button @click="showVersion = true" :loading="loadingVersion">查看版本</el-button>
    </el-form-item>

    <el-dialog v-model="showVersion" title="OCR 版本信息" width="400px">
      <el-descriptions :column="1" border v-if="ocrVersion">
        <el-descriptions-item label="版本">{{ ocrVersion.version }}</el-descriptions-item>
        <el-descriptions-item label="Commit" v-if="ocrVersion.commit">{{ ocrVersion.commit }}</el-descriptions-item>
        <el-descriptions-item label="平台" v-if="ocrVersion.platform">{{ ocrVersion.platform }}</el-descriptions-item>
        <el-descriptions-item label="构建时间" v-if="ocrVersion.build_time">{{ ocrVersion.build_time }}</el-descriptions-item>
      </el-descriptions>
    </el-dialog>
  </el-form>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useModelSettings } from './context'
import { listLLMProviders, listCustomProviders, getOCRVersion } from '@/api/config'
import type { LLMProvider, CustomProvider, OCRVersion } from '@/types/config'

const { form, saving, testingLLM, save, testLLM } = useModelSettings()

const builtInProviders = ref<LLMProvider[]>([])
const customProviders = ref<CustomProvider[]>([])
const ocrVersion = ref<OCRVersion | null>(null)
const showVersion = ref(false)
const loadingVersion = ref(false)

onMounted(async () => {
  try {
    const [builtIn, custom] = await Promise.all([
      listLLMProviders(),
      listCustomProviders(),
    ])
    builtInProviders.value = builtIn
    customProviders.value = custom
  } catch {
    // fallback to empty
  }
})

const providerUrlMap: Record<string, string> = {
  dashscope: 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions',
  deepseek: 'https://api.deepseek.com/v1/chat/completions',
  kimi: 'https://api.moonshot.cn/v1/chat/completions',
  mimo: 'https://api.xiaomimimo.com/v1/chat/completions',
  minimax: 'https://api.minimax.chat/v1/text/chatcompletion_v2',
  openai: 'https://api.openai.com/v1/chat/completions',
  volcengine: 'https://ark.cn-beijing.volces.com/api/v3/chat/completions',
  'z-ai': 'https://open.bigmodel.cn/api/paas/v4/chat/completions',
  anthropic: 'https://api.anthropic.com/v1/messages',
}

function handleProviderChange(provider: string) {
  if (provider === 'custom') return

  if (provider.startsWith('custom:')) {
    const name = provider.slice(7)
    const cp = customProviders.value.find((p) => p.name === name)
    if (cp) {
      form.url = cp.url || ''
      form.model = cp.model || ''
      form.use_anthropic = cp.protocol === 'anthropic'
    }
    return
  }

  const bp = builtInProviders.value.find((p) => p.name === provider)
  if (bp) {
    form.url = bp.base_url || providerUrlMap[provider] || ''
    form.model = bp.default_model?.[0] || ''
    form.use_anthropic = bp.protocol === 'anthropic'
  }
}
</script>
