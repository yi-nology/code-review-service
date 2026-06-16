<template>
  <AppLayout>
    <div class="project-settings-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">PROJECT POLICY</div>
          <h1>项目策略</h1>
          <p>按仓库配置质量门禁、审查范围、项目专属规则和审查员团队。</p>
        </div>
        <div class="header-actions">
          <el-button plain @click="startCreate">
            <el-icon><Plus /></el-icon>
            新建策略
          </el-button>
          <el-button type="primary" @click="handleSave" :loading="saving" :disabled="hasBlockingIssues">
            保存策略
          </el-button>
        </div>
      </header>

      <section class="summary-grid">
        <div class="summary-card">
          <span>已配置项目</span>
          <strong>{{ policies.length }}</strong>
        </div>
        <div class="summary-card">
          <span>严格门禁</span>
          <strong>{{ strictPolicyCount }}</strong>
        </div>
        <div class="summary-card">
          <span>自定义规则</span>
          <strong>{{ totalRuleCount }}</strong>
        </div>
        <div class="summary-card warning" :class="{ active: validationIssues.length > 0 }">
          <span>待处理问题</span>
          <strong>{{ validationIssues.length }}</strong>
        </div>
      </section>

      <div class="content-grid">
        <aside class="policy-sidebar">
          <el-card class="panel-card">
            <template #header>
              <div class="card-header">
                <div>
                  <strong>项目列表</strong>
                  <span>点击项目可继续编辑</span>
                </div>
                <el-button text @click="fetchPolicies" :loading="loading">
                  <el-icon><Refresh /></el-icon>
                </el-button>
              </div>
            </template>

            <el-input
              v-model="policyKeyword"
              class="policy-search"
              clearable
              placeholder="搜索仓库或平台"
            >
              <template #prefix><el-icon><Search /></el-icon></template>
            </el-input>

            <el-empty v-if="!loading && filteredPolicies.length === 0" description="暂无项目策略">
              <el-button type="primary" @click="startCreate">创建第一条策略</el-button>
            </el-empty>

            <div v-else v-loading="loading" class="policy-list">
              <button
                v-for="policy in filteredPolicies"
                :key="policyKey(policy)"
                class="policy-item"
                :class="{ active: policyKey(policy) === currentPolicyKey }"
                type="button"
                @click="handleEdit(policy)"
              >
                <span class="policy-main">
                  <span class="provider-tag">{{ policy.provider }}</span>
                  <strong>{{ policy.repo }}</strong>
                </span>
                <span class="policy-meta">
                  {{ getFailOnLabel(policy.fail_on) }} / 忽略 {{ policy.ignore_paths?.length || 0 }} / 规则 {{ policy.rules?.length || 0 }}
                </span>
              </button>
            </div>
          </el-card>
        </aside>

        <main class="editor-column">
          <el-card class="panel-card">
            <template #header>
              <div class="card-header">
                <div>
                  <strong>{{ editingExisting ? '编辑项目策略' : '新建项目策略' }}</strong>
                  <span>{{ dirty ? '有未保存修改' : '当前表单已同步' }}</span>
                </div>
                <el-button
                  v-if="editingExisting"
                  type="success"
                  plain
                  @click="openAgentDialog"
                >
                  配置审查员
                </el-button>
              </div>
            </template>

            <div class="editor-layout">
              <section class="form-section">
                <div class="section-title">
                  <strong>1. 选择仓库</strong>
                  <span>策略按平台和仓库唯一生效</span>
                </div>

                <div class="field-grid two-columns">
                  <label class="field-block">
                    <span>代码平台</span>
                    <el-select v-model="form.provider" @change="handleProviderChange" placeholder="请先配置代码平台">
                      <el-option
                        v-for="p in platforms"
                        :key="p.provider"
                        :label="p.provider"
                        :value="p.provider"
                      />
                    </el-select>
                    <small v-if="platforms.length === 0">
                      暂无代码平台，请先到 <router-link to="/settings/platforms">Git 平台</router-link> 添加。
                    </small>
                  </label>

                  <label class="field-block">
                    <span>仓库</span>
                    <el-select
                      v-model="form.repo"
                      filterable
                      allow-create
                      default-first-option
                      placeholder="选择或输入 owner/repo"
                      :loading="reposLoading"
                    >
                      <el-option
                        v-for="repo in repos"
                        :key="repo.full_name"
                        :label="repo.full_name"
                        :value="repo.full_name"
                      />
                    </el-select>
                    <small>仓库列表加载失败时也可以手动输入。</small>
                  </label>
                </div>
              </section>

              <section class="form-section">
                <div class="section-title">
                  <strong>2. 设置质量门禁</strong>
                  <span>达到所选严重级别时，审查结果会判定为未通过</span>
                </div>

                <div class="gate-options">
                  <button
                    v-for="option in failOnOptions"
                    :key="option.value"
                    class="gate-option"
                    :class="{ active: form.fail_on === option.value }"
                    type="button"
                    @click="form.fail_on = option.value"
                  >
                    <strong>{{ option.label }}</strong>
                    <span>{{ option.description }}</span>
                  </button>
                </div>
              </section>

              <section class="form-section">
                <div class="section-title">
                  <strong>3. 控制审查范围</strong>
                  <span>忽略路径会在模型调用前排除，适合生成文件、依赖目录和构建产物</span>
                </div>

                <el-input
                  v-model="ignorePathsText"
                  type="textarea"
                  :rows="5"
                  placeholder="vendor/**&#10;dist/**&#10;**/*.pb.go"
                />
                <div class="quick-actions">
                  <el-button size="small" @click="addIgnoreTemplate('vendor/**')">vendor/**</el-button>
                  <el-button size="small" @click="addIgnoreTemplate('dist/**')">dist/**</el-button>
                  <el-button size="small" @click="addIgnoreTemplate('**/*.pb.go')">*.pb.go</el-button>
                  <el-button size="small" @click="addIgnoreTemplate('node_modules/**')">node_modules</el-button>
                </div>
              </section>

              <section class="form-section">
                <div class="section-title">
                  <strong>4. 项目专属规则</strong>
                  <span>这里只写该仓库特有要求，通用规则放到「审查规则」页面</span>
                </div>

                <RuleEditor
                  v-model="form.rules"
                  :issues="projectRuleIssues"
                  empty-description="当前没有项目专属规则"
                />
              </section>
            </div>
          </el-card>
        </main>

        <aside class="inspector-column">
          <el-card class="panel-card">
            <template #header>
              <div class="card-header compact">
                <strong>保存前检查</strong>
              </div>
            </template>
            <div v-if="validationIssues.length === 0" class="check-ok">
              <el-icon><CircleCheckFilled /></el-icon>
              可以保存
            </div>
            <ul v-else class="issue-list">
              <li v-for="issue in validationIssues" :key="issue">{{ issue }}</li>
            </ul>
          </el-card>

          <el-card class="panel-card">
            <template #header>
              <div class="card-header compact">
                <strong>生效预览</strong>
              </div>
            </template>
            <div class="preview-list">
              <div>
                <span>平台 / 仓库</span>
                <strong>{{ form.provider || '--' }} / {{ form.repo || '--' }}</strong>
              </div>
              <div>
                <span>门禁</span>
                <strong>{{ getFailOnLabel(form.fail_on) }}</strong>
              </div>
              <div>
                <span>忽略路径</span>
                <strong>{{ normalizedIgnorePaths.length }} 条</strong>
              </div>
              <div>
                <span>项目规则</span>
                <strong>{{ normalizedRules.length }} 条</strong>
              </div>
            </div>
            <pre class="preview-json">{{ previewJson }}</pre>
          </el-card>

          <el-card class="panel-card danger-card" v-if="editingExisting">
            <template #header>
              <div class="card-header compact">
                <strong>危险操作</strong>
              </div>
            </template>
            <p>删除后该仓库会回退到默认门禁和全局规则。</p>
            <el-button type="danger" plain @click="handleDeleteSelected">删除当前策略</el-button>
          </el-card>
        </aside>
      </div>

      <el-dialog
        v-model="agentDialogVisible"
        :title="`${currentProject?.provider}/${currentProject?.repo} - 审查员配置`"
        width="760px"
      >
        <ProjectAgentConfig
          v-if="currentProject"
          :provider="currentProject.provider"
          :repo="currentProject.repo"
        />
      </el-dialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import {
  CircleCheckFilled,
  Plus,
  Refresh,
  Search,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProjectAgentConfig from '@/components/ProjectAgentConfig.vue'
import RuleEditor from '@/components/RuleEditor.vue'
import { getProjectPolicies, putProjectPolicy, deleteProjectPolicy, getProviderRepos, getGitPlatforms } from '@/api/config'
import type { ProjectPolicy, ReviewRule, PlatformRepo, GitPlatform } from '@/types/config'

const failOnOptions = [
  { value: 'critical', label: '只阻断严重问题', description: '适合刚接入时降低噪音' },
  { value: 'major', label: '阻断高风险及以上', description: '推荐用于大多数业务仓库' },
  { value: 'minor', label: '阻断中风险及以上', description: '适合质量要求较高的核心仓库' },
  { value: 'info', label: '阻断所有问题', description: '适合强规范或发布前专项检查' },
]

const loading = ref(false)
const saving = ref(false)
const policies = ref<ProjectPolicy[]>([])
const agentDialogVisible = ref(false)
const currentProject = ref<{ provider: string; repo: string } | null>(null)
const reposLoading = ref(false)
const repos = ref<PlatformRepo[]>([])
const platforms = ref<GitPlatform[]>([])
const policyKeyword = ref('')
const originalJson = ref('')

const form = reactive<ProjectPolicy & { rules: ReviewRule[] }>({
  provider: '',
  repo: '',
  fail_on: 'major',
  ignore_paths: [],
  rules: [],
})

const ignorePathsText = ref('')

const normalizedIgnorePaths = computed(() => {
  return uniqueLines(ignorePathsText.value)
})

const normalizedRules = computed(() => {
  return form.rules
    .map((rule) => ({
      path: normalizePath(rule.path),
      rule: rule.rule.trim(),
    }))
    .filter((rule) => rule.path || rule.rule)
})

const payloadPreview = computed<ProjectPolicy>(() => ({
  provider: form.provider.trim(),
  repo: form.repo.trim(),
  fail_on: form.fail_on,
  ignore_paths: normalizedIgnorePaths.value,
  rules: normalizedRules.value,
}))

const previewJson = computed(() => JSON.stringify(payloadPreview.value, null, 2))
const dirty = computed(() => previewJson.value !== originalJson.value)
const currentPolicyKey = computed(() => form.provider && form.repo ? `${form.provider}:${form.repo}` : '')
const editingExisting = computed(() => policies.value.some((policy) => policyKey(policy) === currentPolicyKey.value))
const strictPolicyCount = computed(() => policies.value.filter((policy) => ['major', 'minor', 'info'].includes(policy.fail_on)).length)
const totalRuleCount = computed(() => policies.value.reduce((sum, policy) => sum + (policy.rules?.length || 0), 0))

const filteredPolicies = computed(() => {
  const keyword = policyKeyword.value.trim().toLowerCase()
  if (!keyword) return policies.value
  return policies.value.filter((policy) => {
    return policy.provider.toLowerCase().includes(keyword) || policy.repo.toLowerCase().includes(keyword)
  })
})

const validationIssues = computed(() => {
  const issues: string[] = []
  if (!form.provider.trim()) issues.push('请选择代码平台')
  if (!form.repo.trim()) issues.push('请选择或输入仓库')
  if (!failOnOptions.some((option) => option.value === form.fail_on)) issues.push('门禁级别无效')

  const seenIgnorePaths = new Set<string>()
  normalizedIgnorePaths.value.forEach((path) => {
    if (seenIgnorePaths.has(path)) issues.push(`忽略路径重复：${path}`)
    seenIgnorePaths.add(path)
  })

  const seenRules = new Set<string>()
  form.rules.forEach((rule, index) => {
    const path = normalizePath(rule.path)
    const text = rule.rule.trim()
    if (!path && !text) return
    if (!path) issues.push(`第 ${index + 1} 条项目规则缺少适用文件`)
    if (!text) issues.push(`第 ${index + 1} 条项目规则缺少审查要求`)
    const key = `${path}::${text}`
    if (path && text) {
      if (seenRules.has(key)) issues.push(`第 ${index + 1} 条项目规则重复`)
      seenRules.add(key)
    }
  })
  return issues
})

const projectRuleIssues = computed(() => validationIssues.value.filter((issue) => issue.includes('项目规则')))
const hasBlockingIssues = computed(() => validationIssues.value.length > 0)

function uniqueLines(text: string) {
  const values = text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  return Array.from(new Set(values))
}

function normalizePath(path: string) {
  const value = path.trim()
  return value || '**/*'
}

function policyKey(policy: ProjectPolicy) {
  return `${policy.provider}:${policy.repo}`
}

async function fetchPlatforms() {
  try {
    const result = await getGitPlatforms()
    platforms.value = result.items || []
    if (platforms.value.length > 0 && !form.provider) {
      form.provider = platforms.value[0].provider
      await fetchRepos()
      syncOriginal()
    }
  } catch (e) {
    console.error('Failed to load platforms:', e)
  }
}

async function fetchRepos() {
  if (!form.provider) {
    repos.value = []
    return
  }
  reposLoading.value = true
  try {
    repos.value = await getProviderRepos(form.provider)
  } catch (e) {
    console.error('Failed to load repos:', e)
    repos.value = []
    ElMessage.warning('加载仓库列表失败，可手动输入仓库名称')
  } finally {
    reposLoading.value = false
  }
}

function handleProviderChange() {
  form.repo = ''
  fetchRepos()
}

async function fetchPolicies() {
  loading.value = true
  try {
    const result = await getProjectPolicies()
    policies.value = result.items || []
  } finally {
    loading.value = false
  }
}

async function handleSave() {
  if (hasBlockingIssues.value) {
    ElMessage.warning('请先处理保存前检查中的问题')
    return
  }
  saving.value = true
  try {
    const saved = await putProjectPolicy(payloadPreview.value)
    ElMessage.success('项目策略已保存')
    await fetchPolicies()
    handleEdit(saved)
  } finally {
    saving.value = false
  }
}

function handleEdit(policy: ProjectPolicy) {
  form.provider = policy.provider
  form.repo = policy.repo
  form.fail_on = policy.fail_on || 'major'
  form.ignore_paths = policy.ignore_paths || []
  form.rules = policy.rules ? policy.rules.map((rule) => ({ ...rule })) : []
  ignorePathsText.value = (policy.ignore_paths || []).join('\n')
  syncOriginal()
  fetchRepos()
}

function startCreate() {
  form.provider = platforms.value[0]?.provider || ''
  form.repo = ''
  form.fail_on = 'major'
  form.ignore_paths = []
  form.rules = []
  ignorePathsText.value = ''
  syncOriginal()
  if (form.provider) fetchRepos()
}

function openAgentDialog() {
  currentProject.value = {
    provider: form.provider,
    repo: form.repo,
  }
  agentDialogVisible.value = true
}

async function handleDeleteSelected() {
  const policy = policies.value.find((item) => policyKey(item) === currentPolicyKey.value)
  if (policy) await handleDelete(policy)
}

async function handleDelete(policy: ProjectPolicy) {
  try {
    await ElMessageBox.confirm(`确定删除 ${policy.provider}/${policy.repo} 的项目策略？`, '确认删除')
    await deleteProjectPolicy(policy.provider, policy.repo)
    ElMessage.success('项目策略已删除')
    await fetchPolicies()
    if (policyKey(policy) === currentPolicyKey.value) startCreate()
  } catch {
    // 用户取消
  }
}

function addIgnoreTemplate(path: string) {
  const next = new Set(normalizedIgnorePaths.value)
  next.add(path)
  ignorePathsText.value = Array.from(next).join('\n')
}

function syncOriginal() {
  originalJson.value = previewJson.value
}

function getFailOnLabel(failOn: string) {
  const option = failOnOptions.find((item) => item.value === failOn)
  return option?.label || failOn
}

onMounted(async () => {
  await fetchPlatforms()
  await fetchPolicies()
  if (policies.value.length > 0) {
    handleEdit(policies.value[0])
  } else {
    syncOriginal()
  }
})
</script>

<style scoped>
.project-settings-page {
  width: min(1480px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 22px;
}

.page-header h1 {
  margin: 4px 0 8px;
  color: var(--app-text);
  font-size: 30px;
  letter-spacing: -0.6px;
}

.page-header p {
  max-width: 720px;
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
  line-height: 1.7;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.4px;
}

.header-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
}

.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.summary-card {
  min-height: 92px;
  padding: 18px 20px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.summary-card span,
.card-header span,
.section-title span,
.preview-list span {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
}

.summary-card strong {
  display: block;
  margin-top: 8px;
  color: var(--app-text);
  font-size: 28px;
  line-height: 1;
}

.summary-card.warning.active strong {
  color: var(--app-danger);
}

.content-grid {
  display: grid;
  grid-template-columns: 330px minmax(0, 1fr) 340px;
  gap: 18px;
  align-items: start;
}

.panel-card {
  border-radius: 8px;
  box-shadow: var(--app-shadow);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.card-header strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.card-header.compact {
  min-height: 28px;
}

.policy-search {
  margin-bottom: 12px;
}

.policy-list {
  display: grid;
  gap: 8px;
  max-height: 640px;
  overflow: auto;
}

.policy-item {
  width: 100%;
  padding: 13px 14px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #fff;
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.policy-item:hover,
.policy-item.active {
  border-color: #cfd3ff;
  background: #fbfbff;
}

.policy-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.policy-main strong {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-tag {
  display: inline-flex;
  align-items: center;
  min-height: 22px;
  padding: 0 7px;
  border-radius: 6px;
  color: #5656c5;
  background: #f0f1ff;
  font-size: 11px;
  font-weight: 800;
}

.policy-meta {
  display: block;
  margin-top: 7px;
  color: var(--app-muted);
  font-size: 12px;
}

.editor-layout {
  display: grid;
  gap: 24px;
}

.form-section {
  padding-bottom: 22px;
  border-bottom: 1px solid var(--app-border);
}

.form-section:last-child {
  padding-bottom: 0;
  border-bottom: none;
}

.section-title {
  margin-bottom: 14px;
}

.section-title strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.section-title span {
  margin-top: 3px;
}

.field-grid {
  display: grid;
  gap: 14px;
}

.field-grid.two-columns {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.field-block {
  display: grid;
  gap: 8px;
}

.field-block > span {
  color: #4b5568;
  font-size: 12px;
  font-weight: 700;
}

.field-block small {
  color: var(--app-muted);
  font-size: 12px;
}

.gate-options {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.gate-option {
  min-height: 82px;
  padding: 14px;
  border: 1px solid #edf0f5;
  border-radius: 8px;
  background: #fff;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.gate-option.active {
  border-color: #bfc4ff;
  background: #fbfbff;
  box-shadow: inset 0 0 0 1px #dfe2ff;
}

.gate-option strong,
.gate-option span {
  display: block;
}

.gate-option strong {
  font-size: 13px;
}

.gate-option span {
  margin-top: 6px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.5;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 10px;
}

.check-ok {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--app-success);
  font-size: 13px;
  font-weight: 700;
}

.issue-list {
  margin: 0;
  padding-left: 18px;
  color: var(--app-danger);
  font-size: 12px;
  line-height: 1.8;
}

.preview-list {
  display: grid;
  gap: 12px;
  margin-bottom: 14px;
}

.preview-list strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
  color: var(--app-text);
  font-size: 13px;
}

.preview-json {
  max-height: 300px;
  margin: 0;
  padding-top: 14px;
  border-top: 1px solid var(--app-border);
  overflow: auto;
  color: #2d3748;
  font-size: 12px;
  line-height: 1.65;
  white-space: pre-wrap;
  word-break: break-word;
}

.danger-card p {
  margin: 0 0 12px;
  color: var(--app-muted);
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 1280px) {
  .content-grid {
    grid-template-columns: 310px minmax(0, 1fr);
  }

  .inspector-column {
    grid-column: 1 / -1;
  }
}

@media (max-width: 900px) {
  .page-header {
    display: block;
  }

  .header-actions {
    margin-top: 16px;
  }

  .summary-grid,
  .content-grid,
  .field-grid.two-columns,
  .gate-options {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 720px) {
  .project-settings-page {
    padding: 22px 14px 36px;
  }

  .page-header h1 {
    font-size: 24px;
  }
}
</style>
