<template>
  <div class="config-summary">
    <div class="summary-section">
      <h4>
        <el-icon><Connection /></el-icon>
        服务商信息
      </h4>
      <div class="summary-grid">
        <div class="summary-item">
          <span class="label">服务商</span>
          <span class="value">{{ providerName || '自定义配置' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">接口地址</span>
          <span class="value url">{{ config.url || '-' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">请求协议</span>
          <span class="value">{{ config.use_anthropic ? 'Anthropic' : 'OpenAI 兼容' }}</span>
        </div>
      </div>
    </div>

    <div class="summary-section">
      <h4>
        <el-icon><Cpu /></el-icon>
        模型配置
      </h4>
      <div class="summary-grid">
        <div class="summary-item">
          <span class="label">模型</span>
          <span class="value">{{ config.model || '-' }}</span>
        </div>
        <div class="summary-item">
          <span class="label">推理模式</span>
          <span class="value">
            <el-tag :type="reasoningTagType" size="small">{{ reasoningLabel }}</el-tag>
          </span>
        </div>
        <div class="summary-item">
          <span class="label">输出语言</span>
          <span class="value">{{ config.language === 'Chinese' ? '中文' : 'English' }}</span>
        </div>
      </div>
    </div>

    <div class="summary-section">
      <h4>
        <el-icon><Setting /></el-icon>
        执行参数
      </h4>
      <div class="summary-grid">
        <div class="summary-item">
          <span class="label">模型并发</span>
          <span class="value">{{ config.concurrency }} 个</span>
        </div>
        <div class="summary-item">
          <span class="label">Git 并发</span>
          <span class="value">{{ config.max_git_procs }} 个</span>
        </div>
        <div class="summary-item">
          <span class="label">超时时间</span>
          <span class="value">{{ config.timeout_minutes }} 分钟</span>
        </div>
        <div class="summary-item">
          <span class="label">输出受众</span>
          <span class="value">{{ config.audience === 'human' ? '人工可读' : 'Agent 精简' }}</span>
        </div>
      </div>
    </div>

    <div v-if="config.background" class="summary-section">
      <h4>
        <el-icon><Document /></el-icon>
        审查背景
      </h4>
      <div class="background-preview">
        {{ config.background }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Connection, Cpu, Setting, Document } from '@element-plus/icons-vue'
import type { OCRSettings } from '@/types/config'

const props = defineProps<{
  config: OCRSettings
  providerName?: string
}>()

const reasoningLabel = computed(() => {
  const mode = props.config.reasoning_mode
  switch (mode) {
    case 'disabled': return '禁用'
    case 'light': return '轻量 (2K)'
    case 'medium': return '中等 (8K)'
    case 'full': return '充分 (32K)'
    case 'custom': return `自定义 (${props.config.reasoning_budget || 0})`
    default: return '未知'
  }
})

const reasoningTagType = computed(() => {
  const mode = props.config.reasoning_mode
  if (mode === 'disabled') return 'info'
  if (mode === 'light') return 'warning'
  return 'success'
})
</script>

<style scoped>
.config-summary {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.summary-section {
  background: var(--app-bg);
  border-radius: 12px;
  padding: 20px;
}

.summary-section h4 {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 16px;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
}

.summary-section h4 .el-icon {
  color: var(--app-primary);
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 12px;
}

.summary-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.summary-item .label {
  font-size: 12px;
  color: var(--app-muted);
}

.summary-item .value {
  font-size: 14px;
  font-weight: 500;
  color: var(--app-text);
  word-break: break-all;
}

.summary-item .value.url {
  font-family: monospace;
  font-size: 12px;
  color: var(--app-primary);
}

.background-preview {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 12px;
  font-size: 13px;
  line-height: 1.6;
  color: var(--app-text);
  white-space: pre-wrap;
}

@media (max-width: 640px) {
  .summary-grid {
    grid-template-columns: 1fr;
  }
}
</style>
