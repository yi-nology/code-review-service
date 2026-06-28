<template>
  <AppLayout>
    <div class="project-reports-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">PROJECT REPORTS</div>
          <h1>项目报告</h1>
          <p>按项目汇总代码审查结果，追踪仓库质量、风险分布与门禁表现。</p>
        </div>
      </header>

      <el-row :gutter="16" class="stats-row">
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card projects">
            <div class="stat-icon"><el-icon><FolderChecked /></el-icon></div>
            <div class="stat-item">
              <span>项目数</span>
              <strong>{{ summaryStats.totalProjects }}</strong>
              <small>有审查记录的仓库</small>
            </div>
          </el-card>
        </el-col>
        <el-col :xs="12" :sm="12" :md="6">
          <el-card shadow="never" class="stat-card avg">
            <div class="stat-icon"><el-icon><TrendCharts /></el-icon></div>
            <div class="stat-item">
              <span>平均分</span>
              <strong>{{ summaryStats.avgScore }}</strong>
              <small>当前列表项目均值</small>
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
          <el-card shadow="never" class="stat-card risks">
            <div class="stat-icon"><el-icon><WarningFilled /></el-icon></div>
            <div class="stat-item">
              <span>严重问题</span>
              <strong>{{ summaryStats.criticalCount }}</strong>
              <small>严重风险总量</small>
            </div>
          </el-card>
        </el-col>
      </el-row>

      <section class="ocr-observability" v-loading="metricsLoading">
        <div class="obs-card">
          <span>OCR Token 总量</span>
          <strong>{{ formatNumber(ocrMetrics?.total_tokens) }}</strong>
          <small>均值 {{ formatNumber(Math.round(ocrMetrics?.avg_tokens || 0)) }} / 次</small>
        </div>
        <div class="obs-card">
          <span>扫描文件</span>
          <strong>{{ formatNumber(ocrMetrics?.total_reviewed_files) }}</strong>
          <small>{{ ocrMetrics?.review_count || 0 }} 次成功审查</small>
        </div>
        <div class="obs-card">
          <span>问题密度</span>
          <strong>{{ (ocrMetrics?.avg_findings_density || 0).toFixed(2) }}</strong>
          <small>平均 findings / KLOC</small>
        </div>
        <div class="obs-card tools">
          <span>工具调用</span>
          <strong>{{ formatNumber(totalToolCalls) }}</strong>
          <small>{{ topToolLabel }}</small>
        </div>
      </section>

      <section class="ocr-trend-band" v-loading="metricsLoading">
        <div class="trend-head">
          <div>
            <h2>OCR 成本趋势</h2>
            <p>按天聚合 tokens、耗时、质量分与问题密度</p>
          </div>
          <span>{{ ocrTrend.length }} 天</span>
        </div>
        <div v-if="ocrTrend.length" class="trend-points">
          <div v-for="point in ocrTrend.slice(-8)" :key="point.period" class="trend-point">
            <span>{{ formatDay(point.period) }}</span>
            <strong>{{ formatNumber(point.total_tokens) }}</strong>
            <small>{{ point.review_count }} 次 · {{ Math.round(point.avg_elapsed_ms) }}ms · {{ point.avg_score.toFixed(1) }}分</small>
          </div>
        </div>
        <el-empty v-else description="暂无 OCR 趋势数据" :image-size="48" />
      </section>

      <section class="reports-section">
        <div class="reports-header">
          <div class="reports-title">
            <h2>项目质量报告</h2>
            <span class="reports-subtitle">共 {{ pagination.total }} 个项目</span>
          </div>
          <div class="reports-actions">
            <el-button :icon="Refresh" circle @click="fetchProjects" />
          </div>
        </div>

        <div class="filter-bar" aria-label="项目报告筛选">
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
          <el-select v-model="filters.provider" placeholder="全部平台" clearable @change="applyFilters" class="filter-select provider-filter">
            <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <el-select v-model="filters.sort" placeholder="排序" @change="applyFilters" class="filter-select sort-filter">
            <el-option label="按评分" value="avg_score" />
            <el-option label="按审查数" value="review_count" />
            <el-option label="按通过率" value="pass_rate" />
            <el-option label="按严重问题" value="critical_count" />
            <el-option label="按活跃度" value="last_review_at" />
          </el-select>
          <div class="date-filter">
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
          </div>
          <el-button v-if="hasFilters" text @click="resetFilters" class="filter-reset">
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
          <el-table-column label="项目" min-width="240">
            <template #default="{ row }">
              <div class="repo-cell">
                <span class="provider-badge" :class="row.provider">{{ getProviderInitial(row.provider) }}</span>
                <div class="repo-info">
                  <span class="repo-name">{{ row.repo }}</span>
                  <span class="repo-provider">{{ providerLabel(row.provider) }}</span>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="审查" width="70" align="center">
            <template #default="{ row }">
              <span class="metric-value">{{ row.review_count }}</span>
            </template>
          </el-table-column>

          <el-table-column label="平均分" width="140">
            <template #default="{ row }">
              <div class="score-cell">
                <span class="score-value" :class="scoreClass(row.avg_score)">{{ row.avg_score }}</span>
                <div class="score-bar">
                  <div class="score-bar-fill" :style="{ width: row.avg_score + '%', backgroundColor: getScoreColor(row.avg_score) }"></div>
                </div>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="通过率" width="80" align="center">
            <template #default="{ row }">
              <span class="pass-rate-badge" :class="passRateClass(row.pass_rate)">
                {{ row.pass_rate.toFixed(0) }}%
              </span>
            </template>
          </el-table-column>

          <el-table-column label="问题" width="70" align="center">
            <template #default="{ row }">
              <span class="findings-badge" :class="{ 'has-findings': row.total_findings > 0 }">
                {{ row.total_findings }}
              </span>
            </template>
          </el-table-column>

          <el-table-column label="严重" width="70" align="center">
            <template #default="{ row }">
              <span v-if="row.critical_count > 0" class="critical-badge">{{ row.critical_count }}</span>
              <span v-else class="zero-value">-</span>
            </template>
          </el-table-column>

          <el-table-column label="风险分布" min-width="180">
            <template #default="{ row }">
              <div class="risk-bars">
                <template v-if="row.critical_count + row.major_count + row.minor_count + row.info_count > 0">
                  <div v-if="row.critical_count > 0" class="risk-segment critical" :style="{ flex: row.critical_count }" :title="'严重: ' + row.critical_count"></div>
                  <div v-if="row.major_count > 0" class="risk-segment major" :style="{ flex: row.major_count }" :title="'高风险: ' + row.major_count"></div>
                  <div v-if="row.minor_count > 0" class="risk-segment minor" :style="{ flex: row.minor_count }" :title="'中风险: ' + row.minor_count"></div>
                  <div v-if="row.info_count > 0" class="risk-segment info" :style="{ flex: row.info_count }" :title="'提示: ' + row.info_count"></div>
                </template>
                <span v-else class="zero-value">-</span>
              </div>
            </template>
          </el-table-column>

          <el-table-column label="最近审查" width="100">
            <template #default="{ row }">
              <span class="time-value">{{ formatDateRelative(row.last_review_at) }}</span>
            </template>
          </el-table-column>

          <el-table-column width="40" align="right">
            <template #default>
              <el-icon class="row-arrow"><ArrowRight /></el-icon>
            </template>
          </el-table-column>
        </el-table>

        <div class="pagination-wrap">
          <el-pagination
            v-model:current-page="pagination.page"
            v-model:page-size="pagination.pageSize"
            :total="pagination.total"
            :page-sizes="[20, 50, 100]"
            layout="sizes, prev, pager, next"
            @current-change="fetchProjects"
            @size-change="handlePageSizeChange"
          />
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  ArrowRight,
  CircleCheckFilled,
  FolderChecked,
  Refresh,
  Search,
  TrendCharts,
  WarningFilled,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
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
  if (!list.length) return { totalProjects: pagination.total, avgScore: '0', avgPassRate: 0, criticalCount: 0 }
  const avgScore = list.reduce((sum, item) => sum + item.avg_score, 0) / list.length
  const avgPassRate = list.reduce((sum, item) => sum + item.pass_rate, 0) / list.length
  const criticalCount = list.reduce((sum, item) => sum + item.critical_count, 0)
  return {
    totalProjects: pagination.total,
    avgScore: avgScore.toFixed(1),
    avgPassRate: Math.round(avgPassRate),
    criticalCount,
  }
})
const totalToolCalls = computed(() => (ocrMetrics.value?.tool_calls || []).reduce((sum, item) => sum + item.count, 0))
const topToolLabel = computed(() => {
  const first = ocrMetrics.value?.tool_calls?.[0]
  return first ? `最多：${first.tool} ${first.count} 次` : '暂无工具调用数据'
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
  if (score >= 80) return 'score-high'
  if (score >= 60) return 'score-mid'
  return 'score-low'
}

function passRateClass(rate: number) {
  if (rate >= 80) return 'high'
  if (rate >= 50) return 'mid'
  return 'low'
}

function providerLabel(provider: string) {
  const labels: Record<string, string> = { github: 'GitHub', gitlab: 'GitLab', gitea: 'Gitea', gitcode: 'GitCode' }
  return labels[provider] || provider
}

function formatNumber(value?: number) {
  return value ? value.toLocaleString() : '0'
}

function formatDay(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

onMounted(fetchProjects)
</script>

<style scoped>
.project-reports-page {
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

.ocr-observability {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 20px;
}

.obs-card {
  min-height: 94px;
  padding: 16px 18px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.obs-card span,
.obs-card small {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
}

.obs-card strong {
  display: block;
  margin: 8px 0 4px;
  color: var(--app-text);
  font-size: 25px;
  line-height: 1.15;
}

.obs-card.tools strong {
  color: var(--app-primary);
}

.ocr-trend-band {
  margin-bottom: 20px;
  padding: 16px 18px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: var(--app-surface);
}

.trend-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.trend-head h2 {
  margin: 0;
  color: var(--app-text);
  font-size: 16px;
}

.trend-head p {
  margin: 4px 0 0;
  color: var(--app-muted);
  font-size: 12px;
}

.trend-head > span {
  color: var(--app-muted);
  font-size: 12px;
  white-space: nowrap;
}

.trend-points {
  display: grid;
  grid-template-columns: repeat(8, minmax(0, 1fr));
  gap: 10px;
}

.trend-point {
  min-width: 0;
  padding: 10px;
  border: 1px solid #edf0f4;
  border-radius: 8px;
  background: #fafbfc;
}

.trend-point span,
.trend-point small {
  display: block;
  color: var(--app-muted);
  font-size: 11px;
}

.trend-point strong {
  display: block;
  margin: 6px 0 4px;
  color: var(--app-primary);
  font-size: 16px;
  overflow-wrap: anywhere;
}

.stats-row .el-col {
  margin-bottom: 16px;
}

.stat-card {
  height: 110px;
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

.projects .stat-icon { color: #5b5bd6; background: #eeeeff; }
.avg .stat-icon { color: #159570; background: #e6f7f0; }
.pass .stat-icon { color: #2878c7; background: #eaf4ff; }
.risks .stat-icon { color: #d84b5b; background: #fff0f2; }

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

.reports-section {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 12px;
  overflow: hidden;
}

.reports-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 18px 22px;
  border-bottom: 1px solid var(--app-border);
}

.reports-title {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.reports-title h2 {
  margin: 0;
  color: var(--app-text);
  font-size: 16px;
  font-weight: 600;
}

.reports-subtitle {
  color: var(--app-muted);
  font-size: 13px;
}

.filter-bar {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) 132px 142px auto;
  align-items: center;
  grid-template-areas:
    "search provider sort reset"
    "date date date reset";
  gap: 10px;
  padding: 14px 22px;
  border-bottom: 1px solid var(--app-border);
  background: #fbfcfe;
}

.filter-search {
  grid-area: search;
  width: 100%;
}

.filter-select {
  width: 100%;
}

.provider-filter {
  grid-area: provider;
}

.sort-filter {
  grid-area: sort;
}

.date-filter {
  grid-area: date;
  width: 100%;
  max-width: 420px;
}

.date-filter :deep(.filter-date) {
  width: 100%;
  max-width: 100%;
}

.filter-reset {
  grid-area: reset;
  justify-self: end;
  color: var(--app-muted);
}

.project-table {
  --el-table-border-color: transparent;
  --el-table-header-bg-color: transparent;
  --el-table-row-hover-bg-color: #f8f9ff;
}

.project-table :deep(th.el-table__cell) {
  height: 40px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  border-bottom: 1px solid var(--app-border);
}

.project-table :deep(td.el-table__cell) {
  height: 56px;
  border-bottom: 1px solid #f0f2f5;
}

.project-table :deep(.clickable-row) {
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.project-table :deep(.clickable-row:hover td) {
  background-color: #f8f9ff !important;
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
  width: 32px;
  height: 32px;
  border-radius: 8px;
  font-size: 11px;
  font-weight: 700;
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
  min-width: 0;
}

.repo-name {
  color: var(--app-text);
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.repo-provider {
  color: var(--app-muted);
  font-size: 11px;
}

/* 通用指标 */
.metric-value {
  color: var(--app-text);
  font-size: 14px;
  font-weight: 600;
}

/* 评分列 */
.score-cell {
  display: flex;
  align-items: center;
  gap: 10px;
}

.score-value {
  font-size: 14px;
  font-weight: 700;
  min-width: 28px;
}

.score-bar {
  flex: 1;
  height: 4px;
  background: #f0f2f5;
  border-radius: 2px;
  overflow: hidden;
}

.score-bar-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.3s ease;
}

.score-high { color: #10b981; }
.score-mid { color: #f59e0b; }
.score-low { color: #ef4444; }

/* 通过率 */
.pass-rate-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 48px;
  height: 24px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
}

.pass-rate-badge.high { color: #10b981; background: #ecfdf5; }
.pass-rate-badge.mid { color: #f59e0b; background: #fffbeb; }
.pass-rate-badge.low { color: #ef4444; background: #fef2f2; }

/* 问题数 */
.findings-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 24px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--app-muted);
}

.findings-badge.has-findings {
  color: #d97706;
  background: #fffbeb;
}

/* 严重问题 */
.critical-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 24px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 700;
  color: #ef4444;
  background: #fef2f2;
}

.zero-value {
  color: #d1d5db;
  font-size: 14px;
}

/* 风险分布条 */
.risk-bars {
  display: flex;
  height: 8px;
  border-radius: 4px;
  overflow: hidden;
  gap: 2px;
  min-width: 100px;
}

.risk-segment {
  height: 100%;
  min-width: 4px;
  border-radius: 2px;
  transition: flex 0.3s ease;
}

.risk-segment.critical { background: #ef4444; }
.risk-segment.major { background: #f59e0b; }
.risk-segment.minor { background: #3b82f6; }
.risk-segment.info { background: #9ca3af; }

/* 时间 */
.time-value {
  color: var(--app-muted);
  font-size: 12px;
}

/* 箭头 */
.row-arrow {
  color: #d1d5db;
  transition: color 0.15s ease;
}

.clickable-row:hover .row-arrow {
  color: var(--app-primary);
}

/* 分页 */
.pagination-wrap {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  padding: 12px 24px;
  border-top: 1px solid var(--app-border);
}

@media (max-width: 1040px) {
  .ocr-observability {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .trend-points {
    grid-template-columns: repeat(4, minmax(0, 1fr));
  }

}

@media (max-width: 760px) {
  .project-reports-page {
    padding: 24px 16px 36px;
  }

  .filter-bar {
    grid-template-columns: 1fr;
    grid-template-areas:
      "search"
      "provider"
      "sort"
      "date"
      "reset";
  }

  .filter-search,
  .filter-select,
  .date-filter,
  .filter-date {
    width: 100%;
    max-width: none;
  }

  .filter-reset {
    justify-self: start;
  }
}

@media (max-width: 640px) {
  .ocr-observability {
    grid-template-columns: 1fr;
  }

  .trend-head {
    flex-direction: column;
  }

  .trend-points {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .reports-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 12px;
  }
}
</style>
