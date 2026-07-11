<script setup lang="ts">
/**
 * ReviewSummarySidebar — 评审详情左栏（sticky）。
 * 合并了原 ReviewGateCard + 质量评估卡的逻辑：门禁状态、评分环、严重度分布、
 * 门禁解释、优先修复列表，加上元信息和操作按钮。
 */
import { computed } from 'vue'
import { SuccessFilled, CircleCloseFilled, InfoFilled, TopRight } from '@element-plus/icons-vue'
import type { ReviewDetail, Finding, Report } from '@/types/review'
import { getStatusLabel, getStatusType } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import { DEAD_LETTER_THRESHOLD } from '@/constants/review'

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

const isDeadLetter = computed(() =>
  props.detail.status === 'failed' && (props.detail.retry_count ?? 0) >= DEAD_LETTER_THRESHOLD,
)

const isFailed = computed(() =>
  props.detail.status === 'failed' ||
  Boolean(props.detail.gate?.status && ['failed', 'error'].includes(props.detail.gate.status)),
)

const gatePassed = computed(() => props.detail.gate?.passed === true)

const gateLabel = computed(() => {
  if (gatePassed.value) return '门禁通过'
  if (isFailed.value) return props.detail.status === 'failed' ? '执行失败' : '门禁未通过'
  return '等待结果'
})

// 优先修复：保留原始 index，便于点击跳转到对应 finding 卡片。
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
  if (props.detail.status === 'failed') {
    items.push({ title: '执行失败', body: '审查流程未完整完成，请查看失败提示、OCR 配置和任务日志。' })
  }
  if (props.detail.gate?.status === 'failed') {
    items.push({ title: '门禁策略触发', body: props.detail.gate?.reason || '当前审查结果未满足质量门禁。' })
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
  return map[severity] || severity
}

function scoreColor(score: number) {
  if (score >= 85) return '#22c55e'
  if (score >= 70) return '#e6a23c'
  return '#f56c6c'
}
</script>

<template>
  <div class="review-sidebar">
    <!-- 门禁 + 评分 + 严重度 合并卡 -->
    <div class="summary-card" :class="gatePassed ? 'passed' : isFailed ? 'failed' : 'pending'">
      <div class="gate-banner">
        <el-icon :size="32">
          <SuccessFilled v-if="gatePassed" />
          <CircleCloseFilled v-else-if="isFailed" />
          <InfoFilled v-else />
        </el-icon>
        <strong>{{ gateLabel }}</strong>
      </div>

      <!-- 评分环 + 严重度 -->
      <div class="score-severity" v-if="detail.report?.generated">
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
          <span>{{ item.finding.path || '未知' }}<template v-if="item.finding.line">:{{ item.finding.line }}</template></span>
          <el-icon class="blocking-arrow"><TopRight /></el-icon>
        </button>
      </div>
    </div>

    <!-- 元信息 -->
    <div class="meta-card">
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
      <div class="meta-row">
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

/* 操作 */
.action-card {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
