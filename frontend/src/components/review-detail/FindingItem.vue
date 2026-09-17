<template>
  <div :id="`finding-${index}`" class="finding-item" :class="{ highlight, disposed: Boolean(feedback) }">
    <div class="finding-header">
      <el-tooltip v-if="showPatch" :disabled="patchable" :content="patchBlockReason" placement="top">
        <el-checkbox
          class="finding-patch-checkbox"
          :model-value="selected"
          :disabled="!patchable"
          @change="(checked: boolean | string | number) => emit('toggle-patch', Boolean(checked))"
        />
      </el-tooltip>
      <el-tag :type="severityType" size="small">
        {{ finding.severity.toUpperCase() }}
      </el-tag>
      <span class="finding-title">{{ finding.title }}</span>
      <span class="finding-path" v-if="finding.path">
        {{ finding.path }}{{ finding.line ? `:${finding.line}` : '' }}
      </span>
      <el-tag v-if="showPatch && !patchable" size="small" type="info">
        {{ patchBlockReason }}
      </el-tag>
      <el-dropdown trigger="click" @command="(command: string) => emit('feedback', command)">
        <el-button size="small" plain>
          {{ feedbackLabel(feedback?.status) || '标记' }}
        </el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="false_positive">误报</el-dropdown-item>
            <el-dropdown-item command="accepted_risk">接受风险</el-dropdown-item>
            <el-dropdown-item command="deferred">暂不处理</el-dropdown-item>
            <el-dropdown-item command="fixed">已修复</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>
      <el-tag v-if="feedback" size="small" type="info">
        {{ feedbackLabel(feedback.status) }}
      </el-tag>
      <el-tag
        v-if="historicalFp"
        size="small"
        type="warning"
        effect="plain"
      >
        历史误报
      </el-tag>
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
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { getSeverityType } from '@/utils/status'
import type { Finding, FindingFeedback } from '@/types/review'

const props = defineProps<{
  finding: Finding
  index: number
  selected: boolean
  patchable: boolean
  patchBlockReason: string
  feedback?: FindingFeedback
  historicalFp: boolean
  highlight: boolean
  // showPatch 是否展示 Patch 勾选与不可生成占位（argus 等无建议代码的模式置 false）
  showPatch?: boolean
}>()

const emit = defineEmits<{
  'toggle-patch': [checked: boolean]
  feedback: [status: string]
}>()

const severityType = computed(() => getSeverityType(props.finding.severity))

const FEEDBACK_LABELS: Record<string, string> = {
  false_positive: '误报',
  accepted_risk: '接受风险',
  deferred: '暂不处理',
  fixed: '已修复',
}

function feedbackLabel(status?: string) {
  return status ? FEEDBACK_LABELS[status] || status : ''
}
</script>

<style scoped>
.finding-item {
  padding: 14px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 10px;
  scroll-margin-top: 90px;
}

.finding-item.disposed {
  opacity: 0.6;
}

.finding-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-bottom: 8px;
}

.finding-patch-checkbox {
  margin-right: 4px;
}

.finding-title {
  font-weight: 600;
  font-size: 14px;
}

.finding-path {
  color: #8892a4;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.finding-body {
  margin-bottom: 10px;
  color: #5d6677;
  font-size: 13px;
  line-height: 1.6;
}

.finding-code {
  margin-top: 8px;
}

.code-label {
  margin-bottom: 4px;
  color: #8892a4;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
}

.finding-code pre {
  padding: 10px;
  background: #f6f8fa;
  border-radius: 6px;
  font-size: 12px;
  line-height: 1.5;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}
</style>
