<template>
  <el-card class="gate-card">
    <template #header>质量门禁</template>
    <div class="gate-status" :class="gateStatus">
      <div class="gate-icon">
        <el-icon v-if="gatePassed" :size="48"><SuccessFilled /></el-icon>
        <el-icon v-else-if="isFailed" :size="48"><CircleCloseFilled /></el-icon>
        <el-icon v-else :size="48"><InfoFilled /></el-icon>
      </div>
      <div class="gate-text">
        {{ gatePassed ? '门禁通过' : isFailed ? '门禁未通过' : '待定' }}
      </div>
      <div class="gate-reason" v-if="reason">{{ reason }}</div>
      <div v-if="explanations.length" class="gate-explain">
        <div v-for="item in explanations" :key="item.title" class="explain-item">
          <strong>{{ item.title }}</strong>
          <span>{{ item.body }}</span>
        </div>
      </div>
      <div v-if="blockingFindings.length" class="blocking-list">
        <strong>优先修复</strong>
        <div v-for="finding in blockingFindings" :key="`${finding.path}:${finding.line}:${finding.title}`" class="blocking-item">
          <el-tag size="small" :type="finding.severity === 'critical' ? 'danger' : 'warning'">{{ severityLabel(finding.severity) }}</el-tag>
          <span>{{ finding.path || '未知文件' }}<template v-if="finding.line">:{{ finding.line }}</template></span>
          <small>{{ finding.title }}</small>
        </div>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { SuccessFilled, CircleCloseFilled, InfoFilled } from '@element-plus/icons-vue'
import type { Finding, Report } from '@/types/review'

const props = defineProps<{
  gateStatus?: string
  gatePassed?: boolean
  reviewStatus: string
  reason?: string
  report?: Report
  findings?: Finding[]
}>()

const isFailed = computed(() =>
  props.reviewStatus === 'failed' || Boolean(props.gateStatus && ['failed', 'error'].includes(props.gateStatus)),
)

const blockingFindings = computed(() =>
  (props.findings || [])
    .filter((finding) => ['critical', 'major'].includes(finding.severity))
    .slice(0, 5),
)

const explanations = computed(() => {
  const items: Array<{ title: string; body: string }> = []
  const counts = props.report?.counts
  const score = props.report?.score
  if (props.reviewStatus === 'failed') {
    items.push({ title: '执行失败', body: '审查流程没有完整完成，请先查看失败提示、OCR 配置和任务日志。' })
  }
  if (props.gateStatus === 'failed') {
    items.push({ title: '门禁策略触发', body: props.reason || '当前审查结果未满足项目或全局质量门禁。' })
  }
  if (counts && (counts.critical > 0 || counts.major > 0)) {
    items.push({ title: '高严重度问题', body: `严重 ${counts.critical} 个，高风险 ${counts.major} 个。优先修复这些问题通常能解除门禁阻塞。` })
  }
  if (score?.deductions?.length) {
    const top = score.deductions.slice(0, 2).map((d) => `${d.reason} ${d.points > 0 ? `-${d.points}` : `+${Math.abs(d.points)}`}分`).join('；')
    items.push({ title: '评分扣减', body: top })
  }
  if (!items.length && props.gatePassed) {
    items.push({ title: '通过原因', body: '未发现超过当前门禁阈值的风险，评分和严重度满足策略要求。' })
  }
  return items
})

function severityLabel(severity: string) {
  const map: Record<string, string> = {
    critical: '严重',
    major: '高',
    minor: '中',
    info: '提示',
  }
  return map[severity] || '未知'
}
</script>

<style scoped>
.gate-status {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
  text-align: center;
}

.gate-status.passed .gate-icon { color: var(--app-success); }
.gate-status.failed .gate-icon { color: var(--app-danger); }
.gate-status.pending .gate-icon { color: #8e96a5; }

.gate-text {
  font-size: 16px;
  font-weight: 600;
}

.gate-reason {
  color: #8892a4;
  font-size: 13px;
}

.gate-explain,
.blocking-list {
  display: grid;
  width: 100%;
  gap: 10px;
  margin-top: 8px;
  text-align: left;
}

.explain-item {
  display: grid;
  gap: 4px;
  padding: 10px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fafbfc;
}

.explain-item strong,
.blocking-list > strong {
  color: var(--app-text);
  font-size: 13px;
}

.explain-item span {
  color: #667085;
  font-size: 12px;
  line-height: 1.6;
}

.blocking-item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 6px 8px;
  padding: 9px 0;
  border-top: 1px solid var(--app-border);
}

.blocking-item span,
.blocking-item small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.blocking-item span {
  color: #374151;
  font-size: 12px;
  font-weight: 600;
}

.blocking-item small {
  grid-column: 2;
  color: #7a8495;
}
</style>
