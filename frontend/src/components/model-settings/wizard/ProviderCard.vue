<template>
  <div
    class="provider-card"
    :class="{ active: selected, custom: isCustom }"
    @click="$emit('select')"
  >
    <div class="provider-icon" :class="provider.icon">
      <span v-if="isCustom">+</span>
      <span v-else>{{ provider.icon.charAt(0).toUpperCase() }}</span>
    </div>
    <div class="provider-info">
      <strong>{{ isCustom ? '自定义配置' : provider.display_name }}</strong>
      <span class="provider-meta">
        <template v-if="isCustom">手动输入服务商信息</template>
        <template v-else>
          {{ provider.models.length }} 个模型
          <span v-if="provider.supports_thinking" class="thinking-badge">推理</span>
        </template>
      </span>
    </div>
    <div v-if="selected" class="check-mark">
      <el-icon><Check /></el-icon>
    </div>
  </div>
</template>

<script setup lang="ts">
import { Check } from '@element-plus/icons-vue'

interface ModelOption {
  id: string
  display_name: string
  max_tokens: number
  supports_thinking: boolean
}

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

defineProps<{
  provider: ProviderPreset
  selected: boolean
  isCustom?: boolean
}>()

defineEmits<{
  select: []
}>()
</script>

<style scoped>
.provider-card {
  display: flex;
  align-items: center;
  gap: 14px;
  padding: 16px 18px;
  background: var(--app-surface);
  border: 2px solid var(--app-border);
  border-radius: 14px;
  cursor: pointer;
  transition: all 0.2s ease;
  position: relative;
}

.provider-card:hover {
  border-color: var(--app-primary);
  transform: translateY(-2px);
  box-shadow: var(--app-shadow);
}

.provider-card.active {
  border-color: var(--app-primary);
  background: linear-gradient(135deg, rgba(91, 91, 214, 0.05), rgba(91, 91, 214, 0.02));
}

.provider-card.custom {
  border-style: dashed;
}

.provider-icon {
  width: 48px;
  height: 48px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 700;
  color: white;
  flex-shrink: 0;
  background: var(--app-border);
}

.provider-icon.zhipu { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.provider-icon.volcengine { background: linear-gradient(135deg, #f59e0b, #d97706); }
.provider-icon.mimo { background: linear-gradient(135deg, #10b981, #059669); }
.provider-icon.minimax { background: linear-gradient(135deg, #ec4899, #db2777); }
.provider-icon.kimi { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.provider-icon.openai { background: linear-gradient(135deg, #10b981, #059669); }
.provider-icon.anthropic { background: linear-gradient(135deg, #d97706, #b45309); }
.provider-icon.deepseek { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.provider-icon.dashscope { background: linear-gradient(135deg, #6366f1, #4f46e5); }

.provider-info {
  flex: 1;
  min-width: 0;
}

.provider-info strong {
  display: block;
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
  margin-bottom: 4px;
}

.provider-meta {
  font-size: 12px;
  color: var(--app-muted);
  display: flex;
  align-items: center;
  gap: 6px;
}

.thinking-badge {
  background: linear-gradient(135deg, #8b5cf6, #7c3aed);
  color: white;
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: 600;
}

.check-mark {
  position: absolute;
  top: 12px;
  right: 12px;
  width: 24px;
  height: 24px;
  background: var(--app-primary);
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-size: 14px;
}
</style>
