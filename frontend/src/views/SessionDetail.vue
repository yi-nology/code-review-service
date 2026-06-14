<template>
  <AppLayout>
    <div class="session-detail-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">SESSION DETAIL</div>
          <h1>会话详情</h1>
          <p>查看审查会话的详细信息和进度。</p>
        </div>
        <el-button @click="goBack">
          <el-icon><ArrowLeft /></el-icon>
          返回
        </el-button>
      </header>

      <div v-if="loading" class="loading">
        <el-skeleton :rows="10" animated />
      </div>

      <div v-else-if="!session" class="empty">
        <el-empty description="未找到会话" />
      </div>

      <div v-else class="session-content">
        <el-row :gutter="20">
          <el-col :span="16">
            <el-card class="main-card">
              <template #header>
                <div class="card-header">
                  <h3>会话信息</h3>
                  <el-tag :type="getStatusType(session.status)">
                    {{ getStatusLabel(session.status) }}
                  </el-tag>
                </div>
              </template>

              <el-descriptions :column="2" border>
                <el-descriptions-item label="会话ID">
                  {{ session.session_id }}
                </el-descriptions-item>
                <el-descriptions-item label="审查ID">
                  <router-link :to="`/reviews/${session.review_id}`">
                    {{ session.review_id }}
                  </router-link>
                </el-descriptions-item>
                <el-descriptions-item label="轮次">
                  {{ session.round }}
                </el-descriptions-item>
                <el-descriptions-item label="阶段">
                  {{ getPhaseLabel(session.phase) }}
                </el-descriptions-item>
                <el-descriptions-item label="进度">
                  <el-progress :percentage="session.progress" :stroke-width="10" />
                </el-descriptions-item>
                <el-descriptions-item label="创建时间">
                  {{ formatTime(session.created_at) }}
                </el-descriptions-item>
                <el-descriptions-item label="完成时间" v-if="session.completed_at">
                  {{ formatTime(session.completed_at) }}
                </el-descriptions-item>
              </el-descriptions>
            </el-card>

            <el-card class="mt-20" v-if="session.agent_results && session.agent_results.length > 0">
              <template #header>
                <h3>代理结果</h3>
              </template>
              <div v-for="result in session.agent_results" :key="result.agent_name" class="agent-result">
                <div class="agent-header">
                  <h4>{{ result.agent_name }}</h4>
                  <span class="confidence">
                    置信度: {{ (result.confidence * 100).toFixed(0) }}%
                  </span>
                </div>
                <p>{{ result.summary }}</p>
                <div class="findings-count">
                  发现 {{ result.findings.length }} 个问题
                </div>
              </div>
            </el-card>

            <el-card class="mt-20" v-if="session.discussion">
              <template #header>
                <h3>讨论</h3>
              </template>
              <DiscussionViewer :entries="session.discussion.entries || []" />
            </el-card>

            <el-card class="mt-20" v-if="session.final_report">
              <template #header>
                <h3>最终报告</h3>
              </template>
              <div class="final-report">
                <div class="verdict">
                  <el-tag :type="session.final_report.gate_passed ? 'success' : 'danger'" size="large">
                    {{ session.final_report.verdict }}
                  </el-tag>
                  <span class="score">
                    评分: {{ session.final_report.score }}
                  </span>
                </div>
                <p class="summary">{{ session.final_report.summary }}</p>

                <div v-if="session.final_report.blockers && session.final_report.blockers.length > 0" class="findings-section">
                  <h4>阻塞问题</h4>
                  <ul>
                    <li v-for="finding in session.final_report.blockers" :key="finding.id">
                      {{ finding.title }}
                    </li>
                  </ul>
                </div>

                <div v-if="session.final_report.should_fix && session.final_report.should_fix.length > 0" class="findings-section">
                  <h4>需要修复</h4>
                  <ul>
                    <li v-for="finding in session.final_report.should_fix" :key="finding.id">
                      {{ finding.title }}
                    </li>
                  </ul>
                </div>
              </div>
            </el-card>

            <!-- Review Map -->
            <el-card class="mt-20" v-loading="mapLoading">
              <template #header>
                <div class="card-header">
                  <h3>代码审查地图</h3>
                  <el-tag v-if="reviewMap" size="small">{{ reviewMap.file_count }} 个文件</el-tag>
                </div>
              </template>
              <ReviewMap :map="reviewMap" />
            </el-card>
          </el-col>

          <el-col :span="8">
            <el-card class="progress-card">
              <template #header>
                <h3>进度</h3>
              </template>
              <SessionTimeline :session="{
                phase: session.phase,
                progress: session.progress,
                created_at: session.created_at,
                completed_at: session.completed_at || undefined
              }" />
            </el-card>

            <el-card class="mt-20" v-if="session.final_report?.requirements">
              <template #header>
                <h3>需求覆盖</h3>
              </template>
              <RequirementsCoverage
                :requirements="session.final_report.requirements.requirements || []"
                :score="session.final_report.requirements.score || 0"
              />
            </el-card>
          </el-col>
        </el-row>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import DiscussionViewer from '@/components/DiscussionViewer.vue'
import SessionTimeline from '@/components/SessionTimeline.vue'
import RequirementsCoverage from '@/components/RequirementsCoverage.vue'
import ReviewMap from '@/components/ReviewMap.vue'
import { sessionApi, type ReviewSession } from '@/api/session'
import { getSessionMap } from '@/api/review'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const session = ref<ReviewSession | null>(null)
const reviewMap = ref<any>(null)
const mapLoading = ref(false)

function goBack() {
  router.back()
}

function getStatusType(status: string): string {
  switch (status) {
    case 'active': return 'primary'
    case 'completed': return 'success'
    case 'failed': return 'danger'
    case 'cancelled': return 'warning'
    default: return 'info'
  }
}

function getStatusLabel(status: string): string {
  switch (status) {
    case 'active': return '进行中'
    case 'completed': return '已完成'
    case 'failed': return '失败'
    case 'cancelled': return '已取消'
    default: return status
  }
}

function getPhaseLabel(phase: string): string {
  const labels: Record<string, string> = {
    'context': '上下文发现',
    'change_context': '变更分析',
    'analysis': '分析',
    'reviews': '审查',
    'aggregation': '聚合',
    'discourse': '讨论',
    'synthesis': '综合',
    'complete': '完成',
  }
  return labels[phase] || phase
}

function formatTime(dateStr: string): string {
  return new Date(dateStr).toLocaleString('zh-CN')
}

onMounted(async () => {
  const reviewId = route.params.reviewId as string
  const sessionId = route.params.sessionId as string

  try {
    session.value = await sessionApi.getSession(reviewId, sessionId)
  } catch (error) {
    console.error('Failed to load session:', error)
  } finally {
    loading.value = false
  }

  // Fetch review map
  mapLoading.value = true
  try {
    reviewMap.value = await getSessionMap(reviewId, sessionId)
  } catch (error) {
    console.log('No review map found for this session')
  } finally {
    mapLoading.value = false
  }
})
</script>

<style scoped>
.session-detail-page {
  padding: 20px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.page-header h1 {
  margin: 8px 0;
  font-size: 24px;
}

.page-header p {
  margin: 0;
  color: #666;
}

.eyebrow {
  font-size: 12px;
  color: #409eff;
  font-weight: bold;
}

.loading {
  padding: 40px;
}

.empty {
  padding: 40px;
}

.main-card {
  margin-bottom: 20px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header h3 {
  margin: 0;
}

.mt-20 {
  margin-top: 20px;
}

.agent-result {
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 12px;
}

.agent-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.agent-header h4 {
  margin: 0;
}

.confidence {
  font-size: 12px;
  color: #666;
}

.findings-count {
  margin-top: 8px;
  font-size: 12px;
  color: #909399;
}

.final-report {
  padding: 16px;
}

.verdict {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 16px;
}

.score {
  font-size: 18px;
  font-weight: bold;
}

.summary {
  margin-bottom: 16px;
  line-height: 1.6;
}

.findings-section {
  margin-top: 16px;
}

.findings-section h4 {
  margin: 0 0 8px;
  font-size: 14px;
}

.findings-section ul {
  margin: 0;
  padding-left: 20px;
}

.findings-section li {
  margin-bottom: 4px;
}

.progress-card {
  position: sticky;
  top: 20px;
}
</style>
