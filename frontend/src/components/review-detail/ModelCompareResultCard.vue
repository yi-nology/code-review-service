<script setup lang="ts">
/**
 * ModelCompareResultCard — 模型 A/B 对比的单个结果卡。
 * 成功：评分环 + 指标 + 摘要(可展开明细)；失败：错误框。
 * 评分环复用 ReviewSummarySidebar 的 SVG 圆环语言。
 */
import { computed } from 'vue'
import { ArrowDown, ArrowUp } from '@element-plus/icons-vue'
import type { ModelCompareCandidateRun } from '@/types/review'

const props = defineProps<{
  run: ModelCompareCandidateRun
  index: number
  isWinner: boolean
  isFastest: boolean
  expanded: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle'): void
}>()

const score = computed(() => props.run.score ?? 0)
const grade = computed(() => props.run.grade || '-')
const ringColor = computed(() => {
  if (score.value >= 85) return '#22c55e'
  if (score.value >= 70) return '#e6a23c'
  return '#f56c6c'
})

const ringDash = 2 * Math.PI * 34

function formatNumber(value?: number) {
  return value ? value.toLocaleString() : '0'
}

function formatDuration(ms?: number) {
  const s = Math.round((ms || 0) / 1000)
  if (s < 60) return `${s}s`
  const m = Math.floor(s / 60)
  return `${m}m ${s % 60}s`
}

const laneLabel = computed(() => String.fromCharCode(65 + props.index))
</script>

<template>
  <div class="result-card" :class="{ winner: isWinner && run.success, failed: !run.success }">
    <!-- 头部：徽章 + 模型名 + 标记 -->
    <div class="result-head">
      <div class="head-left">
        <div class="lane-badge" :class="`lane-${laneLabel}`">{{ laneLabel }}</div>
        <div class="head-meta">
          <strong class="model-name">{{ run.model }}</strong>
          <span v-if="run.provider" class="model-provider">{{ run.provider }}</span>
        </div>
      </div>
      <div class="head-marks">
        <span v-if="isWinner && run.success" class="mark-chip winner">🏆 质量最高</span>
        <span v-if="isFastest && run.success" class="mark-chip fastest">⚡ 最快</span>
      </div>
    </div>

    <!-- 成功：评分环 + 指标 -->
    <template v-if="run.success">
      <div class="score-area">
        <div class="score-ring" :style="{ '--score-color': ringColor }">
          <svg viewBox="0 0 80 80" class="ring-svg">
            <circle cx="40" cy="40" r="34" fill="none" stroke="#eee" stroke-width="6" />
            <circle
              cx="40" cy="40" r="34" fill="none"
              :stroke="ringColor" stroke-width="6"
              stroke-linecap="round"
              :stroke-dasharray="ringDash"
              :stroke-dashoffset="ringDash * (1 - score / 100)"
              transform="rotate(-90 40 40)"
            />
          </svg>
          <div class="score-center">
            <strong :style="{ color: ringColor }">{{ score }}</strong>
            <small>/ 100</small>
          </div>
        </div>
        <div class="score-side">
          <div class="grade-tag" :style="{ '--grade-color': ringColor }">等级 {{ grade }}</div>
          <div class="metric-grid">
            <div class="metric">
              <span class="metric-label">问题</span>
              <strong class="metric-value">{{ run.findings || 0 }}</strong>
            </div>
            <div class="metric">
              <span class="metric-label">警告</span>
              <strong class="metric-value">{{ run.warnings || 0 }}</strong>
            </div>
            <div class="metric">
              <span class="metric-label">Tokens</span>
              <strong class="metric-value">{{ formatNumber(run.total_tokens) }}</strong>
            </div>
            <div class="metric">
              <span class="metric-label">耗时</span>
              <strong class="metric-value">{{ formatDuration(run.duration_ms) }}</strong>
            </div>
          </div>
        </div>
      </div>

      <!-- 摘要 -->
      <p v-if="run.summary" class="result-summary">{{ run.summary }}</p>

      <!-- 展开明细 -->
      <button type="button" class="expand-btn" @click="emit('toggle')">
        <el-icon :size="12">
          <ArrowDown v-if="!expanded" />
          <ArrowUp v-else />
        </el-icon>
        {{ expanded ? '收起明细' : '展开明细' }}
      </button>
      <div v-show="expanded" class="detail-panel">
        <div v-if="run.input_tokens || run.output_tokens" class="detail-section">
          <span class="detail-title">Token 拆解</span>
          <div class="detail-row">
            <span>输入</span><strong>{{ formatNumber(run.input_tokens) }}</strong>
          </div>
          <div class="detail-row">
            <span>输出</span><strong>{{ formatNumber(run.output_tokens) }}</strong>
          </div>
        </div>
        <div v-if="run.url" class="detail-section">
          <span class="detail-title">推理地址</span>
          <code class="detail-code">{{ run.url }}</code>
        </div>
        <div v-if="run.elapsed" class="detail-section">
          <span class="detail-title">原始耗时</span>
          <span class="detail-text">{{ run.elapsed }}</span>
        </div>
        <div v-if="run.summary" class="detail-section">
          <span class="detail-title">完整摘要</span>
          <p class="detail-text">{{ run.summary }}</p>
        </div>
      </div>
    </template>

    <!-- 失败：错误框 -->
    <div v-else class="error-box">
      <strong>执行失败</strong>
      <span>{{ run.error || '未知错误' }}</span>
    </div>
  </div>
</template>

<style scoped>
.result-card {
  border: 1px solid var(--app-border);
  border-radius: 12px;
  background: #fff;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  transition: border-color 0.2s, box-shadow 0.2s;
}
.result-card.winner {
  border-color: #22c55e;
  box-shadow: 0 0 0 1px #bbf7d0;
}
.result-card.failed {
  border-color: #fecaca;
  background: #fefdfd;
}

/* 头部 */
.result-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
}
.head-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}
.lane-badge {
  width: 28px;
  height: 28px;
  border-radius: 7px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 13px;
  color: #fff;
  flex-shrink: 0;
}
.lane-A { background: linear-gradient(135deg, #3b82f6, #2563eb); }
.lane-B { background: linear-gradient(135deg, #8b5cf6, #7c3aed); }
.lane-C { background: linear-gradient(135deg, #10b981, #059669); }
.lane-D { background: linear-gradient(135deg, #f59e0b, #d97706); }
.lane-E { background: linear-gradient(135deg, #ec4899, #db2777); }
.lane-F { background: linear-gradient(135deg, #6b7280, #4b5563); }

.head-meta {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.model-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--app-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.model-provider {
  font-size: 11px;
  color: var(--app-muted);
}

.head-marks {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 4px;
  flex-shrink: 0;
}
.mark-chip {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 999px;
  white-space: nowrap;
}
.mark-chip.winner { background: #f0fdf4; color: #16a34a; }
.mark-chip.fastest { background: #eff6ff; color: #2563eb; }

/* 评分区 */
.score-area {
  display: flex;
  align-items: center;
  gap: 14px;
}
.score-ring {
  position: relative;
  width: 76px;
  height: 76px;
  flex-shrink: 0;
}
.ring-svg { width: 100%; height: 100%; }
.score-center {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.score-center strong {
  font-size: 20px;
  font-weight: 700;
  line-height: 1;
}
.score-center small {
  font-size: 10px;
  color: #9ca3af;
}

.score-side {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}
.grade-tag {
  align-self: flex-start;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 10px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--grade-color) 12%, #fff);
  color: var(--grade-color);
}

.metric-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px 12px;
}
.metric {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}
.metric-label { color: var(--app-muted); }
.metric-value { color: var(--app-text); font-weight: 600; }

/* 摘要 */
.result-summary {
  margin: 0;
  font-size: 12px;
  line-height: 1.6;
  color: #5d6677;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
}

/* 展开按钮 */
.expand-btn {
  align-self: flex-start;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 6px;
  border: none;
  background: transparent;
  color: var(--app-muted);
  font-size: 12px;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.15s;
}
.expand-btn:hover { background: var(--app-bg); color: var(--app-primary); }

.detail-panel {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 10px;
  background: var(--app-bg);
  border-radius: 8px;
}
.detail-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.detail-title {
  font-size: 11px;
  font-weight: 600;
  color: #8892a4;
  text-transform: uppercase;
  letter-spacing: 0.3px;
}
.detail-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12px;
}
.detail-row span { color: var(--app-muted); }
.detail-row strong { color: var(--app-text); }
.detail-code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  color: var(--app-text);
  word-break: break-all;
}
.detail-text {
  font-size: 12px;
  color: #5d6677;
  line-height: 1.6;
}

/* 失败错误框 */
.error-box {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px;
  background: #fff0f2;
  border-radius: 8px;
  border: 1px solid #fecaca;
}
.error-box strong { font-size: 13px; color: #b4233c; }
.error-box span { font-size: 12px; color: #b4233c; line-height: 1.5; }
</style>
