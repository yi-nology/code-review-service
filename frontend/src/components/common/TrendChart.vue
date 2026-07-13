<script setup lang="ts">
import { computed, ref } from 'vue'
import type { TrendPoint } from '@/types/stats'

const props = defineProps<{
  data: TrendPoint[]
  height?: number
}>()

const chartWidth = 600
const chartHeight = computed(() => props.height ?? 240)
const padLeft = 40
const padRight = 20
const padTop = 30
const padBottom = 30

const uid = Math.random().toString(36).slice(2, 8)
const hoverIndex = ref<number | null>(null)

const hasData = computed(() => props.data.length >= 1)

// 最大问题数（用于第二轴的柱状高度归一化）。
const maxFindings = computed(() => {
  const m = Math.max(...props.data.map((d) => d.findings ?? 0), 1)
  return m
})

const linePath = computed(() => {
  if (props.data.length < 2) return ''
  const pts = props.data.map((pt, i) => `${xToPx(i)},${yToPx(pt.avg_score)}`)
  return `M ${pts.join(' L ')}`
})

const areaPath = computed(() => {
  if (props.data.length < 2) return ''
  const pts = props.data.map((pt, i) => `${xToPx(i)},${yToPx(pt.avg_score)}`)
  const last = props.data.length - 1
  return `M ${xToPx(0)},${chartHeight.value - padBottom} L ${pts.join(' L ')} L ${xToPx(last)},${chartHeight.value - padBottom} Z`
})

function xToPx(i: number): number {
  if (props.data.length <= 1) return (padLeft + chartWidth - padRight) / 2
  const span = chartWidth - padLeft - padRight
  return padLeft + (span * i) / (props.data.length - 1)
}

function yToPx(score: number): number {
  const span = chartHeight.value - padTop - padBottom
  return padTop + span * (1 - score / 100)
}

// 问题数柱状高度（归一化到图表高度的 40%）。
function findingsBarHeight(findings: number): number {
  const span = chartHeight.value - padTop - padBottom
  return (findings / maxFindings.value) * span * 0.4
}

function formatPeriod(period: string): string {
  if (!period) return ''
  const d = new Date(period)
  return d.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}
</script>

<template>
  <div v-if="hasData" class="trend-chart">
    <svg :viewBox="`0 0 ${chartWidth} ${chartHeight}`" preserveAspectRatio="none" class="trend-svg">
      <defs>
        <linearGradient :id="`grad-${uid}`" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stop-color="#5b5bd6" stop-opacity="0.2" />
          <stop offset="100%" stop-color="#5b5bd6" stop-opacity="0" />
        </linearGradient>
      </defs>
      <!-- 网格线 -->
      <line v-for="y in [25, 50, 75]" :key="y" :x1="padLeft" :x2="chartWidth - padRight" :y1="yToPx(y)" :y2="yToPx(y)" stroke="#edf0f4" stroke-width="1" />
      <!-- 问题数柱状（次要维度，半透明） -->
      <rect
        v-for="(pt, i) in data"
        :key="'bar-' + i"
        :x="xToPx(i) - 8"
        :y="chartHeight - padBottom - findingsBarHeight(pt.findings ?? 0)"
        width="16"
        :height="findingsBarHeight(pt.findings ?? 0)"
        fill="#e0e7ff"
        rx="2"
      />
      <!-- 评分趋势线 -->
      <path :d="areaPath" :fill="`url(#grad-${uid})`" />
      <path :d="linePath" fill="none" stroke="#5b5bd6" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
      <!-- 数据点 + hover -->
      <g v-for="(pt, i) in data" :key="i" @mouseenter="hoverIndex = i" @mouseleave="hoverIndex = null">
        <rect :x="xToPx(i) - 15" :y="padTop" width="30" :height="chartHeight - padTop - padBottom" fill="transparent" />
        <circle :cx="xToPx(i)" :cy="yToPx(pt.avg_score)" :r="hoverIndex === i ? 6 : 4" fill="#5b5bd6" />
        <text v-if="hoverIndex !== i" :x="xToPx(i)" :y="yToPx(pt.avg_score) - 10" text-anchor="middle" fill="#7d8698" font-size="11">{{ pt.avg_score }}</text>
      </g>
    </svg>
    <!-- hover tooltip -->
    <div v-if="hoverIndex !== null && data[hoverIndex]" class="trend-tooltip" :style="{ left: `${(xToPx(hoverIndex) / chartWidth) * 100}%` }">
      <div class="tooltip-period">{{ formatPeriod(data[hoverIndex].period) }}</div>
      <div class="tooltip-row"><span>评分</span><strong>{{ data[hoverIndex].avg_score }}</strong></div>
      <div class="tooltip-row"><span>问题数</span><strong>{{ data[hoverIndex].findings ?? 0 }}</strong></div>
      <div class="tooltip-row" v-if="data[hoverIndex].review_count"><span>评审数</span><strong>{{ data[hoverIndex].review_count }}</strong></div>
    </div>
    <div class="trend-labels">
      <span v-for="(pt, i) in data" :key="i" class="trend-label">{{ formatPeriod(pt.period) }}</span>
    </div>
    <div class="trend-legend">
      <span class="legend-item"><span class="legend-line" /> 评分趋势</span>
      <span class="legend-item"><span class="legend-bar" /> 问题数量</span>
    </div>
  </div>
  <div v-else class="empty-chart">
    <p>暂无趋势数据</p>
  </div>
</template>

<style scoped>
.trend-chart {
  width: 100%;
  position: relative;
}
.trend-svg {
  width: 100%;
  height: auto;
}
.trend-labels {
  display: flex;
  justify-content: space-between;
  padding: 0 20px;
  margin-top: 4px;
}
.trend-label {
  font-size: 11px;
  color: #7d8698;
}
.trend-legend {
  display: flex;
  gap: 16px;
  padding: 8px 20px 0;
  font-size: 11px;
  color: #6b7280;
}
.legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
}
.legend-line {
  display: inline-block;
  width: 16px;
  height: 2px;
  background: #5b5bd6;
  border-radius: 1px;
}
.legend-bar {
  display: inline-block;
  width: 10px;
  height: 10px;
  background: #e0e7ff;
  border-radius: 2px;
}
.trend-tooltip {
  position: absolute;
  top: 8px;
  transform: translateX(-50%);
  background: rgba(31, 41, 55, 0.92);
  color: #fff;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
  pointer-events: none;
  white-space: nowrap;
  z-index: 5;
}
.tooltip-period {
  font-weight: 600;
  margin-bottom: 4px;
  color: #d1d5db;
}
.tooltip-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}
.tooltip-row span {
  color: #9ca3af;
}
.empty-chart {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  color: #7d8698;
}
</style>
