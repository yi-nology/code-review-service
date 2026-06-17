<template>
  <AppLayout>
    <div class="platform-settings-page">
      <header class="page-header">
        <div class="eyebrow">INTEGRATIONS</div>
        <h1>Git 平台配置</h1>
        <p>管理代码平台的连接信息，用于访问私有仓库和发布审查评论。修改后立即生效，无需重启服务。</p>
      </header>

      <el-row :gutter="24">
        <el-col :xs="24" :xl="12">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>{{ editing ? '编辑平台' : '添加平台' }}</span>
                <el-button v-if="editing" @click="resetForm" size="small">取消编辑</el-button>
              </div>
            </template>
            <el-form :model="form" label-width="100px">
              <el-form-item label="代码平台" required>
                <el-select v-model="form.provider" :disabled="editing" style="width: 100%">
                  <el-option label="GitHub" value="github" />
                  <el-option label="GitLab" value="gitlab" />
                  <el-option label="Gitea" value="gitea" />
                  <el-option label="GitCode" value="gitcode" />
                  <el-option label="Gitee" value="gitee" />
                  <el-option label="Tencent Code" value="tencent_code" />
                </el-select>
              </el-form-item>
              <el-form-item label="API 地址" required>
                <el-input v-model="form.base_url" placeholder="https://api.github.com" />
              </el-form-item>
              <el-form-item :label="editing ? 'Token (留空不修改)' : 'Token'" :required="!editing">
                <el-input
                  v-model="form.token"
                  type="password"
                  show-password
                  :placeholder="editing ? '留空则保留原 Token' : 'ghp_xxx 或 glpat-xxx'"
                />
              </el-form-item>
              <el-form-item label="启用">
                <el-switch v-model="form.enabled" />
              </el-form-item>
              <el-form-item>
                <el-button type="primary" @click="handleSave" :loading="saving">
                  {{ editing ? '更新配置' : '添加平台' }}
                </el-button>
                <el-button @click="handleTest" :loading="testing" :disabled="!form.provider || !form.token">
                  测试连接
                </el-button>
              </el-form-item>
            </el-form>

            <div v-if="testResult" class="test-result" :class="testResult.connected ? 'success' : 'fail'">
              <el-tag :type="testResult.connected ? 'success' : 'danger'" size="small">
                {{ testResult.connected ? '连接成功' : '连接失败' }}
              </el-tag>
              <span v-if="testResult.user_name">用户: {{ testResult.user_name }}</span>
              <span v-if="testResult.message">{{ testResult.message }}</span>
            </div>
          </el-card>
        </el-col>

        <el-col :xs="24" :xl="12">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>已配置平台</span>
                <el-button @click="fetchPlatforms" size="small">刷新</el-button>
              </div>
            </template>
            <div v-loading="loading">
              <div v-if="platforms.length === 0" class="empty-state">
                暂无配置的 Git 平台
              </div>
              <div v-else class="platform-list">
                <div
                  v-for="p in platforms"
                  :key="p.provider"
                  class="platform-item"
                  :class="{ disabled: !p.enabled }"
                  @click="handleEdit(p)"
                >
                  <div class="platform-header">
                    <el-tag :type="p.enabled ? 'success' : 'info'" size="small">
                      {{ p.enabled ? '启用' : '禁用' }}
                    </el-tag>
                    <span class="platform-name">{{ getProviderLabel(p.provider) }}</span>
                  </div>
                  <div class="platform-meta">
                    {{ p.base_url }}<br>
                    Token: {{ p.token }}
                  </div>
                  <el-button
                    type="danger"
                    size="small"
                    class="delete-btn"
                    @click.stop="handleDelete(p)"
                  >
                    删除
                  </el-button>
                </div>
              </div>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getGitPlatforms, saveGitPlatform, deleteGitPlatform, testGitPlatform } from '@/api/config'
import { getProviderLabel } from '@/utils/url-parser'
import type { GitPlatform, GitTestResult } from '@/types/config'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const editing = ref(false)
const platforms = ref<GitPlatform[]>([])
const testResult = ref<GitTestResult | null>(null)

const form = reactive<GitPlatform>({
  provider: 'github',
  base_url: '',
  token: '',
  enabled: true,
})

async function fetchPlatforms() {
  loading.value = true
  try {
    const res = await getGitPlatforms()
    platforms.value = res.items || []
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (!form.provider || !form.base_url) {
    ElMessage.warning('请填写平台和 API 地址')
    return
  }
  if (!editing.value && !form.token) {
    ElMessage.warning('请填写 Token')
    return
  }

  saving.value = true
  try {
    await saveGitPlatform({ ...form })
    ElMessage.success(editing.value ? '平台配置已更新' : '平台已添加')
    resetForm()
    fetchPlatforms()
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  testing.value = true
  testResult.value = null
  try {
    const result = await testGitPlatform({
      provider: form.provider,
      base_url: form.base_url,
      token: form.token,
    })
    testResult.value = result
    if (result.connected) {
      ElMessage.success(`连接成功${result.user_name ? ': ' + result.user_name : ''}`)
    }
  } catch {
    testResult.value = { connected: false, platform: form.provider, user_name: '', message: '请求失败' }
  } finally {
    testing.value = false
  }
}

function handleEdit(p: GitPlatform) {
  editing.value = true
  form.provider = p.provider
  form.base_url = p.base_url
  form.token = ''
  form.enabled = p.enabled
  testResult.value = null
}

function resetForm() {
  editing.value = false
  form.provider = 'github'
  form.base_url = ''
  form.token = ''
  form.enabled = true
  testResult.value = null
}

async function handleDelete(p: GitPlatform) {
  await ElMessageBox.confirm(`确定删除 ${getProviderLabel(p.provider)} 平台配置？`, '确认')
  try {
    await deleteGitPlatform(p.provider)
    ElMessage.success('已删除')
    fetchPlatforms()
  } catch {
    // 用户取消
  }
}

onMounted(fetchPlatforms)
</script>

<style scoped>
.platform-settings-page {
  width: min(1320px, 100%);
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

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.empty-state {
  text-align: center;
  color: #999;
  padding: 40px 0;
}

.platform-item {
  padding: 12px;
  border: 1px solid #eee;
  border-radius: 8px;
  margin-bottom: 8px;
  cursor: pointer;
  position: relative;
}

.platform-item:hover {
  background: #f5f5f5;
}

.platform-item.disabled {
  opacity: 0.6;
}

.platform-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.platform-name {
  font-weight: bold;
  font-size: 16px;
}

.platform-meta {
  font-size: 12px;
  color: #999;
  font-family: monospace;
}

.delete-btn {
  position: absolute;
  right: 12px;
  top: 12px;
}

.test-result {
  margin-top: 16px;
  padding: 12px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.test-result.success {
  background: #f0f9eb;
}

@media (max-width: 1199px) {
  .platform-settings-page :deep(.el-col + .el-col) {
    margin-top: 16px;
  }
}

@media (max-width: 720px) {
  .platform-settings-page {
    padding: 22px 14px 36px;
  }

  .page-header h1 {
    font-size: 24px;
  }
}

.test-result.fail {
  background: #fef0f0;
}
</style>
