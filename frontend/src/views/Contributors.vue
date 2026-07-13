<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="DEVELOPER QUALITY"
        title="开发者质量"
        description="基于 AI 代码审查结果度量每位开发者的代码质量表现。"
      />

      <StatStrip :items="contributorStats" />

      <el-card class="leaderboard-card">
        <div class="toolbar">
          <div>
            <h2>质量排行榜</h2>
            <p>点击查看开发者质量详情</p>
          </div>
          <el-button circle plain aria-label="刷新" @click="refresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div class="filter-bar">
          <el-select v-model="filters.provider" placeholder="全部平台" clearable @change="applyFilters">
            <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <el-input v-model="filters.repo" placeholder="仓库筛选" clearable @keyup.enter="applyFilters" @clear="applyFilters" style="width: 200px" />
          <el-select v-model="filters.sort" placeholder="排序方式" @change="applyFilters" style="width: 140px">
            <el-option label="平均分" value="avg_score" />
            <el-option label="审查数" value="review_count" />
            <el-option label="通过率" value="pass_rate" />
            <el-option label="问题数" value="total_findings" />
          </el-select>
          <el-button type="primary" plain @click="applyFilters">筛选</el-button>
          <el-button v-if="hasFilters" link @click="resetFilters">重置</el-button>
        </div>

        <el-table
          :data="contributors"
          v-loading="loading"
          class="contributor-table"
          empty-text="暂无开发者数据（新审查记录将自动包含作者信息）"
          @row-click="goToDetail"
        >
          <el-table-column label="排名" width="64" align="center">
            <template #default="{ $index }">
              <span class="rank-badge" :class="rankClass($index)">{{ $index + 1 + (pagination.page - 1) * pagination.pageSize }}</span>
            </template>
          </el-table-column>
          <el-table-column label="开发者" min-width="200">
            <template #default="{ row }">
              <div class="dev-cell">
                <img v-if="row.author_avatar" :src="row.author_avatar" class="avatar" :alt="row.author" />
                <span v-else class="avatar-fallback">{{ getInitials(row.author_name || row.author) }}</span>
                <div class="dev-info">
                  <strong>{{ row.author_name || row.author }}</strong>
                  <small>@{{ row.author }}</small>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="PR 数" width="80" align="center">
            <template #default="{ row }">
              <span class="num-cell">{{ row.review_count }}</span>
            </template>
          </el-table-column>
          <el-table-column label="平均分" width="150">
            <template #default="{ row }">
              <div class="score-cell">
                <strong :class="getScoreClass(row.avg_score)">{{ row.avg_score }}</strong>
                <el-progress :percentage="row.avg_score" :show-text="false" :stroke-width="5" :color="getScoreColor(row.avg_score)" />
              </div>
            </template>
          </el-table-column>
          <el-table-column label="通过率" width="110">
            <template #default="{ row }">
              <span class="pass-rate" :class="{ high: row.pass_rate >= 80, mid: row.pass_rate >= 50 && row.pass_rate < 80, low: row.pass_rate < 50 }">
                {{ row.pass_rate.toFixed(0) }}%
              </span>
            </template>
          </el-table-column>
          <el-table-column label="问题" width="90" align="center">
            <template #default="{ row }">
              <span class="finding-count" :class="{ active: row.total_findings > 0 }">{{ row.total_findings }}</span>
            </template>
          </el-table-column>
          <el-table-column label="严重问题" width="100" align="center">
            <template #default="{ row }">
              <span v-if="row.critical_count > 0" class="severity-tag critical">{{ row.critical_count }}</span>
              <span v-else class="muted">0</span>
            </template>
          </el-table-column>
          <el-table-column label="最后活跃" width="130">
            <template #default="{ row }">
              <span class="time-text">{{ formatDateRelative(row.last_review_at) }}</span>
            </template>
          </el-table-column>
          <el-table-column width="48" align="right">
            <template #default>
              <el-icon class="row-arrow"><ArrowRight /></el-icon>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrap">
          <span>共 {{ pagination.total }} 位开发者</span>
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
  Refresh,
  TrendCharts,
  User,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import { getContributors } from '@/api/stats'
import type { ContributorSummary } from '@/types/stats'
import { getInitials, formatDateRelative } from '@/utils/format'
import { getScoreColor } from '@/utils/colors'
import { PROVIDER_OPTIONS } from '@/utils/url-parser'
import { useListPage } from '@/composables/useListPage'

const router = useRouter()

const {
  loading,
  items: contributors,
  pagination,
  filters,
  hasFilters,
  handlePageChange,
  handlePageSizeChange,
  applyFilters,
  resetFilters,
  refresh,
} = useListPage<ContributorSummary, { provider: string; repo: string; sort: string }>({
  fetchFn: async (params) => {
    const result = await getContributors(params)
    return { items: result.contributors || [], total: result.total }
  },
  defaultFilters: { provider: '', repo: '', sort: 'avg_score' },
  defaultPageSize: 20,
})

const summaryStats = computed(() => {
  const list = contributors.value
  if (!list.length) return { totalContributors: 0, avgScore: '0', avgPassRate: 0, totalReviews: 0 }
  const totalReviews = list.reduce((s, c) => s + c.review_count, 0)
  const avgScore = list.reduce((s, c) => s + c.avg_score, 0) / list.length
  const avgPassRate = list.reduce((s, c) => s + c.pass_rate, 0) / list.length
  return {
    totalContributors: pagination.total,
    avgScore: avgScore.toFixed(1),
    avgPassRate: Math.round(avgPassRate),
    totalReviews,
  }
})

const contributorStats = computed(() => [
  { icon: User, label: '开发者总数', value: summaryStats.value.totalContributors, sublabel: '有审查记录的开发者', tone: 'primary' as const },
  { icon: TrendCharts, label: '团队平均分', value: summaryStats.value.avgScore, sublabel: '所有开发者平均', tone: 'success' as const },
  { icon: CircleCheckFilled, label: '平均通过率', value: `${summaryStats.value.avgPassRate}%`, sublabel: '质量门禁通过率', tone: 'info' as const },
  { icon: Files, label: '审查总数', value: summaryStats.value.totalReviews, sublabel: '所有 PR 审查记录', tone: 'warning' as const },
])

function goToDetail(row: ContributorSummary) {
  router.push(`/contributors/${encodeURIComponent(row.author)}`)
}

function rankClass(index: number) {
  if (index === 0) return 'gold'
  if (index === 1) return 'silver'
  if (index === 2) return 'bronze'
  return ''
}

function getScoreClass(score: number) {
  if (score >= 80) return 'score-high'
  if (score >= 60) return 'score-mid'
  return 'score-low'
}
</script>

<style scoped>
.leaderboard-card {
  overflow: hidden;
  border-radius: var(--card-radius-lg);
  box-shadow: var(--app-shadow);
}

.leaderboard-card :deep(.el-card__body) {
  padding: 0;
}

.toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 20px 22px 16px;
}

.toolbar h2 {
  margin: 0 0 4px;
  color: var(--app-text);
  font-size: 17px;
}

.toolbar p {
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

.contributor-table {
  --el-table-border-color: #edf0f4;
  --el-table-header-bg-color: #fafbfc;
  --el-table-row-hover-bg-color: #fafaff;
}

.contributor-table :deep(th.el-table__cell) {
  height: 44px;
  color: #7d8698;
  font-size: 12px;
  font-weight: 700;
}

.contributor-table :deep(td.el-table__cell) {
  height: 64px;
}

.contributor-table :deep(.el-table__row) {
  cursor: pointer;
}

.rank-badge {
  display: inline-grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border-radius: 8px;
  color: #7f8796;
  background: #f2f4f7;
  font-size: 13px;
  font-weight: 700;
}

.rank-badge.gold { color: #b8860b; background: #fef3d7; }
.rank-badge.silver { color: #7d8698; background: #e8edf3; }
.rank-badge.bronze { color: #b06a3a; background: #f5e0d0; }

.dev-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  flex: 0 0 36px;
  object-fit: cover;
}

.avatar-fallback {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border-radius: 50%;
  color: #fff;
  background: linear-gradient(135deg, #7777eb, #5252c7);
  font-size: 13px;
  font-weight: 700;
}

.dev-info strong {
  display: block;
  color: var(--app-text);
  font-size: 13px;
}

.dev-info small {
  display: block;
  margin-top: 2px;
  color: #9aa2b1;
  font-size: 11px;
}

.num-cell {
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
}

.score-cell {
  display: grid;
  grid-template-columns: 32px 1fr;
  align-items: center;
  gap: 8px;
}

.score-cell strong {
  font-size: 14px;
}

.score-high { color: #159570; }
.score-mid { color: #d98616; }
.score-low { color: #d84b5b; }

.pass-rate {
  font-size: 13px;
  font-weight: 600;
}

.pass-rate.high { color: #159570; }
.pass-rate.mid { color: #d98616; }
.pass-rate.low { color: #d84b5b; }

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

.severity-tag.critical {
  display: inline-grid;
  min-width: 26px;
  height: 26px;
  place-items: center;
  border-radius: 8px;
  color: #d84b5b;
  background: #fff0f2;
  font-size: 12px;
  font-weight: 700;
}

.muted {
  color: #c0c6d0;
  font-size: 13px;
}

.time-text {
  color: #8b93a3;
  font-size: 12px;
}

.row-arrow {
  color: #aeb5c1;
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
}

@media (max-width: 720px) {
  .filter-bar .el-select { width: 100%; }
  .contributor-table { display: none; }
  .pagination-wrap { justify-content: flex-end; }
  .pagination-wrap > span { display: none; }
}
</style>
