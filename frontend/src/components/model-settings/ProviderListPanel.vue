<template>
  <el-card class="panel-card provider-list-card">
    <template #header>
      <div class="card-header">
        <div>
          <strong>模型供应商</strong>
          <span>选择服务商配置连接</span>
        </div>
        <el-button text @click="refresh" :loading="loading">
          <el-icon><Refresh /></el-icon>
        </el-button>
      </div>
    </template>

    <el-input v-model="keyword" class="provider-search" clearable placeholder="搜索供应商">
      <template #prefix><el-icon><Search /></el-icon></template>
    </el-input>

    <div class="provider-section" v-loading="loading">
      <div class="section-label">内置供应商</div>
      <el-empty v-if="!loading && filteredBuiltIn.length === 0" description="无内置供应商" :image-size="40" />
      <button
        v-for="p in filteredBuiltIn"
        :key="p.name"
        class="provider-item"
        :class="{ active: isActive(p.name, 'builtin') }"
        type="button"
        @click="selectProvider(p, 'builtin')"
      >
        <span class="provider-icon builtin">
          <el-icon><Connection /></el-icon>
        </span>
        <span class="provider-info">
          <strong>{{ p.name }}</strong>
          <span class="provider-meta">{{ p.protocol }} / {{ p.default_model?.length || 0 }} 模型</span>
        </span>
        <el-tag v-if="isActive(p.name, 'builtin')" type="success" size="small" effect="dark">当前</el-tag>
      </button>
    </div>

    <el-divider />

    <div class="provider-section">
      <div class="section-header">
        <span class="section-label">自定义供应商</span>
        <el-button text type="primary" size="small" @click="$emit('create')">
          <el-icon><Plus /></el-icon> 添加
        </el-button>
      </div>
      <el-empty v-if="!loading && filteredCustom.length === 0" description="暂无自定义供应商" :image-size="40" />
      <button
        v-for="p in filteredCustom"
        :key="'custom-' + p.id"
        class="provider-item"
        :class="{ active: isActive(p.name, 'custom'), disabled: !p.enabled }"
        type="button"
        @click="selectProvider(p, 'custom')"
      >
        <span class="provider-icon custom">
          <el-icon><Setting /></el-icon>
        </span>
        <span class="provider-info">
          <strong>{{ p.name }}</strong>
          <span class="provider-meta">{{ p.protocol }} / {{ p.model || '未设置模型' }}</span>
        </span>
        <el-tag v-if="isActive(p.name, 'custom')" type="success" size="small" effect="dark">当前</el-tag>
        <el-tag v-else-if="!p.enabled" type="info" size="small">禁用</el-tag>
      </button>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Refresh, Search, Plus, Connection, Setting } from '@element-plus/icons-vue'
import { listLLMProviders, listCustomProviders } from '@/api/config'
import type { LLMProvider, CustomProvider } from '@/types/config'

const props = defineProps<{
  selectedName: string
  selectedType: 'builtin' | 'custom' | null
}>()

const emit = defineEmits<{
  (e: 'select', provider: LLMProvider | CustomProvider, type: 'builtin' | 'custom'): void
  (e: 'create'): void
}>()

const loading = ref(false)
const keyword = ref('')
const builtInProviders = ref<LLMProvider[]>([])
const customProviders = ref<CustomProvider[]>([])

const filteredBuiltIn = computed(() => {
  const kw = keyword.value.toLowerCase()
  if (!kw) return builtInProviders.value
  return builtInProviders.value.filter(p => p.name.toLowerCase().includes(kw))
})

const filteredCustom = computed(() => {
  const kw = keyword.value.toLowerCase()
  if (!kw) return customProviders.value
  return customProviders.value.filter(p => p.name.toLowerCase().includes(kw))
})

function isActive(name: string, type: 'builtin' | 'custom') {
  return props.selectedName === name && props.selectedType === type
}

function selectProvider(provider: LLMProvider | CustomProvider, type: 'builtin' | 'custom') {
  emit('select', provider, type)
}

async function refresh() {
  loading.value = true
  try {
    const [builtIn, custom] = await Promise.all([listLLMProviders(), listCustomProviders()])
    builtInProviders.value = builtIn
    customProviders.value = custom
  } catch {
    // fallback
  } finally {
    loading.value = false
  }
}

onMounted(refresh)

defineExpose({ refresh })
</script>

<style scoped>
.provider-list-card {
  height: 100%;
}

.provider-list-card :deep(.el-card__body) {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow-y: auto;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header strong {
  font-size: 14px;
  color: var(--app-text);
}

.card-header span {
  display: block;
  font-size: 12px;
  color: var(--app-muted);
  margin-top: 2px;
}

.provider-search {
  margin-bottom: 12px;
}

.provider-search :deep(.el-input__wrapper) {
  border-radius: 8px;
}

.section-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--app-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 8px;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.section-header .section-label {
  margin-bottom: 0;
}

.provider-section {
  min-height: 0;
}

.provider-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  padding: 10px 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  cursor: pointer;
  transition: all 0.15s ease;
  text-align: left;
}

.provider-item:hover {
  background: var(--app-bg-hover, rgba(0, 0, 0, 0.04));
}

.provider-item.active {
  background: var(--app-primary-bg, rgba(59, 130, 246, 0.08));
  border-color: var(--app-primary, #3b82f6);
}

.provider-item.disabled {
  opacity: 0.5;
}

.provider-icon {
  width: 32px;
  height: 32px;
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

.provider-info {
  flex: 1;
  min-width: 0;
}

.provider-info strong {
  display: block;
  font-size: 13px;
  font-weight: 500;
  color: var(--app-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.provider-meta {
  font-size: 11px;
  color: var(--app-muted);
}

:deep(.el-divider) {
  margin: 12px 0;
}
</style>
