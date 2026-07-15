<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="REPOSITORY SCAN"
        title="代码库扫描"
        description="对整个仓库或指定目录执行全量代码审查，在合并前一次性看清风险全貌。"
      >
        <template #actions>
          <span class="status-pill" :class="scanState">
            <el-icon v-if="scanState === 'queued' || scanState === 'running'" class="is-loading">
              <Loading />
            </el-icon>
            <el-icon v-else-if="scanState === 'success'"><CircleCheckFilled /></el-icon>
            <el-icon v-else-if="scanState === 'failed'"><CircleCloseFilled /></el-icon>
            <el-icon v-else><SetUp /></el-icon>
            {{ statusLabel }}
          </span>
        </template>
      </PageHeader>

      <StatStrip :items="scanStats" />

      <!-- ===== Restore hint ===== -->
      <el-alert
        v-if="restoredFromCache"
        class="restore-hint"
        type="info"
        :closable="false"
        show-icon
        title="已恢复上次扫描配置"
      >
        <template #default>
          <div class="restore-body">
            <span>可直接发起扫描，或清除后重新填写。</span>
            <el-button size="small" plain @click="clearForm">清除重填</el-button>
          </div>
        </template>
      </el-alert>

      <div class="scan-grid">
        <!-- ===== Left: Config ===== -->
        <section class="panel-shell config-panel">
          <div class="panel-heading">
            <div>
              <strong>扫描配置</strong>
              <span>填写仓库与范围后即可发起扫描</span>
            </div>
          </div>

          <el-form
            ref="formRef"
            class="config-form"
            label-position="top"
            :model="form"
            :rules="formRules"
            :disabled="scanState === 'queued' || scanState === 'running'"
          >
            <!-- Step 1: Basic Info -->
            <div class="form-step">
              <div class="step-index">1</div>
              <div class="form-card">
                <div class="form-card-title">基础信息</div>

                <el-form-item label="代码平台" prop="provider">
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

                <el-form-item label="仓库" prop="repo">
                  <RepoSelect
                    v-model="form.repo"
                    :provider="form.provider"
                    secondary-field="none"
                    placeholder="搜索仓库名称"
                    @change="handleRepoChange"
                  />
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
                  <div v-if="form.head_sha" class="form-tip">
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
            </div>

            <!-- Step 2: Scan Scope -->
            <div class="form-step">
              <div class="step-index">2</div>
              <div class="form-card">
                <div class="form-card-title">扫描范围</div>

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
            </div>

            <!-- Step 3: Advanced (collapsible) -->
            <el-collapse v-model="advancedOpen" class="advanced-collapse">
              <el-collapse-item name="advanced">
                <template #title>
                  <span class="advanced-title">高级选项</span>
                </template>
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
                      <el-option label="默认" value="" />
                      <el-option label="按语言" value="by-language" />
                      <el-option label="按目录" value="by-directory" />
                      <el-option label="不分批" value="none" />
                    </el-select>
                  </el-form-item>
                </div>

                <div class="switch-row">
                  <el-checkbox v-model="form.no_plan">跳过计划</el-checkbox>
                  <el-checkbox v-model="form.no_dedup">跳过去重</el-checkbox>
                  <el-checkbox v-model="form.no_summary">跳过总结</el-checkbox>
                </div>
              </el-collapse-item>
            </el-collapse>

            <!-- Action row -->
            <div class="action-row">
              <el-button
                plain
                :loading="previewing"
                :disabled="!canRun || scanRunning"
                @click="runScan(true)"
              >
                <el-icon><Files /></el-icon>
                预览文件
              </el-button>
              <el-button
                type="primary"
                size="large"
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
        <section class="panel-shell result-panel">
          <!-- Idle -->
          <div v-if="scanState === 'idle' && !hasPreview" class="result-empty">
            <div class="empty-card">
              <el-icon class="empty-icon"><Search /></el-icon>
              <h3>尚未扫描</h3>
              <p>填写左侧配置，即可对整个仓库发起全量审查。</p>
              <ol class="empty-steps">
                <li><strong>选平台</strong><span class="step-sep">—</span>选择代码托管平台</li>
                <li><strong>选仓库</strong><span class="step-sep">—</span>搜索并定位目标仓库与分支</li>
                <li><strong>点扫描</strong><span class="step-sep">—</span>预览范围或直接开始全量扫描</li>
              </ol>
            </div>
          </div>

          <!-- Preview -->
          <ScanPreviewPanel
            v-if="hasPreview && scanState === 'idle'"
            :files="previewFiles"
            @clear="clearPreview"
          />

          <!-- Running: Progress Tracker -->
          <div v-if="scanRunning && activeReviewId" class="progress-section">
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
            <div class="progress-toolbar">
              <el-button
                type="danger"
                plain
                :loading="cancelling"
                @click="cancelScan"
              >
                <el-icon><Close /></el-icon>
                取消扫描
              </el-button>
              <el-tag v-if="elapsedText" type="info" effect="plain" round>
                已用时 {{ elapsedText }}
              </el-tag>
            </div>
          </div>

          <!-- Failed -->
          <div v-if="scanState === 'failed'" class="failed-section">
            <el-alert type="error" :closable="false" title="扫描失败" show-icon>
              <template #default>
                <p class="error-message">{{ scanError || '未知错误' }}</p>
              </template>
            </el-alert>
            <div class="failed-actions">
              <el-button type="primary" @click="resetScan">
                <el-icon><RefreshLeft /></el-icon>
                重新配置
              </el-button>
              <el-button plain @click="copyError">
                <el-icon><CopyDocument /></el-icon>
                复制错误信息
              </el-button>
            </div>
          </div>

          <!-- Success: Results -->
          <div v-if="scanState === 'success' && reviewDetail" class="scan-result">
            <div class="result-toolbar">
              <div class="result-toolbar-id">
                <el-icon class="result-toolbar-icon"><CircleCheckFilled /></el-icon>
                <div>
                  <small>扫描报告已保存</small>
                  <code>{{ reviewDetail.review_id }}</code>
                </div>
              </div>
              <div class="result-toolbar-actions">
                <el-dropdown trigger="click" @command="exportScan">
                  <el-button plain>
                    <el-icon><Download /></el-icon>
                    导出报告
                    <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                  </el-button>
                  <template #dropdown>
                    <el-dropdown-menu>
                      <el-dropdown-item command="markdown">Markdown</el-dropdown-item>
                      <el-dropdown-item command="json">JSON</el-dropdown-item>
                    </el-dropdown-menu>
                  </template>
                </el-dropdown>
                <el-button type="primary" @click="router.push(`/reviews/${reviewDetail.review_id}`)">
                  查看详情
                </el-button>
                <el-button @click="resetScan">新建扫描</el-button>
              </div>
            </div>

            <div class="metric-strip">
              <div
                v-for="m in metrics"
                :key="m.label"
                class="metric-card"
                :class="m.tone"
              >
                <small>{{ m.label }}</small>
                <strong>{{ m.value }}</strong>
              </div>
            </div>

            <!-- Severity breakdown bar -->
            <div v-if="severityBreakdown.total > 0" class="severity-bar-wrap">
              <div class="severity-bar-label">
                <span>风险分布</span>
                <small>共 {{ severityBreakdown.total }} 项发现</small>
              </div>
              <div class="severity-bar">
                <div
                  v-for="seg in severityBreakdown.segments"
                  :key="seg.key"
                  class="severity-seg"
                  :class="seg.key"
                  :style="{ width: seg.percent + '%' }"
                  :title="`${seg.label}: ${seg.count}`"
                />
              </div>
              <div class="severity-legend">
                <span
                  v-for="seg in severityBreakdown.segments"
                  :key="seg.key"
                  class="legend-item"
                >
                  <i class="legend-dot" :class="seg.key" />
                  {{ seg.label }}
                  <strong>{{ seg.count }}</strong>
                </span>
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
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import {
  Files, Search, Check, Warning, Loading, Close,
  CircleCheckFilled, CircleCloseFilled, SetUp, RefreshLeft,
  CopyDocument, Download, ArrowDown,
  Connection, FolderOpened, Discount, Aim,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import ProgressTracker from '@/components/ProgressTracker.vue'
import { getBranchSHA, getGitPlatforms, getProviderBranches } from '@/api/config'
import { cancelReview, getReview, scanRepository } from '@/api/review'
import { sessionApi, type ReviewSession } from '@/api/session'
import type { GitPlatform, PlatformBranch, PlatformRepo } from '@/types/config'
import type { ReviewDetail } from '@/types/review'
import ScanFindingsPanel from '@/components/scan/ScanFindingsPanel.vue'
import ScanPreviewPanel from '@/components/scan/ScanPreviewPanel.vue'
import RepoSelect from '@/components/common/RepoSelect.vue'
import type { PhaseDefinition } from '@/constants/phases'
import { exportReport, type ReviewReport } from '@/utils/export'
import { splitRepo } from '@/utils/repo'

const router = useRouter()
const formRef = ref<FormInstance>()

type ScanState = 'idle' | 'queued' | 'running' | 'success' | 'failed'

const platforms = ref<GitPlatform[]>([])
const branches = ref<PlatformBranch[]>([])
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
// 连续轮询失败（非 404）超过此次数也放弃，避免服务异常时无限重试。
const MAX_POLL_FAILURES = 10
let pollStartTime = 0
let pollFailureCount = 0

const confirmVisible = ref(false)
// 标记本次表单是否从 localStorage 恢复过（用于展示"已恢复上次配置"提示）
const restoredFromCache = ref(false)

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

const statusLabel = computed(() => {
  switch (scanState.value) {
    case 'idle': return canRun.value ? '已配置' : '待配置'
    case 'queued': return '排队中'
    case 'running': return '扫描中'
    case 'success': return '已完成'
    case 'failed': return '扫描失败'
    default: return ''
  }
})

// 预计范围：基于扫描路径条数或"全仓库"
const scopeLabel = computed(() => {
  const count = lines(pathsText.value).length
  if (count > 0) return `${count} 个路径`
  return '整个仓库'
})

const scanStats = computed(() => [
  { icon: Connection, label: '代码平台', value: form.provider || '—', tone: 'primary' as const },
  { icon: FolderOpened, label: '目标仓库', value: form.repo || '—', tone: 'info' as const },
  { icon: Discount, label: '当前分支', value: branch.value || '—', tone: 'warning' as const },
  { icon: Aim, label: '预计范围', value: scopeLabel.value, tone: 'success' as const },
])

// 质量评分着色：≥80 成功 / 60-79 警告 / <60 危险
function scoreTone(score?: number) {
  if (score == null) return ''
  if (score >= 80) return 'tone-success'
  if (score >= 60) return 'tone-warning'
  return 'tone-danger'
}

const metrics = computed(() => {
  const detail = reviewDetail.value
  if (!detail) return []
  const ocr = detail.report?.ocr
  const findingCount = detail.findings?.length ?? 0
  const scoreValue = detail.report?.score?.value
  return [
    {
      label: '发现问题',
      value: findingCount,
      tone: findingCount > 0 ? 'tone-warning' : 'tone-success',
    },
    { label: '扫描文件', value: ocr?.files_reviewed ?? 0, tone: '' },
    { label: 'Tokens', value: formatNumber(ocr?.total_tokens), tone: '' },
    {
      label: '质量评分',
      value: scoreValue ?? '-',
      tone: scoreTone(scoreValue),
    },
  ]
})

// 严重级别分布：用于成功态的堆叠条
const severityBreakdown = computed(() => {
  const findings = reviewDetail.value?.findings ?? []
  const total = findings.length
  const defs = [
    { key: 'critical', label: 'Critical' },
    { key: 'major', label: 'Major' },
    { key: 'minor', label: 'Minor' },
    { key: 'info', label: 'Info' },
  ]
  const segments = defs.map((d) => {
    const count = findings.filter((f) => f.severity === d.key).length
    return {
      ...d,
      count,
      percent: total > 0 ? (count / total) * 100 : 0,
    }
  })
  return { total, segments }
})

function lines(value: string) {
  return value.split('\n').map((l) => l.trim()).filter(Boolean)
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
    // 只有恢复出有效字段（platform/repo/branch 之一）才提示
    const hasAny = saved.provider || saved.repo || saved.branch
    restoredFromCache.value = Boolean(hasAny)
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
    // 恢复前先校验 review 确实存在且仍在运行/排队，避免恢复一个已失效的 session
    // 后陷入 404 无限重试。校验失败直接清掉残留 session。
    const detail = await getReview(session.review_id)
    if (!['queued', 'running'].includes(detail.status)) {
      clearScanSession()
      return
    }
    activeReviewId.value = session.review_id
    scanStartTime.value = session.start_time ?? null
    scanState.value = detail.status as ScanState
    reviewDetail.value = detail
    pollStartTime = Date.now()
    pollFailureCount = 0
    startPolling()
    startElapsedTimer()
    await fetchSessions()
  } catch {
    // review 不存在或查询失败：清掉残留 session，不进入卡死的轮询。
    clearScanSession()
  }
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

async function loadBranches() {
  if (!form.provider || !form.repo || branchesLoading.value) return
  const repoParts = splitRepo(form.repo)
  if (!repoParts) return
  branchesLoading.value = true
  try {
    branches.value = await getProviderBranches(form.provider, repoParts.owner, repoParts.repo)
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
  branches.value = []
  saveFormState()
}

function handleRepoChange(_value: string, repo?: PlatformRepo) {
  form.head_sha = ''
  branches.value = []
  branchTouched.value = true
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
  const repoParts = splitRepo(form.repo)
  if (!repoParts) return
  try {
    const sha = await getBranchSHA(form.provider, repoParts.owner, repoParts.repo, branch.value)
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
    // 区分"review 不存在"（404）与"网络抖动"：
    // - 404 说明 review 已被删除或 session 数据损坏，立即放弃，避免无限重试导致页面卡死在"扫描中"。
    // - 其它错误（超时、5xx、网络断开）按可重试处理，连续失败超过阈值后也放弃。
    const status = e?.response?.status || e?.statusCode
    const isNotFound = status === 404 || /not found|404/i.test(e?.message || '')
    if (isNotFound) {
      scanState.value = 'failed'
      scanError.value = '扫描任务不存在或已被删除。可能是之前的扫描记录已清理，请重新发起扫描。'
      stopPolling()
      stopElapsedTimer()
      clearScanSession()
      return
    }
    pollFailureCount += 1
    if (pollFailureCount >= MAX_POLL_FAILURES) {
      scanState.value = 'failed'
      scanError.value = '连续多次无法获取扫描进度，可能服务异常。请稍后重试。'
      stopPolling()
      stopElapsedTimer()
      clearScanSession()
      return
    }
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

// 清除表单回到空白（用户点击"清除重填"）
function clearForm() {
  form.provider = ''
  form.repo = ''
  form.head_sha = ''
  form.background = ''
  form.max_tokens_budget = 0
  form.batch = ''
  form.no_plan = false
  form.no_dedup = false
  form.no_summary = false
  form.timeout_minutes = 60
  branch.value = ''
  branchTouched.value = false
  pathsText.value = ''
  excludeText.value = ''
  branches.value = []
  restoredFromCache.value = false
  try { localStorage.removeItem(STORAGE_KEY) } catch { /* ignore */ }
}

// 复制失败错误信息，便于反馈/排查
async function copyError() {
  const text = scanError.value || '未知错误'
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('错误信息已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择文本复制')
  }
}

// 导出当前扫描结果（复用通用 exportReport 工具）
function exportScan(format: 'markdown' | 'json') {
  const detail = reviewDetail.value
  if (!detail) return
  const report: ReviewReport = {
    reviewId: detail.review_id,
    provider: detail.provider,
    repo: detail.repo,
    status: detail.status,
    summary: detail.summary || detail.analysis?.overview || '',
    score: detail.report?.score?.value ?? 0,
    grade: detail.report?.score?.grade || '',
    createdAt: detail.created_at,
    updatedAt: detail.updated_at,
    findings: (detail.findings ?? []).map((f) => ({
      severity: f.severity,
      category: f.category || '',
      path: f.path || '',
      line: f.start_line ?? f.line ?? 0,
      title: f.title,
      body: f.body,
      suggestion: f.suggestion_code,
    })),
  }
  try {
    exportReport(report, {
      format,
      includeDetails: true,
      includeFindings: true,
      includeSummary: true,
    })
    ElMessage.success(`已导出 ${format.toUpperCase()} 报告`)
  } catch (e: any) {
    ElMessage.error(e?.message || '导出失败')
  }
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
.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 7px 14px;
  border-radius: 999px;
  background: var(--app-bg);
  color: var(--app-muted);
  font-size: 13px;
  font-weight: 700;
  border: 1px solid var(--app-border);
}

.status-pill.idle { color: var(--app-muted); }
.status-pill.queued,
.status-pill.running {
  color: var(--app-primary);
  background: #eaf1ff;
  border-color: #cdddff;
}
.status-pill.success {
  color: var(--app-success);
  background: #e3f7ef;
  border-color: #c3ebd9;
}
.status-pill.failed {
  color: var(--app-danger);
  background: #fdecee;
  border-color: #f8d2d7;
}

.status-pill .is-loading {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* ===== Restore hint ===== */

.restore-hint {
  --el-alert-padding: 10px 14px;
  margin-bottom: 16px;
  border-radius: var(--card-radius);
}

.restore-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  width: 100%;
}

/* ===== Grid / Panels ===== */

.scan-grid {
  display: grid;
  grid-template-columns: minmax(380px, 440px) minmax(0, 1fr);
  gap: 16px;
  align-items: start;
}

.panel-shell {
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius-lg);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
}

.config-panel {
  overflow: hidden;
}

.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 54px;
  padding: 15px 20px 12px;
  border-bottom: 1px solid var(--app-border);
}

.panel-heading strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.panel-heading span {
  display: block;
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 12px;
}

.config-form {
  padding: 18px 20px 20px;
}

.result-panel {
  padding: 20px;
  min-height: 420px;
}

/* ===== Form steps ===== */

.form-step {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 12px;
  padding-bottom: 16px;
}

.form-step + .form-step {
  padding-top: 4px;
}

.step-index {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  width: 24px;
  height: 24px;
  margin-top: 2px;
  border-radius: 50%;
  background: var(--app-primary);
  color: #fff;
  font-size: 12px;
  font-weight: 800;
  line-height: 24px;
  flex: 0 0 auto;
}

.form-card {
  min-width: 0;
}

.form-card-title {
  margin-bottom: 12px;
  color: var(--app-text);
  font-size: 14px;
  font-weight: 700;
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

.form-tip code {
  overflow: hidden;
  max-width: 100%;
  text-overflow: ellipsis;
  white-space: nowrap;
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

.advanced-title {
  font-size: 14px;
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
  padding-top: 16px;
  border-top: 1px solid var(--app-border);
}

/* ===== Result: Empty ===== */

.result-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 420px;
}

.empty-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  max-width: 420px;
  padding: 32px 24px;
  text-align: center;
}

.empty-icon {
  font-size: 44px;
  color: var(--app-primary);
  opacity: 0.55;
  margin-bottom: 12px;
}

.empty-card h3 {
  margin: 0 0 6px;
  color: var(--app-text);
  font-size: 18px;
}

.empty-card p {
  margin: 0 0 18px;
  color: var(--app-muted);
  font-size: 13px;
}

.empty-steps {
  display: grid;
  gap: 8px;
  margin: 0;
  padding: 14px 18px;
  list-style: none;
  text-align: left;
  background: var(--app-bg);
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
}

.empty-steps li {
  color: var(--app-muted);
  font-size: 13px;
  line-height: 1.6;
}

.empty-steps strong {
  color: var(--app-text);
}

.step-sep {
  margin: 0 8px;
  color: var(--app-muted);
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

.progress-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
}

/* ===== Failed ===== */

.failed-section {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.failed-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.error-message {
  margin: 0;
  word-break: break-word;
  white-space: pre-wrap;
}

/* ===== Success ===== */

.scan-result {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.result-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
  padding: 14px 16px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-bg);
}

.result-toolbar-id {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.result-toolbar-icon {
  font-size: 22px;
  color: var(--app-success);
  flex: 0 0 auto;
}

.result-toolbar-id small {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
}

.result-toolbar-id code {
  display: block;
  color: var(--app-text);
  font-size: 14px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.result-toolbar-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.metric-card {
  position: relative;
  padding: 16px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: var(--app-surface);
  overflow: hidden;
}

.metric-card::before {
  content: "";
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--app-border);
}

.metric-card.tone-success::before { background: var(--app-success); }
.metric-card.tone-warning::before { background: var(--app-warning); }
.metric-card.tone-danger::before { background: var(--app-danger); }

.metric-card small {
  display: block;
  margin-bottom: 8px;
  color: var(--app-muted);
  font-size: 12px;
}

.metric-card strong {
  color: var(--app-text);
  font-size: 26px;
  line-height: 1;
}

.metric-card.tone-success strong { color: var(--app-success); }
.metric-card.tone-warning strong { color: var(--app-warning); }
.metric-card.tone-danger strong { color: var(--app-danger); }

/* ===== Severity breakdown bar ===== */

.severity-bar-wrap {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.severity-bar-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.severity-bar-label span {
  color: var(--app-text);
  font-size: 14px;
  font-weight: 700;
}

.severity-bar-label small {
  color: var(--app-muted);
  font-size: 12px;
}

.severity-bar {
  display: flex;
  height: 10px;
  overflow: hidden;
  border-radius: 999px;
  background: var(--app-bg);
}

.severity-seg {
  height: 100%;
  min-width: 2px;
  transition: width 200ms ease;
}

.severity-seg.critical { background: var(--app-danger); }
.severity-seg.major { background: var(--app-warning); }
.severity-seg.minor { background: var(--app-primary); }
.severity-seg.info { background: var(--app-success); }

.severity-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 14px;
}

.legend-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  color: var(--app-muted);
  font-size: 12px;
}

.legend-item strong {
  color: var(--app-text);
}

.legend-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--app-muted);
}

.legend-dot.critical { background: var(--app-danger); }
.legend-dot.major { background: var(--app-warning); }
.legend-dot.minor { background: var(--app-primary); }
.legend-dot.info { background: var(--app-success); }

.overview {
  --el-alert-padding: 12px 14px;
}

/* ===== Responsive ===== */

@media (max-width: 980px) {
  .scan-grid {
    grid-template-columns: 1fr;
  }

  .inline-fields {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 560px) {
  .result-toolbar {
    flex-direction: column;
    align-items: stretch;
  }

  .result-toolbar-actions {
    justify-content: stretch;
  }

  .result-toolbar-actions .el-button {
    flex: 1;
  }
}
</style>
