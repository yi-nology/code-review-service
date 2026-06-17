<template>
  <AppLayout>
    <div class="project-settings-page">
      <header class="policy-hero">
        <div class="hero-copy">
          <div class="eyebrow">PROJECT POLICY</div>
          <h1>项目策略工作台</h1>
          <p>为每个仓库定义门禁、审查范围、引擎覆盖和专属规则，让高风险变更在合并前被清楚拦住。</p>
        </div>

        <div class="hero-actions">
          <el-button plain @click="startCreate">
            <el-icon><Plus /></el-icon>
            新建策略
          </el-button>
          <el-button type="primary" @click="handleSave" :loading="saving" :disabled="hasBlockingIssues">
            <el-icon><CircleCheckFilled /></el-icon>
            保存策略
          </el-button>
        </div>

        <div class="summary-strip">
          <div class="summary-item">
            <span>已配置</span>
            <strong>{{ policies.length }}</strong>
          </div>
          <div class="summary-item">
            <span>严格门禁</span>
            <strong>{{ strictPolicyCount }}</strong>
          </div>
          <div class="summary-item">
            <span>项目规则</span>
            <strong>{{ totalRuleCount }}</strong>
          </div>
          <div class="summary-item warning" :class="{ active: validationIssues.length > 0 }">
            <span>待处理</span>
            <strong>{{ validationIssues.length }}</strong>
          </div>
        </div>
      </header>

      <div class="content-grid">
        <aside class="policy-sidebar panel-shell">
          <div class="panel-heading">
            <div>
              <strong>策略库</strong>
              <span>选择一个仓库继续编辑</span>
            </div>
            <el-button circle text aria-label="刷新策略列表" @click="fetchPolicies" :loading="loading">
              <el-icon><Refresh /></el-icon>
            </el-button>
          </div>

          <el-input
            v-model="policyKeyword"
            class="policy-search"
            clearable
            placeholder="搜索平台或仓库"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>

          <el-empty v-if="!loading && filteredPolicies.length === 0" description="暂无项目策略">
            <el-button type="primary" @click="startCreate">
              <el-icon><Plus /></el-icon>
              创建第一条策略
            </el-button>
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
                <span>{{ getFailOnLabel(policy.fail_on) }}</span>
                <span>忽略 {{ policy.ignore_paths?.length || 0 }}</span>
                <span>规则 {{ policy.rules?.length || 0 }}</span>
              </span>
            </button>
          </div>
        </aside>

        <main class="editor-column panel-shell">
          <div class="editor-header">
            <div>
              <span class="editor-state">{{ editingExisting ? '正在编辑' : '新建策略' }}</span>
              <strong>{{ form.repo || '选择或输入仓库' }}</strong>
              <small>{{ dirty ? '有未保存修改' : '当前表单已同步' }}</small>
            </div>
            <el-button
              v-if="editingExisting"
              type="success"
              plain
              @click="openAgentDialog"
            >
              <el-icon><User /></el-icon>
              配置审查员
            </el-button>
          </div>

          <div class="editor-layout">
            <section class="form-section">
              <div class="section-title">
                <span class="section-index">01</span>
                <div>
                  <strong>仓库定位</strong>
                  <span>策略按平台和仓库唯一生效</span>
                </div>
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
                <span class="section-index">02</span>
                <div>
                  <strong>质量门禁</strong>
                  <span>达到所选严重级别时，审查结果会判定为未通过</span>
                </div>
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
                <span class="section-index">03</span>
                <div>
                  <strong>审查范围</strong>
                  <span>忽略生成文件、依赖目录和构建产物，减少模型调用噪音</span>
                </div>
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
                <span class="section-index">04</span>
                <div>
                  <strong>审查引擎覆盖</strong>
                  <span>留空或填 0 表示继承全局默认，只为该仓库特殊情况覆盖</span>
                </div>
              </div>

              <div class="field-grid two-columns">
                <label class="field-block">
                  <span>OCR 启用策略</span>
                  <el-select v-model="ocrEnabledMode">
                    <el-option label="继承全局" value="inherit" />
                    <el-option label="强制启用" value="enabled" />
                    <el-option label="强制禁用" value="disabled" />
                  </el-select>
                </label>

                <label class="field-block">
                  <span>默认严重级别</span>
                  <el-select v-model="form.ocr_default_severity" clearable placeholder="继承全局">
                    <el-option label="严重" value="critical" />
                    <el-option label="高危" value="major" />
                    <el-option label="中危" value="minor" />
                    <el-option label="低危" value="info" />
                  </el-select>
                </label>

                <label class="field-block">
                  <span>模型并发数</span>
                  <el-input-number v-model="form.ocr_concurrency" :min="0" :max="32" />
                  <small>0 表示继承全局默认。</small>
                </label>

                <label class="field-block">
                  <span>单文件超时（分钟）</span>
                  <el-input-number v-model="form.ocr_timeout_minutes" :min="0" :max="120" />
                  <small>0 表示继承全局默认。</small>
                </label>

                <label class="field-block">
                  <span>最大工具轮次</span>
                  <el-input-number v-model="form.ocr_max_tools" :min="0" :max="100" />
                  <small>0 表示继承全局或 OCR 内置默认值。</small>
                </label>
              </div>

              <label class="field-block full-width">
                <span>审查背景</span>
                <el-input
                  v-model="form.ocr_background"
                  type="textarea"
                  :rows="4"
                  placeholder="该仓库的业务背景、重点风险、审查偏好；留空继承全局默认"
                />
              </label>
            </section>

            <section class="form-section">
              <div class="section-title">
                <span class="section-index">05</span>
                <div>
                  <strong>项目专属规则</strong>
                  <span>这里只写该仓库特有要求，通用规则放到「审查规则」页面</span>
                </div>
              </div>

              <RuleEditor
                v-model="form.rules"
                :issues="projectRuleIssues"
                empty-description="当前没有项目专属规则"
              />
            </section>
          </div>
        </main>

        <aside class="inspector-column">
          <div class="inspector-sticky">
            <section class="panel-shell inspector-card">
              <div class="panel-heading compact">
                <strong>保存前检查</strong>
              </div>
            <div v-if="validationIssues.length === 0" class="check-ok">
              <el-icon><CircleCheckFilled /></el-icon>
              可以保存
            </div>
            <ul v-else class="issue-list">
              <li v-for="issue in validationIssues" :key="issue">{{ issue }}</li>
            </ul>
            </section>

            <section class="panel-shell inspector-card">
              <div class="panel-heading compact">
                <strong>生效预览</strong>
              </div>
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
              <div>
                <span>OCR 覆盖</span>
                <strong>{{ ocrOverrideSummary }}</strong>
              </div>
            </div>
            <pre class="preview-json">{{ previewJson }}</pre>
            </section>

            <section class="panel-shell danger-card" v-if="editingExisting">
              <div class="panel-heading compact">
                <strong>危险操作</strong>
              </div>
            <p>删除后该仓库会回退到默认门禁和全局规则。</p>
            <el-button type="danger" plain @click="handleDeleteSelected">
              <el-icon><Delete /></el-icon>
              删除当前策略
            </el-button>
            </section>
          </div>
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
  Delete,
  Plus,
  Refresh,
  Search,
  User,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProjectAgentConfig from '@/components/ProjectAgentConfig.vue'
import RuleEditor from '@/components/RuleEditor.vue'
import { getProjectPolicies, putProjectPolicy, deleteProjectPolicy, getProviderRepos, getGitPlatforms } from '@/api/config'
import type { ProjectPolicy, ReviewRule, PlatformRepo, GitPlatform } from '@/types/config'

type OCREnabledMode = 'inherit' | 'enabled' | 'disabled'
type ProjectPolicyForm = Omit<ProjectPolicy, 'ignore_paths' | 'rules' | 'ocr_enabled'> & {
  ignore_paths: string[]
  rules: ReviewRule[]
  ocr_enabled: boolean | null
  ocr_default_severity: ProjectPolicy['ocr_default_severity']
}

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

const form = reactive<ProjectPolicyForm>({
  provider: '',
  repo: '',
  fail_on: 'major',
  ignore_paths: [] as string[],
  rules: [] as ReviewRule[],
  ocr_enabled: null,
  ocr_concurrency: 0,
  ocr_timeout_minutes: 0,
  ocr_max_tools: 0,
  ocr_default_severity: '',
  ocr_background: '',
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
  ocr_enabled: ocrEnabledValue.value,
  ocr_concurrency: form.ocr_concurrency || 0,
  ocr_timeout_minutes: form.ocr_timeout_minutes || 0,
  ocr_max_tools: form.ocr_max_tools || 0,
  ocr_default_severity: form.ocr_default_severity || '',
  ocr_background: form.ocr_background?.trim() || '',
}))

const previewJson = computed(() => JSON.stringify(payloadPreview.value, null, 2))
const ocrEnabledValue = computed<boolean | null>(() => {
  if (ocrEnabledMode.value === 'enabled') return true
  if (ocrEnabledMode.value === 'disabled') return false
  return null
})
const ocrEnabledMode = computed<OCREnabledMode>({
  get() {
    if (form.ocr_enabled === true) return 'enabled'
    if (form.ocr_enabled === false) return 'disabled'
    return 'inherit'
  },
  set(value) {
    form.ocr_enabled = value === 'enabled' ? true : value === 'disabled' ? false : null
  },
})
const dirty = computed(() => previewJson.value !== originalJson.value)
const currentPolicyKey = computed(() => form.provider && form.repo ? `${form.provider}:${form.repo}` : '')
const editingExisting = computed(() => policies.value.some((policy) => policyKey(policy) === currentPolicyKey.value))
const strictPolicyCount = computed(() => policies.value.filter((policy) => ['major', 'minor', 'info'].includes(policy.fail_on)).length)
const totalRuleCount = computed(() => policies.value.reduce((sum, policy) => sum + (policy.rules?.length || 0), 0))
const ocrOverrideSummary = computed(() => {
  const items: string[] = []
  if (ocrEnabledMode.value !== 'inherit') items.push(ocrEnabledMode.value === 'enabled' ? '启用' : '禁用')
  if (form.ocr_default_severity) items.push(`严重级别 ${form.ocr_default_severity}`)
  if (form.ocr_concurrency) items.push(`并发 ${form.ocr_concurrency}`)
  if (form.ocr_timeout_minutes) items.push(`超时 ${form.ocr_timeout_minutes}m`)
  if (form.ocr_max_tools) items.push(`工具 ${form.ocr_max_tools}`)
  if (form.ocr_background?.trim()) items.push('背景')
  return items.length > 0 ? `${items.length} 项` : '继承全局'
})

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
  if (form.ocr_default_severity && !failOnOptions.some((option) => option.value === form.ocr_default_severity)) {
    issues.push('OCR 默认严重级别无效')
  }

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
  form.ocr_enabled = policy.ocr_enabled ?? null
  form.ocr_concurrency = policy.ocr_concurrency || 0
  form.ocr_timeout_minutes = policy.ocr_timeout_minutes || 0
  form.ocr_max_tools = policy.ocr_max_tools || 0
  form.ocr_default_severity = policy.ocr_default_severity || ''
  form.ocr_background = policy.ocr_background || ''
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
  form.ocr_enabled = null
  form.ocr_concurrency = 0
  form.ocr_timeout_minutes = 0
  form.ocr_max_tools = 0
  form.ocr_default_severity = ''
  form.ocr_background = ''
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
  width: min(1520px, 100%);
  margin: 0 auto;
  padding: 30px 34px 48px;
}

.policy-hero {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 24px;
  align-items: start;
  margin-bottom: 18px;
  padding: 24px;
  border: 1px solid #dfe5ef;
  border-radius: 8px;
  background:
    linear-gradient(135deg, rgba(255, 255, 255, 0.96), rgba(247, 250, 255, 0.96)),
    #fff;
  box-shadow: 0 16px 40px rgba(23, 32, 51, 0.07);
}

.hero-copy h1 {
  margin: 5px 0 8px;
  color: var(--app-text);
  font-size: 31px;
  line-height: 1.18;
  letter-spacing: 0;
}

.hero-copy p {
  max-width: 760px;
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

.hero-actions {
  display: flex;
  flex: 0 0 auto;
  gap: 10px;
}

.summary-strip {
  display: grid;
  grid-column: 1 / -1;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 1px;
  overflow: hidden;
  border: 1px solid #e3e7ef;
  border-radius: 8px;
  background: #e3e7ef;
}

.summary-item {
  min-height: 74px;
  padding: 14px 18px;
  background: rgba(255, 255, 255, 0.86);
}

.summary-item span,
.panel-heading span,
.section-title span,
.editor-header small,
.preview-list span {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
}

.summary-item strong {
  display: block;
  margin-top: 6px;
  color: var(--app-text);
  font-size: 27px;
  line-height: 1;
}

.summary-item.warning.active strong {
  color: var(--app-danger);
}

.content-grid {
  display: grid;
  grid-template-columns: 318px minmax(0, 1fr) 336px;
  gap: 16px;
  align-items: start;
}

.panel-shell {
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  box-shadow: 0 12px 32px rgba(23, 32, 51, 0.055);
}

.panel-heading,
.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.panel-heading {
  min-height: 54px;
  padding: 15px 16px 12px;
  border-bottom: 1px solid var(--app-border);
}

.panel-heading strong,
.editor-header strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.panel-heading.compact {
  min-height: auto;
  padding: 0;
  border-bottom: 0;
  margin-bottom: 13px;
}

.policy-sidebar {
  position: sticky;
  top: 18px;
  padding-bottom: 12px;
}

.policy-search {
  padding: 14px 14px 12px;
  margin: 0;
}

.policy-list {
  display: grid;
  gap: 7px;
  align-content: start;
  max-height: calc(100vh - 300px);
  min-height: 280px;
  padding: 0 10px 10px;
  overflow: auto;
}

.policy-item {
  width: 100%;
  padding: 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: #f8fafc;
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.policy-item:hover,
.policy-item.active {
  border-color: #cad2f8;
  background: #fff;
  box-shadow: 0 8px 20px rgba(91, 91, 214, 0.08);
}

.policy-main {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.policy-main strong {
  flex: 1;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-tag {
  display: inline-flex;
  align-items: center;
  flex: 0 0 auto;
  min-height: 22px;
  padding: 0 7px;
  border-radius: 6px;
  color: #5656c5;
  background: #f0f1ff;
  font-size: 11px;
  font-weight: 800;
}

.policy-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
  color: var(--app-muted);
  font-size: 12px;
}

.policy-meta span {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 7px;
  border-radius: 6px;
  background: #eef2f7;
}

.editor-column {
  min-width: 0;
  overflow: hidden;
}

.editor-header {
  min-height: 78px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--app-border);
  background: #fbfcfe;
}

.editor-header strong {
  margin-top: 3px;
  overflow-wrap: anywhere;
  font-size: 19px;
}

.editor-state {
  display: inline-flex;
  min-height: 22px;
  align-items: center;
  padding: 0 8px;
  border-radius: 6px;
  color: #31536f;
  background: #eaf2f8;
  font-size: 11px;
  font-weight: 800;
}

.editor-header small {
  margin-top: 4px;
}

.editor-layout {
  display: grid;
  gap: 0;
}

.form-section {
  padding: 22px 20px 24px;
  border-bottom: 1px solid var(--app-border);
}

.form-section:last-child {
  border-bottom: none;
}

.section-title {
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 12px;
  align-items: start;
  margin-bottom: 16px;
}

.section-index {
  display: grid;
  width: 34px;
  height: 28px;
  place-items: center;
  border-radius: 8px;
  color: #4b54bf;
  background: #eef0ff;
  font-size: 12px;
  font-weight: 850;
}

.section-title strong {
  display: block;
  color: var(--app-text);
  font-size: 16px;
}

.section-title div > span {
  margin-top: 4px;
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

.field-block.full-width {
  margin-top: 14px;
}

.field-block > span {
  color: #445166;
  font-size: 12px;
  font-weight: 750;
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
  padding: 14px 15px;
  border: 1px solid #e2e7f0;
  border-radius: 8px;
  background: #fbfcfe;
  color: var(--app-text);
  text-align: left;
  cursor: pointer;
  transition: 160ms ease;
}

.gate-option.active {
  border-color: #aeb6ef;
  background: #fff;
  box-shadow: inset 0 0 0 1px #dce1ff, 0 8px 18px rgba(91, 91, 214, 0.08);
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
  margin-top: 12px;
}

.inspector-column {
  min-width: 0;
}

.inspector-sticky {
  position: sticky;
  top: 18px;
  display: grid;
  gap: 12px;
}

.inspector-card,
.danger-card {
  padding: 16px;
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
  gap: 1px;
  overflow: hidden;
  margin-bottom: 14px;
  border: 1px solid #e5e9f1;
  border-radius: 8px;
  background: #e5e9f1;
}

.preview-list > div {
  padding: 11px 12px;
  background: #fbfcfe;
}

.preview-list strong {
  display: block;
  margin-top: 3px;
  overflow-wrap: anywhere;
  color: var(--app-text);
  font-size: 13px;
}

.preview-json {
  max-height: 260px;
  margin: 0;
  padding: 12px;
  border: 1px solid #e5e9f1;
  border-radius: 8px;
  background: #f8fafc;
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

@media (max-width: 1600px) {
  .content-grid {
    grid-template-columns: 300px minmax(560px, 1fr);
  }

  .inspector-column {
    grid-column: 1 / -1;
  }

  .inspector-sticky {
    position: static;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .danger-card {
    grid-column: 1 / -1;
  }
}

@media (max-width: 900px) {
  .policy-hero,
  .content-grid,
  .field-grid.two-columns,
  .gate-options,
  .inspector-sticky {
    grid-template-columns: 1fr;
  }

  .hero-actions {
    justify-content: flex-start;
  }

  .summary-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .policy-sidebar {
    position: static;
  }

  .policy-list {
    max-height: 360px;
  }
}

@media (max-width: 720px) {
  .project-settings-page {
    padding: 22px 14px 36px;
  }

  .policy-hero {
    padding: 18px;
  }

  .hero-copy h1 {
    font-size: 24px;
  }

  .hero-actions {
    display: grid;
    grid-template-columns: 1fr;
  }

  .hero-actions :deep(.el-button) {
    width: 100%;
    margin-left: 0;
  }

  .summary-strip {
    grid-template-columns: 1fr;
  }

  .editor-header {
    display: grid;
  }

  .section-title {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
