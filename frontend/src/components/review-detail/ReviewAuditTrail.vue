<script setup lang="ts">
/**
 * ReviewAuditTrail — 审查步骤审计面板（重新设计）。
 *
 * 设计目标：
 * - 默认展示每步的关键信息（状态、耗时、摘要）
 * - 时间线布局，直观展示流程
 * - 点击展开查看完整 Input/Output debug 数据
 * - 失败步骤高亮，成功步骤绿色
 */
import { ref, computed } from 'vue'
import type { Event } from '@/types/review'

const props = defineProps<{
  history: Event[]
}>()

const expandedSteps = ref<Set<number>>(new Set())

function toggleStep(index: number) {
  const s = new Set(expandedSteps.value)
  if (s.has(index)) s.delete(index)
  else s.add(index)
  expandedSteps.value = s
}

const sortedHistory = computed(() =>
  [...(props.history || [])].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime()
  )
)

const totalDuration = computed(() =>
  sortedHistory.value.reduce((sum, e) => sum + (e.duration_ms || 0), 0)
)

const stageConfig: Record<string, { icon: string; label: string; color: string }> = {
  queued:       { icon: '·', label: '任务入队', color: '#94a3b8' },
  dispatch:     { icon: '↗', label: '派发引擎', color: '#0d9488' },
  argus_report: { icon: '◉', label: 'Argus 回报', color: '#0d9488' },
  repository:  { icon: '1', label: '仓库准备', color: '#6366f1' },
  scope:       { icon: '2', label: '文件过滤', color: '#8b5cf6' },
  policy:      { icon: '3', label: '项目策略', color: '#f59e0b' },
  rules:       { icon: '4', label: '审查规则', color: '#8b5cf6' },
  agent:       { icon: '5', label: 'Agent 选择', color: '#ec4899' },
  ocr_config:  { icon: '6', label: 'OCR 参数', color: '#64748b' },
  ocr:         { icon: '7', label: 'OCR 审查', color: '#3b82f6' },
  judge:       { icon: '8', label: 'AI 判断', color: '#06b6d4' },
  gate:        { icon: '9', label: '门禁评估', color: '#f59e0b' },
  writeback:   { icon: '10', label: '写回 PR', color: '#10b981' },
  notify:      { icon: '11', label: '通知', color: '#64748b' },
  workflow:    { icon: 'W', label: '多 Agent', color: '#6366f1' },
  success:     { icon: '✓', label: '完成', color: '#22c55e' },
}

function cfg(stage: string) {
  return stageConfig[stage] || { icon: '?', label: stage, color: '#94a3b8' }
}

function fmtDuration(ms: number): string {
  if (!ms) return ''
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60000)
  const s = Math.round((ms % 60000) / 1000)
  return `${m}m ${s}s`
}

function fmtTime(ts: string): string {
  if (!ts) return ''
  return new Date(ts).toLocaleTimeString('zh-CN', {
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function fmtJSON(data: any): string {
  if (data == null) return '—'
  if (typeof data === 'string') return data
  try { return JSON.stringify(data, null, 2) } catch { return String(data) }
}

function outputSummary(step: Event): string {
  if (!step.output || typeof step.output !== 'object') return step.message || ''
  const o = step.output
  const parts: string[] = []

  // policy 步骤
  if (o.fail_on) parts.push(`门禁阈值: ${o.fail_on}`)
  if (o.ignore_paths?.length) parts.push(`${o.ignore_paths.length} 个过滤规则`)
  if (o.ocr_language) parts.push(`语言: ${o.ocr_language}`)

  // rules 步骤
  if (o.rule_path) parts.push(`规则文件已生成`)
  if (o.rules_content) parts.push(`含规则内容`)

  // agent 步骤
  if (o.available_agents?.length) parts.push(`${o.available_agents.length} 个 Agent`)
  if (o.execution_mode) parts.push(`模式: ${o.execution_mode}`)

  // ocr_config 步骤
  if (o.ready !== undefined) parts.push(o.ready ? '就绪' : '未就绪')
  if (o.has_rules !== undefined) parts.push(o.has_rules ? '有规则' : '无规则')

  // ocr 步骤
  if (o.files_reviewed != null) parts.push(`${o.files_reviewed} 个文件`)
  if (o.findings_count != null) parts.push(`${o.findings_count} 个问题`)
  if (o.comments != null) parts.push(`${o.comments} 条评论`)
  if (o.total_tokens) parts.push(`${(o.total_tokens / 1000).toFixed(1)}K tokens`)

  // judge 步骤
  if (o.confidence) parts.push(`置信度: ${o.confidence}`)
  if (o.recommended_gate) parts.push(`建议: ${o.recommended_gate}`)
  if (o.true_positives != null) parts.push(`${o.true_positives} 真实问题`)

  // gate 步骤
  if (o.passed != null) parts.push(o.passed ? '通过' : '未通过')

  // writeback 步骤
  if (o.published != null) parts.push(`发布 ${o.published} 条`)

  // scope 步骤
  if (o.changed_files != null) parts.push(`${o.changed_files} 个变更文件`)
  if (o.excluded_files) parts.push(`${o.excluded_files} 个排除`)

  // argus 步骤（dispatch / argus_report）
  if (o.verdict) parts.push(`裁决: ${verdictLabel(o.verdict)}`)
  if (o.findings_total != null) parts.push(`${o.findings_total} 个发现`)
  if (o.taskId) parts.push(`任务 ${o.taskId}`)
  if (o.duplicated) parts.push('重复任务命中幂等')

  return parts.join(' · ') || step.message || ''
}

const VERDICT_LABELS: Record<string, string> = {
  approve: '通过',
  comment: '评论',
  request_changes: '要求修改',
  error: '失败',
  pending: '等待中',
}

function verdictLabel(v: string) {
  return VERDICT_LABELS[v] || v
}
</script>

<template>
  <div class="audit-trail">
    <div class="audit-header">
      <div class="audit-title">
        <span class="audit-icon">▸</span>
        <span>审查步骤</span>
        <span class="audit-count">{{ sortedHistory.length }} 步</span>
      </div>
      <div v-if="totalDuration > 0" class="audit-total-time">
        总耗时 {{ fmtDuration(totalDuration) }}
      </div>
    </div>

    <div class="timeline">
      <div
        v-for="(step, i) in sortedHistory"
        :key="i"
        class="step"
        :class="[`step-${step.status}`, { 'step-expanded': expandedSteps.has(i) }]"
      >
        <!-- 左侧时间线 -->
        <div class="step-track">
          <div class="step-dot" :style="{ background: cfg(step.stage).color }">
            <span>{{ cfg(step.stage).icon }}</span>
          </div>
          <div v-if="i < sortedHistory.length - 1" class="step-line" />
        </div>

        <!-- 右侧内容 -->
        <div class="step-body" @click="toggleStep(i)">
          <!-- 第一行：阶段名 + 状态 + 耗时 -->
          <div class="step-row1">
            <span class="step-label" :style="{ color: cfg(step.stage).color }">
              {{ cfg(step.stage).label }}
            </span>
            <span
              class="step-status"
              :class="{
                'status-ok': step.status === 'completed',
                'status-err': step.status === 'failed',
                'status-run': step.status === 'running',
              }"
            >
              {{ step.status === 'completed' ? '完成' : step.status === 'failed' ? '失败' : step.status === 'running' ? '执行中' : step.status }}
            </span>
            <span v-if="step.duration_ms" class="step-time">
              {{ fmtDuration(step.duration_ms) }}
            </span>
            <span class="step-clock">{{ fmtTime(step.created_at) }}</span>
          </div>

          <!-- 第二行：摘要 -->
          <div class="step-row2">
            {{ outputSummary(step) }}
          </div>

          <!-- 展开的详情 -->
          <div v-if="expandedSteps.has(i)" class="step-details" @click.stop>
            <div v-if="step.how" class="detail-block">
              <div class="detail-label">方法</div>
              <div class="detail-value">{{ step.how }}</div>
            </div>

            <div v-if="step.input != null" class="detail-block">
              <div class="detail-label">输入</div>
              <pre class="detail-json">{{ fmtJSON(step.input) }}</pre>
            </div>

            <div v-if="step.output != null" class="detail-block">
              <div class="detail-label">输出</div>
              <pre class="detail-json">{{ fmtJSON(step.output) }}</pre>
            </div>

            <div v-if="step.details && Object.keys(step.details).length" class="detail-block">
              <div class="detail-label">详情</div>
              <pre class="detail-json">{{ fmtJSON(step.details) }}</pre>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.audit-trail {
  background: #fff;
  border: 1px solid #e5e7eb;
  border-radius: 12px;
  padding: 20px 24px;
  margin-bottom: 16px;
}

/* 头部 */
.audit-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  padding-bottom: 12px;
  border-bottom: 1px solid #f1f5f9;
}
.audit-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 600;
  font-size: 15px;
  color: #1e293b;
}
.audit-icon { font-size: 18px; }
.audit-count {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  background: #f1f5f9;
  padding: 2px 8px;
  border-radius: 10px;
}
.audit-total-time {
  font-size: 13px;
  color: #64748b;
}

/* 时间线 */
.timeline {
  display: flex;
  flex-direction: column;
}

.step {
  display: flex;
  gap: 16px;
  min-height: 56px;
}

/* 左侧轨道 */
.step-track {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 36px;
  flex-shrink: 0;
}
.step-dot {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
  box-shadow: 0 2px 8px rgba(0,0,0,0.08);
}
.step-line {
  width: 2px;
  flex: 1;
  min-height: 16px;
  background: #e2e8f0;
  margin: 4px 0;
}

/* 右侧内容 */
.step-body {
  flex: 1;
  padding: 6px 0 20px;
  cursor: pointer;
  min-width: 0;
}
.step-body:hover {
  background: #f8fafc;
  border-radius: 8px;
  margin: -4px;
  padding: 10px 8px 24px;
}

.step-row1 {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}
.step-label {
  font-weight: 600;
  font-size: 14px;
}
.step-status {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}
.status-ok {
  background: #dcfce7;
  color: #16a34a;
}
.status-err {
  background: #fee2e2;
  color: #dc2626;
}
.status-run {
  background: #fef3c7;
  color: #d97706;
}
.step-time {
  font-size: 12px;
  font-weight: 500;
  color: #64748b;
  background: #f1f5f9;
  padding: 2px 6px;
  border-radius: 4px;
}
.step-clock {
  font-size: 12px;
  color: #94a3b8;
  margin-left: auto;
}

.step-row2 {
  font-size: 13px;
  color: #475569;
  margin-top: 4px;
  line-height: 1.5;
}

/* 展开详情 */
.step-details {
  margin-top: 12px;
  padding: 12px;
  background: #f8fafc;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
}
.detail-block {
  margin-bottom: 10px;
}
.detail-block:last-child { margin-bottom: 0; }
.detail-label {
  font-size: 11px;
  font-weight: 600;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 4px;
}
.detail-value {
  font-size: 13px;
  color: #334155;
  line-height: 1.5;
}
.detail-json {
  font-family: 'SF Mono', 'Fira Code', 'Consolas', monospace;
  font-size: 11px;
  line-height: 1.6;
  background: #1e293b;
  color: #e2e8f0;
  padding: 10px 12px;
  border-radius: 6px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 240px;
  overflow-y: auto;
  margin: 0;
}

/* 失败步骤高亮 */
.step-step-failed .step-body {
  background: #fef2f2;
  border-radius: 8px;
  margin: -4px;
  padding: 10px 8px 24px;
  border-left: 3px solid #dc2626;
}
</style>
