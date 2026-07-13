<script setup lang="ts">
/**
 * ReviewContentArea — 评审详情右栏（正常滚动）。
 * 包含：摘要文本、Findings 列表、分析概览、OCR 执行信息（可折叠）、发布面板、调试面板。
 */
import { ref } from 'vue'
import type { ReviewDetail } from '@/types/review'
import ReviewFindingsPanel from './ReviewFindingsPanel.vue'
import ReviewOcrCard from './ReviewOcrCard.vue'
import ReviewDebugPanel from './ReviewDebugPanel.vue'
import PublishPanel from '@/components/PublishPanel.vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { downloadSuggestionPatch } from '@/api/review'

const props = defineProps<{
  detail: ReviewDetail
  debugMode: boolean
}>()

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
    <!-- 摘要 -->
    <div v-if="detail.summary" class="review-summary-text">
      {{ detail.summary }}
    </div>

    <!-- Findings 列表（核心区域） -->
    <ReviewFindingsPanel
      ref="findingsPanelRef"
      v-if="detail.findings?.length"
      :review-id="detail.review_id"
      :findings="detail.findings"
      @download-patch="handleDownloadPatch"
    />

    <el-empty v-else description="本次审查未发现问题" :image-size="100" />

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

    <!-- OCR 执行信息（可折叠，默认收起降低噪音） -->
    <el-card v-if="detail.report?.ocr" class="ocr-collapse-card">
      <el-collapse v-model="ocrExpanded">
        <el-collapse-item name="ocr">
          <template #title>
            <span class="collapse-title">OCR 执行信息</span>
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

.review-summary-text {
  padding: 16px 20px;
  border: 1px solid var(--app-border);
  border-radius: 12px;
  background: #fff;
  font-size: 14px;
  line-height: 1.7;
  color: #4b5563;
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
