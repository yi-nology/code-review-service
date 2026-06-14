<template>
  <div class="review-comparison">
    <h3>审查对比</h3>
    <div v-if="!reviews || reviews.length < 2" class="empty">
      <p>请选择至少两个审查进行对比</p>
    </div>
    <div v-else>
      <div class="comparison-header">
        <div class="review-selector">
          <el-select v-model="selectedReview1" placeholder="选择审查 1" style="width: 200px">
            <el-option
              v-for="review in reviews"
              :key="review.review_id"
              :label="`${review.repo} (${formatDate(review.created_at)})`"
              :value="review.review_id"
            />
          </el-select>
          <span class="vs">VS</span>
          <el-select v-model="selectedReview2" placeholder="选择审查 2" style="width: 200px">
            <el-option
              v-for="review in reviews"
              :key="review.review_id"
              :label="`${review.repo} (${formatDate(review.created_at)})`"
              :value="review.review_id"
            />
          </el-select>
        </div>
      </div>

      <div v-if="review1 && review2" class="comparison-content">
        <el-row :gutter="20">
          <el-col :span="12">
            <el-card class="review-card">
              <template #header>
                <div class="card-header">
                  <span>{{ review1.repo }}</span>
                  <el-tag :type="getStatusType(review1.status)">{{ review1.status }}</el-tag>
                </div>
              </template>
              <div class="review-info">
                <div class="score">
                  <span class="score-value">{{ review1.score }}</span>
                  <span class="score-label">/ 100</span>
                  <span class="score-grade">{{ review1.grade }}</span>
                </div>
                <div class="stats">
                  <div class="stat critical">
                    <span class="stat-value">{{ review1.criticalCount }}</span>
                    <span class="stat-label">严重</span>
                  </div>
                  <div class="stat major">
                    <span class="stat-value">{{ review1.majorCount }}</span>
                    <span class="stat-label">高风险</span>
                  </div>
                  <div class="stat minor">
                    <span class="stat-value">{{ review1.minorCount }}</span>
                    <span class="stat-label">中风险</span>
                  </div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card class="review-card">
              <template #header>
                <div class="card-header">
                  <span>{{ review2.repo }}</span>
                  <el-tag :type="getStatusType(review2.status)">{{ review2.status }}</el-tag>
                </div>
              </template>
              <div class="review-info">
                <div class="score">
                  <span class="score-value">{{ review2.score }}</span>
                  <span class="score-label">/ 100</span>
                  <span class="score-grade">{{ review2.grade }}</span>
                </div>
                <div class="stats">
                  <div class="stat critical">
                    <span class="stat-value">{{ review2.criticalCount }}</span>
                    <span class="stat-label">严重</span>
                  </div>
                  <div class="stat major">
                    <span class="stat-value">{{ review2.majorCount }}</span>
                    <span class="stat-label">高风险</span>
                  </div>
                  <div class="stat minor">
                    <span class="stat-value">{{ review2.minorCount }}</span>
                    <span class="stat-label">中风险</span>
                  </div>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <el-card class="diff-card mt-20">
          <template #header>
            <div class="card-header">
              <span>差异分析</span>
            </div>
          </template>
          <div class="diff-summary">
            <div class="diff-item">
              <span class="diff-label">评分差异</span>
              <span :class="['diff-value', scoreDiff > 0 ? 'positive' : scoreDiff < 0 ? 'negative' : '']">
                {{ scoreDiff > 0 ? '+' : '' }}{{ scoreDiff }}
              </span>
            </div>
            <div class="diff-item">
              <span class="diff-label">发现问题差异</span>
              <span :class="['diff-value', findingsDiff > 0 ? 'negative' : findingsDiff < 0 ? 'positive' : '']">
                {{ findingsDiff > 0 ? '+' : '' }}{{ findingsDiff }}
              </span>
            </div>
            <div class="diff-item">
              <span class="diff-label">严重问题差异</span>
              <span :class="['diff-value', criticalDiff > 0 ? 'negative' : criticalDiff < 0 ? 'positive' : '']">
                {{ criticalDiff > 0 ? '+' : '' }}{{ criticalDiff }}
              </span>
            </div>
          </div>
        </el-card>

        <el-card class="findings-comparison mt-20">
          <template #header>
            <div class="card-header">
              <span>问题对比</span>
            </div>
          </template>
          <el-tabs v-model="activeTab">
            <el-tab-pane label="新增问题" name="new">
              <div v-if="newFindings.length === 0" class="empty-tab">
                无新增问题
              </div>
              <div v-else class="findings-list">
                <div v-for="finding in newFindings" :key="finding.title" class="finding-item new">
                  <el-tag :type="getSeverityType(finding.severity)" size="small">
                    {{ finding.severity.toUpperCase() }}
                  </el-tag>
                  <span class="finding-title">{{ finding.title }}</span>
                  <span class="finding-path">{{ finding.path }}:{{ finding.line }}</span>
                </div>
              </div>
            </el-tab-pane>
            <el-tab-pane label="已解决问题" name="resolved">
              <div v-if="resolvedFindings.length === 0" class="empty-tab">
                无已解决问题
              </div>
              <div v-else class="findings-list">
                <div v-for="finding in resolvedFindings" :key="finding.title" class="finding-item resolved">
                  <el-tag :type="getSeverityType(finding.severity)" size="small">
                    {{ finding.severity.toUpperCase() }}
                  </el-tag>
                  <span class="finding-title">{{ finding.title }}</span>
                  <span class="finding-path">{{ finding.path }}:{{ finding.line }}</span>
                </div>
              </div>
            </el-tab-pane>
            <el-tab-pane label="共同问题" name="common">
              <div v-if="commonFindings.length === 0" class="empty-tab">
                无共同问题
              </div>
              <div v-else class="findings-list">
                <div v-for="finding in commonFindings" :key="finding.title" class="finding-item common">
                  <el-tag :type="getSeverityType(finding.severity)" size="small">
                    {{ finding.severity.toUpperCase() }}
                  </el-tag>
                  <span class="finding-title">{{ finding.title }}</span>
                  <span class="finding-path">{{ finding.path }}:{{ finding.line }}</span>
                </div>
              </div>
            </el-tab-pane>
          </el-tabs>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

interface Review {
  review_id: string
  repo: string
  status: string
  score: number
  grade: string
  findings: Finding[]
  criticalCount: number
  majorCount: number
  minorCount: number
  created_at: string
}

interface Finding {
  severity: string
  title: string
  path: string
  line: number
}

const props = defineProps<{
  reviews: Review[]
}>()

const selectedReview1 = ref<string>('')
const selectedReview2 = ref<string>('')
const activeTab = ref('new')

const review1 = computed(() => 
  props.reviews.find(r => r.review_id === selectedReview1.value)
)

const review2 = computed(() => 
  props.reviews.find(r => r.review_id === selectedReview2.value)
)

const scoreDiff = computed(() => {
  if (!review1.value || !review2.value) return 0
  return review2.value.score - review1.value.score
})

const findingsDiff = computed(() => {
  if (!review1.value || !review2.value) return 0
  return review2.value.findings.length - review1.value.findings.length
})

const criticalDiff = computed(() => {
  if (!review1.value || !review2.value) return 0
  return review2.value.criticalCount - review1.value.criticalCount
})

const newFindings = computed(() => {
  if (!review1.value || !review2.value) return []
  const titles1 = new Set(review1.value.findings.map(f => f.title))
  return review2.value.findings.filter(f => !titles1.has(f.title))
})

const resolvedFindings = computed(() => {
  if (!review1.value || !review2.value) return []
  const titles2 = new Set(review2.value.findings.map(f => f.title))
  return review1.value.findings.filter(f => !titles2.has(f.title))
})

const commonFindings = computed(() => {
  if (!review1.value || !review2.value) return []
  const titles2 = new Set(review2.value.findings.map(f => f.title))
  return review1.value.findings.filter(f => titles2.has(f.title))
})

function getStatusType(status: string): string {
  switch (status) {
    case 'success': return 'success'
    case 'failed': return 'danger'
    case 'running': return 'warning'
    default: return 'info'
  }
}

function getSeverityType(severity: string): string {
  switch (severity) {
    case 'critical': return 'danger'
    case 'major': return 'warning'
    case 'minor': return ''
    case 'info': return 'info'
    default: return 'info'
  }
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString('zh-CN')
}
</script>

<style scoped>
.review-comparison {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.review-comparison h3 {
  margin: 0 0 20px;
  font-size: 18px;
  color: #303133;
}

.empty {
  padding: 40px;
  text-align: center;
  color: #909399;
}

.comparison-header {
  margin-bottom: 20px;
}

.review-selector {
  display: flex;
  align-items: center;
  gap: 16px;
}

.vs {
  font-size: 18px;
  font-weight: bold;
  color: #909399;
}

.review-card {
  height: 100%;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.review-info {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.score {
  text-align: center;
}

.score-value {
  font-size: 36px;
  font-weight: bold;
  color: #409eff;
}

.score-label {
  font-size: 14px;
  color: #909399;
}

.score-grade {
  display: block;
  font-size: 18px;
  color: #303133;
  margin-top: 4px;
}

.stats {
  display: flex;
  justify-content: space-around;
}

.stat {
  text-align: center;
}

.stat-value {
  display: block;
  font-size: 24px;
  font-weight: bold;
}

.stat-label {
  font-size: 12px;
  color: #909399;
}

.stat.critical .stat-value { color: #f56c6c; }
.stat.major .stat-value { color: #e6a23c; }
.stat.minor .stat-value { color: #409eff; }

.diff-card {
  margin-top: 20px;
}

.diff-summary {
  display: flex;
  justify-content: space-around;
}

.diff-item {
  text-align: center;
}

.diff-label {
  display: block;
  font-size: 14px;
  color: #606266;
  margin-bottom: 8px;
}

.diff-value {
  font-size: 24px;
  font-weight: bold;
}

.diff-value.positive {
  color: #67c23a;
}

.diff-value.negative {
  color: #f56c6c;
}

.findings-comparison {
  margin-top: 20px;
}

.empty-tab {
  padding: 20px;
  text-align: center;
  color: #909399;
}

.findings-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.finding-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 4px;
  border-left: 4px solid #dcdfe6;
}

.finding-item.new {
  border-left-color: #f56c6c;
  background: #fef0f0;
}

.finding-item.resolved {
  border-left-color: #67c23a;
  background: #f0f9eb;
}

.finding-item.common {
  border-left-color: #e6a23c;
  background: #fdf6ec;
}

.finding-title {
  flex: 1;
  font-weight: 500;
}

.finding-path {
  font-size: 12px;
  color: #909399;
  font-family: monospace;
}

.mt-20 {
  margin-top: 20px;
}
</style>
