<template>
  <AppLayout>
    <div class="scan-page">
      <header class="page-header">
        <div>
          <h1>代码库扫描</h1>
          <p>对整个仓库或指定目录执行全量代码审查</p>
        </div>
        <div class="header-status">
          <el-tag
            v-if="scanState === 'idle'"
            :type="canRun ? 'success' : 'info'"
            size="large"
          >
            {{ canRun ? '已配置' : '待配置' }}
          </el-tag>
          <el-tag
            v-else-if="scanState === 'queued'"
            type="warning"
            size="large"
          >
            排队中
          </el-tag>
          <el-tag
            v-else-if="scanState === 'running'"
            type="primary"
            size="large"
            effect="light"
          >
            <el-icon class="is-loading">
              <Loading />
            </el-icon>
            扫描中
          </el-tag>
          <el-tag
            v-else-if="scanState === 'success'"
            type="success"
            size="large"
          >
            已完成
          </el-tag>
          <el-tag
            v-else-if="scanState === 'failed'"
            type="danger"
            size="large"
          >
            扫描失败
          </el-tag>
        </div>
      </header>

      <div class="scan-grid">
        <!-- ===== Left: Config ===== -->
        <section class="panel config-panel">
          <el-form
            ref="formRef"
            label-position="top"
            :model="form"
            :rules="formRules"
            :disabled="scanState === 'queued' || scanState === 'running'"
          >
            <!-- Card 1: Basic Info -->
            <div class="form-card">
              <div class="form-card-title">
                基础信息
              </div>

              <el-form-item
                label="代码平台"
                prop="provider"
              >
                <el-select
                  v-model="form.provider"
                  placeholder="选择平台"
                  @change="handleProviderChange"
                >
                  <el-option
                    v-for="p in platforms"
                    :key="p.provider"
                    :label="p.provider"
                    :value="p.provider"
                  />
                </el-select>
              </el-form-item>

              <el-form-item
                label="仓库"
                prop="repo"
              >
                <el-select
                  v-model="form.repo"
                  filterable
                  remote
                  reserve-keyword
                  :loading="reposLoading"
                  :remote-method="searchRepos"
                  placeholder="搜索仓库名称"
                  @change="handleRepoChange"
                  @visible-change="(v: boolean) => v && repos.length === 0 && searchRepos('')"
                >
                  <el-option
                    v-for="repo in repos"
                    :key="repo.full_name"
                    :label="repo.full_name"
                    :value="repo.full_name"
                  />
                </el-select>
              </el-form-item>

              <el-form-item label="分支 / 提交">
                <div class="branch-row">
                  <el-select
                    v-model="branch"
                    filterable
                    allow-create
                    default-first-option
                    :loading="branchesLoading"
                    placeholder="分支名或 commit SHA"
                    @focus="loadBranches"
                    @change="resolveHead"
                  >
                    <el-option
                      v-for="b in branches"
                      :key="b.name"
                      :label="b.name"
                      :value="b.name"
                    />
                  </el-select>
                </div>
                <div
                  v-if="form.head_sha"
                  class="form-tip"
                >
                  <el-icon><Check /></el-icon>
                  <code>{{ form.head_sha }}</code>
                </div>
                <div
                  v-else-if="branchTouched && !form.head_sha"
                  class="form-tip error-tip"
                >
                  <el-icon><Warning /></el-icon>
                  分支尚未解析，请在选择后等待自动解析
                </div>
              </el-form-item>
            </div>

            <!-- Card 2: Scan Scope -->
            <div class="form-card">
              <div class="form-card-title">
                扫描范围
              </div>

              <el-form-item label="扫描路径">
                <el-input
                  v-model="pathsText"
                  type="textarea"
                  :rows="3"
                  placeholder="每行一个目录或文件，留空扫描整个仓库&#10;internal&#10;frontend/src"
                />
              </el-form-item>

              <el-form-item label="排除路径">
                <el-input
                  v-model="excludeText"
                  type="textarea"
                  :rows="3"
                  placeholder="每行一个 gitignore 风格规则&#10;**/generated/**&#10;vendor/**"
                />
              </el-form-item>

              <el-form-item label="业务背景">
                <el-input
                  v-model="form.background"
                  type="textarea"
                  :rows="2"
                  placeholder="例如：迁移前质量体检，重点关注兼容性和安全风险"
                />
              </el-form-item>

              <el-form-item label="超时时间（分钟）">
                <el-input-number
                  v-model="form.timeout_minutes"
                  :min="5"
                  :max="120"
                  :step="5"
                  :defaultValue="60"
                  controls-position="right"
                  style="width: 100%"
                />
              </el-form-item>
            </div>

            <!-- Card 3: Advanced (collapsible) -->
            <el-collapse
              v-model="advancedOpen"
              class="advanced-collapse"
            >
              <el-collapse-item
                title="高级选项"
                name="advanced"
              >
                <div class="inline-fields">
                  <el-form-item label="Token 预算上限">
                    <el-input-number
                      v-model="form.max_tokens_budget"
                      :min="0"
                      :step="100000"
                      controls-position="right"
                      style="width: 100%"
                    />
                  </el-form-item>
                  <el-form-item label="分批策略">
                    <el-select v-model="form.batch">
                      <el-option
                        label="默认"
                        value=""
                      />
                      <el-option
                        label="按语言"
                        value="by-language"
                      />
                      <el-option
                        label="按目录"
                        value="by-directory"
                      />
                      <el-option
                        label="不分批"
                        value="none"
                      />
                    </el-select>
                  </el-form-item>
                </div>

                <div class="switch-row">
                  <el-checkbox v-model="form.no_plan">
                    跳过计划
                  </el-checkbox>
                  <el-checkbox v-model="form.no_dedup">
                    跳过去重
                  </el-checkbox>
                  <el-checkbox v-model="form.no_summary">
                    跳过总结
                  </el-checkbox>
                </div>
              </el-collapse-item>
            </el-collapse>

            <!-- Action row -->
            <div class="action-row">
              <el-button
                :loading="previewing"
                :disabled="!canRun || scanRunning"
                @click="runScan(true)"
              >
                <el-icon><Files /></el-icon>
                预览文件
              </el-button>
              <el-button
                type="primary"
                :loading="starting"
                :disabled="!canRun || scanRunning"
                @click="confirmScan"
              >
                <el-icon><Search /></el-icon>
                开始扫描
              </el-button>
            </div>
          </el-form>
        </section>

        <!-- ===== Right: Execution / Results ===== -->
        <section class="panel result-panel">
          <!-- Idle -->
          <div
            v-if="scanState === 'idle' && !hasPreview"
            class="result-empty"
          >
            <el-empty description="先选择仓库，预览或开始扫描" />
          </div>

          <!-- Preview -->
          <ScanPreviewPanel
            v-if="hasPreview && scanState === 'idle'"
            :files="previewFiles"
            @clear="clearPreview"
          />

          <!-- Running: Progress Tracker -->
          <div
            v-if="scanRunning && activeReviewId"
            class="progress-section"
          >
            <ProgressTracker
              :review-id="activeReviewId"
              :review-status="scanState === 'queued' ? 'queued' : 'running'"
              :sessions="sessions"
              :history="reviewDetail?.history"
              :phase-definitions="scanPhaseDefinitions"
            />
            <el-alert
              v-if="scanWarning"
              class="scan-warning"
              type="warning"
              show-icon
              :closable="false"
              :title="scanWarning"
            />
            <div class="progress-actions">
              <el-button
                type="danger"
                plain
                :loading="cancelling"
                @click="cancelScan"
              >
                <el-icon><Close /></el-icon>
                取消扫描
              </el-button>
              <span
                v-if="elapsedText"
                class="elapsed"
              >已用时 {{ elapsedText }}</span>
            </div>
          </div>

          <!-- Failed -->
          <div
            v-if="scanState === 'failed'"
            class="failed-section"
          >
            <el-alert
              type="error"
              :closable="false"
              title="扫描失败"
              show-icon
            >
              <template #default>
                <p class="error-message">
                  {{ scanError || '未知错误' }}
                </p>
              </template>
            </el-alert>
            <el-button
              type="primary"
              @click="resetScan"
            >
              重新配置
            </el-button>
          </div>

          <!-- Success: Results -->
          <div
            v-if="scanState === 'success' && reviewDetail"
            class="scan-result"
          >
            <el-alert
              v-if="reviewDetail.review_id"
              class="saved-alert"
              type="success"
              :closable="false"
              title="扫描报告已保存"
            >
              <template #default>
                <div class="saved-alert-body">
                  <code>{{ reviewDetail.review_id }}</code>
                  <div class="saved-alert-actions">
                    <el-button
                      size="small"
                      type="success"
                      plain
                      @click="router.push(`/reviews/${reviewDetail.review_id}`)"
                    >
                      查看详情
                    </el-button>
                    <el-button
                      size="small"
                      @click="resetScan"
                    >
                      新建扫描
                    </el-button>
                  </div>
                </div>
              </template>
            </el-alert>

            <div class="metric-strip">
              <div
                v-for="m in metrics"
                :key="m.label"
                class="metric-card"
              >
                <small>{{ m.label }}</small>
                <strong :class="m.class">{{ m.value }}</strong>
              </div>
            </div>

            <el-alert
              v-if="reviewDetail.analysis?.overview"
              class="overview"
              type="info"
              :closable="false"
              :title="reviewDetail.analysis.overview"
            />

            <!-- Findings Table -->
            <ScanFindingsPanel :findings="reviewDetail?.findings ?? []" />
          </div>
        </section>
      </div>

      <!-- Confirmation Dialog -->
      <el-dialog
        v-model="confirmVisible"
        title="确认扫描"
        width="460px"
      >
        <div class="confirm-body">
          <p>即将对以下仓库执行全量扫描，将消耗 LLM Tokens：</p>
          <el-descriptions
            :column="1"
            border
            size="small"
          >
            <el-descriptions-item label="仓库">
              {{ form.repo }}
            </el-descriptions-item>
            <el-descriptions-item label="分支/提交">
              {{ branch }}
            </el-descriptions-item>
            <el-descriptions-item label="HEAD">
              {{ form.head_sha }}
            </el-descriptions-item>
            <el-descriptions-item
              v-if="form.max_tokens_budget > 0"
              label="Token 预算"
            >
              {{ form.max_tokens_budget.toLocaleString() }}
            </el-descriptions-item>
            <el-descriptions-item
              v-else
              label="Token 预算"
            >
              无上限
            </el-descriptions-item>
            <el-descriptions-item
              v-if="pathsText.trim()"
              label="扫描路径"
            >
              {{ pathsText.trim().split('\n').filter(Boolean).length }} 项
            </el-descriptions-item>
          </el-descriptions>
        </div>
        <template #footer>
          <el-button @click="confirmVisible = false">
            取消
          </el-button>
          <el-button
            type="primary"
            :loading="starting"
            @click="runScan(false)"
          >
            确认扫描
          </el-button>
        </template>
      </el-dialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { Files, Search, Check, Warning, Loading, Close } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProgressTracker from '@/components/ProgressTracker.vue'
import { getBranchSHA, getGitPlatforms, getProviderBranches, getProviderRepos } from '@/api/config'
import { cancelReview, getReview, scanRepository } from '@/api/review'
import { sessionApi, type ReviewSession } from '@/api/session'
import type { GitPlatform, PlatformBranch, PlatformRepo } from '@/types/config'
import type { ReviewDetail } from '@/types/review'
import ScanFindingsPanel from '@/components/scan/ScanFindingsPanel.vue'
import ScanPreviewPanel from '@/components/scan/ScanPreviewPanel.vue'
import type { PhaseDefinition } from '@/constants/phases'

const router = useRouter()
const formRef = ref<FormInstance>()

type ScanState = 'idle' | 'queued' | 'running' | 'success' | 'failed'

const platforms = ref<GitPlatform[]>([])
const repos = ref<PlatformRepo[]>([])
const branches = ref<PlatformBranch[]>([])
const reposLoading = ref(false)
const branchesLoading = ref(false)
const previewing = ref(false)
const starting = ref(false)
const cancelling = ref(false)
const previewFiles = ref<string[]>([])
const branch = ref('')
const branchTouched = ref(false)
const pathsText = ref('')
const excludeText = ref('')
const advancedOpen = ref<string[]>([])

const scanState = ref<ScanState>('idle')
const activeReviewId = ref('')
const reviewDetail = ref<ReviewDetail | null>(null)
const sessions = ref<ReviewSession[]>([])
const scanError = ref('')
const scanWarning = ref('')
const scanStartTime = ref<number | null>(null)
const elapsedText = ref('')
let pollTimer: ReturnType<typeof setTimeout> | null = null
let elapsedTimer: ReturnType<typeof setInterval> | null = null
// 轮询保护：超过 MAX_POLL_DURATION_MS 仍 running 则放弃（防止卡死的 review 导致页面无限轮询）
const MAX_POLL_DURATION_MS = 5 * 60 * 1000 // 5 分钟
let pollStartTime = 0
let pollFailureCount = 0

const confirmVisible = ref(false)

const STORAGE_KEY = 'scan-form-state'
const SCAN_SESSION_KEY = 'scan-session'

const scanPhaseDefinitions: PhaseDefinition[] = [
  {
    name: 'context',
    title: '准备工作区',
    description: '加载配置并检出目标仓库',
  },
  {
    name: 'change_context',
    title: '扫描范围',
    description: '解析路径、规则和文件过滤',
  },
  {
    name: 'analysis',
    title: '代码扫描',
    description: '运行 OCR 全量扫描并收集发现',
  },
  {
    name: 'reviews',
    title: '深度审查',
    description: '分析扫描发现并补充上下文',
  },
  {
    name: 'aggregation',
    title: '结果聚合',
    description: '合并并去重扫描发现',
  },
  {
    name: 'discourse',
    title: '风险确认',
    description: '校验关键风险和误报',
  },
  {
    name: 'synthesis',
    title: '生成报告',
    description: '整理评分、摘要和最终报告',
  },
  {
    name: 'complete',
    title: '完成',
    description: '扫描完成',
  },
]

const form = reactive({
  provider: '',
  repo: '',
  head_sha: '',
  background: '',
  max_tokens_budget: 0,
  batch: '' as '' | 'none' | 'by-language' | 'by-directory',
  no_plan: false,
  no_dedup: false,
  no_summary: false,
  timeout_minutes: 60,
})

const formRules: FormRules = {
  provider: [{ required: true, message: '请选择代码平台', trigger: 'change' }],
  repo: [{ required: true, message: '请选择仓库', trigger: 'change' }],
}

const canRun = computed(() => Boolean(form.provider && form.repo && form.head_sha))
const scanRunning = computed(() => scanState.value === 'queued' || scanState.value === 'running')
const hasPreview = computed(() => previewFiles.value.length > 0)

const metrics = computed(() => {
  const detail = reviewDetail.value
  if (!detail) return []
  const ocr = detail.report?.ocr
  return [
    { label: '发现问题', value: detail.findings?.length ?? 0, class: '' },
    { label: '扫描文件', value: ocr?.files_reviewed ?? 0, class: '' },
    { label: 'Tokens', value: formatNumber(ocr?.total_tokens), class: '' },
    { label: '质量评分', value: detail.report?.score?.value ?? '-', class: 'score' },
  ]
})

function lines(value: string) {
  return value.split('\n').map((l) => l.trim()).filter(Boolean)
}

function splitRepo(repo: string) {
  const parts = repo.split('/').filter(Boolean)
  return { owner: parts.slice(0, -1).join('/'), name: parts[parts.length - 1] || '' }
}

function formatNumber(value?: number) {
  return value ? value.toLocaleString() : '-'
}

function stripAnsi(value: string) {
  const escapeChar = String.fromCharCode(27)
  return value.replace(new RegExp(`${escapeChar}\\[[0-9;?]*[ -/]*[@-~]`, 'g'), '')
}

function parsePreviewLine(line: string) {
  let value = stripAnsi(line).trim()
  if (!value) return ''
  if (value.startsWith('[')) {
    const end = value.indexOf(']')
    if (end >= 0) value = value.slice(end + 1).trim()
  }
  value = value.replace(/\s+\((unsupported_ext|excluded|ignored|binary|too_large|generated)[^)]+\)\s*$/i, '').trim()
  value = value.replace(/\s+[+-]\d+.*$/, '').trim()
  return value
}

function normalizePreviewOutput(values: string[]) {
  const reviewable: string[] = []
  const excluded: string[] = []
  let totalChanged = 0
  let section: 'reviewable' | 'excluded' | '' = ''

  for (const raw of values) {
    const line = stripAnsi(raw).trim()
    if (!line) continue
    const summary = line.match(/^Preview:\s*(\d+)\s+file/i)
    if (summary) {
      totalChanged = Number(summary[1]) || totalChanged
      continue
    }
    if (/^Will review/i.test(line)) {
      section = 'reviewable'
      continue
    }
    if (/^Excluded from review/i.test(line)) {
      section = 'excluded'
      continue
    }
    const file = parsePreviewLine(line)
    if (!file) continue
    if (section === 'excluded' || /\((unsupported_ext|excluded|ignored|binary|too_large|generated)/i.test(line)) {
      excluded.push(file)
    } else if (section === 'reviewable' || !/^Preview:/i.test(file)) {
      reviewable.push(file)
    }
  }

  return {
    totalChanged,
    reviewable: Array.from(new Set(reviewable)),
    excluded: Array.from(new Set(excluded)),
  }
}

// ===== Storage =====

function saveFormState() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify({
      provider: form.provider,
      repo: form.repo,
      branch: branch.value,
      head_sha: form.head_sha,
      pathsText: pathsText.value,
      excludeText: excludeText.value,
      background: form.background,
      max_tokens_budget: form.max_tokens_budget,
      batch: form.batch,
      no_plan: form.no_plan,
      no_dedup: form.no_dedup,
      no_summary: form.no_summary,
      timeout_minutes: form.timeout_minutes,
    }))
  } catch { /* ignore */ }
}

function restoreFormState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return
    const saved = JSON.parse(raw)
    form.provider = saved.provider || ''
    form.repo = saved.repo || ''
    branch.value = saved.branch || ''
    form.head_sha = saved.head_sha || ''
    pathsText.value = saved.pathsText || ''
    excludeText.value = saved.excludeText || ''
    form.background = saved.background || ''
    form.max_tokens_budget = saved.max_tokens_budget ?? 0
    form.batch = saved.batch ?? ''
    form.no_plan = saved.no_plan ?? false
    form.no_dedup = saved.no_dedup ?? false
    form.no_summary = saved.no_summary ?? false
    form.timeout_minutes = saved.timeout_minutes ?? 60
  } catch { /* ignore */ }
}

function saveScanSession() {
  if (activeReviewId.value && scanRunning.value) {
    try {
      localStorage.setItem(SCAN_SESSION_KEY, JSON.stringify({
        review_id: activeReviewId.value,
        start_time: scanStartTime.value,
      }))
    } catch { /* ignore */ }
  } else {
    try { localStorage.removeItem(SCAN_SESSION_KEY) } catch { /* ignore */ }
  }
}

function clearScanSession() {
  try { localStorage.removeItem(SCAN_SESSION_KEY) } catch { /* ignore */ }
}

async function resumeScanSession() {
  try {
    const raw = localStorage.getItem(SCAN_SESSION_KEY)
    if (!raw) return
    const session = JSON.parse(raw)
    if (!session.review_id) return
    activeReviewId.value = session.review_id
    scanStartTime.value = session.start_time ?? null
    scanState.value = 'running'
    startPolling()
    startElapsedTimer()
    await pollReview()
  } catch { /* ignore */ }
}

// ===== Data Loading =====

async function loadPlatforms() {
  try {
    const data = await getGitPlatforms()
    platforms.value = data.items || []
    if (!form.provider && platforms.value.length) form.provider = platforms.value[0].provider
  } catch (e: any) {
    ElMessage.error(e.message || '获取平台列表失败')
  }
}

async function searchRepos(query: string) {
  if (!form.provider || reposLoading.value) return
  reposLoading.value = true
  try {
    repos.value = await getProviderRepos(form.provider, 1, query ? 100 : 50)
  } catch (e: any) {
    ElMessage.error(e.message || '搜索仓库失败')
  } finally {
    reposLoading.value = false
  }
}

async function loadBranches() {
  if (!form.provider || !form.repo || branchesLoading.value) return
  const { owner, name } = splitRepo(form.repo)
  if (!owner || !name) return
  branchesLoading.value = true
  try {
    branches.value = await getProviderBranches(form.provider, owner, name)
  } catch (e: any) {
    ElMessage.error(e.message || '获取分支失败')
  } finally {
    branchesLoading.value = false
  }
}

function handleProviderChange() {
  form.repo = ''
  form.head_sha = ''
  branch.value = ''
  branchTouched.value = false
  repos.value = []
  branches.value = []
  saveFormState()
}

function handleRepoChange() {
  form.head_sha = ''
  branches.value = []
  branchTouched.value = true
  const repo = repos.value.find((item) => item.full_name === form.repo)
  branch.value = repo?.default_branch || ''
  if (branch.value) resolveHead()
  saveFormState()
}

async function resolveHead() {
  branchTouched.value = true
  if (!form.provider || !form.repo || !branch.value) return
  const looksLikeSHA = /^[0-9a-f]{7,64}$/i.test(branch.value.trim())
  if (looksLikeSHA) {
    form.head_sha = branch.value.trim()
    return
  }
  const { owner, name } = splitRepo(form.repo)
  if (!owner || !name) return
  try {
    const sha = await getBranchSHA(form.provider, owner, name, branch.value)
    form.head_sha = sha || ''
    if (!form.head_sha) ElMessage.warning('没有解析到分支 SHA')
  } catch (e: any) {
    ElMessage.error(e.message || '解析分支失败')
  }
}

// ===== Scan Flow =====

function confirmScan() {
  if (!formRef.value) return
  formRef.value.validate((valid) => {
    if (!valid) return
    if (!branch.value) {
      ElMessage.warning('请选择或输入分支')
      return
    }
    if (!form.head_sha) {
      ElMessage.warning('请先等待分支解析完成')
      return
    }
    confirmVisible.value = true
  })
}

async function runScan(preview: boolean) {
  if (!canRun.value) return
  if (!preview) confirmVisible.value = false

  if (preview) {
    previewing.value = true
    try {
      const data = await scanRepository({
        ...form,
        paths: lines(pathsText.value),
        exclude: lines(excludeText.value),
        preview: true,
      })
      previewFiles.value = data.preview_files || []
      const info = normalizePreviewOutput(previewFiles.value)
      ElMessage.success(`预览完成：将审查 ${info.reviewable.length} 个文件`)
    } catch (e: any) {
      ElMessage.error(e.message || '预览失败')
    } finally {
      previewing.value = false
    }
    return
  }

  starting.value = true
  saveFormState()
  try {
    const data = await scanRepository({
      ...form,
      paths: lines(pathsText.value),
      exclude: lines(excludeText.value),
      preview: false,
    })
    if (data.review_id) {
      activeReviewId.value = data.review_id
      scanState.value = 'queued'
      scanWarning.value = ''
      pollFailureCount = 0
      scanStartTime.value = Date.now()
      saveScanSession()
      startPolling()
      startElapsedTimer()
    }
  } catch (e: any) {
    ElMessage.error(e.message || '启动扫描失败')
    scanState.value = 'failed'
    scanError.value = e.message || '启动扫描失败'
  } finally {
    starting.value = false
  }
}

function startPolling() {
  if (pollTimer) clearTimeout(pollTimer)
  if (!pollStartTime) pollStartTime = Date.now()
  pollTimer = setTimeout(async () => {
    await pollReview()
    if (scanState.value === 'queued' || scanState.value === 'running') {
      if (document.visibilityState === 'visible') startPolling()
      else pollTimer = null
    }
  }, 3000)
}

async function fetchSessions() {
  if (!activeReviewId.value) return
  try {
    const response = await sessionApi.getSessions(activeReviewId.value)
    sessions.value = response.sessions || []
  } catch {
    sessions.value = []
  }
}

async function pollReview() {
  if (!activeReviewId.value) return
  try {
    const detail = await getReview(activeReviewId.value)
    pollFailureCount = 0
    scanWarning.value = ''
    reviewDetail.value = detail
    if (detail.status === 'success' || detail.status === 'completed') {
      scanState.value = 'success'
      stopPolling()
      stopElapsedTimer()
      clearScanSession()
      ElMessage.success('扫描完成')
      try {
        const final = await getReview(activeReviewId.value)
        reviewDetail.value = final
      } catch {
        // keep the previous detail
      }
      await fetchSessions()
    } else if (detail.status === 'failed') {
      scanState.value = 'failed'
      scanError.value = detail.error || '扫描失败'
      stopPolling()
      stopElapsedTimer()
      clearScanSession()
    } else if (detail.status === 'running') {
      // 轮询超时保护：超过 MAX_POLL_DURATION_MS 仍 running 则放弃，避免页面卡死。
      if (pollStartTime && Date.now() - pollStartTime > MAX_POLL_DURATION_MS) {
        scanState.value = 'failed'
        scanError.value = '扫描超时：任务运行时间过长，可能已停止响应。请重新发起扫描。'
        stopPolling()
        stopElapsedTimer()
        clearScanSession()
      } else {
        scanState.value = 'running'
      }
    }
    await fetchSessions()
  } catch (e: any) {
    pollFailureCount += 1
    if (pollFailureCount >= 2) {
      scanWarning.value = e?.message
        ? `服务连接异常，正在重试：${e.message}`
        : '服务连接异常，正在重试。扫描任务可能仍在后端执行。'
    }
  }
}

function stopPolling() {
  if (pollTimer) { clearTimeout(pollTimer); pollTimer = null }
  pollStartTime = 0
}

function startElapsedTimer() {
  if (elapsedTimer) clearInterval(elapsedTimer)
  elapsedTimer = setInterval(() => {
    if (!scanStartTime.value) return
    const sec = Math.floor((Date.now() - scanStartTime.value) / 1000)
    if (sec < 60) elapsedText.value = `${sec}s`
    else elapsedText.value = `${Math.floor(sec / 60)}m ${sec % 60}s`
  }, 1000)
}

function stopElapsedTimer() {
  if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
}

async function cancelScan() {
  if (!activeReviewId.value) return
  cancelling.value = true
  try {
    await cancelReview(activeReviewId.value)
    scanState.value = 'failed'
    scanError.value = '扫描已取消'
    scanWarning.value = ''
    pollFailureCount = 0
    stopPolling()
    stopElapsedTimer()
    clearScanSession()
  } catch (e: any) {
    ElMessage.error(e.message || '取消失败')
  } finally {
    cancelling.value = false
  }
}

function resetScan() {
  scanState.value = 'idle'
  activeReviewId.value = ''
  reviewDetail.value = null
  sessions.value = []
  scanError.value = ''
  scanWarning.value = ''
  pollFailureCount = 0
  scanStartTime.value = null
  elapsedText.value = ''
  previewFiles.value = []
  clearScanSession()
}

function clearPreview() {
  previewFiles.value = []
}

watch(() => form, saveFormState, { deep: true })
watch([branch, pathsText, excludeText], saveFormState)

function onVisibilityChange() {
  if (document.visibilityState === 'visible' && scanRunning.value && !pollTimer) {
    startPolling()
  }
}

onMounted(async () => {
  restoreFormState()
  await loadPlatforms()
  if (form.provider && form.repo) {
    searchRepos('')
    loadBranches()
  }
  document.addEventListener('visibilitychange', onVisibilityChange)
  resumeScanSession()
})

onBeforeUnmount(() => {
  stopPolling()
  stopElapsedTimer()
  document.removeEventListener('visibilitychange', onVisibilityChange)
})
</script>

<style scoped>
.scan-page {
  width: min(1320px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  margin-bottom: 24px;
}

.page-header h1 {
  margin: 0;
  font-size: 26px;
  color: var(--app-text);
}

.page-header p {
  margin: 4px 0 0;
  color: var(--app-muted);
  font-size: 14px;
}

.header-status .el-tag .el-icon {
  margin-right: 4px;
}

.scan-grid {
  display: grid;
  grid-template-columns: minmax(380px, 440px) minmax(0, 1fr);
  gap: 18px;
  align-items: start;
}

.panel {
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-surface);
}

.config-panel {
  padding: 20px;
}

.result-panel {
  padding: 20px;
  min-height: 400px;
}

/* ===== Form Cards ===== */

.form-card {
  padding-bottom: 8px;
  margin-bottom: 8px;
}

.form-card + .form-card {
  padding-top: 16px;
  border-top: 1px solid var(--app-border);
}

.form-card-title {
  font-size: 14px;
  font-weight: 700;
  color: var(--app-text);
  margin-bottom: 14px;
}

.branch-row {
  display: flex;
  gap: 8px;
  width: 100%;
}

.branch-row .el-select {
  flex: 1;
}

.form-tip {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 6px;
  font-size: 12px;
  color: var(--app-muted);
}

.form-tip.error-tip {
  color: var(--app-danger);
}

.inline-fields {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.switch-row {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
  margin-top: 4px;
}

.advanced-collapse {
  border: none;
  margin-bottom: 8px;
}

.advanced-collapse :deep(.el-collapse-item__header) {
  font-weight: 600;
  color: var(--app-muted);
  border-bottom: 1px solid var(--app-border);
}

.advanced-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: none;
}

.advanced-collapse :deep(.el-collapse-item__content) {
  padding-top: 12px;
}

.action-row {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 16px;
}

/* ===== Result: Empty / Preview ===== */

.result-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 400px;
}

.preview-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.section-head h2 {
  margin: 0;
  font-size: 18px;
  color: var(--app-text);
}

.section-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.preview-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

.preview-summary.has-excluded {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.preview-stat {
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-bg);
}

.preview-stat span,
.preview-stat strong {
  display: block;
}

.preview-stat span {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
}

.preview-stat strong {
  margin-top: 4px;
  color: var(--app-text);
  font-size: 22px;
}

.preview-stat.primary strong {
  color: var(--app-primary);
}

.file-list {
  display: grid;
  max-height: 500px;
  overflow: auto;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-bg);
}

.file-list.empty {
  display: block;
}

.file-row {
  display: grid;
  grid-template-columns: 54px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--app-border);
}

.file-row:last-child {
  border-bottom: 0;
}

.file-row code {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-badge {
  display: inline-flex;
  justify-content: center;
  width: 42px;
  border-radius: 6px;
  color: #0f766e;
  background: #dff7f1;
  font-size: 11px;
  font-weight: 800;
  line-height: 22px;
}

.file-badge.muted {
  color: #7a8292;
  background: #edf0f5;
}

.file-list-empty {
  padding: 24px;
  text-align: center;
  color: var(--app-muted);
}

.excluded-collapse {
  border-top: 1px solid var(--app-border);
  border-bottom: 0;
}

.excluded-collapse :deep(.el-collapse-item__header) {
  color: var(--app-muted);
  font-size: 13px;
  font-weight: 700;
}

.excluded-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
}

.excluded-list {
  max-height: 260px;
}

/* ===== Progress ===== */

.progress-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.scan-warning {
  border-radius: 8px;
}

.progress-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.elapsed {
  font-size: 13px;
  color: var(--app-muted);
}

/* ===== Failed ===== */

.failed-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.error-message {
  margin: 0;
  word-break: break-word;
  white-space: pre-wrap;
}

/* ===== Results ===== */

.scan-result {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.saved-alert {
  --el-alert-padding: 12px 14px;
}

.saved-alert-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-top: 6px;
}

.saved-alert-actions {
  display: flex;
  gap: 8px;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.metric-card {
  padding: 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-bg);
}

.metric-card small {
  display: block;
  margin-bottom: 8px;
  color: var(--app-muted);
  font-size: 12px;
}

.metric-card strong {
  font-size: 24px;
  color: var(--app-text);
}

.metric-card strong.score {
  color: var(--app-primary);
}

.overview {
  --el-alert-padding: 12px 14px;
}

.findings-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* Findings table */
.finding-path {
  color: var(--app-primary);
  font-size: 12px;
}

.finding-body {
  margin: 0;
  color: var(--app-muted);
  white-space: pre-wrap;
  line-height: 1.6;
}

/* Finding cards (grouped view) */
.finding-groups :deep(.el-collapse-item__header) {
  gap: 8px;
}

.group-path {
  font-size: 13px;
  color: var(--app-primary);
}

.group-badge {
  margin-left: auto;
  margin-right: 12px;
}

.finding-cards {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.finding-card {
  padding: 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.finding-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.finding-line {
  font-size: 12px;
  color: var(--app-muted);
  margin-left: auto;
}

.finding-code {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-top: 10px;
}

@media (max-width: 720px) {
  .finding-code {
    grid-template-columns: 1fr;
  }
}

.code-block small {
  display: block;
  margin-bottom: 4px;
  font-size: 11px;
  color: var(--app-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.code-block pre {
  margin: 0;
  padding: 10px;
  background: var(--app-bg);
  border: 1px solid var(--app-border);
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.5;
  overflow-x: auto;
  max-height: 200px;
}

.pagination-wrap {
  display: flex;
  justify-content: center;
  margin-top: 8px;
}

/* ===== Confirm Dialog ===== */

.confirm-body p {
  margin: 0 0 12px;
  color: var(--app-muted);
}

/* ===== Responsive ===== */

@media (max-width: 1100px) {
  .metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 980px) {
  .scan-page {
    padding: 22px 16px 32px;
  }

  .scan-grid {
    grid-template-columns: 1fr;
  }

  .page-header {
    flex-direction: column;
    align-items: flex-start;
  }

  .inline-fields {
    grid-template-columns: 1fr;
  }

  .preview-summary {
    grid-template-columns: 1fr;
  }
}
</style>
