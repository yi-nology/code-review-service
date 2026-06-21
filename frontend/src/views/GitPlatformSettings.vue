<template>
  <AppLayout>
    <div class="platform-settings-page">
      <section class="page-hero">
        <div>
          <div class="eyebrow">INTEGRATIONS</div>
          <h1>Git 平台配置</h1>
          <p>集中管理代码平台凭证、API 地址和启用状态，供仓库同步、审查创建和评论写回使用。</p>
        </div>
        <div class="hero-actions">
          <el-button :icon="Refresh" :loading="loading" @click="fetchPlatforms">刷新</el-button>
          <el-button type="primary" :icon="Plus" @click="resetForm">新增平台</el-button>
        </div>
      </section>

      <section class="metric-strip">
        <div class="metric-item">
          <span>已配置</span>
          <strong>{{ platforms.length }}</strong>
        </div>
        <div class="metric-item">
          <span>启用中</span>
          <strong>{{ enabledCount }}</strong>
        </div>
        <div class="metric-item">
          <span>未启用</span>
          <strong>{{ disabledCount }}</strong>
        </div>
        <div class="metric-item">
          <span>当前编辑</span>
          <strong>{{ editing ? getProviderLabel(form.provider) : '新平台' }}</strong>
        </div>
      </section>

      <section class="workspace-grid">
        <aside class="platform-panel">
          <div class="panel-heading">
            <div>
              <h2>平台列表</h2>
              <p>{{ platforms.length ? `显示 ${filteredPlatforms.length} / ${platforms.length}` : '还没有保存的平台' }}</p>
            </div>
          </div>

          <div class="list-tools">
            <el-input v-model="keyword" clearable placeholder="搜索平台或地址">
              <template #prefix>
                <el-icon><Search /></el-icon>
              </template>
            </el-input>
            <el-segmented v-model="statusFilter" :options="statusOptions" block />
          </div>

          <div v-loading="loading" class="platform-list">
            <button
              v-for="p in filteredPlatforms"
              :key="platformKey(p)"
              type="button"
              class="platform-row"
              :class="{ active: editing && form.provider === p.provider, disabled: !p.enabled }"
              @click="handleEdit(p)"
            >
              <span class="provider-mark">{{ providerInitial(p.provider) }}</span>
              <span class="platform-main">
                <strong>{{ getProviderLabel(p.provider) }}</strong>
                <small>{{ p.base_url || '未配置 API 地址' }}</small>
              </span>
              <span class="platform-state" :class="{ enabled: p.enabled }">
                {{ p.enabled ? '启用' : '禁用' }}
              </span>
            </button>

            <div v-if="!loading && platforms.length === 0" class="empty-state">
              <el-icon><Connection /></el-icon>
              <strong>暂无平台</strong>
              <span>添加第一个 Git 平台后即可选择仓库并写回审查评论。</span>
            </div>
            <div v-else-if="!loading && filteredPlatforms.length === 0" class="empty-state compact">
              <el-icon><Search /></el-icon>
              <strong>没有匹配结果</strong>
              <span>换个关键词或状态筛选再试。</span>
            </div>
          </div>
        </aside>

        <main class="config-panel">
          <div class="panel-heading form-heading">
            <div>
              <h2>{{ editing ? '编辑平台' : '新增平台' }}</h2>
              <p>{{ editing ? 'Token 留空时会保留现有密钥' : '保存前可以先测试连接' }}</p>
            </div>
            <el-tag :type="form.enabled ? 'success' : 'info'" effect="plain">
              {{ form.enabled ? '启用' : '禁用' }}
            </el-tag>
          </div>

          <el-form :model="form" label-position="top" class="platform-form">
            <div class="form-grid">
              <el-form-item label="代码平台" required>
                <el-select v-model="form.provider" :disabled="editing" class="full-control" @change="handleProviderChange">
                  <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
                </el-select>
              </el-form-item>

              <el-form-item label="启用状态">
                <div class="switch-line">
                  <el-switch v-model="form.enabled" />
                  <span>{{ form.enabled ? '参与仓库访问和写回' : '暂不参与自动流程' }}</span>
                </div>
              </el-form-item>
            </div>

            <div class="provider-hint">
              <span class="hint-label">{{ getProviderLabel(form.provider) }}</span>
              <span>{{ providerHint }}</span>
            </div>

            <el-form-item label="API 地址" required>
              <el-input v-model="form.base_url" :placeholder="baseUrlPlaceholder" />
            </el-form-item>

            <el-form-item :label="editing ? 'Token（留空不修改）' : 'Token'" :required="!editing">
              <el-input
                v-model="form.token"
                type="password"
                show-password
                :placeholder="editing ? '留空则保留原 Token' : 'ghp_xxx 或 glpat-xxx'"
              />
            </el-form-item>

            <div v-if="editing && selectedPlatform" class="saved-credential">
              <span>已保存 Token</span>
              <strong>{{ selectedPlatform.token || '已保存' }}</strong>
            </div>

            <div class="action-bar">
              <div class="left-actions">
                <el-button type="primary" :icon="Check" :loading="saving" @click="handleSave">
                  {{ editing ? '保存修改' : '添加平台' }}
                </el-button>
                <el-button :icon="Connection" :loading="testing" :disabled="!canTest" @click="handleTest">
                  测试连接
                </el-button>
              </div>
              <div class="right-actions">
                <el-button v-if="editing" @click="resetForm">取消编辑</el-button>
                <el-button v-if="editing" type="danger" plain :icon="Delete" @click="handleDeleteSelected">
                  删除平台
                </el-button>
              </div>
            </div>
          </el-form>

          <div v-if="testResult" class="test-result" :class="testResult.connected ? 'success' : 'fail'">
            <el-icon>
              <CircleCheckFilled v-if="testResult.connected" />
              <CircleCloseFilled v-else />
            </el-icon>
            <div>
              <strong>{{ testResult.connected ? '连接成功' : '连接失败' }}</strong>
              <span>
                {{ testResult.user_name ? `用户 ${testResult.user_name}` : testResult.message || '未返回额外信息' }}
              </span>
            </div>
          </div>
        </main>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import {
  Check,
  CircleCheckFilled,
  CircleCloseFilled,
  Connection,
  Delete,
  Plus,
  Refresh,
  Search,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getGitPlatforms, saveGitPlatform, deleteGitPlatform, testGitPlatform } from '@/api/config'
import { getProviderLabel, PROVIDER_OPTIONS } from '@/utils/url-parser'
import type { GitPlatform, GitTestResult } from '@/types/config'

const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const editing = ref(false)
const keyword = ref('')
const statusFilter = ref<'all' | 'enabled' | 'disabled'>('all')
const platforms = ref<GitPlatform[]>([])
const testResult = ref<GitTestResult | null>(null)

const providerDefaults: Record<string, { baseUrl: string; hint: string }> = {
  github: { baseUrl: 'https://api.github.com', hint: '适用于 GitHub Cloud；企业版请填写对应 API 地址。' },
  gitlab: { baseUrl: 'https://gitlab.com', hint: '支持 GitLab.com 和自建 GitLab，通常填写站点根地址。' },
  gitea: { baseUrl: '', hint: '填写 Gitea 站点根地址，例如 https://git.example.com。' },
  forgejo: { baseUrl: '', hint: '填写 Forgejo 站点根地址（兼容 Gitea API）。' },
  gitcode: { baseUrl: 'https://api.gitcode.com/api/v5', hint: '适用于 GitCode API v5。' },
  gitee: { baseUrl: 'https://gitee.com', hint: '适用于 Gitee 公开云服务或兼容部署。' },
  tencent_code: { baseUrl: '', hint: '填写腾讯工蜂/代码平台服务地址。' },
}

const statusOptions = [
  { label: '全部', value: 'all' },
  { label: '启用', value: 'enabled' },
  { label: '禁用', value: 'disabled' },
] as const

const form = reactive<GitPlatform>({
  provider: 'github',
  base_url: '',
  token: '',
  enabled: true,
})

const enabledCount = computed(() => platforms.value.filter((p) => p.enabled).length)
const disabledCount = computed(() => platforms.value.length - enabledCount.value)
const canTest = computed(() => Boolean(form.provider && form.base_url && form.token))
const baseUrlPlaceholder = computed(() => providerDefaults[form.provider]?.baseUrl || 'https://git.example.com')
const providerHint = computed(() => providerDefaults[form.provider]?.hint || '填写平台 API 或站点地址。')
const selectedPlatform = computed(() => platforms.value.find((p) => platformKey(p) === platformKey(form)))
const filteredPlatforms = computed(() => {
  const query = keyword.value.trim().toLowerCase()
  return platforms.value.filter((p) => {
    const matchesStatus =
      statusFilter.value === 'all' ||
      (statusFilter.value === 'enabled' && p.enabled) ||
      (statusFilter.value === 'disabled' && !p.enabled)
    if (!matchesStatus) return false
    if (!query) return true
    return `${getProviderLabel(p.provider)} ${p.provider} ${p.base_url}`.toLowerCase().includes(query)
  })
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
    await fetchPlatforms()
  } finally {
    saving.value = false
  }
}

async function handleTest() {
  if (!canTest.value) {
    ElMessage.warning('请填写平台、API 地址和 Token')
    return
  }

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
  form.base_url = providerDefaults.github.baseUrl
  form.token = ''
  form.enabled = true
  testResult.value = null
}

function handleProviderChange(value: string) {
  const nextDefault = providerDefaults[value]?.baseUrl || ''
  if (!editing.value && (!form.base_url || Object.values(providerDefaults).some((item) => item.baseUrl === form.base_url))) {
    form.base_url = nextDefault
  }
}

async function handleDeleteSelected() {
  const platform = platforms.value.find((p) => p.provider === form.provider)
  if (platform) await handleDelete(platform)
}

async function handleDelete(p: GitPlatform) {
  try {
    await ElMessageBox.confirm(`确定删除 ${getProviderLabel(p.provider)} 平台配置？`, '确认删除')
    await deleteGitPlatform(p.provider)
    ElMessage.success('已删除')
    resetForm()
    await fetchPlatforms()
  } catch {
    // User cancelled or request failed; request failures are surfaced by the global interceptor.
  }
}

function providerInitial(provider: string) {
  return getProviderLabel(provider).slice(0, 1).toUpperCase()
}

function platformKey(platform: GitPlatform) {
  return `${platform.provider}:${platform.base_url}`
}

onMounted(() => {
  resetForm()
  fetchPlatforms()
})
</script>

<style scoped>
.platform-settings-page {
  width: min(1360px, 100%);
  margin: 0 auto;
  padding: 32px 36px 48px;
}

.page-hero {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.page-hero h1 {
  margin: 4px 0 8px;
  color: var(--app-text);
  font-size: 28px;
  letter-spacing: 0;
}

.page-hero p {
  max-width: 720px;
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
  line-height: 1.7;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}

.hero-actions,
.action-bar,
.left-actions,
.right-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 22px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.metric-item {
  min-width: 0;
  padding: 16px 18px;
  border-right: 1px solid var(--app-border);
}

.metric-item:last-child {
  border-right: 0;
}

.metric-item span {
  display: block;
  margin-bottom: 6px;
  color: var(--app-muted);
  font-size: 12px;
}

.metric-item strong {
  display: block;
  overflow: hidden;
  color: var(--app-text);
  font-size: 24px;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.workspace-grid {
  display: grid;
  grid-template-columns: minmax(300px, 400px) minmax(0, 1fr);
  gap: 22px;
  align-items: start;
}

.platform-panel,
.config-panel {
  min-width: 0;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.platform-panel {
  overflow: hidden;
}

.config-panel {
  padding: 22px;
}

.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 18px 14px;
  border-bottom: 1px solid var(--app-border);
}

.form-heading {
  padding: 0 0 18px;
  margin-bottom: 20px;
}

.panel-heading h2 {
  margin: 0 0 4px;
  color: var(--app-text);
  font-size: 16px;
  letter-spacing: 0;
}

.panel-heading p {
  margin: 0;
  color: var(--app-muted);
  font-size: 12px;
}

.platform-list {
  min-height: 260px;
  padding: 10px;
}

.list-tools {
  display: grid;
  gap: 10px;
  padding: 12px 14px;
  border-bottom: 1px solid var(--app-border);
}

.list-tools :deep(.el-segmented) {
  --el-segmented-item-selected-bg-color: var(--app-primary);
  --el-segmented-item-selected-color: #fff;
}

.platform-row {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr) auto;
  width: 100%;
  min-height: 74px;
  align-items: center;
  gap: 12px;
  padding: 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  color: inherit;
  cursor: pointer;
  text-align: left;
}

.platform-row:hover,
.platform-row.active {
  border-color: color-mix(in srgb, var(--app-primary) 30%, transparent);
  background: color-mix(in srgb, var(--app-primary) 8%, transparent);
}

.platform-row.disabled {
  opacity: 0.68;
}

.provider-mark {
  display: grid;
  width: 40px;
  height: 40px;
  place-items: center;
  border-radius: 8px;
  background: #101828;
  color: #fff;
  font-size: 17px;
  font-weight: 800;
}

.platform-main {
  min-width: 0;
}

.platform-main strong,
.platform-main small {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.platform-main strong {
  color: var(--app-text);
  font-size: 14px;
}

.platform-main small {
  margin-top: 4px;
  color: var(--app-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.platform-state {
  min-width: 46px;
  padding: 4px 8px;
  border-radius: 999px;
  background: #f1f5f9;
  color: #64748b;
  font-size: 12px;
  text-align: center;
}

.platform-state.enabled {
  background: #e8f7ef;
  color: #138a4b;
}

.empty-state {
  display: grid;
  min-height: 210px;
  place-items: center;
  padding: 32px;
  color: var(--app-muted);
  text-align: center;
}

.empty-state :deep(.el-icon) {
  margin-bottom: 8px;
  color: var(--app-primary);
  font-size: 28px;
}

.empty-state strong {
  color: var(--app-text);
  font-size: 15px;
}

.empty-state span {
  max-width: 260px;
  margin-top: 6px;
  font-size: 13px;
  line-height: 1.6;
}

.empty-state.compact {
  min-height: 150px;
}

.platform-form {
  max-width: 820px;
}

.form-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(220px, 280px);
  gap: 18px;
}

.full-control {
  width: 100%;
}

.switch-line {
  display: flex;
  min-height: 32px;
  align-items: center;
  gap: 10px;
  color: var(--app-muted);
  font-size: 13px;
}

.provider-hint,
.saved-credential {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: -2px 0 18px;
  padding: 10px 12px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #f8fbff;
  color: #52627a;
  font-size: 13px;
  line-height: 1.5;
}

.hint-label,
.saved-credential span {
  flex: 0 0 auto;
  color: var(--app-primary);
  font-weight: 700;
}

.saved-credential {
  justify-content: space-between;
  margin: -4px 0 18px;
  border-color: var(--app-border);
  background: #fafafa;
}

.saved-credential strong {
  overflow: hidden;
  color: var(--app-text);
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.action-bar {
  justify-content: space-between;
  margin-top: 4px;
  padding-top: 18px;
  border-top: 1px solid var(--app-border);
}

.test-result {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 20px;
  padding: 14px 16px;
  border: 1px solid;
  border-radius: 8px;
}

.test-result :deep(.el-icon) {
  flex: 0 0 auto;
  font-size: 22px;
}

.test-result strong,
.test-result span {
  display: block;
}

.test-result strong {
  color: var(--app-text);
  font-size: 14px;
}

.test-result span {
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 13px;
}

.test-result.success {
  border-color: #b9ebcf;
  background: #f0fbf5;
}

.test-result.success :deep(.el-icon) {
  color: #139b55;
}

.test-result.fail {
  border-color: #f8c7c7;
  background: #fff5f5;
}

.test-result.fail :deep(.el-icon) {
  color: #d93434;
}

@media (max-width: 1080px) {
  .page-hero,
  .action-bar {
    align-items: flex-start;
    flex-direction: column;
  }

  .metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .metric-item:nth-child(2) {
    border-right: 0;
  }

  .metric-item:nth-child(-n + 2) {
    border-bottom: 1px solid var(--app-border);
  }

  .workspace-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .platform-settings-page {
    padding: 22px 14px 36px;
  }

  .page-hero h1 {
    font-size: 24px;
  }

  .hero-actions,
  .left-actions,
  .right-actions {
    width: 100%;
    flex-wrap: wrap;
  }

  .form-grid,
  .metric-strip {
    grid-template-columns: 1fr;
  }

  .metric-item {
    border-right: 0;
    border-bottom: 1px solid var(--app-border);
  }

  .metric-item:last-child {
    border-bottom: 0;
  }

  .config-panel {
    padding: 18px;
  }

  .provider-hint,
  .saved-credential {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
