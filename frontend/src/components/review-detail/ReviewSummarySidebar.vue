<script setup lang="ts">
/**
 * ReviewSummarySidebar — 评审详情左栏（sticky），按审查模式差异化呈现。
 *
 * - ocr   : 门禁 + 评分环 + 严重度分布 + 门禁解释 + 优先修复
 * - argus : Argus 裁决卡（verdict hero + 发现分布 + 任务信息 + 原始报告入口），无本地评分
 * - scan  : 门禁 + 扫描范围统计（文件数/tokens/耗时），无 PR 语义
 * 底部统一为元信息卡与操作按钮。
 */
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  SuccessFilled, CircleCloseFilled, InfoFilled, TopRight,
  View, Cpu, Search, WarningFilled, DataLine,
} from '@element-plus/icons-vue'
import type { ReviewDetail, Finding } from '@/types/review'
import { getStatusLabel, getStatusType } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import { formatGateReason } from '@/utils/reviewText'
import { DEAD_LETTER_THRESHOLD } from '@/constants/review'
import {
  resolveReviewMode, REVIEW_MODE_META,
  extractArgusOutcome, ARGUS_VERDICT_META,
} from '@/constants/reviewMode'

const props = defineProps<{
  detail: ReviewDetail
  acting: boolean
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'rerun'): void
  (e: 'requeue'): void
  (e: 'refresh'): void
  (e: 'focus-finding', index: number): void
}>()

const router = useRouter()

const mode = computed(() => resolveReviewMode(props.detail))
const modeMeta = computed(() => REVIEW_MODE_META[mode.value])
const argusOutcome = computed(() =>
  mode.value === 'argus' ? extractArgusOutcome(props.detail) : null,
)
const verdictMeta = computed(() =>
  argusOutcome.value ? ARGUS_VERDICT_META[argusOutcome.value.verdict] : null,
)

const isDeadLetter = computed(() =>
  props.detail.status === 'failed' && (props.detail.retry_count ?? 0) >= DEAD_LETTER_THRESHOLD,
)

const isFailed = computed(() =>
  props.detail.status === 'failed' ||
  Boolean(props.detail.gate?.status && ['failed', 'error'].includes(props.detail.gate.status)),
)

const gatePassed = computed(() => props.detail.gate?.passed === true)

const gateLabel = computed(() => {
  if (mode.value === 'argus') {
    if (argusOutcome.value?.verdict === 'pending') return '等待 Argus 回报'
    return 'Argus 已裁决'
  }
  if (gatePassed.value) return '门禁通过'
  if (isFailed.value) return props.detail.status === 'failed' ? '执行失败' : '门禁未通过'
  return '等待结果'
})

const argusFindings = computed(() => {
  const o = argusOutcome.value
  if (!o || o.verdict === 'pending' || o.findingsTotal == null) return null
  return [
    { key: 'high', label: '高风险', count: o.findingsHigh ?? 0, color: '#f56c6c' },
    { key: 'medium', label: '中风险', count: o.findingsMedium ?? 0, color: '#e6a23c' },
    { key: 'low', label: '低风险', count: o.findingsLow ?? 0, color: '#409eff' },
  ]
})

const argusFindingsMax = computed(() =>
  Math.max(1, ...(argusFindings.value || []).map((f) => f.count)),
)

const scope = computed(() => props.detail.report?.scope)
const ocrMeta = computed(() => props.detail.report?.ocr)

// 优先修复：保留原始 index，便于点击跳转到对应 finding 卡片（ocr/scan 模式）。
const blockingFindings = computed(() =>
  (props.detail.findings || [])
    .map((f: Finding, index: number) => ({ finding: f, index }))
    .filter(({ finding }) => ['critical', 'major'].includes(finding.severity))
    .slice(0, 5),
)

const explanations = computed(() => {
  const items: Array<{ title: string; body: string }> = []
  const counts = props.detail.report?.counts
  const score = props.detail.report?.score

  if (mode.value === 'argus') {
    const o = argusOutcome.value
    if (o?.verdict === 'pending') {
      items.push({ title: '异步回报中', body: 'Argus 审查完成后会自动回报裁决，本页轮询刷新即可看到结果。' })
    } else if (o) {
      items.push({ title: `裁决：${verdictMeta.value?.label || o.verdict}`, body: props.detail.summary || 'Argus 裁决已写入门禁结果。' })
    }
    if (props.detail.gate?.status === 'failed') {
      items.push({ title: '门禁策略触发', body: formatGateReason(props.detail.gate?.reason) || 'Argus 裁决未满足质量门禁。' })
    }
    return items
  }

  if (props.detail.status === 'failed') {
    items.push({ title: '执行失败', body: '审查流程未完整完成，请查看失败提示、OCR 配置和任务日志。' })
  }
  if (props.detail.gate?.status === 'failed') {
    items.push({ title: '门禁策略触发', body: formatGateReason(props.detail.gate?.reason) || '当前审查结果未满足质量门禁。' })
  }
  if (counts && (counts.critical > 0 || counts.major > 0)) {
    items.push({ title: '高严重度问题', body: `严重 ${counts.critical} 个，高风险 ${counts.major} 个。优先修复可解除门禁阻塞。` })
  }
  if (score?.deductions?.length) {
    const top = score.deductions.slice(0, 2).map((d) => `${d.reason} ${d.points > 0 ? `-${d.points}` : `+${Math.abs(d.points)}`}分`).join('；')
    items.push({ title: '评分扣减', body: top })
  }
  if (!items.length && gatePassed.value) {
    items.push({ title: '通过原因', body: '未发现超过门禁阈值的风险，评分和严重度满足策略要求。' })
  }
  return items
})

function severityLabel(severity: string) {
  const map: Record<string, string> = { critical: '严重', major: '高', minor: '中', info: '提示' }
  return map[severity] || '未知'
}

function scoreColor(score: number) {
  if (score >= 85) return '#22c55e'
  if (score >= 70) return '#e6a23c'
  return '#f56c6c'
}

function fmtDuration(ms?: number) {
  if (!ms) return '—'
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  const m = Math.floor(ms / 60000)
  const s = Math.round((ms % 60000) / 1000)
  return `${m}m ${s}s`
}

function goArgusReport() {
  const id = argusOutcome.value?.argusReviewId
  if (id) router.push(`/argus/${id}`)
}
</script>

<template>
  <div class="review-sidebar">
    <!-- ═══ Argus 模式：裁决卡 ═══ -->
    <div
      v-if="mode === 'argus' && argusOutcome && verdictMeta"
      class="summary-card argus-card"
      :style="{ '--mode-color': modeMeta.color, '--mode-soft': modeMeta.soft }"
    >
      <div class="mode-strip">
        <el-icon :size="14"><View /></el-icon>
        <span>{{ modeMeta.label }}</span>
      </div>

      <div class="verdict-hero" :style="{ background: verdictMeta.soft, borderColor: verdictMeta.border }">
        <el-icon :size="34" :color="verdictMeta.color">
          <CircleCloseFilled v-if="argusOutcome.verdict === 'request_changes' || argusOutcome.verdict === 'error'" />
          <SuccessFilled v-else-if="argusOutcome.verdict === 'approve'" />
          <InfoFilled v-else />
        </el-icon>
        <div class="verdict-text">
          <strong :style="{ color: verdictMeta.color }">{{ verdictMeta.label }}</strong>
          <small>Argus 外部引擎裁决</small>
        </div>
      </div>

      <!-- 发现分布 -->
      <div v-if="argusFindings" class="argus-findings">
        <div class="findings-header">
          <span>发现分布</span>
          <strong>{{ argusOutcome.findingsTotal ?? 0 }}</strong>
        </div>
        <div v-for="row in argusFindings" :key="row.key" class="finding-row">
          <span class="finding-label">{{ row.label }}</span>
          <div class="finding-bar">
            <div
              class="finding-bar-fill"
              :style="{ width: `${(row.count / argusFindingsMax) * 100}%`, background: row.color }"
            ></div>
          </div>
          <strong class="finding-count" :style="{ color: row.count > 0 ? row.color : '#9ca3af' }">{{ row.count }}</strong>
        </div>
      </div>

      <!-- 任务信息 -->
      <div class="task-info">
        <div v-if="argusOutcome.durationMs" class="task-row">
          <span>审查耗时</span><strong>{{ fmtDuration(argusOutcome.durationMs) }}</strong>
        </div>
        <div v-if="argusOutcome.argusTaskId" class="task-row">
          <span>Argus 任务</span><small class="task-id">{{ argusOutcome.argusTaskId }}</small>
        </div>
      </div>

      <el-button
        v-if="argusOutcome.argusReviewId"
        class="argus-link"
        plain size="small"
        :icon="DataLine"
        @click="goArgusReport"
      >查看 Argus 原始报告</el-button>

      <!-- 裁决解释 -->
      <div v-if="explanations.length" class="gate-explanations">
        <div v-for="(item, i) in explanations" :key="i" class="explain-item">
          <strong>{{ item.title }}</strong>
          <span>{{ item.body }}</span>
        </div>
      </div>
    </div>

    <!-- ═══ OCR / Scan 模式：门禁 + 评分/范围 合并卡 ═══ -->
    <div
      v-else
      class="summary-card"
      :class="[gatePassed ? 'passed' : isFailed ? 'failed' : 'pending', `mode-${mode}`]"
      :style="{ '--mode-color': modeMeta.color }"
    >
      <div class="mode-strip">
        <el-icon :size="14">
          <Search v-if="mode === 'scan'" />
          <Cpu v-else />
        </el-icon>
        <span>{{ modeMeta.label }}</span>
      </div>

      <div class="gate-banner">
        <el-icon :size="32">
          <SuccessFilled v-if="gatePassed" />
          <CircleCloseFilled v-else-if="isFailed" />
          <WarningFilled v-else-if="mode === 'scan'" />
          <InfoFilled v-else />
        </el-icon>
        <strong>{{ gateLabel }}</strong>
      </div>

      <!-- Judge 置信度标签 -->
      <div v-if="detail.judgment" class="judgment-badge">
        <el-tag
          :type="detail.judgment.confidence === 'high' ? 'success' : detail.judgment.confidence === 'medium' ? 'warning' : 'danger'"
          size="small"
          effect="plain"
        >
          AI 置信度: {{ detail.judgment.confidence === 'high' ? '高' : detail.judgment.confidence === 'medium' ? '中' : '低' }}
        </el-tag>
        <el-tag
          v-if="detail.judgment.recommended_gate !== (gatePassed ? 'pass' : 'fail')"
          :type="detail.judgment.recommended_gate === 'pass' ? 'success' : detail.judgment.recommended_gate === 'fail' ? 'danger' : 'warning'"
          size="small"
          effect="plain"
        >
          AI 建议: {{ detail.judgment.recommended_gate === 'pass' ? '通过' : detail.judgment.recommended_gate === 'fail' ? '拒绝' : '有条件通过' }}
        </el-tag>
      </div>

      <!-- OCR 模式：评分环 + 严重度 -->
      <div class="score-severity" v-if="mode === 'ocr' && detail.report?.generated">
        <div class="score-ring" :style="{ '--score-color': scoreColor(detail.report.score.value) }">
          <svg viewBox="0 0 80 80" class="ring-svg">
            <circle cx="40" cy="40" r="34" fill="none" stroke="#eee" stroke-width="6" />
            <circle
              cx="40" cy="40" r="34" fill="none"
              :stroke="'var(--score-color)'" stroke-width="6"
              stroke-linecap="round"
              :stroke-dasharray="2 * Math.PI * 34"
              :stroke-dashoffset="2 * Math.PI * 34 * (1 - detail.report.score.value / 100)"
              transform="rotate(-90 40 40)"
            />
          </svg>
          <div class="score-center">
            <strong>{{ detail.report.score.value }}</strong>
            <small>/ 100</small>
          </div>
        </div>
        <div class="severity-list">
          <div class="severity-row critical">
            <span>严重</span><strong>{{ detail.report.counts.critical }}</strong>
          </div>
          <div class="severity-row major">
            <span>高风险</span><strong>{{ detail.report.counts.major }}</strong>
          </div>
          <div class="severity-row minor">
            <span>中风险</span><strong>{{ detail.report.counts.minor }}</strong>
          </div>
          <div class="severity-row info">
            <span>提示</span><strong>{{ detail.report.counts.info }}</strong>
          </div>
        </div>
      </div>

      <!-- Scan 模式：扫描范围统计 -->
      <div v-if="mode === 'scan'" class="scope-grid">
        <div class="scope-cell">
          <strong>{{ scope?.reviewed_files ?? ocrMeta?.files_reviewed ?? 0 }}</strong>
          <span>审查文件</span>
        </div>
        <div class="scope-cell">
          <strong>{{ scope?.excluded_files ?? 0 }}</strong>
          <span>排除文件</span>
        </div>
        <div class="scope-cell">
          <strong>{{ ocrMeta?.total_tokens ? `${(ocrMeta.total_tokens / 1000).toFixed(1)}K` : '—' }}</strong>
          <span>Tokens</span>
        </div>
        <div class="scope-cell">
          <strong>{{ ocrMeta?.elapsed || '—' }}</strong>
          <span>耗时</span>
        </div>
      </div>

      <!-- 门禁解释 -->
      <div v-if="explanations.length" class="gate-explanations">
        <div v-for="(item, i) in explanations" :key="i" class="explain-item">
          <strong>{{ item.title }}</strong>
          <span>{{ item.body }}</span>
        </div>
      </div>

      <!-- 优先修复：点击跳转到对应 finding 并高亮 -->
      <div v-if="blockingFindings.length" class="blocking-list">
        <strong>优先修复</strong>
        <button
          v-for="item in blockingFindings"
          :key="item.index"
          type="button"
          class="blocking-item"
          @click="emit('focus-finding', item.index)"
        >
          <el-tag size="small" :type="item.finding.severity === 'critical' ? 'danger' : 'warning'">{{ severityLabel(item.finding.severity) }}</el-tag>
          <span :title="`${item.finding.path || '未知'}${item.finding.line ? ':' + item.finding.line : ''}`">{{ item.finding.path || '未知' }}<template v-if="item.finding.line">:{{ item.finding.line }}</template></span>
          <el-icon class="blocking-arrow"><TopRight /></el-icon>
        </button>
      </div>
    </div>

    <!-- 元信息 -->
    <div class="meta-card">
      <div class="meta-row">
        <span>模式</span>
        <span class="mode-chip" :style="{ color: modeMeta.color, borderColor: modeMeta.border, background: modeMeta.soft }">
          {{ modeMeta.label }}
        </span>
      </div>
      <div class="meta-row">
        <span>状态</span>
        <el-tag :type="getStatusType(detail.status)" size="small">{{ getStatusLabel(detail.status) }}</el-tag>
      </div>
      <div class="meta-row" v-if="detail.pull_request_id">
        <span>PR</span><strong>#{{ detail.pull_request_id }}</strong>
      </div>
      <div class="meta-row" v-if="detail.merge_request_iid">
        <span>MR</span><strong>!{{ detail.merge_request_iid }}</strong>
      </div>
      <div class="meta-row" v-if="detail.author">
        <span>开发者</span><strong>{{ detail.author_name || detail.author }}</strong>
      </div>
      <div class="meta-row" v-if="mode !== 'argus'">
        <span>评分</span><strong>{{ detail.report?.score?.grade || '-' }}</strong>
      </div>
      <div class="meta-row">
        <span>更新</span><small>{{ formatDateTime(detail.updated_at) }}</small>
      </div>
    </div>

    <!-- 操作按钮 -->
    <div class="action-card">
      <el-button
        v-if="detail.status === 'queued' || detail.status === 'running'"
        type="danger" plain size="small" :loading="acting"
        @click="emit('cancel')"
      >取消审查</el-button>
      <el-button
        v-if="isDeadLetter"
        type="warning" plain size="small" :loading="acting"
        @click="emit('requeue')"
      >重新入队</el-button>
      <el-button
        v-if="detail.status === 'failed' || detail.status === 'success'"
        type="primary" plain size="small" :loading="acting"
        @click="emit('rerun')"
      >重新审查</el-button>
      <el-button plain size="small" @click="emit('refresh')">刷新</el-button>
    </div>
  </div>
</template>

<style scoped>
.review-sidebar {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

/* 模式标识条（卡片顶部） */
.mode-strip {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 18px;
  background: var(--mode-soft, #f8fafc);
  border-bottom: 1px solid var(--app-border);
  color: var(--mode-color, #64748b);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.3px;
}

/* 门禁+评分合并卡 */
.summary-card {
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: #fff;
  overflow: hidden;
}
.summary-card.passed { border-color: #bbf7d0; }
.summary-card.failed { border-color: #fecaca; }
.summary-card.pending { border-color: #e5e7eb; }
.summary-card.mode-argus { border-color: var(--mode-border, #99f6e4); }
.summary-card.mode-scan { border-color: var(--mode-border, #fde68a); }

.argus-card {
  border-color: var(--mode-border, #99f6e4);
}

.gate-banner {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 18px;
  font-size: 15px;
  font-weight: 600;
}
.summary-card.passed .gate-banner { background: #f0fdf4; color: #16a34a; }
.summary-card.failed .gate-banner { background: #fef2f2; color: #dc2626; }
.summary-card.pending .gate-banner { background: #f9fafb; color: #6b7280; }

/* ── Argus 裁决卡 ── */
.verdict-hero {
  display: flex;
  align-items: center;
  gap: 14px;
  margin: 14px 16px 0;
  padding: 14px 16px;
  border: 1px solid;
  border-radius: 12px;
}
.verdict-text {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.verdict-text strong {
  font-size: 19px;
  line-height: 1.3;
}
.verdict-text small {
  margin-top: 2px;
  color: #64748b;
  font-size: 11.5px;
}

.argus-findings {
  padding: 14px 18px 4px;
}
.findings-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
  font-size: 12px;
  color: #6b7280;
}
.findings-header strong {
  font-size: 15px;
  color: var(--app-text);
}
.finding-row {
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr) 28px;
  align-items: center;
  gap: 8px;
  padding: 3px 0;
  font-size: 12px;
}
.finding-label { color: #6b7280; }
.finding-bar {
  height: 7px;
  border-radius: 4px;
  background: #f1f5f9;
  overflow: hidden;
}
.finding-bar-fill {
  height: 100%;
  border-radius: 4px;
  min-width: 2px;
  transition: width 400ms ease;
}
.finding-count {
  text-align: right;
  font-size: 13px;
}

.task-info {
  padding: 10px 18px 2px;
}
.task-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 3px 0;
  font-size: 12px;
}
.task-row span { color: #9ca3af; }
.task-row strong { color: var(--app-text); }
.task-id {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: #64748b;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
}

.argus-link {
  margin: 10px 16px 0;
  width: calc(100% - 32px);
}

/* Judge 置信度标签 */
.judgment-badge {
  display: flex;
  gap: 6px;
  padding: 8px 16px;
  background: var(--el-fill-color-lighter);
  border-bottom: 1px solid var(--el-border-color-lighter);
}

.score-severity {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 18px;
}

.score-ring {
  position: relative;
  width: 80px;
  height: 80px;
  flex-shrink: 0;
}
.ring-svg {
  width: 100%;
  height: 100%;
}
.score-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.score-center strong {
  font-size: 22px;
  font-weight: 700;
  color: var(--score-color);
}
.score-center small {
  font-size: 10px;
  color: #9ca3af;
}

.severity-list {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.severity-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 13px;
}
.severity-row span { color: #6b7280; }
.severity-row strong { font-size: 15px; }
.severity-row.critical strong { color: #f56c6c; }
.severity-row.major strong { color: #e6a23c; }
.severity-row.minor strong { color: #409eff; }
.severity-row.info strong { color: #909399; }

/* ── Scan 范围统计 ── */
.scope-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  padding: 16px 18px;
}
.scope-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: #fafbfc;
}
.scope-cell strong {
  font-size: 16px;
  color: var(--app-text);
}
.scope-cell span {
  font-size: 11px;
  color: #9ca3af;
}

.gate-explanations {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 0 18px 14px;
}
.explain-item {
  padding: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fafbfc;
}
.explain-item strong {
  display: block;
  font-size: 12px;
  color: var(--app-text);
  margin-bottom: 2px;
}
.explain-item span {
  font-size: 11px;
  color: #667085;
  line-height: 1.5;
}

.blocking-list {
  padding: 0 18px 16px;
}
.blocking-list > strong {
  display: block;
  font-size: 12px;
  color: var(--app-text);
  margin-bottom: 8px;
}
.blocking-item {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 6px 4px;
  border: 0;
  border-top: 1px solid var(--app-border);
  border-radius: 0;
  background: transparent;
  color: #374151;
  text-align: left;
  font-size: 12px;
  cursor: pointer;
  transition: background 120ms ease;
}

.blocking-item:hover {
  background: #f3f5f9;
}

.blocking-item span {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.blocking-arrow {
  flex-shrink: 0;
  color: #c4cad6;
  font-size: 11px;
}

.blocking-item:hover .blocking-arrow {
  color: #2563eb;
}

/* 元信息 */
.meta-card {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  background: #fff;
  padding: 14px 16px;
}
.meta-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 5px 0;
  font-size: 12px;
}
.meta-row span { color: #9ca3af; }
.meta-row strong { color: var(--app-text); }
.meta-row small { color: #9ca3af; font-size: 11px; }
.mode-chip {
  padding: 2px 8px;
  border: 1px solid;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
}

/* 操作 */
.action-card {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
