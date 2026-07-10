<template>
  <AppLayout>
    <div class="detail-page" v-loading="loading">
      <!-- 页头 -->
      <header class="page-header">
        <el-button class="back-button" circle plain aria-label="返回审查列表" @click="router.push('/reviews')">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <div class="header-title">
          <div class="eyebrow">REVIEW REPORT</div>
          <h1>{{ detail?.repo || '审查详情' }}</h1>
          <p v-if="detail">{{ getProviderLabel(detail.provider) }} · {{ detail.review_id }}</p>
        </div>
        <div class="header-actions">
          <el-button class="debug-toggle-btn" :type="debugMode ? 'warning' : 'default'" plain size="small" @click="toggleDebugMode">
            {{ debugMode ? 'Debug ON' : 'Debug' }}
          </el-button>
        </div>
      </header>

      <template v-if="detail">
        <!-- 失败告警 -->
        <el-alert
          v-if="detail.status === 'failed' && detail.error"
          class="failure-alert"
          title="审查执行失败"
          type="error"
          :description="detail.error"
          show-icon
          :closable="false"
        />

        <!-- 实时进度追踪器 -->
        <ProgressTracker
          v-if="showProgressTracker"
          :review-id="detail.review_id"
          :review-status="detail.status"
          :sessions="sessions"
          :history="detail.history"
        />

        <!-- 左右双栏布局 -->
        <div class="review-layout">
          <!-- 左栏：门禁 + 评分 + 严重度 + 元信息 + 操作（sticky） -->
          <div class="review-sidebar-wrapper">
            <ReviewSummarySidebar
              :detail="detail"
              :acting="acting"
              @cancel="handleCancel"
              @rerun="handleRerun"
              @refresh="fetchDetail"
            />

            <!-- 历史 / 会话（左栏底部） -->
            <div class="sidebar-extra">
              <ReviewHistoryCard v-if="detail.history?.length" :history="detail.history" />
              <ReviewSessionsCard v-if="sessions.length > 0" :sessions="sessions" @go-to-session="goToSession" />
            </div>
          </div>

          <!-- 右栏：摘要 + Findings + 分析 + OCR + 发布 + 调试 -->
          <ReviewContentArea
            ref="contentRef"
            :detail="detail"
            :debug-mode="debugMode"
            @toggle-debug="toggleDebugMode"
          />
        </div>
      </template>

      <el-empty v-else-if="!loading" description="未找到这条审查记录">
        <el-button type="primary" @click="router.push('/reviews')">返回审查列表</el-button>
      </el-empty>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProgressTracker from '@/components/ProgressTracker.vue'
import ReviewSummarySidebar from '@/components/review-detail/ReviewSummarySidebar.vue'
import ReviewContentArea from '@/components/review-detail/ReviewContentArea.vue'
import ReviewHistoryCard from '@/components/review-detail/ReviewHistoryCard.vue'
import ReviewSessionsCard from '@/components/review-detail/ReviewSessionsCard.vue'
import { getReview, cancelReview, rerunReview } from '@/api/review'
import { sessionApi, type ReviewSession } from '@/api/session'
import { getProviderLabel } from '@/utils/url-parser'
import type { ReviewDetail as ReviewDetailType } from '@/types/review'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const detail = ref<ReviewDetailType | null>(null)
const sessions = ref<ReviewSession[]>([])
const acting = ref(false)
const contentRef = ref<InstanceType<typeof ReviewContentArea> | null>(null)
let pollTimer: ReturnType<typeof setTimeout> | null = null
let fetchInFlight = false

const showProgressTracker = computed(() => Boolean(detail.value && (
  ['queued', 'running', 'failed'].includes(detail.value.status)
)))

const debugMode = ref(localStorage.getItem('review-debug-mode') === 'true')

function toggleDebugMode() {
  debugMode.value = !debugMode.value
  localStorage.setItem('review-debug-mode', String(debugMode.value))
}

async function fetchDetail(background = false) {
  if (fetchInFlight) return
  fetchInFlight = true
  if (!background) loading.value = true
  try {
    detail.value = await getReview(route.params.id as string)
    contentRef.value?.resetPatchState()
    await fetchSessions()
  } finally {
    fetchInFlight = false
    if (!background) loading.value = false
    schedulePolling()
  }
}

async function fetchSessions() {
  try {
    const response = await sessionApi.getSessions(route.params.id as string)
    sessions.value = response.sessions || []
  } catch {
    sessions.value = []
  }
}

function goToSession(sess: ReviewSession) {
  router.push(`/reviews/${route.params.id}/sessions/${sess.session_id}`)
}

async function handleCancel() {
  try {
    await ElMessageBox.confirm('确定取消这条审查任务？', '取消审查', {
      type: 'warning', confirmButtonText: '取消任务', cancelButtonText: '再想想',
    })
  } catch { return }
  acting.value = true
  try {
    await cancelReview(route.params.id as string)
    ElMessage.success('审查任务已取消')
    await fetchDetail()
  } catch {
    ElMessage.error('取消失败，任务可能已完成或状态不允许')
  } finally {
    acting.value = false
  }
}

async function handleRerun() {
  acting.value = true
  try {
    await rerunReview(route.params.id as string)
    ElMessage.success('已重新加入审查队列')
    await fetchDetail()
  } catch {
    ElMessage.error('重新审查失败，请稍后重试')
  } finally {
    acting.value = false
  }
}

onMounted(() => { fetchDetail() })

onBeforeUnmount(() => {
  if (pollTimer) { clearTimeout(pollTimer); pollTimer = null }
})

function schedulePolling() {
  if (pollTimer) { clearTimeout(pollTimer); pollTimer = null }
  if (!detail.value || !['queued', 'running'].includes(detail.value.status)) return
  pollTimer = setTimeout(() => {
    if (document.visibilityState === 'visible') {
      fetchDetail(true)
    } else {
      schedulePolling()
    }
  }, 5000)
}
</script>

<style scoped>
.detail-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 24px 28px 48px;
}

/* 页头 */
.page-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 20px;
}
.header-title { min-width: 0; flex: 1; }
.page-header h1 {
  margin: 2px 0 3px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 24px;
  letter-spacing: -0.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.page-header p {
  margin: 0;
  overflow: hidden;
  color: #929aab;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 11px;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.eyebrow {
  color: var(--app-primary);
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 1.3px;
}
.back-button { flex: 0 0 auto; }

/* 失败告警 */
.failure-alert { margin-bottom: 18px; }
.failure-alert :deep(.el-alert__description) {
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  line-height: 1.6;
}

/* 左右双栏布局 */
.review-layout {
  display: grid;
  grid-template-columns: 300px 1fr;
  gap: 20px;
  align-items: start;
}

/* 左栏 sticky */
.review-sidebar-wrapper {
  position: sticky;
  top: 72px;
  max-height: calc(100vh - 84px);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-bottom: 20px;

  /* 滚动条美化 */
  scrollbar-width: thin;
  scrollbar-color: #d1d5db transparent;
}
.review-sidebar-wrapper::-webkit-scrollbar { width: 4px; }
.review-sidebar-wrapper::-webkit-scrollbar-thumb { background: #d1d5db; border-radius: 2px; }

.sidebar-extra {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* 响应式：<1024px 变单栏 */
@media (max-width: 1023px) {
  .review-layout {
    grid-template-columns: 1fr;
  }
  .review-sidebar-wrapper {
    position: static;
    max-height: none;
    overflow-y: visible;
  }
}
</style>
