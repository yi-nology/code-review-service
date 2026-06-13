<script setup lang="ts">
import { computed } from 'vue'
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

const hasData = computed(() => props.data.length > 1)

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
  if (props.data.length <= 1) return padLeft
  const span = chartWidth - padLeft - padRight
  return padLeft + (span * i) / (props.data.length - 1)
}

function yToPx(score: number): number {
  const span = chartHeight.value - padTop - padBottom
  return padTop + span * (1 - score / 100)
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
      <line v-for="y in [25, 50, 75]" :key="y" :x1="padLeft" :x2="chartWidth - padRight" :y1="yToPx(y)" :y2="yToPx(y)" stroke="#edf0f4" stroke-width="1" />
      <path :d="areaPath" :fill="`url(#grad-${uid})`" />
      <path :d="linePath" fill="none" stroke="#5b5bd6" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
      <g v-for="(pt, i) in data" :key="i">
        <circle :cx="xToPx(i)" :cy="yToPx(pt.avg_score)" r="4" fill="#5b5bd6" />
        <text :x="xToPx(i)" :y="yToPx(pt.avg_score) - 10" text-anchor="middle" fill="#7d8698" font-size="11">{{ pt.avg_score }}</text>
      </g>
    </svg>
    <div class="trend-labels">
      <span v-for="(pt, i) in data" :key="i" class="trend-label">{{ formatPeriod(pt.period) }}</span>
    </div>
  </div>
  <div v-else class="empty-chart">
    <p>数据不足，至少需要 2 个数据点才能绘制趋势图</p>
  </div>
</template>

<style scoped>
.trend-chart {
  width: 100%;
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
.empty-chart {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
  color: #7d8698;
}
</style>
