<script setup lang="ts">
/**
 * ProviderPresetLibrary — 模型预设库管理。
 * 激活后端 custom_providers 表：新增/编辑/删除命名模型配置，
 * 供 A/B 对比下拉引用（后端按 id 解析真实凭据，前端不接触 key）。
 */
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, Edit, Delete, Refresh } from '@element-plus/icons-vue'
import {
  listCustomProviders,
  createCustomProvider,
  updateCustomProvider,
  deleteCustomProvider,
} from '@/api/config'
import type { CustomProvider } from '@/types/config'
import PresetFormFields from './PresetFormFields.vue'

type PresetForm = {
  name: string
  protocol: string
  url: string
  api_key: string
  model: string
  auth_header: string
  extra_body: string
  enabled: boolean
}

const presets = ref<CustomProvider[]>([])
const loading = ref(false)

// 行内编辑/新建态：creating=true 新建行；editingId>0 编辑该行
const creating = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)

const emptyForm = (): PresetForm => ({
  name: '',
  protocol: 'openai',
  url: '',
  api_key: '',
  model: '',
  auth_header: '',
  extra_body: '',
  enabled: true,
})
const form = ref<PresetForm>(emptyForm())

async function fetchPresets() {
  loading.value = true
  try {
    presets.value = await listCustomProviders()
  } finally {
    loading.value = false
  }
}

function startCreate() {
  creating.value = true
  editingId.value = null
  form.value = emptyForm()
}

function startEdit(preset: CustomProvider) {
  editingId.value = preset.id
  creating.value = false
  form.value = {
    name: preset.name,
    protocol: preset.protocol,
    url: preset.url || '',
    api_key: '', // 后端屏蔽为 ***，回填无意义；留空表示不改
    model: preset.model || '',
    auth_header: preset.auth_header || '',
    extra_body: preset.extra_body || '',
    enabled: preset.enabled,
  }
}

function cancelEdit() {
  creating.value = false
  editingId.value = null
}

function protocolLabel(p: string) {
  return p === 'anthropic' ? 'Anthropic' : 'OpenAI 兼容'
}

async function save() {
  if (!form.value.name.trim()) {
    ElMessage.warning('预设名称不能为空')
    return
  }
  if (!form.value.protocol) {
    ElMessage.warning('请选择协议')
    return
  }
  saving.value = true
  try {
    const payload = {
      name: form.value.name.trim(),
      protocol: form.value.protocol,
      url: form.value.url || undefined,
      api_key: form.value.api_key || undefined,
      model: form.value.model || undefined,
      auth_header: form.value.auth_header || undefined,
      extra_body: form.value.extra_body || undefined,
      enabled: form.value.enabled,
    }
    if (creating.value) {
      await createCustomProvider(payload)
      ElMessage.success('已创建预设')
    } else if (editingId.value !== null) {
      await updateCustomProvider(editingId.value, payload)
      ElMessage.success('已更新预设')
    }
    creating.value = false
    editingId.value = null
    await fetchPresets()
  } finally {
    saving.value = false
  }
}

async function remove(preset: CustomProvider) {
  try {
    await ElMessageBox.confirm(`确定删除预设「${preset.name}」？`, '删除确认', {
      type: 'warning',
      confirmButtonText: '删除',
      cancelButtonText: '取消',
    })
  } catch {
    return
  }
  try {
    await deleteCustomProvider(preset.id)
    ElMessage.success('已删除预设')
    await fetchPresets()
  } catch {
    // 全局拦截器会提示错误
  }
}

function initials(name: string) {
  return name?.trim() ? name.trim().charAt(0).toUpperCase() : '?'
}

onMounted(fetchPresets)
</script>

<template>
  <section class="preset-library" v-loading="loading">
    <div class="library-head">
      <div>
        <h3>模型预设库</h3>
        <p>保存多套模型配置，供 A/B 对比快速引用。凭据由后端安全解析，不会暴露到前端。</p>
      </div>
      <div class="library-actions">
        <el-button size="small" plain :icon="Refresh" :loading="loading" @click="fetchPresets">刷新</el-button>
        <el-button size="small" type="primary" plain :icon="Plus" @click="startCreate">新建预设</el-button>
      </div>
    </div>

    <!-- 新建表单 -->
    <div v-if="creating" class="preset-form-card">
      <div class="form-title">新建预设</div>
      <PresetFormFields v-model="form" />
      <div class="form-actions">
        <el-button size="small" @click="cancelEdit">取消</el-button>
        <el-button size="small" type="primary" :loading="saving" @click="save">创建</el-button>
      </div>
    </div>

    <!-- 列表 -->
    <div v-if="presets.length" class="preset-list">
      <div v-for="preset in presets" :key="preset.id" class="preset-row">
        <template v-if="editingId === preset.id">
          <div class="form-title">编辑「{{ preset.name }}」</div>
          <PresetFormFields v-model="form" is-edit />
          <div class="form-actions">
            <el-button size="small" @click="cancelEdit">取消</el-button>
            <el-button size="small" type="primary" :loading="saving" @click="save">保存</el-button>
          </div>
        </template>
        <template v-else>
          <div class="preset-badge">{{ initials(preset.name) }}</div>
          <div class="preset-info">
            <div class="info-top">
              <strong>{{ preset.name }}</strong>
              <el-tag size="small" effect="plain" :type="preset.protocol === 'anthropic' ? 'warning' : 'info'">
                {{ protocolLabel(preset.protocol) }}
              </el-tag>
              <el-tag v-if="!preset.enabled" size="small" type="info">未启用</el-tag>
            </div>
            <div class="info-meta">
              <span class="mono">{{ preset.model || '未设置模型' }}</span>
              <span v-if="preset.url" class="sep">·</span>
              <span v-if="preset.url" class="mono url">{{ preset.url }}</span>
            </div>
          </div>
          <div class="preset-ops">
            <el-button size="small" plain :icon="Edit" @click="startEdit(preset)">编辑</el-button>
            <el-button size="small" plain :icon="Delete" @click="remove(preset)">删除</el-button>
          </div>
        </template>
      </div>
    </div>

    <el-empty v-else-if="!creating" description="还没有预设，点击「新建预设」添加" :image-size="72" />
  </section>
</template>

<style scoped>
.preset-library {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 16px;
  padding: 22px 24px;
  box-shadow: var(--app-shadow);
}

.library-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}
.library-head h3 {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 600;
  color: var(--app-text);
}
.library-head p {
  margin: 0;
  font-size: 12px;
  color: var(--app-muted);
  line-height: 1.5;
  max-width: 540px;
}
.library-actions {
  display: flex;
  gap: 8px;
  flex-shrink: 0;
}

/* 新建/编辑表单卡 */
.preset-form-card,
.preset-row {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  padding: 14px;
}
.preset-form-card {
  margin-bottom: 12px;
  background: #fbfcfe;
}
.form-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
  margin-bottom: 10px;
}
.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 10px;
}

/* 列表行 */
.preset-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}
.preset-row {
  display: flex;
  align-items: center;
  gap: 12px;
  background: #fff;
  flex-wrap: wrap;
}
.preset-badge {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 15px;
  color: #fff;
  flex-shrink: 0;
  background: linear-gradient(135deg, #6366f1, #4f46e5);
}
.preset-info {
  flex: 1;
  min-width: 180px;
}
.info-top {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 3px;
}
.info-top strong {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
}
.info-meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--app-muted);
  min-width: 0;
}
.info-meta .mono {
  font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
}
.info-meta .url {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.info-meta .sep {
  color: #c4ccd9;
}
.preset-ops {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}

@media (max-width: 720px) {
  .preset-row {
    align-items: flex-start;
  }
  .preset-ops {
    width: 100%;
  }
}
</style>
