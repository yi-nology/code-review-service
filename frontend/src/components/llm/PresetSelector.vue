<template>
  <div class="preset-selector">
    <div v-for="category in categories" :key="category.id" class="preset-category">
      <div class="category-header">
        <span class="category-icon">{{ category.icon }}</span>
        <span class="category-name">{{ category.name }}</span>
      </div>
      <div class="preset-grid">
        <button
          v-for="preset in category.presets"
          :key="preset.id"
          class="preset-card"
          :class="{ active: selectedPreset?.id === preset.id }"
          @click="selectPreset(preset)"
        >
          <div class="preset-icon" :class="preset.icon">
            <span>{{ preset.icon.charAt(0).toUpperCase() }}</span>
          </div>
          <div class="preset-info">
            <strong>{{ preset.display_name }}</strong>
            <span class="preset-meta">
              {{ preset.models.length }} 模型
              <span v-if="preset.supports_thinking" class="thinking-badge">Plan</span>
            </span>
          </div>
        </button>
      </div>
    </div>

    <div class="preset-category">
      <div class="category-header">
        <span class="category-icon">⚙️</span>
        <span class="category-name">自定义配置</span>
      </div>
      <div class="preset-grid">
        <button
          class="preset-card custom"
          :class="{ active: selectedPreset === null && isCustomMode }"
          @click="selectCustom"
        >
          <div class="preset-icon custom">
            <span>+</span>
          </div>
          <div class="preset-info">
            <strong>自定义 Provider</strong>
            <span class="preset-meta">手动配置</span>
          </div>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { ProviderPreset } from '@/types/config'

const props = defineProps<{
  presets: ProviderPreset[]
  selectedPreset: ProviderPreset | null
}>()

const emit = defineEmits<{
  (e: 'select', preset: ProviderPreset | null): void
}>()

const isCustomMode = ref(false)

const categories = computed(() => {
  const grouped = {
    coding_plan: { id: 'coding_plan', name: 'Coding Plan (包月套餐)', icon: '📦', presets: [] as ProviderPreset[] },
    direct: { id: 'direct', name: '直接调用', icon: '🔗', presets: [] as ProviderPreset[] },
    local: { id: 'local', name: '本地部署', icon: '💻', presets: [] as ProviderPreset[] },
  }

  for (const preset of props.presets) {
    if (grouped[preset.category as keyof typeof grouped]) {
      grouped[preset.category as keyof typeof grouped].presets.push(preset)
    }
  }

  return Object.values(grouped).filter(c => c.presets.length > 0)
})

function selectPreset(preset: ProviderPreset) {
  isCustomMode.value = false
  emit('select', preset)
}

function selectCustom() {
  isCustomMode.value = true
  emit('select', null)
}
</script>

<style scoped>
.preset-selector {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.preset-category {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.category-header {
  display: flex;
  align-items: center;
  gap: 8px;
}

.category-icon {
  font-size: 16px;
}

.category-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
}

.preset-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}

.preset-card {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border: 1px solid var(--app-border, #e5e7eb);
  border-radius: 12px;
  background: var(--app-bg, #fff);
  cursor: pointer;
  transition: all 0.2s ease;
  text-align: left;
}

.preset-card:hover {
  border-color: var(--app-primary, #3b82f6);
  background: var(--app-bg-hover, #f9fafb);
}

.preset-card.active {
  border-color: var(--app-primary, #3b82f6);
  background: var(--app-primary-bg, #eff6ff);
}

.preset-card.custom {
  border-style: dashed;
}

.preset-icon {
  width: 40px;
  height: 40px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 16px;
  color: white;
  flex-shrink: 0;
}

.preset-icon.zhipu { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.preset-icon.volcengine { background: linear-gradient(135deg, #f59e0b, #d97706); }
.preset-icon.mimo { background: linear-gradient(135deg, #10b981, #059669); }
.preset-icon.minimax { background: linear-gradient(135deg, #ec4899, #db2777); }
.preset-icon.kimi { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.preset-icon.openai { background: linear-gradient(135deg, #10b981, #059669); }
.preset-icon.anthropic { background: linear-gradient(135deg, #d97706, #b45309); }
.preset-icon.deepseek { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.preset-icon.dashscope { background: linear-gradient(135deg, #6366f1, #4f46e5); }
.preset-icon.ollama { background: linear-gradient(135deg, #6b7280, #4b5563); }
.preset-icon.custom { background: var(--app-border, #e5e7eb); color: var(--app-text); }

.preset-info {
  flex: 1;
  min-width: 0;
}

.preset-info strong {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.preset-meta {
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
</style>
