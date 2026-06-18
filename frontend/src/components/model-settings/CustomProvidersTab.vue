<template>
  <div>
    <div style="display: flex; justify-content: flex-end; margin-bottom: 12px;">
      <el-button type="primary" size="small" @click="showCreate = true">
        <el-icon><Plus /></el-icon>
        添加 Provider
      </el-button>
    </div>

    <el-table :data="providers" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" width="140" />
      <el-table-column prop="protocol" label="协议" width="100" />
      <el-table-column prop="url" label="接口地址" min-width="200" show-overflow-tooltip />
      <el-table-column prop="model" label="模型" width="140" />
      <el-table-column label="状态" width="80">
        <template #default="{ row }">
          <el-tag :type="row.enabled ? 'success' : 'info'" size="small">
            {{ row.enabled ? '启用' : '禁用' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="140">
        <template #default="{ row }">
          <el-button link type="primary" size="small" @click="handleEdit(row)">编辑</el-button>
          <el-popconfirm title="确认删除？" @confirm="handleDelete(row.id)">
            <template #reference>
              <el-button link type="danger" size="small">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <el-dialog
      v-model="showCreate"
      :title="editingId ? '编辑 Provider' : '添加 Provider'"
      width="500px"
      @closed="resetForm"
    >
      <el-form :model="cpForm" label-width="100px">
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
          <el-input v-model="cpForm.api_key" type="password" placeholder="留空则不设置" show-password />
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
        <el-form-item label="启用">
          <el-switch v-model="cpForm.enabled" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" @click="handleSave" :loading="saving">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Plus } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { listCustomProviders, createCustomProvider, updateCustomProvider, deleteCustomProvider } from '@/api/config'
import type { CustomProvider } from '@/types/config'

const providers = ref<CustomProvider[]>([])
const loading = ref(false)
const saving = ref(false)
const showCreate = ref(false)
const editingId = ref<number | null>(null)

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

async function fetchProviders() {
  loading.value = true
  try {
    providers.value = await listCustomProviders()
  } finally {
    loading.value = false
  }
}

function handleEdit(row: CustomProvider) {
  editingId.value = row.id
  cpForm.value = {
    name: row.name,
    protocol: row.protocol,
    url: row.url || '',
    api_key: '',
    model: row.model || '',
    auth_header: row.auth_header || '',
    extra_body: row.extra_body || '',
    enabled: row.enabled,
  }
  showCreate.value = true
}

function resetForm() {
  editingId.value = null
  cpForm.value = {
    name: '',
    protocol: 'openai',
    url: '',
    api_key: '',
    model: '',
    auth_header: '',
    extra_body: '',
    enabled: true,
  }
}

async function handleSave() {
  if (!cpForm.value.name.trim()) {
    ElMessage.warning('名称不能为空')
    return
  }
  saving.value = true
  try {
    if (editingId.value) {
      await updateCustomProvider(editingId.value, cpForm.value)
      ElMessage.success('更新成功')
    } else {
      await createCustomProvider(cpForm.value)
      ElMessage.success('创建成功')
    }
    showCreate.value = false
    await fetchProviders()
  } catch (e: any) {
    ElMessage.error(e.message || '操作失败')
  } finally {
    saving.value = false
  }
}

async function handleDelete(id: number) {
  try {
    await deleteCustomProvider(id)
    ElMessage.success('已删除')
    await fetchProviders()
  } catch (e: any) {
    ElMessage.error(e.message || '删除失败')
  }
}

onMounted(fetchProviders)
</script>
