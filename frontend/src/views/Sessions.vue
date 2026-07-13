<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="REVIEW SESSIONS"
        title="审查会话"
        description="跨所有评审的会话总览，追踪多阶段审查的执行进度与状态。"
      >
        <template #actions>
          <el-button circle plain aria-label="刷新列表" @click="refresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </template>
      </PageHeader>

      <StatStrip :items="sessionStats" />

      <el-card class="records-card">
        <div class="records-toolbar">
          <div>
            <h2>会话记录</h2>
            <p>点击记录查看会话详情。</p>
          </div>
        </div>

        <div class="filter-bar">
          <el-select v-model="filters.status" placeholder="全部状态" clearable @change="applyFilters">
            <el-option label="执行中" value="active" />
            <el-option label="已完成" value="completed" />
            <el-option label="执行失败" value="failed" />
            <el-option label="已取消" value="cancelled" />
          </el-select>
          <el-select v-model="filters.phase" placeholder="全部阶段" clearable @change="applyFilters">
            <el-option label="上下文收集" value="context" />
            <el-option label="变更上下文" value="change_context" />
            <el-option label="分析" value="analysis" />
            <el-option label="审查" value="reviews" />
            <el-option label="聚合" value="aggregation" />
            <el-option label="讨论" value="discourse" />
            <el-option label="综合" value="synthesis" />
            <el-option label="完成" value="complete" />
          </el-select>
          <el-button v-if="hasFilters" link @click="resetFilters">重置</el-button>
        </div>

        <el-table
          :data="sessions"
          v-loading="loading"
          class="session-table desktop-table"
          row-class-name="session-row"
          empty-text="暂无符合条件的会话记录"
          @row-click="goToDetail"
        >
          <el-table-column label="会话" min-width="200">
            <template #default="{ row }">
              <div class="session-cell">
                <strong>{{ shortId(row.session_id) }}</strong>
                <div class="session-sub">第 {{ row.round }} 轮</div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="所属评审" min-width="200">
            <template #default="{ row }">
              <router-link :to="`/reviews/${row.review_id}`" class="review-link" @click.stop>
                {{ shortId(row.review_id) }}
              </router-link>
            </template>
          </el-table-column>
          <el-table-column label="阶段" width="140">
            <template #default="{ row }">
              <span class="phase-pill">{{ getPhaseLabel(row.phase) }}</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="120">
            <template #default="{ row }">
              <span class="status-pill" :class="row.status">
                <i />
                {{ getSessionStatusLabel(row.status) }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="进度" width="160">
            <template #default="{ row }">
              <el-progress
                :percentage="Math.round(row.progress)"
                :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : undefined"
                :stroke-width="6"
              />
            </template>
          </el-table-column>
          <el-table-column label="更新时间" width="160">
            <template #default="{ row }">
              <div class="time-cell">
                <strong>{{ formatDateShort(row.updated_at) }}</strong>
                <span>{{ formatClock(row.updated_at) }}</span>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <div v-loading="loading" class="mobile-session-list">
          <button
            v-for="row in sessions"
            :key="row.session_id"
            class="mobile-session-card"
            type="button"
            @click="goToDetail(row)"
          >
            <span class="mobile-session-main">
              <strong>{{ shortId(row.session_id) }}</strong>
              <small>评审 {{ shortId(row.review_id) }} · 第 {{ row.round }} 轮</small>
              <span class="mobile-session-meta">
                <span class="phase-pill">{{ getPhaseLabel(row.phase) }}</span>
                <span class="status-pill" :class="row.status"><i />{{ getSessionStatusLabel(row.status) }}</span>
              </span>
              <el-progress
                :percentage="Math.round(row.progress)"
                :status="row.status === 'failed' ? 'exception' : row.status === 'completed' ? 'success' : undefined"
                :stroke-width="5"
              />
            </span>
            <el-icon><ArrowRight /></el-icon>
          </button>
          <div v-if="!loading && sessions.length === 0" class="mobile-empty">
            暂无符合条件的会话记录
          </div>
        </div>

        <div class="pagination-wrap">
          <span>共 {{ pagination.total }} 条记录</span>
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :total="pagination.total"
            :page-sizes="[20, 50, 100]"
            layout="sizes, prev, pager, next"
            @current-change="handlePageChange"
            @size-change="handlePageSizeChange"
          />
        </div>
      </el-card>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowRight,
  CircleCheckFilled,
  Files,
  Loading,
  Refresh,
  WarningFilled,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import { listAllSessions } from '@/api/session'
import { getSessionStatusLabel, getPhaseLabel } from '@/utils/status'
import { formatDateShort, formatClock, shortId } from '@/utils/format'
import type { ReviewSession } from '@/types/session'
import { useListPage } from '@/composables/useListPage'

const router = useRouter()

const {
  loading,
  items: sessions,
  pagination,
  filters,
  hasFilters,
  handlePageChange,
  handlePageSizeChange,
  applyFilters,
  resetFilters,
  refresh,
} = useListPage<ReviewSession, { status: string; phase: string }>({
  fetchFn: async (params) => {
    const result = await listAllSessions({
      page: params.page,
      page_size: params.page_size,
      status: params.status || undefined,
      phase: params.phase || undefined,
    })
    return { items: result.sessions, total: result.total }
  },
  defaultFilters: { status: '', phase: '' },
  shouldPoll: (result) => {
    const sessions = (result as { items?: ReviewSession[] }).items
    return Array.isArray(sessions) && sessions.some((s) => s.status === 'active')
  },
  pollInterval: 10000,
})

/** 从当前页数据派生状态计数（列表已分页，这是当前页的快照）。 */
const statusCounts = computed(() => {
  const counts = { active: 0, completed: 0, failed: 0 }
  for (const s of sessions.value) {
    if (s.status === 'active') counts.active++
    else if (s.status === 'completed') counts.completed++
    else if (s.status === 'failed') counts.failed++
  }
  return counts
})

const sessionStats = computed(() => [
  { icon: Files, label: '会话总数', value: pagination.total, tone: 'primary' as const },
  { icon: Loading, label: '执行中', value: statusCounts.value.active, tone: 'warning' as const },
  { icon: CircleCheckFilled, label: '已完成', value: statusCounts.value.completed, tone: 'success' as const },
  { icon: WarningFilled, label: '失败', value: statusCounts.value.failed, tone: 'danger' as const },
])

function goToDetail(row: ReviewSession) {
  router.push(`/reviews/${row.review_id}/sessions/${row.session_id}`)
}
</script>

<style scoped>
.records-card { border-radius: var(--card-radius-lg); }
.records-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
}
.records-toolbar h2 { font-size: 18px; font-weight: 600; margin: 0 0 4px; color: var(--app-text); }
.records-toolbar p { font-size: 13px; color: var(--app-muted); margin: 0; }

.filter-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}
.filter-bar .el-select { width: 160px; }

.session-table :deep(.session-row) { cursor: pointer; }
.session-cell strong { font-size: 14px; color: var(--app-text); }
.session-sub { font-size: 12px; color: var(--app-muted); margin-top: 2px; }
.review-link { color: var(--app-primary); font-weight: 500; text-decoration: none; }
.review-link:hover { text-decoration: underline; }

.phase-pill {
  display: inline-block;
  padding: 2px 10px;
  border-radius: var(--card-radius);
  font-size: 12px;
  font-weight: 500;
  background: rgba(37, 99, 235, 0.08);
  color: var(--app-primary);
}

.status-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 10px;
  border-radius: var(--card-radius);
  font-size: 12px;
  font-weight: 500;
}
.status-pill i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.status-pill.active { background: rgba(217, 134, 22, 0.12); color: var(--app-warning); }
.status-pill.completed { background: rgba(21, 149, 112, 0.12); color: var(--app-success); }
.status-pill.failed { background: rgba(216, 75, 91, 0.12); color: var(--app-danger); }
.status-pill.cancelled { background: rgba(105, 115, 134, 0.12); color: var(--app-muted); }

.time-cell { display: flex; flex-direction: column; }
.time-cell strong { font-size: 13px; color: var(--app-text); }
.time-cell span { font-size: 12px; color: var(--app-muted); }

.pagination-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-top: 16px;
}
.pagination-wrap > span { font-size: 13px; color: var(--app-muted); }

.mobile-session-list { display: none; }
.mobile-session-card {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px 16px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  margin-bottom: 10px;
  text-align: left;
  cursor: pointer;
}
.mobile-session-main { flex: 1; display: flex; flex-direction: column; gap: 4px; }
.mobile-session-main strong { font-size: 14px; color: var(--app-text); }
.mobile-session-main small { font-size: 12px; color: var(--app-muted); }
.mobile-session-meta { display: flex; gap: 8px; flex-wrap: wrap; }
.mobile-empty { text-align: center; padding: 32px; color: var(--app-muted); font-size: 14px; }

@media (max-width: 720px) {
  .desktop-table { display: none; }
  .mobile-session-list { display: block; }
}
</style>
