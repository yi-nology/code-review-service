<script setup lang="ts">
/**
 * JudgmentPanel — OCR 结果二次综合判断面板。
 *
 * 展示 Judge（agentkit LLM）对 OCR 审查结果的二次分析：
 * - 置信度（对 OCR 结果的可信程度）
 * - 综合评估（人可读的文字分析）
 * - 门禁建议（pass/fail/conditional_pass）
 * - Finding 优先级排序 + 误报标记
 */
import { computed } from 'vue'
import { WarningFilled, SuccessFilled, InfoFilled, QuestionFilled } from '@element-plus/icons-vue'
import type { Judgment, FindingJudgment } from '@/types/review'

const props = defineProps<{
  judgment: Judgment
}>()

const confidenceIcon = computed(() => {
  switch (props.judgment.confidence) {
    case 'high': return SuccessFilled
    case 'medium': return WarningFilled
    case 'low': return QuestionFilled
    default: return InfoFilled
  }
})

const confidenceType = computed(() => {
  switch (props.judgment.confidence) {
    case 'high': return 'success'
    case 'medium': return 'warning'
    case 'low': return 'danger'
    default: return 'info'
  }
})

const confidenceLabel = computed(() => {
  switch (props.judgment.confidence) {
    case 'high': return '高置信度'
    case 'medium': return '中置信度'
    case 'low': return '低置信度'
    default: return '未知'
  }
})

const gateIcon = computed(() => {
  switch (props.judgment.recommended_gate) {
    case 'pass': return SuccessFilled
    case 'fail': return WarningFilled
    case 'conditional_pass': return InfoFilled
    default: return InfoFilled
  }
})

const gateType = computed(() => {
  switch (props.judgment.recommended_gate) {
    case 'pass': return 'success'
    case 'fail': return 'danger'
    case 'conditional_pass': return 'warning'
    default: return 'info'
  }
})

const gateLabel = computed(() => {
  switch (props.judgment.recommended_gate) {
    case 'pass': return '建议通过'
    case 'fail': return '建议拒绝'
    case 'conditional_pass': return '有条件通过'
    default: return '未知'
  }
})

const trueFindings = computed(() =>
  (props.judgment.prioritized_findings || []).filter(f => f.is_likely_true)
)

const falsePositives = computed(() =>
  (props.judgment.prioritized_findings || []).filter(f => !f.is_likely_true)
)
</script>

<template>
  <el-card class="judgment-panel">
    <template #header>
      <div class="judgment-header">
        <el-icon><WarningFilled /></el-icon>
        <span>AI 综合判断</span>
        <el-tag size="small" type="info" class="agentkit-tag">agentkit</el-tag>
      </div>
    </template>

    <!-- 置信度 + 门禁建议 -->
    <div class="judgment-badges">
      <el-tag :type="confidenceType" size="large" effect="dark">
        <el-icon><component :is="confidenceIcon" /></el-icon>
        {{ confidenceLabel }}
      </el-tag>
      <el-tag :type="gateType" size="large" effect="dark">
        <el-icon><component :is="gateIcon" /></el-icon>
        {{ gateLabel }}
      </el-tag>
    </div>

    <!-- 门禁理由 -->
    <div v-if="judgment.gate_reason" class="judgment-reason">
      <strong>理由：</strong>{{ judgment.gate_reason }}
    </div>

    <!-- 综合评估 -->
    <div v-if="judgment.overall_assessment" class="judgment-assessment">
      <h4>综合评估</h4>
      <p>{{ judgment.overall_assessment }}</p>
    </div>

    <!-- Finding 优先级列表 -->
    <div v-if="judgment.prioritized_findings?.length" class="judgment-findings">
      <h4>Finding 分析</h4>

      <!-- 可能是真实问题 -->
      <div v-if="trueFindings.length" class="finding-group">
        <h5 class="finding-group-title true-positive">
          <el-icon><WarningFilled /></el-icon>
          可能是真实问题（{{ trueFindings.length }}）
        </h5>
        <div v-for="f in trueFindings" :key="f.path + f.line + f.title" class="finding-item">
          <div class="finding-item-header">
            <el-tag :type="f.severity === 'critical' ? 'danger' : f.severity === 'major' ? 'warning' : 'info'" size="small">
              {{ f.severity }}
            </el-tag>
            <span class="finding-path">{{ f.path }}{{ f.line ? `:${f.line}` : '' }}</span>
            <span class="finding-priority">#{{ f.priority }}</span>
          </div>
          <div class="finding-title">{{ f.title }}</div>
          <div v-if="f.reason" class="finding-reason">{{ f.reason }}</div>
        </div>
      </div>

      <!-- 可能是误报 -->
      <div v-if="falsePositives.length" class="finding-group">
        <h5 class="finding-group-title false-positive">
          <el-icon><InfoFilled /></el-icon>
          可能是误报（{{ falsePositives.length }}）
        </h5>
        <div v-for="f in falsePositives" :key="f.path + f.line + f.title" class="finding-item faded">
          <div class="finding-item-header">
            <el-tag size="small" type="info">{{ f.severity }}</el-tag>
            <span class="finding-path">{{ f.path }}{{ f.line ? `:${f.line}` : '' }}</span>
          </div>
          <div class="finding-title">{{ f.title }}</div>
          <div v-if="f.reason" class="finding-reason">{{ f.reason }}</div>
        </div>
      </div>
    </div>
  </el-card>
</template>

<style scoped>
.judgment-panel {
  margin-bottom: 16px;
}
.judgment-header {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
}
.agentkit-tag {
  margin-left: auto;
}
.judgment-badges {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.judgment-badges .el-tag {
  display: flex;
  align-items: center;
  gap: 4px;
}
.judgment-reason {
  padding: 12px;
  background: var(--el-fill-color-light);
  border-radius: 8px;
  margin-bottom: 16px;
  font-size: 14px;
}
.judgment-assessment {
  margin-bottom: 16px;
}
.judgment-assessment h4 {
  margin: 0 0 8px;
  font-size: 14px;
  color: var(--el-text-color-regular);
}
.judgment-assessment p {
  margin: 0;
  line-height: 1.6;
  white-space: pre-wrap;
}
.judgment-findings h4 {
  margin: 0 0 12px;
  font-size: 14px;
  color: var(--el-text-color-regular);
}
.finding-group {
  margin-bottom: 16px;
}
.finding-group-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  margin: 0 0 8px;
}
.finding-group-title.true-positive {
  color: var(--el-color-danger);
}
.finding-group-title.false-positive {
  color: var(--el-color-info);
}
.finding-item {
  padding: 10px 12px;
  border: 1px solid var(--el-border-color-lighter);
  border-radius: 6px;
  margin-bottom: 6px;
}
.finding-item.faded {
  opacity: 0.6;
}
.finding-item-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}
.finding-path {
  font-family: monospace;
  font-size: 12px;
  color: var(--el-text-color-secondary);
}
.finding-priority {
  margin-left: auto;
  font-size: 12px;
  color: var(--el-text-color-placeholder);
}
.finding-title {
  font-size: 13px;
  font-weight: 500;
}
.finding-reason {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin-top: 4px;
  font-style: italic;
}
</style>
