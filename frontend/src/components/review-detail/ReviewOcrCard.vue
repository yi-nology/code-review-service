<script setup lang="ts">
/**
 * ReviewOcrCard — OCR 执行信息卡片（token 统计 / 引擎状态 / 工具调用 / Viewer）。
 * 从 ReviewDetail.vue 抽出，包含全部 OCR display 计算逻辑（buildOCRDisplay 及其辅助函数）。
 * 父组件只传 review detail 数据，本组件内部完成所有展示计算。
 */
import { ref, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { getViewerStatus, locateViewerSession, startViewer } from '@/api/config'
import type { ReviewDetail } from '@/types/review'
import type { ViewerStatus } from '@/types/config'
import { ElMessage } from 'element-plus/es/components/message/index'

const props = defineProps<{
  review: ReviewDetail
}>()

const auth = useAuthStore()
const viewportWidth = ref(window.innerWidth)
const viewerStatus = ref<ViewerStatus | null>(null)
const viewerLoading = ref(false)
const reviewViewerOpen = ref(false)
const reviewViewerPath = ref('')
const reviewViewerMatched = ref(false)

const descriptionColumns = computed(() => viewportWidth.value < 720 ? 1 : 3)

// ---- OCR display 计算辅助函数（从 ReviewDetail 迁移，纯函数无副作用）----

function asRecord(value: unknown): Record<string, any> {
  if (!value) return {}
  if (typeof value === 'string') {
    try {
      const parsed = JSON.parse(value)
      return parsed && typeof parsed === 'object' ? parsed : {}
    } catch {
      return {}
    }
  }
  return typeof value === 'object' ? value as Record<string, any> : {}
}

function positiveNumber(...values: unknown[]) {
  for (const value of values) {
    const n = Number(value)
    if (Number.isFinite(n) && n > 0) return n
  }
  return undefined
}

function nonNegativeNumber(...values: unknown[]) {
  for (const value of values) {
    const n = Number(value)
    if (Number.isFinite(n) && n >= 0) return n
  }
  return undefined
}

function firstText(...values: unknown[]) {
  for (const value of values) {
    const text = String(value ?? '').trim()
    if (text) return text
  }
  return ''
}

function formatCount(value?: number) {
  return value === undefined ? '-' : value.toLocaleString()
}

function tokenMetric(totalTokens: number | undefined, ...values: unknown[]) {
  return totalTokens ? nonNegativeNumber(...values) : positiveNumber(...values)
}

function normalizeOCRStatus(value: string, review?: ReviewDetail | null) {
  const status = value.trim()
  if (status) return status
  if (review?.status === 'success' || review?.status === 'completed') return 'success'
  if (review?.status === 'failed') return 'failed'
  if (review?.status === 'running' || review?.status === 'queued') return review.status
  if (review?.report?.generated) return review.report.complete === false ? 'partial' : 'success'
  return ''
}

function getOCRStatusType(status: string) {
  if (status === 'success' || status === 'completed') return 'success'
  if (status === 'skipped' || status === 'partial' || status === 'queued' || status === 'running') return 'warning'
  if (status === 'failed' || status === 'error') return 'danger'
  return 'info'
}

function getOCRStatusLabel(status: string) {
  const labels: Record<string, string> = {
    success: '已完成', completed: '已完成', partial: '部分完成',
    skipped: '已跳过', queued: '排队中', running: '执行中',
    failed: '执行失败', error: '执行失败',
  }
  return labels[status] || status || '暂无执行状态'
}

function getEngineLabel(engine: string, hasOCRData: boolean) {
  if (engine === 'open-code-review') return 'AI Code Review 引擎'
  if (engine) return engine
  return hasOCRData ? 'AI Code Review 引擎' : '暂无引擎信息'
}

function findOCRHistoryDetails(review?: ReviewDetail | null) {
  const events = review?.history || []
  for (let i = events.length - 1; i >= 0; i -= 1) {
    const details = asRecord(events[i].details)
    if (positiveNumber(details.files_reviewed, details.total_tokens, details.comments) || firstText(details.elapsed)) {
      return details
    }
  }
  return {}
}

const ocrDisplay = computed(() => {
  const review = props.review
  const report = review?.report
  const ocr = report?.ocr || {}
  const scope = asRecord(report?.scope)
  const rawOCR = asRecord(review?.raw_ocr)
  const rawSummary = asRecord(rawOCR.summary)
  const rawToolCalls = asRecord(rawOCR.tool_calls)
  const historyDetails = findOCRHistoryDetails(review)
  const findingsCount = review?.findings?.length || 0

  const filesReviewed = positiveNumber(ocr.files_reviewed, rawSummary.files_reviewed, historyDetails.files_reviewed, scope.reviewed_files)
  const comments = positiveNumber(ocr.comments, rawSummary.comments, historyDetails.comments, findingsCount)
  const totalTokens = positiveNumber(ocr.total_tokens, rawSummary.total_tokens, historyDetails.total_tokens)
  const inputTokens = tokenMetric(totalTokens, ocr.input_tokens, rawSummary.input_tokens)
  const outputTokens = tokenMetric(totalTokens, ocr.output_tokens, rawSummary.output_tokens)
  const cacheReadTokens = tokenMetric(totalTokens, ocr.cache_read_tokens, rawSummary.cache_read_tokens)
  const cacheWriteTokens = tokenMetric(totalTokens, ocr.cache_write_tokens, rawSummary.cache_write_tokens)
  const changedFiles = positiveNumber(scope.changed_files, filesReviewed)
  const excludedFiles = nonNegativeNumber(scope.excluded_files)
  const toolCallsTotal = positiveNumber(ocr.tool_calls?.total, rawToolCalls.total)
  const status = normalizeOCRStatus(firstText(ocr.status, rawOCR.status), review)
  const density = report?.score?.findings_density
  const hasOCRData = Boolean(filesReviewed || comments || totalTokens || rawOCR.summary || report?.generated)

  return {
    rawToolCalls: ocr.tool_calls || rawToolCalls,
    statusLabel: getOCRStatusLabel(status),
    statusType: getOCRStatusType(status),
    engine: getEngineLabel(firstText(ocr.engine, rawOCR.engine), hasOCRData),
    filesReviewed: formatCount(filesReviewed),
    comments: formatCount(comments),
    totalTokens: formatCount(totalTokens),
    inputTokens: formatCount(inputTokens),
    outputTokens: formatCount(outputTokens),
    cacheReadTokens: formatCount(cacheReadTokens),
    cacheWriteTokens: formatCount(cacheWriteTokens),
    tokensPerFile: totalTokens && filesReviewed ? Math.round(totalTokens / filesReviewed).toLocaleString() : '-',
    elapsed: firstText(ocr.elapsed, rawSummary.elapsed, historyDetails.elapsed) || '-',
    scopeLabel: `${formatCount(filesReviewed || scope.reviewed_files)} / ${formatCount(changedFiles)} 文件`,
    findingDensity: density === undefined || density === null ? '-' : `${density.toFixed(2)} / 百行`,
    excludedFiles: formatCount(excludedFiles),
    toolCalls: formatCount(toolCallsTotal),
  }
})

const toolCallRows = computed(() => {
  const byTool = ocrDisplay.value.rawToolCalls?.by_tool || {}
  return Object.entries(byTool)
    .map(([name, count]) => ({ name, count: Number(count) || 0 }))
    .sort((a, b) => b.count - a.count)
})

// ---- OCR Viewer 逻辑 ----

const reviewViewerRepoSlug = computed(() => {
  const workspaceDir = String(props.review?.raw_ocr_input?.workspace?.dir || '').trim()
  if (!workspaceDir) return ''
  return workspaceDir.replace(/^[/\\]+/, '').replace(/[/\\]+/g, '-')
})

const reviewViewerSrc = computed(() => {
  let target: string
  if (reviewViewerPath.value) target = `/api/v1/settings/ocr/viewer/proxy${reviewViewerPath.value}`
  else if (!reviewViewerRepoSlug.value) target = '/api/v1/settings/ocr/viewer/proxy/'
  else target = `/api/v1/settings/ocr/viewer/proxy/r/${encodeURIComponent(reviewViewerRepoSlug.value)}`
  if (!auth.token) return target
  const sep = target.includes('?') ? '&' : '?'
  return `${target}${sep}token=${encodeURIComponent(auth.token)}`
})

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
    await locateReviewViewer()
    reviewViewerOpen.value = true
  } catch (error: any) {
    ElMessage.error(error?.message || '启动 OCR Viewer 失败')
  } finally {
    viewerLoading.value = false
  }
}

async function locateReviewViewer() {
  reviewViewerPath.value = ''
  reviewViewerMatched.value = false
  const workspaceDir = String(props.review?.raw_ocr_input?.workspace?.dir || '').trim()
  const startedAt = String(props.review?.raw_ocr_input?.started_at || props.review?.created_at || '').trim()
  if (!workspaceDir && !startedAt) return
  try {
    const location = await locateViewerSession({ workspace_dir: workspaceDir, started_at: startedAt })
    if (location?.path) {
      reviewViewerPath.value = location.path
      reviewViewerMatched.value = Boolean(location.matched)
    }
  } catch {
    // 定位失败时保留前端 workspace fallback
  }
}
</script>

<template>
  <el-card class="ocr-card">
    <template #header>
      <div class="card-header">
        <span>OCR 执行信息</span>
        <div class="ocr-header-actions">
          <el-tag :type="ocrDisplay.statusType" size="small">{{ ocrDisplay.statusLabel }}</el-tag>
          <el-button size="small" plain :loading="viewerLoading" @click="toggleReviewViewer">
            {{ reviewViewerOpen ? '收起 Viewer' : '打开 OCR Viewer' }}
          </el-button>
        </div>
      </div>
    </template>
    <el-descriptions :column="descriptionColumns" border size="small">
      <el-descriptions-item label="引擎">{{ ocrDisplay.engine }}</el-descriptions-item>
      <el-descriptions-item label="审查文件数">{{ ocrDisplay.filesReviewed }}</el-descriptions-item>
      <el-descriptions-item label="评论数">{{ ocrDisplay.comments }}</el-descriptions-item>
      <el-descriptions-item label="总 Tokens">{{ ocrDisplay.totalTokens }}</el-descriptions-item>
      <el-descriptions-item label="输入 Tokens">{{ ocrDisplay.inputTokens }}</el-descriptions-item>
      <el-descriptions-item label="输出 Tokens">{{ ocrDisplay.outputTokens }}</el-descriptions-item>
      <el-descriptions-item label="缓存读 Tokens">{{ ocrDisplay.cacheReadTokens }}</el-descriptions-item>
      <el-descriptions-item label="缓存写 Tokens">{{ ocrDisplay.cacheWriteTokens }}</el-descriptions-item>
      <el-descriptions-item label="平均 Tokens/文件">{{ ocrDisplay.tokensPerFile }}</el-descriptions-item>
      <el-descriptions-item label="耗时">{{ ocrDisplay.elapsed }}</el-descriptions-item>
      <el-descriptions-item label="审查范围">{{ ocrDisplay.scopeLabel }}</el-descriptions-item>
      <el-descriptions-item label="风险密度">{{ ocrDisplay.findingDensity }}</el-descriptions-item>
      <el-descriptions-item label="排除文件">{{ ocrDisplay.excludedFiles }}</el-descriptions-item>
      <el-descriptions-item label="工具调用">{{ ocrDisplay.toolCalls }}</el-descriptions-item>
    </el-descriptions>
    <el-alert
      v-if="review.report.ocr.project_summary"
      class="ocr-project-summary"
      type="info"
      :closable="false"
      :title="review.report.ocr.project_summary"
    />
    <div v-if="toolCallRows.length" class="tool-call-grid">
      <div v-for="row in toolCallRows" :key="row.name" class="tool-call-item">
        <span>{{ row.name }}</span>
        <strong>{{ row.count }}</strong>
      </div>
    </div>
    <div v-if="reviewViewerOpen" class="review-viewer-frame">
      <div class="viewer-context">
        <span>{{ review.repo }}</span>
        <code>{{ review.review_id }}</code>
        <code v-if="review.head_sha">{{ review.head_sha }}</code>
        <el-tag v-if="reviewViewerMatched" size="small" type="success">已定位 session</el-tag>
        <el-tag v-else-if="reviewViewerRepoSlug" size="small" type="success">已定位 workspace</el-tag>
        <el-tag v-else size="small" type="warning">未记录 workspace</el-tag>
      </div>
      <iframe
        title="OCR Viewer"
        :src="reviewViewerSrc"
        sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox allow-downloads"
      />
    </div>
  </el-card>
</template>

<style scoped>
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.ocr-header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.ocr-project-summary {
  margin-top: 12px;
}
.tool-call-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
.tool-call-item {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 4px 10px;
  border: 1px solid var(--app-border);
  border-radius: 6px;
  font-size: 12px;
}
.tool-call-item strong {
  color: var(--app-primary);
}
.review-viewer-frame {
  margin-top: 12px;
}
.viewer-context {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 12px;
  color: #6b7280;
}
.viewer-context code {
  font-size: 11px;
}
.review-viewer-frame iframe {
  width: 100%;
  height: 500px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
}
</style>
