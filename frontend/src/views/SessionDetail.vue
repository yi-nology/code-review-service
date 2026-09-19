<template>
  <AppLayout>
    <PageShell>
      <div
        v-loading="loading"
        class="session-detail-content"
      >
        <el-button
          class="back-button"
          circle
          plain
          aria-label="返回"
          @click="goBack"
        >
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <PageHeader
          eyebrow="SESSION DETAIL"
          title="会话详情"
          :description="session ? `第 ${session.round} 轮 · ${getPhaseLabel(session.phase)}` : undefined"
        >
          <template #actions>
            <el-tag v-if="session" :type="getSessionStatusType(session.status)" size="small">
              {{ getSessionStatusLabel(session.status) }}
            </el-tag>
            <template v-if="session">
              <el-button
                v-if="session.status === 'active'"
                type="danger"
                plain
                size="small"
                :loading="acting"
                @click="handleCancel"
              >
                取消会话
              </el-button>
              <el-button
                v-if="session.status === 'completed' || session.status === 'failed'"
                type="primary"
                plain
                size="small"
                :loading="acting"
                @click="handleNewRound"
              >
                发起新一轮
              </el-button>
            </template>
          </template>
        </PageHeader>

      <div
        v-if="!loading && !session"
        class="empty"
      >
        <el-empty description="未找到会话" />
      </div>

      <div
        v-else-if="session"
        class="session-content"
      >
        <el-row :gutter="16">
          <el-col
            :xs="24"
            :lg="16"
          >
            <el-card class="main-card">
              <template #header>
                <div class="card-header">
                  <span>会话信息</span>
                  <el-tag :type="getSessionStatusType(session.status)">
                    {{ getSessionStatusLabel(session.status) }}
                  </el-tag>
                </div>
              </template>

              <el-descriptions
                :column="descColumns"
                border
                size="small"
              >
                <el-descriptions-item label="会话ID">
                  <code>{{ session.session_id }}</code>
                </el-descriptions-item>
                <el-descriptions-item label="审查ID">
                  <router-link
                    :to="`/reviews/${session.review_id}`"
                    class="review-link"
                  >
                    {{ session.review_id }}
                  </router-link>
                </el-descriptions-item>
                <el-descriptions-item label="轮次">
                  {{ session.round }}
                </el-descriptions-item>
                <el-descriptions-item label="阶段">
                  {{ getPhaseLabel(session.phase) }}
                </el-descriptions-item>
                <el-descriptions-item
                  label="进度"
                  :span="descColumns"
                >
                  <el-progress
                    :percentage="session.progress"
                    :stroke-width="10"
                  />
                </el-descriptions-item>
                <el-descriptions-item label="创建时间">
                  {{ formatDateTime(session.created_at) }}
                </el-descriptions-item>
                <el-descriptions-item
                  v-if="session.completed_at"
                  label="完成时间"
                >
                  {{ formatDateTime(session.completed_at) }}
                </el-descriptions-item>
              </el-descriptions>
            </el-card>

            <el-card
              v-if="session.agent_results && session.agent_results.length > 0"
              class="section-card"
            >
              <template #header>
                <div class="card-header">
                  <span>代理结果</span>
                  <el-tag size="small">
                    {{ session.agent_results.length }} 个
                  </el-tag>
                </div>
              </template>
              <div
                v-for="result in session.agent_results"
                :key="result.agent_name"
                class="agent-result"
              >
                <div class="agent-header">
                  <h4>{{ result.agent_name }}</h4>
                  <span class="confidence">置信度 {{ (result.confidence * 100).toFixed(0) }}%</span>
                </div>
                <p v-if="result.summary">
                  {{ result.summary }}
                </p>
                <div class="findings-count">
                  发现 {{ result.findings.length }} 个问题
                </div>
              </div>
            </el-card>

            <el-card
              v-if="session.discussion"
              class="section-card"
            >
              <template #header>
                <span>讨论</span>
              </template>
              <DiscussionViewer :entries="session.discussion.entries || []" />
            </el-card>

            <el-card
              v-if="session.final_report"
              class="section-card"
            >
              <template #header>
                <span>最终报告</span>
              </template>
              <div class="final-report">
                <div class="verdict">
                  <el-tag
                    :type="session.final_report.gate_passed ? 'success' : 'danger'"
                    size="large"
                  >
                    {{ session.final_report.verdict }}
                  </el-tag>
                  <span class="score">评分 {{ session.final_report.score }}</span>
                </div>
                <p class="summary">
                  {{ session.final_report.summary }}
                </p>

                <div
                  v-if="session.final_report.blockers?.length"
                  class="findings-section"
                >
                  <h4>阻塞问题</h4>
                  <ul>
                    <li
                      v-for="finding in session.final_report.blockers"
                      :key="finding.id"
                    >
                      {{ finding.title }}
                    </li>
                  </ul>
                </div>

                <div
                  v-if="session.final_report.should_fix?.length"
                  class="findings-section"
                >
                  <h4>需要修复</h4>
                  <ul>
                    <li
                      v-for="finding in session.final_report.should_fix"
                      :key="finding.id"
                    >
                      {{ finding.title }}
                    </li>
                  </ul>
                </div>
              </div>
            </el-card>
          </el-col>

          <el-col
            :xs="24"
            :lg="8"
          >
            <el-card class="progress-card">
              <template #header>
                <span>进度</span>
              </template>
              <SessionTimeline
                :session="{
                  phase: session.phase,
                  progress: session.progress,
                  status: session.status,
                  created_at: session.created_at,
                  completed_at: session.completed_at || undefined
                }"
              />
            </el-card>

            <el-card
              v-if="session.final_report?.requirements"
              class="section-card"
            >
              <template #header>
                <span>需求覆盖</span>
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
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import DiscussionViewer from '@/components/DiscussionViewer.vue'
import SessionTimeline from '@/components/SessionTimeline.vue'
import RequirementsCoverage from '@/components/RequirementsCoverage.vue'
import { sessionApi, type ReviewSession } from '@/api/session'
import { getSessionStatusType, getSessionStatusLabel, getPhaseLabel } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'

const route = useRoute()
const router = useRouter()
const loading = ref(true)
const acting = ref(false)
const session = ref<ReviewSession | null>(null)
const viewportWidth = ref(window.innerWidth)
let pollTimer: ReturnType<typeof setTimeout> | null = null

const descColumns = computed(() => viewportWidth.value < 720 ? 1 : 2)

function updateViewportWidth() {
  viewportWidth.value = window.innerWidth
}

function goBack() {
  router.back()
}

async function fetchSession() {
  const reviewId = route.params.reviewId as string
  const sessionId = route.params.sessionId as string
  try {
    session.value = await sessionApi.getSession(reviewId, sessionId)
  } catch (error) {
    console.error('Failed to load session:', error)
  }
}

function schedulePolling() {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
  if (!session.value || session.value.status !== 'active') return
  pollTimer = setTimeout(async () => {
    if (document.visibilityState === 'visible') {
      await fetchSession()
      schedulePolling()
    } else {
      schedulePolling()
    }
  }, 5000)
}

async function handleCancel() {
  try {
    await ElMessageBox.confirm('确定取消这个进行中的会话？', '取消会话', {
      type: 'warning',
      confirmButtonText: '取消会话',
      cancelButtonText: '再想想',
    })
  } catch {
    return
  }
  acting.value = true
  try {
    await sessionApi.cancelSession(route.params.reviewId as string, route.params.sessionId as string)
    ElMessage.success('会话已取消')
    await fetchSession()
  } catch {
    ElMessage.error('取消失败，会话状态可能已变化')
  } finally {
    acting.value = false
  }
}

async function handleNewRound() {
  acting.value = true
  try {
    const created = await sessionApi.createSession(route.params.reviewId as string)
    ElMessage.success('已发起新一轮会话')
    if (created?.session_id) {
      router.replace(`/reviews/${route.params.reviewId}/sessions/${created.session_id}`)
    } else {
      await fetchSession()
    }
  } catch {
    ElMessage.error('发起会话失败，请稍后重试')
  } finally {
    acting.value = false
  }
}

onMounted(async () => {
  window.addEventListener('resize', updateViewportWidth)
  await fetchSession()
  loading.value = false
  schedulePolling()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewportWidth)
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
})
</script>

<style scoped>
.back-button {
  flex: 0 0 auto;
  margin-bottom: 12px;
}

.empty {
  padding: 40px;
}

.main-card,
.section-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-header > span:first-child {
  color: var(--app-text);
  font-weight: 700;
}

.review-link {
  color: var(--app-primary);
  text-decoration: none;
}

.review-link:hover {
  text-decoration: underline;
}

code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  background: #f5f5f5;
  padding: 2px 4px;
  border-radius: 2px;
  overflow-wrap: anywhere;
}

.agent-result {
  padding: 16px;
  background: #f7f8fc;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  margin-bottom: 12px;
}

.agent-result:last-child {
  margin-bottom: 0;
}

.agent-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
}

.agent-header h4 {
  margin: 0;
  color: var(--app-text);
  font-size: 14px;
}

.confidence {
  font-size: 12px;
  color: #909399;
}

.agent-result p {
  margin: 0 0 8px;
  color: #606266;
  font-size: 13px;
  line-height: 1.6;
}

.findings-count {
  font-size: 12px;
  color: #909399;
}

.final-report {
  padding: 4px 0;
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
  color: var(--app-text);
}

.summary {
  margin-bottom: 16px;
  color: #606266;
  line-height: 1.6;
}

.findings-section {
  margin-top: 16px;
}

.findings-section h4 {
  margin: 0 0 8px;
  font-size: 14px;
  color: var(--app-text);
}

.findings-section ul {
  margin: 0;
  padding-left: 20px;
  color: #606266;
}

.findings-section li {
  margin-bottom: 4px;
  line-height: 1.5;
}

.progress-card {
  position: sticky;
  top: 20px;
}

@media (max-width: 720px) {
  .progress-card {
    position: static;
  }
}
</style>
