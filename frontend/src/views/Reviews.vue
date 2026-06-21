<template>
  <AppLayout>
    <div class="reviews-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">REVIEW WORKSPACE</div>
          <h1>代码审查</h1>
          <p>集中查看审查进度、质量门禁与待处理风险。</p>
        </div>
        <el-button type="primary" size="large" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>
          发起审查
        </el-button>
      </header>

      <el-row :gutter="16" class="stats-row">
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card total">
            <div class="stat-icon"><el-icon><Files /></el-icon></div>
            <div class="stat-item">
              <span>审查总数</span>
              <strong>{{ stats.total }}</strong>
              <small>全部任务记录</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card running">
            <div class="stat-icon"><el-icon><Loading /></el-icon></div>
            <div class="stat-item">
              <span>正在执行</span>
              <strong>{{ stats.running }}</strong>
              <small>排队或分析中</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card failed">
            <div class="stat-icon"><el-icon><WarningFilled /></el-icon></div>
            <div class="stat-item">
              <span>门禁未通过</span>
              <strong>{{ stats.gate_failed }}</strong>
              <small>需要优先处理</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card findings">
            <div class="stat-icon"><el-icon><Aim /></el-icon></div>
            <div class="stat-item">
              <span>发现问题</span>
              <strong>{{ stats.findings }}</strong>
              <small>所有风险项</small>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-card class="records-card">
        <div class="records-toolbar">
          <div>
            <h2>审查记录</h2>
            <p>点击任意记录查看完整报告</p>
          </div>
          <el-button circle plain aria-label="刷新列表" @click="fetchReviews">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div class="filter-bar">
          <el-input
            v-model="filters.repo"
            class="repo-search"
            placeholder="搜索仓库，例如 owner/repo"
            clearable
            @keyup.enter="applyFilters"
            @clear="applyFilters"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filters.provider" placeholder="全部平台" clearable @change="applyFilters">
              <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <el-select v-model="filters.status" placeholder="全部状态" clearable @change="applyFilters">
              <el-option label="排队中" value="queued" />
              <el-option label="审查中" value="running" />
              <el-option label="已完成" value="success" />
              <el-option label="执行失败" value="failed" />
          </el-select>
          <el-button type="primary" plain @click="applyFilters">筛选</el-button>
          <el-button v-if="hasFilters" link @click="resetFilters">重置</el-button>
        </div>

        <el-table
          :data="reviews"
          v-loading="loading"
          class="review-table desktop-table"
          row-class-name="review-row"
          empty-text="暂无符合条件的审查记录"
          @row-click="goToDetail"
        >
          <el-table-column label="仓库" min-width="240">
            <template #default="{ row }">
              <div class="repo-cell">
                <span class="provider-mark">{{ getProviderInitial(row.provider) }}</span>
                <div>
                  <strong>{{ row.repo }}</strong>
                  <div class="review-id">{{ getProviderLabel(row.provider) }} · {{ shortId(row.review_id) }}</div>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="开发者" width="160">
            <template #default="{ row }">
              <div v-if="row.author" class="author-cell">
                <img v-if="row.author_avatar" :src="row.author_avatar" class="author-avatar" :alt="row.author" />
                <span v-else class="author-avatar-fallback">{{ (row.author_name || row.author).slice(0, 1).toUpperCase() }}</span>
                <router-link v-if="row.author" :to="`/contributors/${encodeURIComponent(row.author)}`" class="author-link" @click.stop>
                  {{ row.author_name || row.author }}
                </router-link>
              </div>
              <span v-else class="muted-value">--</span>
            </template>
          </el-table-column>
          <el-table-column label="执行状态" width="120">
            <template #default="{ row }">
              <span class="status-pill" :class="row.status">
                <i />
                {{ getStatusLabel(row.status) }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="质量门禁" width="130">
            <template #default="{ row }">
              <div class="gate-result" :class="getGateClass(row)">
                <el-icon v-if="row.gate?.passed"><CircleCheckFilled /></el-icon>
                <el-icon v-else-if="row.gate?.status !== 'pending'"><CircleCloseFilled /></el-icon>
                <el-icon v-else><Clock /></el-icon>
                {{ getGateLabel(row) }}
              </div>
            </template>
          </el-table-column>
          <el-table-column label="质量评分" width="130">
            <template #default="{ row }">
              <div v-if="row.report_generated" class="score-cell">
                <strong>{{ row.score }}</strong>
                <el-progress :percentage="row.score" :show-text="false" :stroke-width="5" :color="getScoreColor(row.score)" />
              </div>
              <span v-else class="muted-value">尚未生成</span>
            </template>
          </el-table-column>
          <el-table-column label="问题" width="90" align="center">
            <template #default="{ row }">
              <span class="finding-count" :class="{ active: row.findings > 0 }">{{ row.findings }}</span>
            </template>
          </el-table-column>
          <el-table-column label="最后更新" width="150">
            <template #default="{ row }">
              <div class="time-cell">
                <strong>{{ formatDateShort(row.updated_at) }}</strong>
                <span>{{ formatClock(row.updated_at) }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column width="120" align="right">
            <template #default="{ row }">
              <div class="row-actions" @click.stop>
                <el-button
                  v-if="row.status === 'queued' || row.status === 'running'"
                  size="small"
                  type="danger"
                  plain
                  :loading="actingId === row.review_id"
                  @click="handleCancel(row)"
                >
                  取消
                </el-button>
                <el-button
                  v-if="row.status === 'failed' || row.status === 'success'"
                  size="small"
                  type="primary"
                  plain
                  :loading="actingId === row.review_id"
                  @click="handleRerun(row)"
                >
                  重新审查
                </el-button>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div v-loading="loading" class="mobile-review-list">
          <button
            v-for="row in reviews"
            :key="row.review_id"
            class="mobile-review-card"
            type="button"
            @click="goToDetail(row)"
          >
            <span class="provider-mark">{{ getProviderInitial(row.provider) }}</span>
            <span class="mobile-review-main">
              <strong>{{ row.repo }}</strong>
              <small>{{ getProviderLabel(row.provider) }} · {{ shortId(row.review_id) }}</small>
              <span class="mobile-review-meta">
                <span class="status-pill" :class="row.status"><i />{{ getStatusLabel(row.status) }}</span>
                <span class="gate-result" :class="getGateClass(row)">{{ getGateLabel(row) }}</span>
              </span>
            </span>
            <span class="mobile-review-side">
              <strong v-if="row.report_generated">{{ row.score }}<small>分</small></strong>
              <strong v-else>--</strong>
              <small>{{ row.findings }} 个问题</small>
              <div class="mobile-actions" @click.stop>
                <el-button
                  v-if="row.status === 'queued' || row.status === 'running'"
                  size="small"
                  type="danger"
                  plain
                  :loading="actingId === row.review_id"
                  @click="handleCancel(row)"
                >
                  取消
                </el-button>
                <el-button
                  v-if="row.status === 'failed' || row.status === 'success'"
                  size="small"
                  type="primary"
                  plain
                  :loading="actingId === row.review_id"
                  @click="handleRerun(row)"
                >
                  重新审查
                </el-button>
              </div>
              <el-icon><ArrowRight /></el-icon>
            </span>
          </button>
          <div v-if="!loading && reviews.length === 0" class="mobile-empty">
            暂无符合条件的审查记录
          </div>
        </div>

        <div class="pagination-wrap">
          <span>共 {{ pagination.total }} 条记录</span>
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :total="pagination.total"
            :page-sizes="[10, 20, 50, 100]"
            layout="sizes, prev, pager, next"
            @current-change="fetchReviews"
            @size-change="handlePageSizeChange"
          />
        </div>
      </el-card>

      <!-- 发起审查对话框 -->
      <ReviewCreateDialog
        v-model="showCreateDialog"
        @created="fetchReviews"
        @goto="(id) => router.push(`/reviews/${id}`)"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import {
  Aim,
  ArrowRight,
  CircleCheckFilled,
  CircleCloseFilled,
  Clock,
  Files,
  Loading,
  Plus,
  Refresh,
  Search,
  WarningFilled,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ReviewCreateDialog from '@/components/ReviewCreateDialog.vue'
import { getReviews, cancelReview, rerunReview } from '@/api/review'
import { getProviderLabel, PROVIDER_OPTIONS } from '@/utils/url-parser'
import { getStatusLabel } from '@/utils/status'
import { getScoreColor } from '@/utils/colors'
import { formatDateShort, formatClock, getProviderInitial, shortId } from '@/utils/format'
import type { Review, ListStats } from '@/types/review'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'

const router = useRouter()
const loading = ref(false)
const showCreateDialog = ref(false)
const reviews = ref<Review[]>([])
const stats = ref<ListStats>({ total: 0, running: 0, gate_failed: 0, findings: 0 })
const actingId = ref('')

const filters = reactive({ repo: '', provider: '', status: '' })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const hasFilters = computed(() => !!(filters.repo || filters.provider || filters.status))

let listPollTimer: ReturnType<typeof setTimeout> | null = null
let fetchInFlight = false

function openCreateDialog() {
  showCreateDialog.value = true
}

async function fetchReviews(background = false) {
  if (fetchInFlight) return
  fetchInFlight = true
  if (!background) loading.value = true
  try {
    const result = await getReviews({
      ...filters,
      page: pagination.page,
      page_size: pagination.pageSize,
    })
    reviews.value = result.items
    pagination.total = result.total
    stats.value = result.stats

  } finally {
    fetchInFlight = false
    if (!background) loading.value = false
    scheduleListPolling()
  }
}

function scheduleListPolling() {
  if (listPollTimer) {
    clearTimeout(listPollTimer)
    listPollTimer = null
  }
  if (stats.value.running <= 0) return
  listPollTimer = setTimeout(() => {
    if (document.visibilityState === 'visible') {
      fetchReviews(true)
    } else {
      scheduleListPolling()
    }
  }, 10000)
}

function goToDetail(row: Review) {
  router.push(`/reviews/${row.review_id}`)
}

async function handleCancel(row: Review) {
  try {
    await ElMessageBox.confirm(`确定取消这条审查任务？(${shortId(row.review_id)})`, '取消审查', {
      type: 'warning',
      confirmButtonText: '取消任务',
      cancelButtonText: '再想想',
    })
  } catch {
    return
  }
  actingId.value = row.review_id
  try {
    await cancelReview(row.review_id)
    ElMessage.success('审查任务已取消')
    await fetchReviews(true)
  } catch {
    ElMessage.error('取消失败，任务可能已完成或状态不允许')
  } finally {
    actingId.value = ''
  }
}

async function handleRerun(row: Review) {
  actingId.value = row.review_id
  try {
    await rerunReview(row.review_id)
    ElMessage.success('已重新加入审查队列')
    await fetchReviews(true)
  } catch {
    ElMessage.error('重新审查失败，请稍后重试')
  } finally {
    actingId.value = ''
  }
}

function getGateLabel(row: Review) {
  if (row.gate?.status === 'pending') return '待评估'
  return row.gate?.passed ? '已通过' : '未通过'
}

function getGateClass(row: Review) {
  if (row.gate?.status === 'pending') return 'pending'
  return row.gate?.passed ? 'passed' : 'failed'
}

function applyFilters() {
  pagination.page = 1
  fetchReviews()
}

function resetFilters() {
  filters.repo = ''
  filters.provider = ''
  filters.status = ''
  applyFilters()
}

function handlePageSizeChange() {
  pagination.page = 1
  fetchReviews()
}

onMounted(fetchReviews)

onBeforeUnmount(() => {
  if (listPollTimer) {
    clearTimeout(listPollTimer)
    listPollTimer = null
  }
})
</script>

<style scoped>
.reviews-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 28px;
}

.page-header h1 {
  margin: 4px 0 7px;
  color: var(--app-text);
  font-size: 30px;
  letter-spacing: -0.8px;
}

.page-header p {
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.4px;
}

.page-header .el-button {
  min-height: 44px;
  padding-inline: 18px;
  border-radius: 10px;
  box-shadow: 0 7px 18px rgba(91, 91, 214, 0.2);
}

.stats-row {
  margin-bottom: 20px;
}

.stats-row .el-col {
  margin-bottom: 16px;
}

.stat-card {
  height: 126px;
  overflow: hidden;
}

.stat-card :deep(.el-card__body) {
  display: flex;
  align-items: center;
  gap: 15px;
  height: 100%;
  padding: 20px;
}

.stat-icon {
  display: grid;
  width: 44px;
  height: 44px;
  flex: 0 0 44px;
  place-items: center;
  border-radius: 12px;
  font-size: 20px;
}

.total .stat-icon {
  color: #5b5bd6;
  background: #eeeeff;
}

.running .stat-icon {
  color: #2878c7;
  background: #eaf4ff;
}

.failed .stat-icon {
  color: #d84b5b;
  background: #fff0f2;
}

.findings .stat-icon {
  color: #d98616;
  background: #fff6e8;
}

.stat-item {
  min-width: 0;
}

.stat-item span {
  display: block;
  margin-bottom: 2px;
  color: var(--app-muted);
  font-size: 13px;
  font-weight: 600;
}

.stat-item strong {
  display: block;
  color: var(--app-text);
  font-size: 27px;
  line-height: 1.15;
}

.stat-item small {
  display: block;
  margin-top: 4px;
  color: #9aa2b1;
  font-size: 11px;
}

.records-card {
  overflow: hidden;
  box-shadow: var(--app-shadow);
}

.records-card :deep(.el-card__body) {
  padding: 0;
}

.records-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 22px 16px;
}

.records-toolbar h2 {
  margin: 0 0 4px;
  color: var(--app-text);
  font-size: 17px;
}

.records-toolbar p {
  margin: 0;
  color: #929aab;
  font-size: 12px;
}

.filter-bar {
  display: flex;
  gap: 10px;
  padding: 0 22px 18px;
  border-bottom: 1px solid var(--app-border);
}

.repo-search {
  width: min(360px, 35%);
}

.filter-bar .el-select {
  width: 150px;
}

.review-table {
  --el-table-border-color: #edf0f4;
  --el-table-header-bg-color: #fafbfc;
  --el-table-row-hover-bg-color: #fafaff;
  color: var(--app-text);
}

.mobile-review-list {
  display: none;
}

.review-table :deep(th.el-table__cell) {
  height: 44px;
  color: #7d8698;
  font-size: 12px;
  font-weight: 700;
}

.review-table :deep(td.el-table__cell) {
  height: 70px;
}

.review-table :deep(.review-row) {
  cursor: pointer;
}

.repo-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.repo-cell strong {
  display: block;
  max-width: 230px;
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.provider-mark {
  display: grid;
  width: 34px;
  height: 34px;
  flex: 0 0 34px;
  place-items: center;
  border: 1px solid #e1e4f5;
  border-radius: 9px;
  color: #5656c5;
  background: #f4f4ff;
  font-size: 10px;
  font-weight: 800;
}

.review-id {
  margin-top: 3px;
  color: #9aa2b1;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 10px;
}

.author-cell {
  display: flex;
  align-items: center;
  gap: 8px;
}

.author-avatar {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  flex-shrink: 0;
  object-fit: cover;
}

.author-avatar-fallback {
  display: grid;
  width: 24px;
  height: 24px;
  flex-shrink: 0;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(135deg, #7777eb, #5252c7);
  font-size: 11px;
  font-weight: 700;
}

.author-link {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-decoration: none;
  text-overflow: ellipsis;
  white-space: nowrap;
  transition: color 160ms ease;
}

.author-link:hover {
  color: var(--app-primary);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #687284;
  font-size: 12px;
  font-weight: 600;
}

.status-pill i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #aab1be;
}

.status-pill.success i {
  background: var(--app-success);
}

.status-pill.running i {
  background: #3984cf;
  box-shadow: 0 0 0 4px rgba(57, 132, 207, 0.12);
}

.status-pill.failed i {
  background: var(--app-danger);
}

.gate-result {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
}

.gate-result.passed {
  color: var(--app-success);
}

.gate-result.failed {
  color: var(--app-danger);
}

.gate-result.pending {
  color: #8e96a5;
}

.score-cell {
  display: grid;
  grid-template-columns: 28px 1fr;
  align-items: center;
  gap: 8px;
}

.score-cell strong {
  font-size: 13px;
}

.muted-value {
  color: #a0a7b5;
  font-size: 12px;
}

.finding-count {
  display: inline-grid;
  min-width: 26px;
  height: 26px;
  place-items: center;
  border-radius: 8px;
  color: #7f8796;
  background: #f2f4f7;
  font-size: 12px;
  font-weight: 700;
}

.finding-count.active {
  color: #c77910;
  background: #fff3df;
}

.time-cell strong,
.time-cell span {
  display: block;
}

.time-cell strong {
  color: #5d6677;
  font-size: 12px;
  font-weight: 600;
}

.time-cell span {
  margin-top: 2px;
  color: #a0a7b5;
  font-size: 11px;
}

.row-arrow {
  color: #aeb5c1;
}

.row-actions {
  display: flex;
  justify-content: flex-end;
  gap: 4px;
}

.mobile-actions {
  display: flex;
  gap: 6px;
  margin-top: 6px;
}

.pagination-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 66px;
  padding: 0 22px;
  color: #929aab;
  font-size: 12px;
}

@media (max-width: 1100px) {
  .reviews-page {
    padding-inline: 24px;
  }

  .filter-bar {
    flex-wrap: wrap;
  }

  .repo-search {
    width: 100%;
  }
}

@media (max-width: 720px) {
  .reviews-page {
    padding: 22px 14px 36px;
  }

  .page-header {
    align-items: center;
    margin-bottom: 20px;
  }

  .page-header h1 {
    font-size: 25px;
  }

  .page-header p,
  .eyebrow {
    display: none;
  }

  .page-header .el-button {
    min-height: 40px;
  }

  .stat-card {
    height: 104px;
  }

  .stat-card :deep(.el-card__body) {
    padding: 14px;
  }

  .stat-icon {
    display: none;
  }

  .stat-item small {
    display: none;
  }

  .filter-bar .el-select {
    width: calc(50% - 5px);
  }

  .review-table {
    display: none;
  }

  .records-card {
    overflow: hidden;
  }

  .mobile-review-list {
    display: block;
  }

  .mobile-review-card {
    display: flex;
    width: 100%;
    align-items: flex-start;
    gap: 11px;
    padding: 16px 14px;
    border: 0;
    border-bottom: 1px solid var(--app-border);
    color: inherit;
    text-align: left;
    background: #fff;
  }

  .mobile-review-main {
    min-width: 0;
    flex: 1;
  }

  .mobile-review-main > strong,
  .mobile-review-main > small {
    display: block;
  }

  .mobile-review-main > strong {
    overflow: hidden;
    color: var(--app-text);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-review-main > small {
    margin-top: 3px;
    overflow: hidden;
    color: #9aa2b1;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 9px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-review-meta {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 9px;
  }

  .mobile-review-side {
    display: grid;
    min-width: 54px;
    justify-items: end;
    color: #9ba3b1;
  }

  .mobile-review-side > strong {
    color: var(--app-text);
    font-size: 17px;
  }

  .mobile-review-side > strong small {
    margin-left: 1px;
    font-size: 10px;
  }

  .mobile-review-side > small {
    margin-top: 2px;
    font-size: 9px;
  }

  .mobile-review-side .el-icon {
    margin-top: 9px;
  }

  .mobile-empty {
    padding: 44px 16px;
    color: #9aa2b1;
    text-align: center;
    font-size: 13px;
  }

  .pagination-wrap {
    justify-content: flex-end;
  }

  .pagination-wrap > span {
    display: none;
  }
}
</style>
