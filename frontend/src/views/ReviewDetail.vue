<template>
  <AppLayout>
    <div class="detail-page" v-loading="loading">
      <header class="page-header">
        <el-button class="back-button" circle plain aria-label="返回审查列表" @click="router.push('/reviews')">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <div>
          <div class="eyebrow">REVIEW REPORT</div>
          <h1>{{ detail?.repo || '审查详情' }}</h1>
          <p v-if="detail">{{ getProviderLabel(detail.provider) }} · {{ detail.review_id }}</p>
        </div>
        <div class="header-actions">
          <el-button
            v-if="detail && (detail.status === 'queued' || detail.status === 'running')"
            type="danger"
            plain
            size="small"
            :loading="acting"
            @click="handleCancel"
          >
            取消审查
          </el-button>
          <el-button
            v-if="detail && (detail.status === 'failed' || detail.status === 'success')"
            type="primary"
            plain
            size="small"
            :loading="acting"
            @click="handleRerun"
          >
            重新审查
          </el-button>
          <el-button
            class="debug-toggle-btn"
            :type="debugMode ? 'warning' : 'default'"
            plain
            size="small"
            @click="toggleDebugMode"
          >
            {{ debugMode ? 'Debug ON' : 'Debug OFF' }}
          </el-button>
          <el-button v-if="detail" plain @click="fetchDetail">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </header>

      <template v-if="detail">
        <el-alert
          v-if="detail.status === 'failed' && detail.error"
          class="failure-alert"
          title="审查执行失败"
          type="error"
          :description="detail.error"
          show-icon
          :closable="false"
        />

        <!-- 实时进度追踪器 -->
        <ProgressTracker
          v-if="showProgressTracker"
          :review-id="detail.review_id"
          :review-status="detail.status"
          :sessions="sessions"
          :history="detail.history"
        />

        <section class="summary-strip">
          <div class="summary-result" :class="getSummaryClass(detail)">
            <span class="summary-icon">
              <el-icon v-if="detail.gate?.passed"><SuccessFilled /></el-icon>
              <el-icon v-else-if="detail.status === 'failed' || detail.gate?.status === 'failed'"><CircleCloseFilled /></el-icon>
              <el-icon v-else><InfoFilled /></el-icon>
            </span>
            <div>
              <small>质量门禁</small>
              <strong>{{ getSummaryLabel(detail) }}</strong>
            </div>
          </div>
          <div class="summary-metric">
            <small>质量评分</small>
            <strong>{{ detail.report?.generated ? detail.report.score.value : '--' }}<span v-if="detail.report?.generated"> / 100</span></strong>
          </div>
          <div class="summary-metric">
            <small>发现问题</small>
            <strong>{{ detail.findings?.length || 0 }}<span> 项</span></strong>
          </div>
          <div class="summary-metric">
            <small>执行状态</small>
            <el-tag :type="getStatusType(detail.status)">{{ getStatusLabel(detail.status) }}</el-tag>
          </div>
          <div class="summary-metric">
            <small>最后更新</small>
            <strong class="summary-time">{{ formatTime(detail.updated_at) }}</strong>
          </div>
        </section>

        <el-row :gutter="16">
          <el-col :xs="24" :lg="16">
            <el-card class="info-card">
              <template #header>
                <div class="card-header">
                  <span>审查信息</span>
                  <span class="card-caption">提交范围与执行参数</span>
                </div>
              </template>
              <el-descriptions :column="descriptionColumns" border>
                <el-descriptions-item label="Review ID">{{ detail.review_id }}</el-descriptions-item>
                <el-descriptions-item label="代码平台">{{ detail.provider }}</el-descriptions-item>
                <el-descriptions-item label="PR/MR">
                  <span v-if="detail.pull_request_id">#{{ detail.pull_request_id }}</span>
                  <span v-else-if="detail.merge_request_iid">!{{ detail.merge_request_iid }}</span>
                  <span v-else>-</span>
                </el-descriptions-item>
                <el-descriptions-item label="开发者">
                  <router-link v-if="detail.author" :to="`/contributors/${encodeURIComponent(detail.author)}`" class="detail-author-link">
                    {{ detail.author_name || detail.author }}
                  </router-link>
                  <span v-else class="text-muted">-</span>
                </el-descriptions-item>
                <el-descriptions-item label="基准 SHA">
                  <code v-if="detail.base_sha">{{ detail.base_sha }}</code>
                  <span v-else class="text-muted">未指定</span>
                </el-descriptions-item>
                <el-descriptions-item label="目标 SHA">
                  <code v-if="detail.head_sha">{{ detail.head_sha }}</code>
                  <span v-else class="text-muted">未指定</span>
                </el-descriptions-item>
                <el-descriptions-item label="创建时间">{{ formatTime(detail.created_at) }}</el-descriptions-item>
                <el-descriptions-item label="更新时间">{{ formatTime(detail.updated_at) }}</el-descriptions-item>
              </el-descriptions>
            </el-card>

            <!-- 代码 Review 执行信息 -->
            <el-card class="ocr-card" v-if="detail.report?.ocr">
              <template #header>
                <div class="card-header">
                  <span>代码 Review 执行信息</span>
                  <div class="ocr-header-actions">
                    <el-tag :type="detail.report.ocr.status === 'success' ? 'success' : detail.report.ocr.status === 'skipped' ? 'warning' : 'info'" size="small">
                      {{ detail.report.ocr.status || '未知' }}
                    </el-tag>
                    <el-button size="small" plain :loading="viewerLoading" @click="toggleReviewViewer">
                      {{ reviewViewerOpen ? '收起 Viewer' : '打开 OCR Viewer' }}
                    </el-button>
                  </div>
                </div>
              </template>
              <el-descriptions :column="ocrDescriptionColumns" border size="small">
                <el-descriptions-item label="引擎">
                  {{ detail.report.ocr.engine === 'open-code-review' ? 'AI Code Review 引擎' : detail.report.ocr.engine || '-' }}
                </el-descriptions-item>
                <el-descriptions-item label="审查文件数">{{ detail.report.ocr.files_reviewed ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="评论数">{{ detail.report.ocr.comments ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="总 Tokens">{{ detail.report.ocr.total_tokens ? detail.report.ocr.total_tokens.toLocaleString() : '-' }}</el-descriptions-item>
                <el-descriptions-item label="输入 Tokens">{{ detail.report.ocr.input_tokens ? detail.report.ocr.input_tokens.toLocaleString() : '-' }}</el-descriptions-item>
                <el-descriptions-item label="输出 Tokens">{{ detail.report.ocr.output_tokens ? detail.report.ocr.output_tokens.toLocaleString() : '-' }}</el-descriptions-item>
                <el-descriptions-item label="耗时">{{ detail.report.ocr.elapsed || '-' }}</el-descriptions-item>
                <el-descriptions-item label="审查范围">
                  {{ detail.report.scope?.reviewed_files ?? '-' }} / {{ detail.report.scope?.changed_files ?? '-' }} 文件
                </el-descriptions-item>
                <el-descriptions-item label="排除文件">{{ detail.report.scope?.excluded_files ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="工具调用">{{ detail.report.ocr.tool_calls?.total ?? '-' }}</el-descriptions-item>
              </el-descriptions>
              <el-alert
                v-if="detail.report.ocr.project_summary"
                class="ocr-project-summary"
                type="info"
                :closable="false"
                :title="detail.report.ocr.project_summary"
              />
              <div v-if="toolCallRows.length" class="tool-call-grid">
                <div v-for="row in toolCallRows" :key="row.name" class="tool-call-item">
                  <span>{{ row.name }}</span>
                  <strong>{{ row.count }}</strong>
                </div>
              </div>
              <div v-if="reviewViewerOpen" class="review-viewer-frame">
                <div class="viewer-context">
                  <span>{{ detail.repo }}</span>
                  <code>{{ detail.review_id }}</code>
                  <code v-if="detail.head_sha">{{ detail.head_sha }}</code>
                </div>
                <iframe title="OCR Viewer" src="/api/v1/settings/ocr/viewer/proxy/" />
              </div>
            </el-card>

            <!-- 质量评估 -->
            <el-card class="quality-card" v-if="detail.report?.generated">
              <template #header>
                <div class="card-header">
                  <span>质量评估</span>
                  <el-tag v-if="!detail.report.complete" type="warning" size="small">审查未完整完成</el-tag>
                </div>
              </template>
              <el-row :gutter="24">
                <el-col :xs="24" :sm="8">
                  <div class="score-block">
                    <div class="score-value">{{ detail.report.score.value }}</div>
                    <div class="score-label">/ 100</div>
                    <div class="score-grade">{{ detail.report.score.grade }}</div>
                  </div>
                </el-col>
                <el-col :xs="24" :sm="16">
                  <div class="severity-block">
                    <div class="severity-item critical">
                      <span>严重</span>
                      <strong>{{ detail.report.counts.critical }}</strong>
                    </div>
                    <div class="severity-item major">
                      <span>高风险</span>
                      <strong>{{ detail.report.counts.major }}</strong>
                    </div>
                    <div class="severity-item minor">
                      <span>中风险</span>
                      <strong>{{ detail.report.counts.minor }}</strong>
                    </div>
                    <div class="severity-item info">
                      <span>提示</span>
                      <strong>{{ detail.report.counts.info }}</strong>
                    </div>
                  </div>
                  <!-- 扣分详情 -->
                  <div v-if="detail.report.score.deductions?.length" class="deductions">
                    <div v-for="(d, i) in detail.report.score.deductions" :key="i" class="deduction-item">
                      <el-text :type="d.points < 0 ? 'success' : 'danger'" size="small">
                        <template v-if="d.points < 0">+{{ Math.abs(d.points) }}分: {{ d.reason }}</template>
                        <template v-else>-{{ d.points }}分: {{ d.reason }}</template>
                      </el-text>
                    </div>
                    <div class="deduction-total">
                      <el-text type="info" size="small">
                        总扣分: {{ 100 - detail.report.score.value }}分
                      </el-text>
                    </div>
                  </div>
                </el-col>
              </el-row>
            </el-card>

            <!-- 警告 -->
            <el-card class="warnings-card" v-if="detail.report?.warnings?.length">
              <template #header>
                <div class="card-header">
                  <span>警告</span>
                  <el-tag type="warning" size="small">{{ detail.report.warnings.length }}</el-tag>
                </div>
              </template>
              <div v-for="(w, i) in detail.report.warnings" :key="i" class="warning-item">
                <el-text type="warning">
                  <span v-if="w.type">[{{ w.type }}] </span>
                  {{ w.message }}
                  <span v-if="w.path" class="warning-path"> ({{ w.path }})</span>
                </el-text>
              </div>
            </el-card>

            <!-- 分析概览 -->
            <el-card class="analysis-card" v-if="detail.analysis?.overview || detail.analysis?.root_cause?.length || detail.analysis?.risks?.length || detail.analysis?.recommendations?.length">
              <template #header>分析概览</template>
              <div v-if="detail.analysis.overview" class="analysis-section">
                <p>{{ detail.analysis.overview }}</p>
              </div>
              <div v-if="detail.analysis.root_cause?.length" class="analysis-section">
                <h4>根因分析</h4>
                <ul>
                  <li v-for="(item, i) in detail.analysis.root_cause" :key="i">{{ item }}</li>
                </ul>
              </div>
              <div v-if="detail.analysis.risks?.length" class="analysis-section">
                <h4>风险评估</h4>
                <ul>
                  <li v-for="(item, i) in detail.analysis.risks" :key="i">{{ item }}</li>
                </ul>
              </div>
              <div v-if="detail.analysis.recommendations?.length" class="analysis-section">
                <h4>改进建议</h4>
                <ul>
                  <li v-for="(item, i) in detail.analysis.recommendations" :key="i">{{ item }}</li>
                </ul>
              </div>
            </el-card>

            <!-- 问题报告 -->
            <el-card class="findings-card" v-if="detail.findings?.length">
              <template #header>
                <div class="card-header">
                  <span>问题报告</span>
                  <div class="finding-header-actions">
                    <el-tag>{{ detail.findings.length }} 个问题</el-tag>
                    <el-button size="small" plain @click="selectPatchableFindings">全选可生成</el-button>
                  </div>
                </div>
              </template>
              <div class="suggestion-patch-panel">
                <div class="patch-summary">
                  <div>
                    <strong>修复 Patch</strong>
                    <span>已选 {{ selectedPatchIndexes.length }} / 可生成 {{ patchableFindingIndexes.length }}</span>
                  </div>
                  <div class="patch-actions">
                    <el-button size="small" @click="clearPatchSelection">清空</el-button>
                    <el-button size="small" type="primary" plain :disabled="!patchSelectionIndexes.length" :loading="patchLoading" @click="handlePreviewPatch">
                      生成预览
                    </el-button>
                    <el-button size="small" plain :disabled="!patchPreview?.patch" :loading="patchChecking" @click="checkPatch">
                      检查可应用
                    </el-button>
                    <el-button size="small" plain :disabled="!patchPreview?.patch" @click="copyPatch">复制</el-button>
                    <el-button size="small" plain :disabled="!patchPreview?.patch" :loading="patchDownloading" @click="downloadPatch">
                      下载 .diff
                    </el-button>
                  </div>
                </div>
                <div v-if="patchPreview" class="patch-result">
                  <div class="patch-result-meta">
                    <el-tag type="success" size="small">生成 {{ patchPreview.generated_count }}</el-tag>
                    <el-tag v-if="patchPreview.skipped_count" type="warning" size="small">跳过 {{ patchPreview.skipped_count }}</el-tag>
                    <el-tag v-if="patchCheck" :type="patchCheck.applicable ? 'success' : 'danger'" size="small">
                      {{ patchCheck.applicable ? '可应用' : '不可应用' }}
                    </el-tag>
                  </div>
                  <div v-if="patchCheck && (!patchCheck.applicable || patchCheck.output)" class="patch-check-output">
                    {{ patchCheck.error || 'git apply --check' }}<template v-if="patchCheck.output">：{{ patchCheck.output }}</template>
                  </div>
                  <div v-if="patchPreview.skipped?.length" class="patch-skipped">
                    <span v-for="item in patchPreview.skipped" :key="`${item.index}-${item.reason}`">
                      #{{ item.index + 1 }} {{ item.path || item.title || '未命名问题' }}：{{ item.reason }}
                    </span>
                  </div>
                  <pre v-if="patchPreview.patch" class="patch-preview">{{ patchPreview.patch }}</pre>
                  <el-empty v-else description="当前选择没有可生成的 Patch" :image-size="48" />
                </div>
              </div>
              <div class="finding-list">
                <div v-for="(finding, index) in detail.findings" :key="index" class="finding-item">
                  <div class="finding-header">
                    <el-checkbox
                      class="finding-patch-checkbox"
                      :model-value="selectedPatchIndexes.includes(index)"
                      :disabled="!isPatchableFinding(finding)"
                      @change="(checked: boolean | string | number) => togglePatchFinding(index, Boolean(checked))"
                    />
                    <el-tag :type="getSeverityType(finding.severity)" size="small">
                      {{ finding.severity.toUpperCase() }}
                    </el-tag>
                    <span class="finding-title">{{ finding.title }}</span>
                    <span class="finding-path" v-if="finding.path">
                      {{ finding.path }}{{ finding.line ? `:${finding.line}` : '' }}
                    </span>
                  </div>
                  <div class="finding-body" v-if="finding.body">{{ finding.body }}</div>
                  <div class="finding-code" v-if="finding.existing_code">
                    <div class="code-label">现有代码</div>
                    <pre>{{ finding.existing_code }}</pre>
                  </div>
                  <div class="finding-code" v-if="finding.suggestion_code">
                    <div class="code-label">建议代码</div>
                    <pre>{{ finding.suggestion_code }}</pre>
                  </div>
                </div>
              </div>
            </el-card>

            <!-- 发布到代码平台 -->
            <PublishPanel
              v-if="detail.status === 'success' && detail.report?.generated"
              :review-id="detail.review_id"
            />

            <!-- Debug 区域 - OCR 原始输入 -->
            <el-card class="debug-card" v-if="debugMode && detail.raw_ocr_input">
              <template #header>
                <div class="card-header">
                  <span>
                    <el-tag type="warning" size="small" style="margin-right: 8px;">DEBUG</el-tag>
                    OCR 原始输入
                  </span>
                  <el-button size="small" @click="showOcrInput = !showOcrInput">
                    {{ showOcrInput ? '收起' : '展开' }}
                  </el-button>
                </div>
              </template>
              <el-collapse v-model="ocrInputActive">
                <el-collapse-item title="查看完整 JSON" name="input">
                  <pre class="debug-json">{{ formatJson(detail.raw_ocr_input) }}</pre>
                </el-collapse-item>
              </el-collapse>
            </el-card>

            <!-- Debug 区域 - OCR 原始输出 -->
            <el-card class="debug-card" v-if="debugMode && detail.raw_ocr">
              <template #header>
                <div class="card-header">
                  <span>
                    <el-tag type="warning" size="small" style="margin-right: 8px;">DEBUG</el-tag>
                    OCR 原始输出
                  </span>
                  <el-button size="small" @click="showOcrOutput = !showOcrOutput">
                    {{ showOcrOutput ? '收起' : '展开' }}
                  </el-button>
                </div>
              </template>
              <el-collapse v-model="ocrOutputActive">
                <el-collapse-item title="查看完整 JSON" name="output">
                  <pre class="debug-json">{{ formatJson(detail.raw_ocr) }}</pre>
                </el-collapse-item>
              </el-collapse>
            </el-card>
          </el-col>

          <el-col :xs="24" :lg="8">
            <el-card class="gate-card">
              <template #header>质量门禁</template>
              <div class="gate-status" :class="detail.gate?.status">
                <div class="gate-icon">
                  <el-icon v-if="detail.gate?.passed" :size="48"><SuccessFilled /></el-icon>
                  <el-icon v-else-if="isGateFailed(detail)" :size="48"><CircleCloseFilled /></el-icon>
                  <el-icon v-else :size="48"><InfoFilled /></el-icon>
                </div>
                <div class="gate-text">
                  {{ detail.gate?.passed ? '门禁通过' : isGateFailed(detail) ? '执行失败' : '待定' }}
                </div>
                <div class="gate-reason" v-if="detail.gate?.reason">{{ detail.gate.reason }}</div>
              </div>
            </el-card>

            <el-card class="history-card" v-if="detail.history?.length">
              <template #header>执行历史</template>
              <el-timeline>
                <el-timeline-item
                  v-for="(event, index) in detail.history"
                  :key="index"
                  :timestamp="formatTime(event.created_at)"
                  :type="getEventType(event.status)"
                >
                  <h4>{{ event.message }}</h4>
                  <p v-if="event.details">{{ formatDetails(event.details) }}</p>
                </el-timeline-item>
              </el-timeline>
            </el-card>

            <el-card class="sessions-card" v-if="sessions.length > 0">
              <template #header>
                <div class="card-header">
                  <span>审查会话</span>
                  <el-tag size="small">{{ sessions.length }} 轮</el-tag>
                </div>
              </template>
              <div class="session-list">
                <div
                  v-for="sess in sessions"
                  :key="sess.session_id"
                  class="session-item"
                  @click="goToSession(sess)"
                >
                  <div class="session-info">
                    <div class="session-header">
                      <span class="session-round">第 {{ sess.round }} 轮</span>
                      <el-tag :type="getSessionStatusType(sess.status)" size="small">
                        {{ getSessionStatusLabel(sess.status) }}
                      </el-tag>
                    </div>
                    <div class="session-phase">
                      {{ getPhaseLabel(sess.phase) }}
                    </div>
                    <el-progress :percentage="sess.progress" :stroke-width="4" :show-text="false" />
                  </div>
                  <el-icon><ArrowRight /></el-icon>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
      </template>
      <el-empty v-else-if="!loading" description="未找到这条审查记录">
        <el-button type="primary" @click="router.push('/reviews')">返回审查列表</el-button>
      </el-empty>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft, ArrowRight, SuccessFilled, CircleCloseFilled, InfoFilled, Refresh } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProgressTracker from '@/components/ProgressTracker.vue'
import PublishPanel from '@/components/PublishPanel.vue'
import { getReview, cancelReview, rerunReview, previewSuggestionPatch, downloadSuggestionPatch, checkSuggestionPatch } from '@/api/review'
import { getViewerStatus, startViewer } from '@/api/config'
import { sessionApi, type ReviewSession } from '@/api/session'
import { getProviderLabel } from '@/utils/url-parser'
import { getStatusLabel, getStatusType, getSeverityType, getPhaseLabel, getSessionStatusType, getSessionStatusLabel } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import type { Finding, ReviewDetail, SuggestionPatchCheck, SuggestionPatchPreview } from '@/types/review'
import type { ViewerStatus } from '@/types/config'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const detail = ref<ReviewDetail | null>(null)
const sessions = ref<ReviewSession[]>([])
const acting = ref(false)
const viewportWidth = ref(window.innerWidth)
const viewerStatus = ref<ViewerStatus | null>(null)
const viewerLoading = ref(false)
const reviewViewerOpen = ref(false)
const selectedPatchIndexes = ref<number[]>([])
const patchPreview = ref<SuggestionPatchPreview | null>(null)
const patchCheck = ref<SuggestionPatchCheck | null>(null)
const patchLoading = ref(false)
const patchDownloading = ref(false)
const patchChecking = ref(false)
let pollTimer: ReturnType<typeof setTimeout> | null = null
let fetchInFlight = false
const descriptionColumns = computed(() => viewportWidth.value < 720 ? 1 : 2)
const ocrDescriptionColumns = computed(() => viewportWidth.value < 720 ? 1 : 3)
const toolCallRows = computed(() => {
  const byTool = detail.value?.report?.ocr?.tool_calls?.by_tool || {}
  return Object.entries(byTool)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
})
const patchableFindingIndexes = computed(() => {
  return (detail.value?.findings || [])
    .map((finding, index) => ({ finding, index }))
    .filter(({ finding }) => isPatchableFinding(finding))
    .map(({ index }) => index)
})
const patchSelectionIndexes = computed(() => {
  const allowed = new Set(patchableFindingIndexes.value)
  return selectedPatchIndexes.value.filter((index) => allowed.has(index))
})
const showProgressTracker = computed(() => Boolean(detail.value && (
  ['queued', 'running', 'failed', 'success', 'completed'].includes(detail.value.status) ||
  sessions.value.length > 0 ||
  (detail.value.history?.length || 0) > 0
)))

// Debug mode
const debugMode = ref(localStorage.getItem('review-debug-mode') === 'true')
const showOcrInput = ref(false)
const showOcrOutput = ref(false)
const ocrInputActive = ref<string[]>([])
const ocrOutputActive = ref<string[]>([])

function toggleDebugMode() {
  debugMode.value = !debugMode.value
  localStorage.setItem('review-debug-mode', String(debugMode.value))
}

function formatJson(data: any): string {
  if (!data) return ''
  try {
    if (typeof data === 'string') {
      return JSON.stringify(JSON.parse(data), null, 2)
    }
    return JSON.stringify(data, null, 2)
  } catch {
    return String(data)
  }
}

function updateViewportWidth() {
  viewportWidth.value = window.innerWidth
}

function getSummaryLabel(review: ReviewDetail) {
  if (review.gate?.passed) return '审查通过'
  if (review.status === 'failed') return '执行失败'
  if (review.gate?.status === 'failed') return '需要修复'
  return '等待结果'
}

function getSummaryClass(review: ReviewDetail) {
  if (review.gate?.passed) return 'passed'
  if (review.status === 'failed' || review.gate?.status === 'failed') return 'failed'
  return 'pending'
}

function isGateFailed(review: ReviewDetail) {
  return review.status === 'failed' || Boolean(review.gate?.status && ['failed', 'error'].includes(review.gate.status))
}

async function fetchDetail(background = false) {
  if (fetchInFlight) return
  fetchInFlight = true
  if (!background) loading.value = true
  try {
    detail.value = await getReview(route.params.id as string)
    selectedPatchIndexes.value = patchableFindingIndexes.value
    patchPreview.value = null
    patchCheck.value = null
    await fetchSessions()
  } finally {
    fetchInFlight = false
    if (!background) loading.value = false
    schedulePolling()
  }
}

function isPatchableFinding(finding: Finding) {
  const line = finding.start_line || finding.line || 0
  return Boolean(finding.path?.trim() && finding.existing_code?.trim() && finding.suggestion_code?.trim() && line > 0)
}

function togglePatchFinding(index: number, checked: boolean) {
  const current = new Set(selectedPatchIndexes.value)
  if (checked) {
    current.add(index)
  } else {
    current.delete(index)
  }
  selectedPatchIndexes.value = Array.from(current).sort((a, b) => a - b)
}

function selectPatchableFindings() {
  selectedPatchIndexes.value = patchableFindingIndexes.value
}

function clearPatchSelection() {
  selectedPatchIndexes.value = []
  patchPreview.value = null
  patchCheck.value = null
}

async function handlePreviewPatch() {
  if (!detail.value || patchSelectionIndexes.value.length === 0) return
  patchLoading.value = true
  try {
    patchPreview.value = await previewSuggestionPatch(detail.value.review_id, patchSelectionIndexes.value)
    patchCheck.value = null
  } finally {
    patchLoading.value = false
  }
}

async function checkPatch() {
  if (!detail.value || patchSelectionIndexes.value.length === 0) return
  patchChecking.value = true
  try {
    patchCheck.value = await checkSuggestionPatch(detail.value.review_id, patchSelectionIndexes.value)
    if (patchCheck.value.applicable) {
      ElMessage.success('Patch 可应用')
    } else {
      ElMessage.warning(patchCheck.value.error || 'Patch 当前不可应用')
    }
  } finally {
    patchChecking.value = false
  }
}

async function copyPatch() {
  if (!patchPreview.value?.patch) return
  try {
    await navigator.clipboard.writeText(patchPreview.value.patch)
    ElMessage.success('Patch 已复制')
  } catch {
    ElMessage.error('复制失败，请手动复制预览内容')
  }
}

async function downloadPatch() {
  if (!detail.value || patchSelectionIndexes.value.length === 0) return
  patchDownloading.value = true
  try {
    const blob = await downloadSuggestionPatch(detail.value.review_id, patchSelectionIndexes.value)
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${detail.value.review_id}-suggestion.patch`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
  } finally {
    patchDownloading.value = false
  }
}

async function fetchSessions() {
  try {
    const response = await sessionApi.getSessions(route.params.id as string)
    sessions.value = response.sessions || []
  } catch (error) {
    console.error('Failed to fetch sessions:', error)
    sessions.value = []
  }
}

function goToSession(sess: ReviewSession) {
  router.push(`/reviews/${route.params.id}/sessions/${sess.session_id}`)
}

async function handleCancel() {
  try {
    await ElMessageBox.confirm('确定取消这条审查任务？', '取消审查', {
      type: 'warning',
      confirmButtonText: '取消任务',
      cancelButtonText: '再想想',
    })
  } catch {
    return
  }
  acting.value = true
  try {
    await cancelReview(route.params.id as string)
    ElMessage.success('审查任务已取消')
    await fetchDetail()
  } catch {
    ElMessage.error('取消失败，任务可能已完成或状态不允许')
  } finally {
    acting.value = false
  }
}

async function handleRerun() {
  acting.value = true
  try {
    await rerunReview(route.params.id as string)
    ElMessage.success('已重新加入审查队列')
    await fetchDetail()
  } catch {
    ElMessage.error('重新审查失败，请稍后重试')
  } finally {
    acting.value = false
  }
}

async function toggleReviewViewer() {
  if (reviewViewerOpen.value) {
    reviewViewerOpen.value = false
    return
  }
  viewerLoading.value = true
  try {
    let status = await getViewerStatus()
    if (!status.running) {
      status = await startViewer()
    }
    viewerStatus.value = status
    if (!status.running) {
      ElMessage.error(status.message || 'OCR Viewer 未运行')
      return
    }
    reviewViewerOpen.value = true
  } catch (error: any) {
    ElMessage.error(error?.message || '启动 OCR Viewer 失败')
  } finally {
    viewerLoading.value = false
  }
}

function getEventType(status: string) {
  const map: Record<string, string> = {
    completed: 'success',
    running: 'warning',
    failed: 'danger',
  }
  return map[status] || 'info'
}

function formatTime(time: string) {
  return formatDateTime(time)
}

function formatDetails(details: Record<string, any>) {
  return Object.entries(details)
    .map(([key, value]) => `${key}: ${value}`)
    .join(', ')
}

onMounted(() => {
  window.addEventListener('resize', updateViewportWidth)
  fetchDetail()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewportWidth)
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
})

function schedulePolling() {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
  if (!detail.value || !['queued', 'running'].includes(detail.value.status)) return
  pollTimer = setTimeout(() => {
    if (document.visibilityState === 'visible') {
      fetchDetail(true)
    } else {
      schedulePolling()
    }
  }, 5000)
}
</script>

<style scoped>
.detail-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 22px;
}

.page-header > div {
  min-width: 0;
  flex: 1;
}

.page-header h1 {
  margin: 3px 0 4px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 28px;
  letter-spacing: -0.6px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.page-header p {
  margin: 0;
  overflow: hidden;
  color: #929aab;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}

.back-button {
  flex: 0 0 auto;
}

.summary-strip {
  display: grid;
  grid-template-columns: minmax(210px, 1.3fr) repeat(4, minmax(120px, 1fr));
  margin-bottom: 18px;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.failure-alert {
  margin-bottom: 18px;
}

.failure-alert :deep(.el-alert__description) {
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  line-height: 1.6;
}

.summary-result,
.summary-metric {
  display: flex;
  min-height: 92px;
  align-items: center;
  padding: 18px 20px;
  border-right: 1px solid var(--app-border);
}

.summary-metric {
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
}

.summary-metric:last-child {
  border-right: 0;
}

.summary-icon {
  display: grid;
  width: 42px;
  height: 42px;
  margin-right: 12px;
  place-items: center;
  border-radius: 12px;
  font-size: 21px;
}

.summary-result.passed .summary-icon {
  color: var(--app-success);
  background: #eaf8f3;
}

.summary-result.failed .summary-icon {
  color: var(--app-danger);
  background: #fff0f2;
}

.summary-result.pending .summary-icon {
  color: #8d96a7;
  background: #f1f3f6;
}

.summary-result small,
.summary-metric small {
  display: block;
  margin-bottom: 5px;
  color: #929aab;
  font-size: 11px;
}

.summary-result strong,
.summary-metric strong {
  color: var(--app-text);
  font-size: 17px;
}

.summary-metric strong span {
  color: #9aa2b1;
  font-size: 11px;
  font-weight: 500;
}

.summary-metric .summary-time {
  font-size: 12px;
}

.info-card,
.quality-card,
.ocr-card,
.warnings-card,
.analysis-card,
.findings-card,
.raw-ocr-card,
.debug-card,
.gate-card,
.history-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header > span:first-child {
  color: var(--app-text);
  font-weight: 700;
}

.ocr-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.card-caption {
  color: #9aa2b1;
  font-size: 11px;
  font-weight: 400;
}

.text-muted {
  color: #999;
}

code {
  max-width: 100%;
  font-family: monospace;
  font-size: 12px;
  background: #f5f5f5;
  padding: 2px 4px;
  border-radius: 2px;
  overflow-wrap: anywhere;
  white-space: normal;
}

:deep(.el-descriptions__content) {
  min-width: 0;
  overflow-wrap: anywhere;
}

.ocr-project-summary {
  margin-top: 14px;
}

.tool-call-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px;
  margin-top: 12px;
}

.tool-call-item {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 9px 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
}

.tool-call-item strong {
  color: #0f172a;
}

.finding-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.suggestion-patch-panel {
  margin-bottom: 14px;
  padding: 12px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #f8fafc;
}

.patch-summary,
.patch-actions,
.patch-result-meta {
  display: flex;
  align-items: center;
  gap: 8px;
}

.patch-summary {
  justify-content: space-between;
}

.patch-summary strong,
.patch-summary span {
  display: block;
}

.patch-summary span {
  margin-top: 3px;
  color: #64748b;
  font-size: 12px;
}

.patch-result {
  margin-top: 12px;
}

.patch-skipped {
  display: grid;
  gap: 5px;
  margin-top: 8px;
  color: #92400e;
  font-size: 12px;
}

.patch-check-output {
  margin-top: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  background: #fff7ed;
  color: #9a3412;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.patch-preview {
  max-height: 360px;
  margin: 10px 0 0;
  padding: 12px;
  overflow: auto;
  border-radius: 6px;
  background: #0f172a;
  color: #dbeafe;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre;
}

.review-viewer-frame {
  margin-top: 14px;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
}

.viewer-context {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  padding: 10px 12px;
  border-bottom: 1px solid var(--app-border);
  background: #f8fafc;
  color: #475569;
  font-size: 12px;
}

.viewer-context span {
  font-weight: 700;
}

.review-viewer-frame iframe {
  display: block;
  width: 100%;
  height: 520px;
  border: 0;
}

.score-block {
  text-align: center;
  padding: 20px;
}

.score-value {
  font-size: 48px;
  font-weight: bold;
  color: var(--app-primary);
}

.score-label {
  font-size: 16px;
  color: #999;
}

.score-grade {
  font-size: 18px;
  margin-top: 8px;
  color: #333;
}

.severity-block {
  display: flex;
  gap: 16px;
}

.severity-item {
  flex: 1;
  text-align: center;
  padding: 16px;
  border-radius: 8px;
  background: #f5f5f5;
}

.severity-item span {
  display: block;
  font-size: 14px;
  color: #666;
  margin-bottom: 8px;
}

.severity-item strong {
  font-size: 24px;
}

.severity-item.critical strong { color: #f56c6c; }
.severity-item.major strong { color: #e6a23c; }
.severity-item.minor strong { color: #409eff; }
.severity-item.info strong { color: #909399; }

.deductions {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #eee;
}

.deduction-item {
  margin-bottom: 4px;
}

.deduction-total {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed #ddd;
}

.warning-item {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}

.warning-item:last-child {
  border-bottom: none;
}

.warning-path {
  color: #999;
  font-size: 12px;
}

.analysis-section {
  margin-bottom: 16px;
}

.analysis-section:last-child {
  margin-bottom: 0;
}

.analysis-section h4 {
  margin: 0 0 8px;
  font-size: 14px;
  color: #333;
}

.analysis-section p {
  margin: 0;
  color: #606266;
  line-height: 1.6;
}

.analysis-section ul {
  margin: 0;
  padding-left: 20px;
  color: #606266;
}

.analysis-section li {
  margin-bottom: 4px;
  line-height: 1.5;
}

.finding-item {
  padding: 16px;
  border: 1px solid #eee;
  border-radius: 8px;
  margin-bottom: 12px;
}

.finding-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.finding-patch-checkbox {
  flex: 0 0 auto;
  height: 22px;
}

.finding-title {
  font-weight: bold;
}

.finding-path {
  color: #999;
  font-size: 14px;
}

.finding-body {
  color: #666;
  margin-bottom: 8px;
}

.finding-code {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 4px;
  margin-top: 8px;
}

.code-label {
  font-size: 12px;
  color: #999;
  margin-bottom: 4px;
}

.finding-code pre {
  margin: 0;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
}

.raw-ocr-json {
  font-family: monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 500px;
  overflow-y: auto;
  background: #f8f8f8;
  padding: 12px;
  border-radius: 4px;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.debug-card {
  border: 1px dashed #e6a23c;
  background: #fdf6ec;
}

.debug-card :deep(.el-card__header) {
  background: #faecd8;
  border-bottom: 1px dashed #e6a23c;
}

.debug-json {
  font-family: 'Fira Code', 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 12px;
  line-height: 1.6;
  max-height: 600px;
  overflow: auto;
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 6px;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.gate-status {
  text-align: center;
  padding: 24px;
}

.gate-icon {
  margin-bottom: 16px;
}

.gate-status.passed .gate-icon { color: #67c23a; }
.gate-status.failed .gate-icon,
.gate-status.error .gate-icon { color: #f56c6c; }
.gate-status.pending .gate-icon { color: #909399; }

.gate-text {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 8px;
}

.gate-reason {
  color: #666;
  font-size: 14px;
}

@media (max-width: 1180px) {
  .summary-strip {
    grid-template-columns: repeat(4, 1fr);
  }

  .summary-result {
    grid-column: span 2;
  }

  .summary-metric:last-child {
    grid-column: span 2;
  }
}

@media (max-width: 720px) {
  .detail-page {
    padding: 22px 14px 36px;
  }

  .page-header {
    align-items: center;
  }

  .page-header h1 {
    font-size: 21px;
  }

  .page-header .eyebrow,
  .page-header .debug-toggle-btn {
    display: none;
  }

  .header-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .summary-strip {
    grid-template-columns: 1fr 1fr;
  }

  .summary-result {
    grid-column: span 2;
  }

  .summary-result,
  .summary-metric {
    min-height: 78px;
    padding: 14px;
  }

  .summary-metric:nth-child(3) {
    border-right: 0;
  }

  .summary-metric:last-child {
    grid-column: span 2;
    border-top: 1px solid var(--app-border);
  }

  .severity-block {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .finding-header {
    flex-wrap: wrap;
  }

  .finding-header-actions,
  .patch-summary,
  .patch-actions {
    flex-wrap: wrap;
  }

  .finding-path {
    width: 100%;
    overflow-wrap: anywhere;
  }

  .ocr-header-actions {
    flex-wrap: wrap;
    justify-content: flex-end;
  }

  .review-viewer-frame iframe {
    height: 430px;
  }
}

.sessions-card {
  margin-top: 16px;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.session-item:hover {
  background: #ecf5ff;
  transform: translateX(4px);
}

.session-info {
  flex: 1;
}

.session-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.session-round {
  font-weight: 600;
  font-size: 14px;
}

.session-phase {
  font-size: 12px;
  color: #666;
  margin-bottom: 8px;
}

.session-item .el-icon {
  color: #909399;
}
</style>
