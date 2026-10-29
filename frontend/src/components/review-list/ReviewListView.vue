<template>
  <AppLayout>
    <PageShell>
      <PageHeader :eyebrow="eyebrow" :title="title" :description="description">
        <template #actions>
          <el-button type="primary" size="large" @click="showCreateDialog = true">
            <el-icon><Plus /></el-icon>
            发起审查
          </el-button>
        </template>
      </PageHeader>

      <StatStrip :items="reviewStats" />

      <el-card class="records-card">
        <div class="records-toolbar">
          <div>
            <h2>{{ title }}</h2>
            <p>点击记录查看完整报告，运行中任务会自动刷新。</p>
          </div>
          <el-button circle plain aria-label="刷新列表" @click="refresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div v-if="showSavedViews" class="saved-views">
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
            placeholder="搜索仓库名（按 owner/repo 模糊匹配）"
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
          <el-select v-if="!pinnedEngine" v-model="filters.engine" placeholder="全部引擎" clearable @change="applyFilters">
            <el-option label="OCR 引擎" value="ocr" />
            <el-option label="Argus 引擎" value="argus" />
          </el-select>
          <el-select v-model="filters.gate_status" placeholder="全部门禁" clearable @change="applyFilters">
            <el-option label="门禁通过" value="passed" />
            <el-option label="门禁失败" value="failed" />
            <el-option label="待评估" value="pending" />
          </el-select>
          <el-button type="primary" plain @click="applyFilters">筛选</el-button>
          <el-button v-if="hasFilters" link @click="resetFilters">重置</el-button>
        </div>

        <ReviewBulkBar
          :selected-count="selectedRows.length"
          :cancellable-count="cancellableCount"
          :rerunnable-count="rerunnableCount"
          :busy="bulkAction"
          @cancel="handleBulkCancel"
          @rerun="handleBulkRerun"
        />

        <ReviewTable
          :items="reviews"
          :loading="loading"
          :acting-id="actingId"
          :pinned-engine="pinnedEngine"
          @row-click="goToDetail"
          @selection-change="handleSelectionChange"
          @sort-change="onSortChange"
          @cancel="handleCancel"
          @rerun="handleRerun"
          @requeue="handleRequeue"
        />

        <ReviewMobileList
          :items="reviews"
          :loading="loading"
          :acting-id="actingId"
          @row-click="goToDetail"
          @cancel="handleCancel"
          @rerun="handleRerun"
          @requeue="handleRequeue"
        />

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

      <!-- 发起审查对话框：调用页锁定引擎时以该引擎为默认 -->
      <ReviewCreateDialog
        v-model="showCreateDialog"
        :default-engine="pinnedEngine === 'ocr' ? 'ocr' : ''"
        @created="refresh"
        @goto="(id) => router.push(`/reviews/${id}`)"
      />
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
// 审查列表共享视图：审查记录（全部引擎）与 OCR 审查（锁定 ocr）两个页面的
// 单一实现。引擎锁定、保存视图等差异通过 props 表达；页面组件只传配置。
// 行级/批量操作在 useReviewActions，表格与移动卡片是纯展示组件。
import { ref, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Aim,
  CircleCloseFilled,
  Coin,
  Files,
  Loading,
  Plus,
  Refresh,
  Search,
  Stopwatch,
  WarningFilled,
} from '@element-plus/icons-vue'
import { formatTokens } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import ReviewTable from '@/components/review-list/ReviewTable.vue'
import ReviewMobileList from '@/components/review-list/ReviewMobileList.vue'
import ReviewBulkBar from '@/components/review-list/ReviewBulkBar.vue'
import ReviewCreateDialog from '@/components/ReviewCreateDialog.vue'
import { getReviews } from '@/api/review'
import { PROVIDER_OPTIONS } from '@/utils/url-parser'
import type { Review, ListStats } from '@/types/review'
import { useListPage } from '@/composables/useListPage'
import { useReviewActions } from '@/composables/useReviewActions'

const props = withDefaults(
  defineProps<{
    /** 页头 eyebrow/title/description 三件套 */
    eyebrow: string
    title: string
    description: string
    /** 锁定审查引擎（OCR 审查页 = 'ocr'）；留空 = 全部引擎可筛选 */
    pinnedEngine?: string
    /** 是否显示保存视图快捷筛选（跨引擎语义，锁定引擎的页面隐藏） */
    showSavedViews?: boolean
  }>(),
  { pinnedEngine: '', showSavedViews: true },
)

const router = useRouter()
const route = useRoute()
const showCreateDialog = ref(false)

const initialFilters = {
  repo: typeof route.query.repo === 'string' ? route.query.repo : '',
  provider: typeof route.query.provider === 'string' ? route.query.provider : '',
  status: typeof route.query.status === 'string' ? route.query.status : '',
  gate_status: typeof route.query.gate_status === 'string' ? route.query.gate_status : '',
  // 锁定引擎优先于查询参数（OCR 审查页固定 engine=ocr）。
  engine: props.pinnedEngine || (typeof route.query.engine === 'string' ? route.query.engine : ''),
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
  applySort,
  clearSort,
} = useListPage<Review, { repo: string; provider: string; status: string; gate_status: string; engine: string }>({
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

const {
  actingId,
  selectedRows,
  bulkAction,
  cancellableCount,
  rerunnableCount,
  handleCancel,
  handleRerun,
  handleRequeue,
  handleSelectionChange,
  handleBulkCancel,
  handleBulkRerun,
} = useReviewActions(refreshBackground)

// 服务端排序：el-table sort-change 的 order 为 ascending/descending/null，null 表示取消排序。
function onSortChange({ prop, order }: { prop: string | null; order: 'ascending' | 'descending' | null }) {
  if (!prop || !order) {
    clearSort()
    return
  }
  applySort(prop, order === 'ascending' ? 'asc' : 'desc')
}

const stats = computed<ListStats>(() => (extraData.value as { stats?: ListStats }).stats ?? { total: 0, running: 0, gate_failed: 0, findings: 0, total_tokens: 0 })

const reviewStats = computed(() => [
  { icon: Files, label: '审查总数', value: stats.value.total, tone: 'primary' as const },
  { icon: Loading, label: '正在执行', value: stats.value.running, tone: 'info' as const },
  { icon: WarningFilled, label: '门禁未通过', value: stats.value.gate_failed, tone: 'danger' as const },
  { icon: Aim, label: '发现问题', value: stats.value.findings, tone: 'warning' as const },
  { icon: Coin, label: '累计 Tokens', value: formatTokens(stats.value.total_tokens), title: stats.value.total_tokens ? stats.value.total_tokens.toLocaleString() : '0', tone: 'success' as const },
])

const activeSavedView = computed<SavedViewKey | ''>(() => {
  // 保存视图是跨引擎语义：带引擎过滤（或锁定引擎）时不匹配任何视图。
  if (filters.engine) return ''
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
  // 引擎过滤独立于保存视图（锁定引擎的页面不可清除）。
  if (!props.pinnedEngine) filters.engine = ''
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

function goToDetail(row: Review) {
  router.push(`/reviews/${row.review_id}`)
}
</script>

<style scoped>
.records-card {
  overflow: hidden;
  border-radius: var(--card-radius-lg);
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
  .filter-bar {
    flex-wrap: wrap;
  }

  .repo-search {
    width: 100%;
  }
}

@media (max-width: 720px) {
  .filter-bar .el-select {
    width: calc(50% - 5px);
  }

  .records-card {
    overflow: hidden;
  }

  .pagination-wrap {
    justify-content: flex-end;
  }

  .pagination-wrap > span {
    display: none;
  }
}
</style>
