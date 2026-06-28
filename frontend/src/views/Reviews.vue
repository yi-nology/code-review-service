<template>
  <AppLayout>
    <div class="reviews-page">
      <header class="page-header">
        <div>
          <h1>代码审查</h1>
          <p>跟踪审查队列、质量门禁和风险项处理状态。</p>
        </div>
        <el-button type="primary" size="large" @click="openCreateDialog">
          <el-icon><Plus /></el-icon>
          发起审查
        </el-button>
      </header>

      <section class="metric-strip" aria-label="审查概览">
        <div class="metric-item">
          <span class="metric-icon total"><el-icon><Files /></el-icon></span>
          <span class="metric-label">审查总数</span>
          <strong>{{ stats.total }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon running"><el-icon><Loading /></el-icon></span>
          <span class="metric-label">正在执行</span>
          <strong>{{ stats.running }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon failed"><el-icon><WarningFilled /></el-icon></span>
          <span class="metric-label">门禁未通过</span>
          <strong>{{ stats.gate_failed }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon findings"><el-icon><Aim /></el-icon></span>
          <span class="metric-label">发现问题</span>
          <strong>{{ stats.findings }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon tokens"><el-icon><Coin /></el-icon></span>
          <span class="metric-label">累计 Tokens</span>
          <strong>{{ stats.total_tokens ? stats.total_tokens.toLocaleString() : 0 }}</strong>
        </div>
      </section>

      <el-card class="records-card">
        <div class="records-toolbar">
          <div>
            <h2>审查记录</h2>
            <p>点击记录查看完整报告，运行中任务会自动刷新。</p>
          </div>
          <el-button circle plain aria-label="刷新列表" @click="refresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div class="saved-views">
          <button
            v-for="view in savedViews"
            :key="view.key"
            type="button"
            class="saved-view"
            :class="{ active: activeSavedView === view.key }"
            @click="applySavedView(view.key)"
          >
            <el-icon><component :is="view.icon" /></el-icon>
            <span>{{ view.label }}</span>
          </button>
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
          <el-select v-model="filters.gate_status" placeholder="全部门禁" clearable @change="applyFilters">
              <el-option label="门禁通过" value="passed" />
              <el-option label="门禁失败" value="failed" />
              <el-option label="待评估" value="pending" />
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
            @current-change="handlePageChange"
            @size-change="handlePageSizeChange"
          />
        </div>
      </el-card>

      <!-- 发起审查对话框 -->
      <ReviewCreateDialog
        v-model="showCreateDialog"
        @created="handleReviewCreated"
        @goto="(id) => router.push(`/reviews/${id}`)"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Aim,
  ArrowRight,
  CircleCheckFilled,
  CircleCloseFilled,
  Clock,
  Coin,
  Files,
  Loading,
  Plus,
  Refresh,
  Search,
  Stopwatch,
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
import { useListPage } from '@/composables/useListPage'

const router = useRouter()
const route = useRoute()
const showCreateDialog = ref(false)
const actingId = ref('')
const initialFilters = {
  repo: typeof route.query.repo === 'string' ? route.query.repo : '',
  provider: typeof route.query.provider === 'string' ? route.query.provider : '',
  status: typeof route.query.status === 'string' ? route.query.status : '',
  gate_status: typeof route.query.gate_status === 'string' ? route.query.gate_status : '',
}

type SavedViewKey = 'all' | 'running' | 'gate_failed' | 'failed' | 'high_risk'

const savedViews: Array<{ key: SavedViewKey; label: string; icon: any }> = [
  { key: 'all', label: '全部记录', icon: Files },
  { key: 'running', label: '运行中', icon: Stopwatch },
  { key: 'gate_failed', label: '门禁失败', icon: WarningFilled },
  { key: 'failed', label: '执行失败', icon: CircleCloseFilled },
  { key: 'high_risk', label: '高风险队列', icon: Aim },
]

const {
  loading,
  items: reviews,
  extraData,
  pagination,
  filters,
  hasFilters,
  handlePageChange,
  handlePageSizeChange,
  applyFilters,
  resetFilters,
  refresh,
  refreshBackground,
} = useListPage<Review, { repo: string; provider: string; status: string; gate_status: string }>({
  fetchFn: async (params) => {
    const result = await getReviews(params)
    return { items: result.items, total: result.total, stats: result.stats }
  },
  defaultFilters: initialFilters,
  shouldPoll: (result) => {
    const stats = (result as { stats?: ListStats }).stats
    return (stats?.running ?? 0) > 0
  },
  pollInterval: 10000,
})

const stats = computed<ListStats>(() => (extraData.value as { stats?: ListStats }).stats ?? { total: 0, running: 0, gate_failed: 0, findings: 0, total_tokens: 0 })

const activeSavedView = computed<SavedViewKey | ''>(() => {
  if (!filters.repo && !filters.provider && !filters.status && !filters.gate_status) return 'all'
  if (!filters.repo && !filters.provider && filters.status === 'running' && !filters.gate_status) return 'running'
  if (!filters.repo && !filters.provider && !filters.status && filters.gate_status === 'failed') return 'gate_failed'
  if (!filters.repo && !filters.provider && filters.status === 'failed' && !filters.gate_status) return 'failed'
  if (!filters.repo && !filters.provider && filters.status === 'success' && filters.gate_status === 'failed') return 'high_risk'
  return ''
})

function applySavedView(key: SavedViewKey) {
  filters.repo = ''
  filters.provider = ''
  filters.status = ''
  filters.gate_status = ''
  if (key === 'running') {
    filters.status = 'running'
  } else if (key === 'gate_failed') {
    filters.gate_status = 'failed'
  } else if (key === 'failed') {
    filters.status = 'failed'
  } else if (key === 'high_risk') {
    filters.status = 'success'
    filters.gate_status = 'failed'
  }
  applyFilters()
}

function openCreateDialog() {
  showCreateDialog.value = true
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
    refreshBackground()
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
    refreshBackground()
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

function handleReviewCreated() {
  refresh()
}
</script>

<style scoped>
.reviews-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 28px 32px 44px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 24px;
  margin-bottom: 18px;
}

.page-header h1 {
  margin: 0 0 5px;
  color: var(--app-text);
  font-size: 26px;
  letter-spacing: 0;
}

.page-header p {
  margin: 0;
  color: var(--app-muted);
  font-size: 14px;
}

.page-header .el-button {
  min-height: 40px;
  padding-inline: 16px;
  border-radius: 8px;
  box-shadow: none;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  margin-bottom: 16px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.metric-item {
  display: grid;
  min-width: 0;
  grid-template-columns: 34px 1fr;
  grid-template-rows: auto auto;
  column-gap: 10px;
  padding: 16px 18px;
  border-right: 1px solid var(--app-border);
}

.metric-item:last-child {
  border-right: 0;
}

.metric-icon {
  display: grid;
  width: 34px;
  height: 34px;
  grid-row: 1 / span 2;
  place-items: center;
  border-radius: 8px;
  font-size: 17px;
}

.metric-icon.total {
  color: #2563eb;
  background: #eff6ff;
}

.metric-icon.running {
  color: #0891b2;
  background: #ecfeff;
}

.metric-icon.failed {
  color: #dc2626;
  background: #fef2f2;
}

.metric-icon.findings {
  color: #d97706;
  background: #fffbeb;
}

.metric-icon.tokens {
  color: #059669;
  background: #ecfdf5;
}

.metric-label {
  min-width: 0;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}

.metric-item strong {
  overflow: hidden;
  color: var(--app-text);
  font-size: 22px;
  line-height: 1.25;
  text-overflow: ellipsis;
  white-space: nowrap;
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
  padding: 16px 18px 14px;
  border-bottom: 1px solid var(--app-border);
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

.saved-views {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding: 12px 18px 0;
  background: #fafbfc;
}

.saved-view {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  min-height: 34px;
  flex: 0 0 auto;
  padding: 0 12px;
  border: 1px solid #dce3ef;
  border-radius: 8px;
  color: #4b5563;
  background: #fff;
  cursor: pointer;
  font-size: 13px;
  font-weight: 600;
}

.saved-view:hover {
  border-color: #93c5fd;
  color: #1d4ed8;
}

.saved-view.active {
  border-color: #2563eb;
  color: #fff;
  background: #2563eb;
}

.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 18px;
  border-bottom: 1px solid var(--app-border);
  background: #fafbfc;
}

.repo-search {
  width: min(420px, 40%);
}

.filter-bar .el-select {
  width: 148px;
}

.review-table {
  --el-table-border-color: #edf0f4;
  --el-table-header-bg-color: #fafbfc;
  --el-table-row-hover-bg-color: #f8fafc;
  color: var(--app-text);
}

.mobile-review-list {
  display: none;
}

.review-table :deep(th.el-table__cell) {
  height: 42px;
  color: #7d8698;
  font-size: 12px;
  font-weight: 700;
}

.review-table :deep(td.el-table__cell) {
  height: 62px;
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
  background: #2563eb;
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

  .metric-strip {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .metric-item {
    border-bottom: 1px solid var(--app-border);
  }

  .metric-item:nth-child(3n) {
    border-right: 0;
  }

  .metric-item:nth-last-child(-n + 2) {
    border-bottom: 0;
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

  .page-header p {
    display: none;
  }

  .page-header .el-button {
    min-height: 40px;
  }

  .metric-strip {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .metric-item {
    grid-template-columns: 1fr;
    padding: 12px;
  }

  .metric-item:nth-child(3n) {
    border-right: 1px solid var(--app-border);
  }

  .metric-item:nth-child(2n) {
    border-right: 0;
  }

  .metric-item:nth-last-child(-n + 2) {
    border-bottom: 1px solid var(--app-border);
  }

  .metric-item:last-child {
    border-bottom: 0;
  }

  .metric-icon {
    display: none;
  }

  .metric-item strong {
    font-size: 20px;
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
