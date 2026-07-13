<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="PROJECT REPORTS"
        title="项目报告"
        description="按项目汇总代码审查结果，追踪仓库质量、风险分布与门禁表现。"
      >
        <template #actions>
          <el-button :icon="Refresh" circle @click="fetchProjects" :loading="loading" />
        </template>
      </PageHeader>

      <StatStrip :items="reportStats" v-loading="metricsLoading" />

      <!-- ===== OCR 成本趋势（复用 TrendChart） ===== -->
      <section class="panel-shell trend-panel" v-loading="metricsLoading">
        <div class="panel-heading trend-head">
          <div>
            <strong>OCR 成本趋势</strong>
            <span>按天聚合 Token 消耗 · 鼠标悬停查看明细</span>
          </div>
          <div class="trend-meta">
            <span class="meta-pill">
              <i class="meta-dot" />
              {{ ocrTrend.length }} 天
            </span>
            <span class="meta-pill" v-if="ocrMetrics?.review_count">
              {{ ocrMetrics.review_count }} 次审查
            </span>
          </div>
        </div>
        <div class="trend-body">
          <TrendChart :data="trendChartData" :height="220" />
        </div>
      </section>

      <!-- ===== 项目质量报告 ===== -->
      <section class="panel-shell reports-panel">
        <div class="panel-heading reports-header">
          <div class="reports-title">
            <strong>项目质量报告</strong>
            <span>共 {{ pagination.total }} 个项目</span>
          </div>
        </div>

        <!-- 筛选条：单行紧凑布局 -->
        <div class="filter-bar">
          <el-input
            v-model="filters.repo"
            class="filter-search"
            placeholder="搜索仓库..."
            clearable
            @keyup.enter="applyFilters"
            @clear="applyFilters"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select
            v-model="filters.provider"
            placeholder="全部平台"
            clearable
            @change="applyFilters"
            class="filter-select"
          >
            <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <el-select v-model="filters.sort" placeholder="排序" @change="applyFilters" class="filter-select">
            <el-option label="按评分" value="avg_score" />
            <el-option label="按审查数" value="review_count" />
            <el-option label="按通过率" value="pass_rate" />
            <el-option label="按严重问题" value="critical_count" />
            <el-option label="按活跃度" value="last_review_at" />
          </el-select>
          <el-date-picker
            v-model="dateRange"
            class="filter-date"
            type="daterange"
            range-separator="至"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            unlink-panels
            @change="applyFilters"
          />
          <el-button v-if="hasFilters" text type="primary" @click="resetFilters" class="filter-reset">
            清除筛选
          </el-button>
        </div>

        <el-table
          :data="projects"
          v-loading="loading"
          class="project-table"
          empty-text="暂无项目报告数据"
          @row-click="goToDetail"
          :row-class-name="() => 'clickable-row'"
        >
          <!-- 项目 + 次要指标合并到副文本 -->
          <el-table-column label="项目" min-width="280">
            <template #default="{ row }">
              <div class="repo-cell">
                <span class="provider-badge" :class="row.provider">{{ getProviderInitial(row.provider) }}</span>
                <div class="repo-info">
                  <span class="repo-name">{{ row.repo }}</span>
                  <span class="repo-sub">
                    {{ providerLabel(row.provider) }}
                    <em class="dot-sep">·</em>
                    {{ row.review_count }} 次审查
                    <em class="dot-sep">·</em>
                    {{ formatDateRelative(row.last_review_at) }}
                  </span>
                </div>
              </div>
            </template>
          </el-table-column>

          <!-- 平均分（数值 + 进度条） -->
          <el-table-column label="平均分" width="150">
            <template #default="{ row }">
              <div class="score-cell">
                <span class="score-value" :class="scoreClass(row.avg_score)">{{ row.avg_score.toFixed(1) }}</span>
                <div class="score-bar">
                  <div
                    class="score-bar-fill"
                    :style="{ width: row.avg_score + '%', backgroundColor: getScoreColor(row.avg_score) }"
                  />
                </div>
              </div>
            </template>
          </el-table-column>

          <!-- 通过率 -->
          <el-table-column label="通过率" width="90" align="center">
            <template #default="{ row }">
              <span class="pass-rate-badge" :class="passRateClass(row.pass_rate)">
                {{ row.pass_rate.toFixed(0) }}%
              </span>
            </template>
          </el-table-column>

          <!-- 风险分布：堆叠条 + 数量 -->
          <el-table-column label="风险分布" min-width="200">
            <template #default="{ row }">
              <div class="risk-cell">
                <div
                  v-if="row.critical_count + row.major_count + row.minor_count + row.info_count > 0"
                  class="risk-bars"
                >
                  <div
                    v-if="row.critical_count > 0"
                    class="risk-segment critical"
                    :style="{ flex: row.critical_count }"
                    :title="'严重: ' + row.critical_count"
                  />
                  <div
                    v-if="row.major_count > 0"
                    class="risk-segment major"
                    :style="{ flex: row.major_count }"
                    :title="'高风险: ' + row.major_count"
                  />
                  <div
                    v-if="row.minor_count > 0"
                    class="risk-segment minor"
                    :style="{ flex: row.minor_count }"
                    :title="'中风险: ' + row.minor_count"
                  />
                  <div
                    v-if="row.info_count > 0"
                    class="risk-segment info"
                    :style="{ flex: row.info_count }"
                    :title="'提示: ' + row.info_count"
                  />
                </div>
                <span v-else class="risk-empty">无风险</span>
                <span class="risk-count" :class="{ critical: row.critical_count > 0 }">
                  {{ row.total_findings }} 项
                  <em v-if="row.critical_count > 0" class="risk-critical-tag">{{ row.critical_count }} 严重</em>
                </span>
              </div>
            </template>
          </el-table-column>

          <!-- 操作 -->
          <el-table-column label="操作" width="140" align="right">
            <template #default="{ row }">
              <div class="row-actions" @click.stop>
                <el-button size="small" plain @click="goToWeekly(row)">周报</el-button>
                <el-button size="small" type="primary" plain @click="goToDetail(row)">详情</el-button>
              </div>
            </template>
          </el-table-column>

          <template #empty>
            <el-empty description="暂无项目报告数据" :image-size="80">
              <el-button v-if="hasFilters" type="primary" plain @click="resetFilters">清除筛选</el-button>
            </el-empty>
          </template>
        </el-table>

        <div class="pagination-wrap">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :total="pagination.total"
            :page-sizes="[20, 50, 100]"
            layout="sizes, prev, pager, next, total"
            @current-change="fetchProjects"
            @size-change="handlePageSizeChange"
          />
        </div>
      </section>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh, Search, Files, TrendCharts, CircleCheckFilled, WarningFilled, Coin, Aim } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import TrendChart from '@/components/common/TrendChart.vue'
import type { TrendPoint } from '@/types/stats'
import { getProjectOCRMetrics, getProjectOCRTrend, getProjects } from '@/api/stats'
import type { ProjectOCRMetrics, ProjectOCRTrendPoint, ProjectSummary } from '@/types/stats'
import { formatDateRelative, getProviderInitial } from '@/utils/format'
import { getScoreColor } from '@/utils/colors'
import { PROVIDER_OPTIONS } from '@/utils/url-parser'

const router = useRouter()
const loading = ref(false)
const metricsLoading = ref(false)
const projects = ref<ProjectSummary[]>([])
const ocrMetrics = ref<ProjectOCRMetrics | null>(null)
const ocrTrend = ref<ProjectOCRTrendPoint[]>([])
const filters = reactive({ provider: '', repo: '', sort: 'avg_score' })
const dateRange = ref<[Date, Date] | []>([])
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const hasFilters = computed(() => Boolean(filters.provider || filters.repo || dateRange.value.length))

const summaryStats = computed(() => {
  const list = projects.value
  if (!list.length) {
    return {
      totalProjects: pagination.total,
      avgScore: '0.0',
      avgScoreNum: 0,
      avgPassRate: 0,
      criticalCount: 0,
    }
  }
  const avgScoreNum = list.reduce((sum, item) => sum + item.avg_score, 0) / list.length
  const avgPassRate = list.reduce((sum, item) => sum + item.pass_rate, 0) / list.length
  const criticalCount = list.reduce((sum, item) => sum + item.critical_count, 0)
  return {
    totalProjects: pagination.total,
    avgScore: avgScoreNum.toFixed(1),
    avgScoreNum,
    avgPassRate: Math.round(avgPassRate),
    criticalCount,
  }
})

type Tone = 'primary' | 'success' | 'warning' | 'danger' | 'info'
function scoreTone(score: number): Tone {
  if (score >= 80) return 'success'
  if (score >= 60) return 'warning'
  return 'danger'
}
function passRateTone(rate: number): Tone {
  if (rate >= 80) return 'success'
  if (rate >= 50) return 'warning'
  return 'danger'
}

const reportStats = computed(() => [
  { icon: Files, label: '项目数', value: summaryStats.value.totalProjects, tone: 'primary' as const },
  { icon: TrendCharts, label: '平均分', value: summaryStats.value.avgScore, tone: scoreTone(summaryStats.value.avgScoreNum) },
  { icon: CircleCheckFilled, label: '平均通过率', value: `${summaryStats.value.avgPassRate}%`, tone: passRateTone(summaryStats.value.avgPassRate) },
  { icon: WarningFilled, label: '严重问题', value: summaryStats.value.criticalCount, tone: 'danger' as const },
  { icon: Coin, label: 'Token 总量', value: formatNumber(ocrMetrics.value?.total_tokens), tone: 'success' as const },
  { icon: Aim, label: '问题密度', value: (ocrMetrics.value?.avg_findings_density || 0).toFixed(2), sublabel: 'findings / KLOC', tone: 'info' as const },
])

// 把 OCR 趋势数据适配成 TrendChart 需要的 TrendPoint 格式：
// 用 avg_score 画评分折线，用 total_tokens 映射为 findings 柱状（次维度）。
const trendChartData = computed<TrendPoint[]>(() => {
  return ocrTrend.value.map((pt) => ({
    period: pt.period,
    avg_score: pt.avg_score,
    review_count: pt.review_count,
    findings: Math.round(pt.total_tokens / 1000), // 以 K Token 为单位做柱高
  }))
})

async function fetchProjects() {
  loading.value = true
  metricsLoading.value = true
  try {
    const params = {
      ...filters,
      ...dateParams(),
      page: pagination.page,
      page_size: pagination.pageSize,
    }
    const [result, metrics, trend] = await Promise.all([
      getProjects(params),
      getProjectOCRMetrics({ provider: filters.provider || undefined, repo: filters.repo || undefined, ...dateParams() }),
      getProjectOCRTrend({ provider: filters.provider || undefined, repo: filters.repo || undefined, ...dateParams() }),
    ])
    projects.value = result.projects || []
    pagination.total = result.total
    ocrMetrics.value = metrics
    ocrTrend.value = trend || []
  } finally {
    loading.value = false
    metricsLoading.value = false
  }
}

function applyFilters() {
  pagination.page = 1
  fetchProjects()
}

function resetFilters() {
  filters.provider = ''
  filters.repo = ''
  filters.sort = 'avg_score'
  dateRange.value = []
  applyFilters()
}

function handlePageSizeChange() {
  pagination.page = 1
  fetchProjects()
}

function goToDetail(row: ProjectSummary) {
  router.push({
    path: '/projects/reports/detail',
    query: {
      provider: row.provider,
      repo: row.repo,
      ...dateParams(),
    },
  })
}

function goToWeekly(row: ProjectSummary) {
  router.push({
    path: '/reports/weekly',
    query: { provider: row.provider, repo: row.repo },
  })
}

function dateParams() {
  if (dateRange.value.length !== 2) return {}
  const [from, to] = dateRange.value
  const start = new Date(from)
  const end = new Date(to)
  start.setHours(0, 0, 0, 0)
  end.setHours(23, 59, 59, 999)
  return {
    from: start.toISOString(),
    to: end.toISOString(),
  }
}

function scoreClass(score: number) {
  if (score >= 80) return 'tone-success'
  if (score >= 60) return 'tone-warning'
  return 'tone-danger'
}

function passRateClass(rate: number) {
  if (rate >= 80) return 'tone-success'
  if (rate >= 50) return 'tone-warning'
  return 'tone-danger'
}

function providerLabel(provider: string) {
  const labels: Record<string, string> = { github: 'GitHub', gitlab: 'GitLab', gitea: 'Gitea', gitcode: 'GitCode' }
  return labels[provider] || provider
}

function formatNumber(value?: number) {
  return value ? value.toLocaleString() : '0'
}

onMounted(fetchProjects)
</script>

<style scoped>
/* ===== Panels ===== */

.panel-shell {
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius-lg);
  background: var(--app-surface);
  box-shadow: var(--app-shadow);
  overflow: hidden;
}

.panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 54px;
  padding: 15px 20px;
  border-bottom: 1px solid var(--app-border);
}

.panel-heading strong {
  display: block;
  color: var(--app-text);
  font-size: 15px;
}

.panel-heading span {
  display: block;
  margin-top: 2px;
  color: var(--app-muted);
  font-size: 12px;
}

/* ===== Trend ===== */

.trend-panel {
  margin-bottom: 16px;
}

.trend-head {
  min-height: 58px;
}

.trend-meta {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 999px;
  background: var(--app-bg);
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}

.meta-dot {
  display: inline-block;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--app-primary);
}

.trend-body {
  padding: 16px 20px 20px;
}

/* ===== Reports panel ===== */

.reports-header {
  border-bottom: 1px solid var(--app-border);
}

.reports-title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

/* ===== Filter bar（单行 flex） ===== */

.filter-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  padding: 14px 20px;
  border-bottom: 1px solid var(--app-border);
  background: var(--app-bg);
}

.filter-search {
  flex: 1 1 240px;
  min-width: 200px;
}

.filter-select {
  width: 140px;
  flex: 0 0 auto;
}

.filter-date {
  flex: 0 0 auto;
  width: auto;
}

.filter-reset {
  flex: 0 0 auto;
}

/* ===== Table ===== */

.project-table {
  --el-table-border-color: transparent;
  --el-table-header-bg-color: transparent;
  --el-table-row-hover-bg-color: #f6f8ff;
}

.project-table :deep(th.el-table__cell) {
  height: 42px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid var(--app-border);
}

.project-table :deep(td.el-table__cell) {
  height: 60px;
  border-bottom: 1px solid var(--app-border);
}

.project-table :deep(.clickable-row) {
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.project-table :deep(.clickable-row:hover td) {
  background-color: #f6f8ff !important;
}

/* 项目列 */
.repo-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.provider-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  border-radius: 9px;
  font-size: 11px;
  font-weight: 800;
  color: #fff;
  flex-shrink: 0;
}

.provider-badge.github { background: #24292f; }
.provider-badge.gitlab { background: #fc6d26; }
.provider-badge.gitea { background: #609926; }
.provider-badge.gitcode { background: #1a73e8; }

.repo-info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.repo-name {
  color: var(--app-text);
  font-size: 13px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.repo-sub {
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--app-muted);
  font-size: 11px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dot-sep {
  color: var(--app-border);
  font-style: normal;
}

/* 评分列 */
.score-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.score-value {
  font-size: 15px;
  font-weight: 700;
  min-width: 34px;
}

.score-value.tone-success { color: var(--app-success); }
.score-value.tone-warning { color: var(--app-warning); }
.score-value.tone-danger { color: var(--app-danger); }

.score-bar {
  flex: 1;
  height: 5px;
  background: var(--app-bg);
  border-radius: 3px;
  overflow: hidden;
}

.score-bar-fill {
  height: 100%;
  border-radius: 3px;
  transition: width 0.3s ease;
}

/* 通过率 */
.pass-rate-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 52px;
  height: 26px;
  border-radius: 7px;
  font-size: 12px;
  font-weight: 700;
  background: var(--app-bg);
  color: var(--app-muted);
}

.pass-rate-badge.tone-success { color: var(--app-success); background: #e3f7ef; }
.pass-rate-badge.tone-warning { color: var(--app-warning); background: #fdf2e3; }
.pass-rate-badge.tone-danger { color: var(--app-danger); background: #fdecee; }

/* 风险分布列 */
.risk-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.risk-bars {
  display: flex;
  flex: 1;
  height: 8px;
  max-width: 120px;
  border-radius: 4px;
  overflow: hidden;
  gap: 2px;
}

.risk-segment {
  height: 100%;
  min-width: 4px;
  border-radius: 2px;
  transition: flex 0.3s ease;
}

.risk-segment.critical { background: var(--app-danger); }
.risk-segment.major { background: var(--app-warning); }
.risk-segment.minor { background: var(--app-primary); }
.risk-segment.info { background: var(--app-success); }

.risk-empty {
  flex: 1;
  color: var(--app-muted);
  font-size: 12px;
}

.risk-count {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
}

.risk-count.critical {
  color: var(--app-danger);
}

.risk-critical-tag {
  padding: 1px 6px;
  border-radius: 4px;
  background: #fdecee;
  color: var(--app-danger);
  font-size: 11px;
  font-style: normal;
  font-weight: 700;
}

/* 操作 */
.row-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

/* 分页 */
.pagination-wrap {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 14px 20px;
  border-top: 1px solid var(--app-border);
}

/* ===== Responsive ===== */

@media (max-width: 760px) {
  .filter-bar {
    flex-direction: column;
    align-items: stretch;
  }

  .filter-search,
  .filter-select,
  .filter-date {
    width: 100%;
  }

  .filter-reset {
    align-self: flex-start;
  }

  .risk-cell {
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
  }
}
</style>
