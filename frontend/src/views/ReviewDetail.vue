<template>
  <AppLayout>
    <div class="detail-page" v-loading="loading">
      <header class="page-header">
        <el-button class="back-button" circle plain aria-label="返回审查列表" @click="router.push('/reviews')">
          <el-icon><ArrowLeft /></el-icon>
        </el-button>
        <div>
          <div class="eyebrow">REVIEW REPORT</div>
          <h1>{{ detail?.repo || '审查详情' }}</h1>
          <p v-if="detail">{{ getProviderLabel(detail.provider) }} · {{ detail.review_id }}</p>
        </div>
        <div class="header-actions">
          <el-button 
            :type="debugMode ? 'warning' : 'default'" 
            plain 
            size="small"
            @click="toggleDebugMode"
          >
            {{ debugMode ? 'Debug ON' : 'Debug OFF' }}
          </el-button>
          <el-button v-if="detail" plain @click="fetchDetail">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </header>

      <template v-if="detail">
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

        <section class="summary-strip">
          <div class="summary-result" :class="getSummaryClass(detail)">
            <span class="summary-icon">
              <el-icon v-if="detail.gate?.passed"><SuccessFilled /></el-icon>
              <el-icon v-else-if="detail.status === 'failed' || detail.gate?.status === 'failed'"><CircleCloseFilled /></el-icon>
              <el-icon v-else><InfoFilled /></el-icon>
            </span>
            <div>
              <small>质量门禁</small>
              <strong>{{ getSummaryLabel(detail) }}</strong>
            </div>
          </div>
          <div class="summary-metric">
            <small>质量评分</small>
            <strong>{{ detail.report?.generated ? detail.report.score.value : '--' }}<span v-if="detail.report?.generated"> / 100</span></strong>
          </div>
          <div class="summary-metric">
            <small>发现问题</small>
            <strong>{{ detail.findings?.length || 0 }}<span> 项</span></strong>
          </div>
          <div class="summary-metric">
            <small>执行状态</small>
            <el-tag :type="getStatusType(detail.status)">{{ getStatusLabel(detail.status) }}</el-tag>
          </div>
          <div class="summary-metric">
            <small>最后更新</small>
            <strong class="summary-time">{{ formatTime(detail.updated_at) }}</strong>
          </div>
        </section>

        <el-row :gutter="16">
          <el-col :xs="24" :lg="16">
            <el-card class="info-card">
              <template #header>
                <div class="card-header">
                  <span>审查信息</span>
                  <span class="card-caption">提交范围与执行参数</span>
                </div>
              </template>
              <el-descriptions :column="descriptionColumns" border>
                <el-descriptions-item label="Review ID">{{ detail.review_id }}</el-descriptions-item>
                <el-descriptions-item label="代码平台">{{ detail.provider }}</el-descriptions-item>
                <el-descriptions-item label="PR/MR">
                  <span v-if="detail.pull_request_id">#{{ detail.pull_request_id }}</span>
                  <span v-else-if="detail.merge_request_iid">!{{ detail.merge_request_iid }}</span>
                  <span v-else>-</span>
                </el-descriptions-item>
                <el-descriptions-item label="开发者">
                  <router-link v-if="detail.author" :to="`/contributors/${encodeURIComponent(detail.author)}`" class="detail-author-link">
                    {{ detail.author_name || detail.author }}
                  </router-link>
                  <span v-else class="text-muted">-</span>
                </el-descriptions-item>
                <el-descriptions-item label="基准 SHA">
                  <code v-if="detail.base_sha">{{ detail.base_sha }}</code>
                  <span v-else class="text-muted">未指定</span>
                </el-descriptions-item>
                <el-descriptions-item label="目标 SHA">
                  <code v-if="detail.head_sha">{{ detail.head_sha }}</code>
                  <span v-else class="text-muted">未指定</span>
                </el-descriptions-item>
                <el-descriptions-item label="创建时间">{{ formatTime(detail.created_at) }}</el-descriptions-item>
                <el-descriptions-item label="更新时间">{{ formatTime(detail.updated_at) }}</el-descriptions-item>
              </el-descriptions>
            </el-card>

            <!-- 代码 Review 执行信息 -->
            <el-card class="ocr-card" v-if="detail.report?.ocr">
              <template #header>
                <div class="card-header">
                  <span>代码 Review 执行信息</span>
                  <el-tag :type="detail.report.ocr.status === 'success' ? 'success' : detail.report.ocr.status === 'skipped' ? 'warning' : 'info'" size="small">
                    {{ detail.report.ocr.status || '未知' }}
                  </el-tag>
                </div>
              </template>
              <el-descriptions :column="ocrDescriptionColumns" border size="small">
                <el-descriptions-item label="引擎">
                  {{ detail.report.ocr.engine === 'open-code-review' ? 'AI Code Review 引擎' : detail.report.ocr.engine || '-' }}
                </el-descriptions-item>
                <el-descriptions-item label="审查文件数">{{ detail.report.ocr.files_reviewed ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="评论数">{{ detail.report.ocr.comments ?? '-' }}</el-descriptions-item>
                <el-descriptions-item label="总 Tokens">{{ detail.report.ocr.total_tokens ? detail.report.ocr.total_tokens.toLocaleString() : '-' }}</el-descriptions-item>
                <el-descriptions-item label="输入 Tokens">{{ detail.report.ocr.input_tokens ? detail.report.ocr.input_tokens.toLocaleString() : '-' }}</el-descriptions-item>
                <el-descriptions-item label="输出 Tokens">{{ detail.report.ocr.output_tokens ? detail.report.ocr.output_tokens.toLocaleString() : '-' }}</el-descriptions-item>
                <el-descriptions-item label="耗时">{{ detail.report.ocr.elapsed || '-' }}</el-descriptions-item>
                <el-descriptions-item label="审查范围">
                  {{ detail.report.scope?.reviewed_files ?? '-' }} / {{ detail.report.scope?.changed_files ?? '-' }} 文件
                </el-descriptions-item>
                <el-descriptions-item label="排除文件">{{ detail.report.scope?.excluded_files ?? '-' }}</el-descriptions-item>
              </el-descriptions>
            </el-card>

            <!-- 质量评估 -->
            <el-card class="quality-card" v-if="detail.report?.generated">
              <template #header>
                <div class="card-header">
                  <span>质量评估</span>
                  <el-tag v-if="!detail.report.complete" type="warning" size="small">审查未完整完成</el-tag>
                </div>
              </template>
              <el-row :gutter="24">
                <el-col :xs="24" :sm="8">
                  <div class="score-block">
                    <div class="score-value">{{ detail.report.score.value }}</div>
                    <div class="score-label">/ 100</div>
                    <div class="score-grade">{{ detail.report.score.grade }}</div>
                  </div>
                </el-col>
                <el-col :xs="24" :sm="16">
                  <div class="severity-block">
                    <div class="severity-item critical">
                      <span>严重</span>
                      <strong>{{ detail.report.counts.critical }}</strong>
                    </div>
                    <div class="severity-item major">
                      <span>高风险</span>
                      <strong>{{ detail.report.counts.major }}</strong>
                    </div>
                    <div class="severity-item minor">
                      <span>中风险</span>
                      <strong>{{ detail.report.counts.minor }}</strong>
                    </div>
                    <div class="severity-item info">
                      <span>提示</span>
                      <strong>{{ detail.report.counts.info }}</strong>
                    </div>
                  </div>
                  <!-- 扣分详情 -->
                  <div v-if="detail.report.score.deductions?.length" class="deductions">
                    <div v-for="(d, i) in detail.report.score.deductions" :key="i" class="deduction-item">
                      <el-text type="danger" size="small">-{{ d.points }}分: {{ d.reason }}</el-text>
                    </div>
                  </div>
                </el-col>
              </el-row>
            </el-card>

            <!-- 警告 -->
            <el-card class="warnings-card" v-if="detail.report?.warnings?.length">
              <template #header>
                <div class="card-header">
                  <span>警告</span>
                  <el-tag type="warning" size="small">{{ detail.report.warnings.length }}</el-tag>
                </div>
              </template>
              <div v-for="(w, i) in detail.report.warnings" :key="i" class="warning-item">
                <el-text type="warning">
                  <span v-if="w.type">[{{ w.type }}] </span>
                  {{ w.message }}
                  <span v-if="w.path" class="warning-path"> ({{ w.path }})</span>
                </el-text>
              </div>
            </el-card>

            <!-- 分析概览 -->
            <el-card class="analysis-card" v-if="detail.analysis?.overview || detail.analysis?.root_cause?.length || detail.analysis?.risks?.length || detail.analysis?.recommendations?.length">
              <template #header>分析概览</template>
              <div v-if="detail.analysis.overview" class="analysis-section">
                <p>{{ detail.analysis.overview }}</p>
              </div>
              <div v-if="detail.analysis.root_cause?.length" class="analysis-section">
                <h4>根因分析</h4>
                <ul>
                  <li v-for="(item, i) in detail.analysis.root_cause" :key="i">{{ item }}</li>
                </ul>
              </div>
              <div v-if="detail.analysis.risks?.length" class="analysis-section">
                <h4>风险评估</h4>
                <ul>
                  <li v-for="(item, i) in detail.analysis.risks" :key="i">{{ item }}</li>
                </ul>
              </div>
              <div v-if="detail.analysis.recommendations?.length" class="analysis-section">
                <h4>改进建议</h4>
                <ul>
                  <li v-for="(item, i) in detail.analysis.recommendations" :key="i">{{ item }}</li>
                </ul>
              </div>
            </el-card>

            <!-- 问题报告 -->
            <el-card class="findings-card" v-if="detail.findings?.length">
              <template #header>
                <div class="card-header">
                  <span>问题报告</span>
                  <el-tag>{{ detail.findings.length }} 个问题</el-tag>
                </div>
              </template>
              <div class="finding-list">
                <div v-for="(finding, index) in detail.findings" :key="index" class="finding-item">
                  <div class="finding-header">
                    <el-tag :type="getSeverityType(finding.severity)" size="small">
                      {{ finding.severity.toUpperCase() }}
                    </el-tag>
                    <span class="finding-title">{{ finding.title }}</span>
                    <span class="finding-path" v-if="finding.path">
                      {{ finding.path }}{{ finding.line ? `:${finding.line}` : '' }}
                    </span>
                  </div>
                  <div class="finding-body" v-if="finding.body">{{ finding.body }}</div>
                  <div class="finding-code" v-if="finding.existing_code">
                    <div class="code-label">现有代码</div>
                    <pre>{{ finding.existing_code }}</pre>
                  </div>
                  <div class="finding-code" v-if="finding.suggestion_code">
                    <div class="code-label">建议代码</div>
                    <pre>{{ finding.suggestion_code }}</pre>
                  </div>
                </div>
              </div>
            </el-card>

            <!-- 发布到代码平台 -->
            <PublishPanel
              v-if="detail.status === 'success' && detail.report?.generated"
              :review-id="detail.review_id"
            />

            <!-- 代码审查地图 -->
            <el-card class="review-map-card" v-if="reviewMaps.length > 0">
              <template #header>
                <div class="card-header">
                  <span>代码审查地图</span>
                  <el-tag size="small">{{ reviewMaps.length }} 个</el-tag>
                </div>
              </template>
              <ReviewMap v-for="map in reviewMaps" :key="map.id" :map="map" style="margin-bottom: 16px;" />
            </el-card>

            <!-- 无地图时生成入口 -->
            <el-card class="review-map-card" v-else-if="canGenerateMap">
              <template #header>
                <div class="card-header">
                  <span>代码审查地图</span>
                </div>
              </template>
              <el-empty description="本次审查尚未生成代码地图" :image-size="80">
                <el-button type="primary" plain :loading="generatingMap" @click="handleGenerateMap">
                  <el-icon><MagicStick /></el-icon>
                  生成评审地图
                </el-button>
              </el-empty>
            </el-card>

            <!-- Debug 区域 - OCR 原始输入 -->
            <el-card class="debug-card" v-if="debugMode && detail.raw_ocr_input">
              <template #header>
                <div class="card-header">
                  <span>
                    <el-tag type="warning" size="small" style="margin-right: 8px;">DEBUG</el-tag>
                    OCR 原始输入
                  </span>
                  <el-button size="small" @click="showOcrInput = !showOcrInput">
                    {{ showOcrInput ? '收起' : '展开' }}
                  </el-button>
                </div>
              </template>
              <el-collapse v-model="ocrInputActive">
                <el-collapse-item title="查看完整 JSON" name="input">
                  <pre class="debug-json">{{ formatJson(detail.raw_ocr_input) }}</pre>
                </el-collapse-item>
              </el-collapse>
            </el-card>

            <!-- Debug 区域 - OCR 原始输出 -->
            <el-card class="debug-card" v-if="debugMode && detail.raw_ocr">
              <template #header>
                <div class="card-header">
                  <span>
                    <el-tag type="warning" size="small" style="margin-right: 8px;">DEBUG</el-tag>
                    OCR 原始输出
                  </span>
                  <el-button size="small" @click="showOcrOutput = !showOcrOutput">
                    {{ showOcrOutput ? '收起' : '展开' }}
                  </el-button>
                </div>
              </template>
              <el-collapse v-model="ocrOutputActive">
                <el-collapse-item title="查看完整 JSON" name="output">
                  <pre class="debug-json">{{ formatJson(detail.raw_ocr) }}</pre>
                </el-collapse-item>
              </el-collapse>
            </el-card>
          </el-col>

          <el-col :xs="24" :lg="8">
            <el-card class="gate-card">
              <template #header>质量门禁</template>
              <div class="gate-status" :class="detail.gate?.status">
                <div class="gate-icon">
                  <el-icon v-if="detail.gate?.passed" :size="48"><SuccessFilled /></el-icon>
                  <el-icon v-else-if="isGateFailed(detail)" :size="48"><CircleCloseFilled /></el-icon>
                  <el-icon v-else :size="48"><InfoFilled /></el-icon>
                </div>
                <div class="gate-text">
                  {{ detail.gate?.passed ? '门禁通过' : isGateFailed(detail) ? '执行失败' : '待定' }}
                </div>
                <div class="gate-reason" v-if="detail.gate?.reason">{{ detail.gate.reason }}</div>
              </div>
            </el-card>

            <el-card class="history-card" v-if="detail.history?.length">
              <template #header>执行历史</template>
              <el-timeline>
                <el-timeline-item
                  v-for="(event, index) in detail.history"
                  :key="index"
                  :timestamp="formatTime(event.created_at)"
                  :type="getEventType(event.status)"
                >
                  <h4>{{ event.message }}</h4>
                  <p v-if="event.details">{{ formatDetails(event.details) }}</p>
                </el-timeline-item>
              </el-timeline>
            </el-card>

            <el-card class="sessions-card" v-if="sessions.length > 0">
              <template #header>
                <div class="card-header">
                  <span>审查会话</span>
                  <el-tag size="small">{{ sessions.length }} 轮</el-tag>
                </div>
              </template>
              <div class="session-list">
                <div
                  v-for="sess in sessions"
                  :key="sess.session_id"
                  class="session-item"
                  @click="goToSession(sess)"
                >
                  <div class="session-info">
                    <div class="session-header">
                      <span class="session-round">第 {{ sess.round }} 轮</span>
                      <el-tag :type="getSessionStatusType(sess.status)" size="small">
                        {{ getSessionStatusLabel(sess.status) }}
                      </el-tag>
                    </div>
                    <div class="session-phase">
                      {{ getPhaseLabel(sess.phase) }}
                    </div>
                    <el-progress :percentage="sess.progress" :stroke-width="4" :show-text="false" />
                  </div>
                  <el-icon><ArrowRight /></el-icon>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>
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
import { ArrowLeft, ArrowRight, SuccessFilled, CircleCloseFilled, InfoFilled, Refresh, MagicStick } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ReviewMap from '@/components/ReviewMap.vue'
import ProgressTracker from '@/components/ProgressTracker.vue'
import PublishPanel from '@/components/PublishPanel.vue'
import { getReview, getReviewMaps, getReviewMap, generateReviewMap } from '@/api/review'
import { sessionApi, type ReviewSession } from '@/api/session'
import { getProviderLabel } from '@/utils/url-parser'
import { getStatusLabel, getStatusType, getSeverityType, getPhaseLabel, getSessionStatusType, getSessionStatusLabel } from '@/utils/status'
import { formatDateTime } from '@/utils/format'
import type { ReviewDetail } from '@/types/review'
import { ElMessage } from 'element-plus/es/components/message/index'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const detail = ref<ReviewDetail | null>(null)
const sessions = ref<ReviewSession[]>([])
const reviewMaps = ref<any[]>([])
const generatingMap = ref(false)
const viewportWidth = ref(window.innerWidth)
let pollTimer: ReturnType<typeof setTimeout> | null = null
let fetchInFlight = false
const descriptionColumns = computed(() => viewportWidth.value < 720 ? 1 : 2)
const ocrDescriptionColumns = computed(() => viewportWidth.value < 720 ? 1 : 3)
const showProgressTracker = computed(() => Boolean(detail.value && (
  ['queued', 'running', 'failed', 'success', 'completed'].includes(detail.value.status) ||
  sessions.value.length > 0 ||
  (detail.value.history?.length || 0) > 0
)))

const canGenerateMap = computed(() => Boolean(
  detail.value
  && ['success', 'completed'].includes(detail.value.status)
  && !generatingMap.value
  && sessions.value.length > 0
))

const mapSessionId = computed(() => {
  const completed = sessions.value.find((s) => s.status === 'completed')
  return (completed || sessions.value[sessions.value.length - 1])?.session_id || ''
})

// Debug mode
const debugMode = ref(localStorage.getItem('review-debug-mode') === 'true')
const showOcrInput = ref(false)
const showOcrOutput = ref(false)
const ocrInputActive = ref<string[]>([])
const ocrOutputActive = ref<string[]>([])

function toggleDebugMode() {
  debugMode.value = !debugMode.value
  localStorage.setItem('review-debug-mode', String(debugMode.value))
}

function formatJson(data: any): string {
  if (!data) return ''
  try {
    if (typeof data === 'string') {
      return JSON.stringify(JSON.parse(data), null, 2)
    }
    return JSON.stringify(data, null, 2)
  } catch {
    return String(data)
  }
}

function updateViewportWidth() {
  viewportWidth.value = window.innerWidth
}

function getSummaryLabel(review: ReviewDetail) {
  if (review.gate?.passed) return '审查通过'
  if (review.status === 'failed') return '执行失败'
  if (review.gate?.status === 'failed') return '需要修复'
  return '等待结果'
}

function getSummaryClass(review: ReviewDetail) {
  if (review.gate?.passed) return 'passed'
  if (review.status === 'failed' || review.gate?.status === 'failed') return 'failed'
  return 'pending'
}

function isGateFailed(review: ReviewDetail) {
  return review.status === 'failed' || Boolean(review.gate?.status && ['failed', 'error'].includes(review.gate.status))
}

async function fetchDetail(background = false) {
  if (fetchInFlight) return
  fetchInFlight = true
  if (!background) loading.value = true
  try {
    detail.value = await getReview(route.params.id as string)
    await Promise.all([fetchSessions(), fetchReviewMaps()])
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
  } catch (error) {
    console.error('Failed to fetch sessions:', error)
    sessions.value = []
  }
}

async function fetchReviewMaps() {
  try {
    const response = await getReviewMaps(route.params.id as string)
    const mapList = response.maps || response || []
    // 获取每个 map 的完整数据
    const fullMaps = await Promise.all(
      mapList.map((m: any) => getReviewMap(route.params.id as string, m.id).catch(() => m))
    )
    reviewMaps.value = fullMaps
  } catch (error) {
    console.error('Failed to fetch review maps:', error)
    reviewMaps.value = []
  }
}

function goToSession(sess: ReviewSession) {
  router.push(`/reviews/${route.params.id}/sessions/${sess.session_id}`)
}

async function handleGenerateMap() {
  if (!mapSessionId.value) {
    ElMessage.warning('暂无可用会话，无法生成评审地图')
    return
  }
  generatingMap.value = true
  try {
    await generateReviewMap(route.params.id as string, mapSessionId.value)
    ElMessage.success('评审地图已生成')
    await fetchReviewMaps()
  } catch {
    ElMessage.error('生成评审地图失败，请稍后重试')
  } finally {
    generatingMap.value = false
  }
}

function getEventType(status: string) {
  const map: Record<string, string> = {
    completed: 'success',
    running: 'warning',
    failed: 'danger',
  }
  return map[status] || 'info'
}

function formatTime(time: string) {
  return formatDateTime(time)
}

function formatDetails(details: Record<string, any>) {
  return Object.entries(details)
    .map(([key, value]) => `${key}: ${value}`)
    .join(', ')
}

onMounted(() => {
  window.addEventListener('resize', updateViewportWidth)
  fetchDetail()
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', updateViewportWidth)
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
})

function schedulePolling() {
  if (pollTimer) {
    clearTimeout(pollTimer)
    pollTimer = null
  }
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
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 22px;
}

.page-header > div {
  min-width: 0;
  flex: 1;
}

.page-header h1 {
  margin: 3px 0 4px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 28px;
  letter-spacing: -0.6px;
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

.back-button {
  flex: 0 0 auto;
}

.summary-strip {
  display: grid;
  grid-template-columns: minmax(210px, 1.3fr) repeat(4, minmax(120px, 1fr));
  margin-bottom: 18px;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 14px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.failure-alert {
  margin-bottom: 18px;
}

.failure-alert :deep(.el-alert__description) {
  overflow-wrap: anywhere;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  line-height: 1.6;
}

.summary-result,
.summary-metric {
  display: flex;
  min-height: 92px;
  align-items: center;
  padding: 18px 20px;
  border-right: 1px solid var(--app-border);
}

.summary-metric {
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
}

.summary-metric:last-child {
  border-right: 0;
}

.summary-icon {
  display: grid;
  width: 42px;
  height: 42px;
  margin-right: 12px;
  place-items: center;
  border-radius: 12px;
  font-size: 21px;
}

.summary-result.passed .summary-icon {
  color: var(--app-success);
  background: #eaf8f3;
}

.summary-result.failed .summary-icon {
  color: var(--app-danger);
  background: #fff0f2;
}

.summary-result.pending .summary-icon {
  color: #8d96a7;
  background: #f1f3f6;
}

.summary-result small,
.summary-metric small {
  display: block;
  margin-bottom: 5px;
  color: #929aab;
  font-size: 11px;
}

.summary-result strong,
.summary-metric strong {
  color: var(--app-text);
  font-size: 17px;
}

.summary-metric strong span {
  color: #9aa2b1;
  font-size: 11px;
  font-weight: 500;
}

.summary-metric .summary-time {
  font-size: 12px;
}

.info-card,
.quality-card,
.ocr-card,
.warnings-card,
.analysis-card,
.findings-card,
.review-map-card,
.raw-ocr-card,
.debug-card,
.gate-card,
.history-card {
  margin-bottom: 16px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.card-header > span:first-child {
  color: var(--app-text);
  font-weight: 700;
}

.card-caption {
  color: #9aa2b1;
  font-size: 11px;
  font-weight: 400;
}

.text-muted {
  color: #999;
}

code {
  max-width: 100%;
  font-family: monospace;
  font-size: 12px;
  background: #f5f5f5;
  padding: 2px 4px;
  border-radius: 2px;
  overflow-wrap: anywhere;
  white-space: normal;
}

:deep(.el-descriptions__content) {
  min-width: 0;
  overflow-wrap: anywhere;
}

.score-block {
  text-align: center;
  padding: 20px;
}

.score-value {
  font-size: 48px;
  font-weight: bold;
  color: var(--app-primary);
}

.score-label {
  font-size: 16px;
  color: #999;
}

.score-grade {
  font-size: 18px;
  margin-top: 8px;
  color: #333;
}

.severity-block {
  display: flex;
  gap: 16px;
}

.severity-item {
  flex: 1;
  text-align: center;
  padding: 16px;
  border-radius: 8px;
  background: #f5f5f5;
}

.severity-item span {
  display: block;
  font-size: 14px;
  color: #666;
  margin-bottom: 8px;
}

.severity-item strong {
  font-size: 24px;
}

.severity-item.critical strong { color: #f56c6c; }
.severity-item.major strong { color: #e6a23c; }
.severity-item.minor strong { color: #409eff; }
.severity-item.info strong { color: #909399; }

.deductions {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid #eee;
}

.deduction-item {
  margin-bottom: 4px;
}

.warning-item {
  padding: 8px 0;
  border-bottom: 1px solid #f0f0f0;
}

.warning-item:last-child {
  border-bottom: none;
}

.warning-path {
  color: #999;
  font-size: 12px;
}

.analysis-section {
  margin-bottom: 16px;
}

.analysis-section:last-child {
  margin-bottom: 0;
}

.analysis-section h4 {
  margin: 0 0 8px;
  font-size: 14px;
  color: #333;
}

.analysis-section p {
  margin: 0;
  color: #606266;
  line-height: 1.6;
}

.analysis-section ul {
  margin: 0;
  padding-left: 20px;
  color: #606266;
}

.analysis-section li {
  margin-bottom: 4px;
  line-height: 1.5;
}

.finding-item {
  padding: 16px;
  border: 1px solid #eee;
  border-radius: 8px;
  margin-bottom: 12px;
}

.finding-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.finding-title {
  font-weight: bold;
}

.finding-path {
  color: #999;
  font-size: 14px;
}

.finding-body {
  color: #666;
  margin-bottom: 8px;
}

.finding-code {
  background: #f5f5f5;
  padding: 12px;
  border-radius: 4px;
  margin-top: 8px;
}

.code-label {
  font-size: 12px;
  color: #999;
  margin-bottom: 4px;
}

.finding-code pre {
  margin: 0;
  font-family: monospace;
  white-space: pre-wrap;
  word-break: break-all;
}

.raw-ocr-json {
  font-family: monospace;
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 500px;
  overflow-y: auto;
  background: #f8f8f8;
  padding: 12px;
  border-radius: 4px;
  margin: 0;
}

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.debug-card {
  border: 1px dashed #e6a23c;
  background: #fdf6ec;
}

.debug-card :deep(.el-card__header) {
  background: #faecd8;
  border-bottom: 1px dashed #e6a23c;
}

.debug-json {
  font-family: 'Fira Code', 'SF Mono', Monaco, 'Cascadia Code', monospace;
  font-size: 12px;
  line-height: 1.6;
  max-height: 600px;
  overflow: auto;
  background: #1e1e1e;
  color: #d4d4d4;
  padding: 16px;
  border-radius: 6px;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
}

.gate-status {
  text-align: center;
  padding: 24px;
}

.gate-icon {
  margin-bottom: 16px;
}

.gate-status.passed .gate-icon { color: #67c23a; }
.gate-status.failed .gate-icon,
.gate-status.error .gate-icon { color: #f56c6c; }
.gate-status.pending .gate-icon { color: #909399; }

.gate-text {
  font-size: 18px;
  font-weight: bold;
  margin-bottom: 8px;
}

.gate-reason {
  color: #666;
  font-size: 14px;
}

@media (max-width: 1180px) {
  .summary-strip {
    grid-template-columns: repeat(4, 1fr);
  }

  .summary-result {
    grid-column: span 2;
  }

  .summary-metric:last-child {
    grid-column: span 2;
  }
}

@media (max-width: 720px) {
  .detail-page {
    padding: 22px 14px 36px;
  }

  .page-header {
    align-items: center;
  }

  .page-header h1 {
    font-size: 21px;
  }

  .page-header .eyebrow,
  .page-header > .el-button:last-child {
    display: none;
  }

  .summary-strip {
    grid-template-columns: 1fr 1fr;
  }

  .summary-result {
    grid-column: span 2;
  }

  .summary-result,
  .summary-metric {
    min-height: 78px;
    padding: 14px;
  }

  .summary-metric:nth-child(3) {
    border-right: 0;
  }

  .summary-metric:last-child {
    grid-column: span 2;
    border-top: 1px solid var(--app-border);
  }

  .severity-block {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 8px;
  }

  .finding-header {
    flex-wrap: wrap;
  }

  .finding-path {
    width: 100%;
    overflow-wrap: anywhere;
  }
}

.sessions-card {
  margin-top: 16px;
}

.session-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px;
  background: #f5f7fa;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.session-item:hover {
  background: #ecf5ff;
  transform: translateX(4px);
}

.session-info {
  flex: 1;
}

.session-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.session-round {
  font-weight: 600;
  font-size: 14px;
}

.session-phase {
  font-size: 12px;
  color: #666;
  margin-bottom: 8px;
}

.session-item .el-icon {
  color: #909399;
}
</style>
