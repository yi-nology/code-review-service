<template>
  <AppLayout>
    <div class="contributors-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">DEVELOPER QUALITY</div>
          <h1>开发者质量</h1>
          <p>基于 AI 代码审查结果度量每位开发者的代码质量表现。</p>
        </div>
      </header>

      <el-row :gutter="16" class="stats-row">
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card total">
            <div class="stat-icon"><el-icon><User /></el-icon></div>
            <div class="stat-item">
              <span>开发者总数</span>
              <strong>{{ summaryStats.totalContributors }}</strong>
              <small>有审查记录的开发者</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card avg">
            <div class="stat-icon"><el-icon><TrendCharts /></el-icon></div>
            <div class="stat-item">
              <span>团队平均分</span>
              <strong>{{ summaryStats.avgScore }}</strong>
              <small>所有开发者平均</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card pass">
            <div class="stat-icon"><el-icon><CircleCheckFilled /></el-icon></div>
            <div class="stat-item">
              <span>平均通过率</span>
              <strong>{{ summaryStats.avgPassRate }}%</strong>
              <small>质量门禁通过率</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card reviews">
            <div class="stat-icon"><el-icon><Files /></el-icon></div>
            <div class="stat-item">
              <span>审查总数</span>
              <strong>{{ summaryStats.totalReviews }}</strong>
              <small>所有 PR 审查记录</small>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <el-card class="leaderboard-card">
        <div class="toolbar">
          <div>
            <h2>质量排行榜</h2>
            <p>点击查看开发者质量详情</p>
          </div>
          <el-button circle plain aria-label="刷新" @click="fetchContributors">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div class="filter-bar">
          <el-select v-model="filters.provider" placeholder="全部平台" clearable @change="applyFilters">
            <el-option label="GitHub" value="github" />
            <el-option label="GitLab" value="gitlab" />
            <el-option label="Gitea" value="gitea" />
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
            @current-change="fetchContributors"
            @size-change="handlePageSizeChange"
          />
        </div>
      </el-card>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
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
import { getContributors } from '@/api/stats'
import type { ContributorSummary } from '@/types/stats'
import { getInitials, formatDateRelative } from '@/utils/format'
import { getScoreColor } from '@/utils/colors'

const router = useRouter()
const loading = ref(false)
const contributors = ref<ContributorSummary[]>([])
const filters = reactive({ provider: '', repo: '', sort: 'avg_score' })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const hasFilters = computed(() => !!(filters.provider || filters.repo))

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

async function fetchContributors() {
  loading.value = true
  try {
    const result = await getContributors({
      ...filters,
      page: pagination.page,
      page_size: pagination.pageSize,
    })
    contributors.value = result.contributors || []
    pagination.total = result.total
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  pagination.page = 1
  fetchContributors()
}

function resetFilters() {
  filters.provider = ''
  filters.repo = ''
  filters.sort = 'avg_score'
  applyFilters()
}

function handlePageSizeChange() {
  pagination.page = 1
  fetchContributors()
}

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

onMounted(fetchContributors)
</script>

<style scoped>
.contributors-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
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

.stats-row {
  margin-bottom: 20px;
}

.stats-row .el-col {
  margin-bottom: 16px;
}

.stat-card {
  height: 110px;
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

.total .stat-icon { color: #5b5bd6; background: #eeeeff; }
.avg .stat-icon { color: #159570; background: #e6f7f0; }
.pass .stat-icon { color: #2878c7; background: #eaf4ff; }
.reviews .stat-icon { color: #d98616; background: #fff6e8; }

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

.leaderboard-card {
  overflow: hidden;
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
  .contributors-page {
    padding-inline: 24px;
  }
  .filter-bar {
    flex-wrap: wrap;
  }
}

@media (max-width: 720px) {
  .contributors-page {
    padding: 22px 14px 36px;
  }
  .page-header h1 {
    font-size: 25px;
  }
  .page-header p, .eyebrow {
    display: none;
  }
  .stat-card { height: 96px; }
  .stat-card :deep(.el-card__body) { padding: 14px; }
  .stat-icon { display: none; }
  .stat-item small { display: none; }
  .filter-bar .el-select { width: 100%; }
  .contributor-table { display: none; }
  .pagination-wrap { justify-content: flex-end; }
  .pagination-wrap > span { display: none; }
}
</style>
