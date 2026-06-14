<template>
  <div class="requirements-coverage">
    <h3>需求覆盖</h3>
    <div v-if="requirements.length === 0" class="empty">
      暂无需求
    </div>
    <div v-else>
      <div class="coverage-summary">
        <div class="score">
          <el-progress
            type="circle"
            :percentage="score"
            :width="80"
            :stroke-width="8"
            :color="getScoreColor(score)"
          />
          <span class="score-label">覆盖率</span>
        </div>
        <div class="stats">
          <div class="stat">
            <span class="stat-value">{{ metCount }}</span>
            <span class="stat-label">已满足</span>
          </div>
          <div class="stat">
            <span class="stat-value">{{ partialCount }}</span>
            <span class="stat-label">部分满足</span>
          </div>
          <div class="stat">
            <span class="stat-value">{{ missingCount }}</span>
            <span class="stat-label">未满足</span>
          </div>
        </div>
      </div>
      <div class="requirements-list">
        <div
          v-for="req in requirements"
          :key="req.requirement"
          :class="['requirement', req.status]"
        >
          <div class="requirement-header">
            <span :class="['status-badge', req.status]">
              {{ getStatusLabel(req.status) }}
            </span>
            <span class="confidence">
              {{ (req.confidence * 100).toFixed(0) }}%
            </span>
          </div>
          <p class="requirement-text">{{ req.requirement }}</p>
          <div v-if="req.evidence" class="evidence">
            <strong>证据:</strong> {{ req.evidence }}
          </div>
          <div v-if="req.findings && req.findings.length > 0" class="related-findings">
            <strong>相关发现:</strong>
            <ul>
              <li v-for="finding in req.findings" :key="finding">{{ finding }}</li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface RequirementCoverage {
  requirement: string
  status: 'met' | 'partial' | 'missing' | 'unclear'
  confidence: number
  evidence: string
  findings: string[]
}

const props = defineProps<{
  requirements: RequirementCoverage[]
  score: number
}>()

const metCount = computed(() =>
  props.requirements.filter(r => r.status === 'met').length
)

const partialCount = computed(() =>
  props.requirements.filter(r => r.status === 'partial').length
)

const missingCount = computed(() =>
  props.requirements.filter(r => r.status === 'missing').length
)

function getStatusLabel(status: string): string {
  switch (status) {
    case 'met':
      return '已满足'
    case 'partial':
      return '部分满足'
    case 'missing':
      return '未满足'
    case 'unclear':
      return '不明确'
    default:
      return status
  }
}

function getScoreColor(score: number): string {
  if (score >= 80) return '#67c23a'
  if (score >= 60) return '#e6a23c'
  return '#f56c6c'
}
</script>

<style scoped>
.requirements-coverage {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.requirements-coverage h3 {
  margin: 0 0 20px;
  font-size: 18px;
  color: #303133;
}

.empty {
  padding: 40px;
  text-align: center;
  color: #909399;
  font-size: 14px;
}

.coverage-summary {
  display: flex;
  align-items: center;
  gap: 32px;
  margin-bottom: 24px;
  padding: 20px;
  background: #f5f7fa;
  border-radius: 8px;
}

.score {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
}

.score-label {
  font-size: 14px;
  color: #606266;
}

.stats {
  display: flex;
  gap: 24px;
}

.stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.stat-value {
  font-size: 24px;
  font-weight: 600;
  color: #303133;
}

.stat-label {
  font-size: 12px;
  color: #909399;
}

.requirements-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.requirement {
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  border-left: 4px solid #dcdfe6;
}

.requirement.met {
  border-left-color: #67c23a;
  background: #f0f9eb;
}

.requirement.partial {
  border-left-color: #e6a23c;
  background: #fdf6ec;
}

.requirement.missing {
  border-left-color: #f56c6c;
  background: #fef0f0;
}

.requirement.unclear {
  border-left-color: #909399;
  background: #f5f7fa;
}

.requirement-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.status-badge {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.status-badge.met {
  background: #e1f3d8;
  color: #67c23a;
}

.status-badge.partial {
  background: #faecd8;
  color: #e6a23c;
}

.status-badge.missing {
  background: #fde2e2;
  color: #f56c6c;
}

.status-badge.unclear {
  background: #e4e7ed;
  color: #909399;
}

.confidence {
  font-size: 12px;
  color: #909399;
  font-weight: 500;
}

.requirement-text {
  margin: 0 0 8px;
  font-size: 14px;
  color: #303133;
  line-height: 1.6;
}

.evidence {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 4px;
  font-size: 13px;
  color: #606266;
  margin-bottom: 8px;
}

.evidence strong {
  color: #303133;
}

.related-findings {
  font-size: 13px;
  color: #606266;
}

.related-findings strong {
  color: #303133;
}

.related-findings ul {
  margin: 4px 0 0;
  padding-left: 20px;
}

.related-findings li {
  margin-bottom: 4px;
}
</style>
