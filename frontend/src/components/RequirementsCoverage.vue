<template>
  <div class="requirements-coverage">
    <div
      v-if="requirements.length === 0"
      class="empty"
    >
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
          <p class="requirement-text">
            {{ req.requirement }}
          </p>
          <div
            v-if="req.evidence"
            class="evidence"
          >
            <strong>证据:</strong> {{ req.evidence }}
          </div>
          <div
            v-if="req.findings && req.findings.length > 0"
            class="related-findings"
          >
            <strong>相关发现:</strong>
            <ul>
              <li
                v-for="finding in req.findings"
                :key="finding"
              >
                {{ finding }}
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { getScoreColor } from '@/utils/colors'
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

</script>

<style scoped>
.requirements-coverage {
  width: 100%;
}

.empty {
  padding: 28px 12px;
  text-align: center;
  color: #909399;
  font-size: 13px;
}

.coverage-summary {
  display: flex;
  align-items: center;
  gap: 28px;
  margin-bottom: 20px;
  padding: 18px;
  background: #f7f8fc;
  border: 1px solid var(--app-border, #e4e7ed);
  border-radius: 10px;
  flex-wrap: wrap;
}

.score {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
}

.score-label {
  font-size: 13px;
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
  font-weight: 700;
  color: var(--app-text, #303133);
}

.stat-label {
  font-size: 12px;
  color: #9aa2b1;
}

.requirements-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.requirement {
  padding: 14px;
  background: #f7f8fc;
  border: 1px solid var(--app-border, #e4e7ed);
  border-left: 4px solid #dcdfe6;
  border-radius: 10px;
}

.requirement.met {
  border-left-color: var(--app-success, #67c23a);
  background: #f0f9eb;
}

.requirement.partial {
  border-left-color: #e6a23c;
  background: #fdf6ec;
}

.requirement.missing {
  border-left-color: var(--app-danger, #f56c6c);
  background: #fef0f0;
}

.requirement.unclear {
  border-left-color: #909399;
  background: #f7f8fc;
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
  font-size: 11px;
  font-weight: 600;
}

.status-badge.met {
  background: #e1f3d8;
  color: #529b2e;
}

.status-badge.partial {
  background: #faecd8;
  color: #b88230;
}

.status-badge.missing {
  background: #fde2e2;
  color: #c45656;
}

.status-badge.unclear {
  background: #e4e7ed;
  color: #909399;
}

.confidence {
  font-size: 12px;
  color: #9aa2b1;
  font-weight: 500;
}

.requirement-text {
  margin: 0 0 8px;
  font-size: 13px;
  color: var(--app-text, #303133);
  line-height: 1.6;
}

.evidence {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 6px;
  font-size: 12px;
  color: #606266;
  margin-bottom: 8px;
  line-height: 1.5;
}

.evidence strong {
  color: var(--app-text, #303133);
}

.related-findings {
  font-size: 12px;
  color: #606266;
}

.related-findings strong {
  color: var(--app-text, #303133);
}

.related-findings ul {
  margin: 4px 0 0;
  padding-left: 20px;
}

.related-findings li {
  margin-bottom: 4px;
}
</style>
