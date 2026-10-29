<script setup lang="ts">
/**
 * ReviewContentArea — 评审详情右栏（正常滚动）。
 * 包含：裁决/摘要文本、Findings 列表、分析概览、OCR 执行信息（可折叠）、发布面板、调试面板。
 * 按 mode 适配：argus 模式摘要卡带裁决色条、隐藏 OCR 执行信息；空状态文案随模式变化。
 */
import { ref, computed } from 'vue'
import type { ReviewDetail } from '@/types/review'
import { formatReviewSummary } from '@/utils/reviewText'
import ReviewFindingsPanel from './ReviewFindingsPanel.vue'
import ReviewOcrCard from './ReviewOcrCard.vue'
import ReviewDebugPanel from './ReviewDebugPanel.vue'
import JudgmentPanel from './JudgmentPanel.vue'
import ReviewAuditTrail from './ReviewAuditTrail.vue'
import PublishPanel from '@/components/PublishPanel.vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { downloadSuggestionPatch } from '@/api/review'
import {
  extractArgusOutcome, ARGUS_VERDICT_META,
  type ReviewMode, type ArgusVerdict,
} from '@/constants/reviewMode'

const props = defineProps<{
  detail: ReviewDetail
  debugMode: boolean
  mode?: ReviewMode
}>()

const mode = computed(() => props.mode || 'ocr')

// argus 模式：裁决色条（等待回报时不着色）
const argusAccent = computed(() => {
  if (mode.value !== 'argus') return null
  const outcome = extractArgusOutcome(props.detail)
  if (outcome.verdict === 'pending') return null
  const meta = ARGUS_VERDICT_META[outcome.verdict as ArgusVerdict]
  return { ...meta, verdict: outcome.verdict as ArgusVerdict }
})

const emptyFindingsText = computed(() => {
  if (mode.value === 'argus') return argusAccent.value ? 'Argus 裁决完成，未回报具体代码发现' : 'Argus 审查进行中，回报后展示发现'
  if (mode.value === 'scan') return '全库扫描完成，未发现问题'
  return '本次审查未发现问题'
})

// 简易 Markdown 渲染（加粗、换行、代码块）
function renderMarkdown(md: string): string {
  if (!md) return ''
  return md
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
    .replace(/```[\s\S]*?```/g, (m) => `<pre class="md-code">${m.slice(3, -3).trim()}</pre>`)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    .replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
    .replace(/\n/g, '<br>')
}

const emit = defineEmits<{
  (e: 'toggle-debug'): void
}>()

const findingsPanelRef = ref<InstanceType<typeof ReviewFindingsPanel> | null>(null)
const ocrExpanded = ref<string[]>([])

function resetPatchState() {
  findingsPanelRef.value?.resetPatchState()
}

// 转发来自侧栏「优先修复」的跳转请求到发现面板。
function focusFinding(index: number) {
  findingsPanelRef.value?.scrollToFinding(index)
}

async function handleDownloadPatch(indexes: number[]) {
  if (!props.detail || !indexes.length) return
  const blob = await downloadSuggestionPatch(props.detail.review_id, indexes)
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `${props.detail.review_id}-suggestion.patch`
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(url)
}

defineExpose({ resetPatchState, focusFinding })
</script>

<template>
  <div class="review-content">
    <!-- 摘要（argus 模式带裁决色条） -->
    <div
      v-if="detail.summary"
      class="review-summary-card"
      :class="{ 'has-verdict': argusAccent }"
      :style="argusAccent ? { '--verdict-color': argusAccent.color, '--verdict-soft': argusAccent.soft, '--verdict-border': argusAccent.border } : undefined"
    >
      <div v-if="argusAccent" class="verdict-strip">
        <span class="verdict-dot" :style="{ background: argusAccent.color }"></span>
        <strong :style="{ color: argusAccent.color }">Argus 裁决：{{ argusAccent.label }}</strong>
      </div>
      <div class="summary-content" v-html="renderMarkdown(formatReviewSummary(detail.summary))"></div>
    </div>

    <!-- AI 综合判断（agentkit Judge） -->
    <JudgmentPanel v-if="detail.judgment" :judgment="detail.judgment" />

    <!-- 审查步骤审计面板 -->
    <ReviewAuditTrail v-if="detail.history?.length" :history="detail.history" />

    <!-- Findings 列表（核心区域） -->
    <ReviewFindingsPanel
      ref="findingsPanelRef"
      v-if="detail.findings?.length"
      :review-id="detail.review_id"
      :findings="detail.findings"
      :mode="mode"
      @download-patch="handleDownloadPatch"
    />

    <el-empty v-else :description="emptyFindingsText" :image-size="100" />

    <!-- 分析概览 -->
    <el-card v-if="detail.analysis?.overview || detail.analysis?.root_cause?.length || detail.analysis?.risks?.length || detail.analysis?.recommendations?.length" class="analysis-card">
      <template #header>分析概览</template>
      <div v-if="detail.analysis.overview" class="analysis-section">
        <p>{{ detail.analysis.overview }}</p>
      </div>
      <div v-if="detail.analysis.root_cause?.length" class="analysis-section">
        <h4>根因分析</h4>
        <ul><li v-for="(item, i) in detail.analysis.root_cause" :key="i">{{ item }}</li></ul>
      </div>
      <div v-if="detail.analysis.risks?.length" class="analysis-section">
        <h4>风险评估</h4>
        <ul><li v-for="(item, i) in detail.analysis.risks" :key="i">{{ item }}</li></ul>
      </div>
      <div v-if="detail.analysis.recommendations?.length" class="analysis-section">
        <h4>改进建议</h4>
        <ul><li v-for="(item, i) in detail.analysis.recommendations" :key="i">{{ item }}</li></ul>
      </div>
    </el-card>

    <!-- OCR 执行信息（可折叠，默认收起降低噪音；argus 无本地执行数据，隐藏） -->
    <el-card v-if="mode !== 'argus' && detail.report?.ocr" class="ocr-collapse-card">
      <el-collapse v-model="ocrExpanded">
        <el-collapse-item name="ocr">
          <template #title>
            <span class="collapse-title">{{ mode === 'scan' ? '扫描执行信息' : 'OCR 执行信息' }}</span>
            <el-tag v-if="detail.report.ocr.total_tokens" size="small" type="info" class="collapse-tag">
              {{ detail.report.ocr.total_tokens.toLocaleString() }} tokens
            </el-tag>
          </template>
          <ReviewOcrCard :review="detail" />
        </el-collapse-item>
      </el-collapse>
    </el-card>

    <!-- 发布面板 -->
    <PublishPanel
      v-if="detail.status === 'success' && detail.report?.generated"
      :review-id="detail.review_id"
    />

    <!-- 调试面板 -->
    <ReviewDebugPanel
      :debug-mode="debugMode"
      :raw-ocr-input="detail.raw_ocr_input"
      :raw-ocr="detail.raw_ocr"
    />
  </div>
</template>

<style scoped>
.review-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.review-summary-card {
  padding: 20px 24px;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  background: linear-gradient(135deg, #f8fafc 0%, #fff 100%);
  font-size: 14px;
  line-height: 1.8;
  color: #334155;
}
.review-summary-card.has-verdict {
  border-color: var(--verdict-border);
  background: linear-gradient(135deg, var(--verdict-soft) 0%, #fff 45%);
  padding-top: 0;
}
.verdict-strip {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 -24px;
  padding: 10px 24px;
  border-bottom: 1px dashed var(--verdict-border);
  font-size: 14px;
}
.verdict-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--verdict-color) 18%, transparent);
}
.review-summary-card.has-verdict .summary-content {
  padding-top: 14px;
}
.review-summary-card :deep(strong) {
  color: #1e293b;
  font-weight: 600;
}
.review-summary-card :deep(code) {
  background: #f1f5f9;
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 13px;
  font-family: 'SF Mono', 'Fira Code', monospace;
}
.review-summary-card :deep(.md-code) {
  background: #1e293b;
  color: #e2e8f0;
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
  font-size: 12px;
}

.analysis-card {
  margin-bottom: 0;
}
.analysis-section {
  margin-bottom: 12px;
}
.analysis-section h4 {
  font-size: 13px;
  font-weight: 600;
  color: var(--app-text);
  margin: 0 0 6px;
}
.analysis-section p {
  font-size: 13px;
  color: #6b7280;
  line-height: 1.6;
  margin: 0;
}
.analysis-section ul {
  margin: 0;
  padding-left: 18px;
}
.analysis-section li {
  font-size: 13px;
  color: #6b7280;
  line-height: 1.8;
}

.ocr-collapse-card {
  border: 1px solid var(--app-border);
}
.ocr-collapse-card :deep(.el-card__body) {
  padding: 0;
}
.ocr-collapse-card :deep(.el-collapse-item__header) {
  padding: 0 16px;
  font-weight: 600;
}
.ocr-collapse-card :deep(.el-collapse-item__content) {
  padding: 0;
}
.ocr-collapse-card :deep(.el-card) {
  border: none;
  box-shadow: none;
}
.collapse-title {
  font-size: 14px;
}
.collapse-tag {
  margin-left: 10px;
}
</style>
