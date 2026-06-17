<template>
  <el-form :model="form" label-width="120px" :disabled="saving">
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

    <el-form-item>
      <el-button type="primary" @click="save" :loading="saving">保存配置</el-button>
      <el-button @click="testLLM" :loading="testingLLM">测试 LLM 连接</el-button>
    </el-form-item>
  </el-form>
</template>

<script setup lang="ts">
import { useModelSettings } from './context'

const { form, saving, testingLLM, save, testLLM } = useModelSettings()

const presets: Record<string, { url: string; model: string }> = {
  qwen: { url: 'https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions', model: 'qwen-plus' },
  zhipu: { url: 'https://open.bigmodel.cn/api/paas/v4/chat/completions', model: 'glm-4' },
  minimax: { url: 'https://api.minimax.chat/v1/text/chatcompletion_v2', model: 'abab6.5-chat' },
  mimo: { url: 'https://token-plan-cn.xiaomimimo.com/v1/chat/completions', model: 'mimo-v2.5-pro' },
  deepseek: { url: 'https://api.deepseek.com/v1/chat/completions', model: 'deepseek-coder' },
  ollama: { url: 'http://127.0.0.1:11434/v1/chat/completions', model: 'qwen2.5-coder:7b' },
}

function handleProviderChange(provider: string) {
  const preset = presets[provider]
  if (preset) {
    form.url = preset.url
    form.model = preset.model
  }
}
</script>
