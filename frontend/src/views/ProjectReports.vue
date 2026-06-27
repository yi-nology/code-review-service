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

      <el-card class="reports-card">
        <div class="toolbar">
          <div>
            <h2>项目质量报告</h2>
            <p>点击项目查看完整聚合报告</p>
          </div>
          <el-button circle plain aria-label="刷新" @click="fetchProjects">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </div>

        <div class="filter-bar">
          <el-select v-model="filters.provider" placeholder="全部平台" clearable @change="applyFilters">
            <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <el-input v-model="filters.repo" class="repo-search" placeholder="搜索仓库，例如 owner/repo" clearable @keyup.enter="applyFilters" @clear="applyFilters">
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-select v-model="filters.sort" placeholder="排序方式" @change="applyFilters">
            <el-option label="平均分" value="avg_score" />
            <el-option label="审查数" value="review_count" />
            <el-option label="通过率" value="pass_rate" />
            <el-option label="严重问题" value="critical_count" />
            <el-option label="最近活跃" value="last_review_at" />
          </el-select>
          <el-date-picker
            v-model="dateRange"
            class="date-range"
            type="daterange"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            unlink-panels
            @change="applyFilters"
          />
          <el-button type="primary" plain @click="applyFilters">筛选</el-button>
          <el-button v-if="hasFilters" link @click="resetFilters">重置</el-button>
        </div>

        <el-table
          :data="projects"
          v-loading="loading"
          class="project-table"
          empty-text="暂无项目报告数据"
          @row-click="goToDetail"
        >
          <el-table-column label="项目" min-width="260">
            <template #default="{ row }">
              <div class="repo-cell">
                <span class="provider-mark">{{ getProviderInitial(row.provider) }}</span>
                <div>
                  <strong>{{ row.repo }}</strong>
                  <small>{{ providerLabel(row.provider) }}</small>
                </div>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="审查数" width="90" align="center">
            <template #default="{ row }">
              <span class="num-cell">{{ row.review_count }}</span>
            </template>
          </el-table-column>
          <el-table-column label="平均分" width="150">
            <template #default="{ row }">
              <div class="score-cell">
                <strong :class="scoreClass(row.avg_score)">{{ row.avg_score }}</strong>
                <el-progress :percentage="row.avg_score" :show-text="false" :stroke-width="5" :color="getScoreColor(row.avg_score)" />
              </div>
            </template>
          </el-table-column>
          <el-table-column label="通过率" width="110">
            <template #default="{ row }">
              <span class="pass-rate" :class="passRateClass(row.pass_rate)">{{ row.pass_rate.toFixed(0) }}%</span>
            </template>
          </el-table-column>
          <el-table-column label="问题总数" width="100" align="center">
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
          <el-table-column label="问题分布" min-width="200">
            <template #default="{ row }">
              <div class="severity-stack">
                <span class="critical" v-if="row.critical_count > 0" title="严重">严重 {{ row.critical_count }}</span>
                <span class="major" v-if="row.major_count > 0" title="高风险">高风险 {{ row.major_count }}</span>
                <span class="minor" v-if="row.minor_count > 0" title="中风险">中风险 {{ row.minor_count }}</span>
                <span class="info" v-if="row.info_count > 0" title="提示">提示 {{ row.info_count }}</span>
                <span v-if="row.critical_count + row.major_count + row.minor_count + row.info_count === 0" class="muted">无问题</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="最近审查" width="130">
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
          <span>共 {{ pagination.total }} 个项目</span>
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
      </el-card>
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
import { getProjectOCRMetrics, getProjects } from '@/api/stats'
import type { ProjectOCRMetrics, ProjectSummary } from '@/types/stats'
import { formatDateRelative, getProviderInitial } from '@/utils/format'
import { getScoreColor } from '@/utils/colors'
import { PROVIDER_OPTIONS } from '@/utils/url-parser'

const router = useRouter()
const loading = ref(false)
const metricsLoading = ref(false)
const projects = ref<ProjectSummary[]>([])
const ocrMetrics = ref<ProjectOCRMetrics | null>(null)
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
    const [result, metrics] = await Promise.all([
      getProjects(params),
      getProjectOCRMetrics({ provider: filters.provider || undefined, repo: filters.repo || undefined, ...dateParams() }),
    ])
    projects.value = result.projects || []
    pagination.total = result.total
    ocrMetrics.value = metrics
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

.reports-card {
  overflow: hidden;
  box-shadow: var(--app-shadow);
}

.reports-card :deep(.el-card__body) {
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
  flex-wrap: wrap;
  gap: 10px;
  padding: 0 22px 18px;
  border-bottom: 1px solid var(--app-border);
}

.repo-search {
  width: 260px;
}

.date-range {
  width: 280px;
}

.project-table {
  --el-table-border-color: #edf0f4;
  --el-table-header-bg-color: #fafbfc;
  --el-table-row-hover-bg-color: #fafaff;
}

.project-table :deep(th.el-table__cell) {
  height: 44px;
  color: #7d8698;
  font-size: 12px;
  font-weight: 700;
}

.project-table :deep(td.el-table__cell) {
  height: 64px;
}

.project-table :deep(.el-table__row) {
  cursor: pointer;
}

.repo-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.provider-mark {
  display: grid;
  width: 36px;
  height: 36px;
  flex: 0 0 36px;
  place-items: center;
  border-radius: 10px;
  color: #fff;
  background: #535bd2;
  font-size: 12px;
  font-weight: 800;
}

.repo-cell strong {
  display: block;
  color: var(--app-text);
  font-size: 13px;
}

.repo-cell small {
  display: block;
  margin-top: 2px;
  color: #9aa2b1;
  font-size: 11px;
}

.num-cell,
.time-text {
  color: var(--app-text);
  font-size: 13px;
  font-weight: 600;
}

.score-cell {
  display: grid;
  grid-template-columns: 38px 1fr;
  align-items: center;
  gap: 8px;
}

.score-high { color: #159570; }
.score-mid { color: #d98616; }
.score-low { color: #d84b5b; }

.pass-rate {
  font-size: 13px;
  font-weight: 700;
}

.pass-rate.high { color: #159570; }
.pass-rate.mid { color: #d98616; }
.pass-rate.low { color: #d84b5b; }

.finding-count,
.severity-tag.critical {
  display: inline-grid;
  min-width: 26px;
  height: 26px;
  place-items: center;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 700;
}

.finding-count {
  color: #7f8796;
  background: #f2f4f7;
}

.finding-count.active {
  color: #c77910;
  background: #fff3df;
}

.severity-tag.critical {
  color: #d84b5b;
  background: #fff0f2;
}

.severity-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.severity-stack span {
  display: inline-flex;
  align-items: center;
  padding: 2px 7px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
}

.severity-stack .critical { color: #d84b5b; background: #fff0f2; }
.severity-stack .major { color: #d98616; background: #fff6e8; }
.severity-stack .minor { color: #2878c7; background: #eaf4ff; }
.severity-stack .info { color: #687385; background: #f2f4f7; }

.muted {
  color: #c0c6d0;
  font-size: 13px;
}

.row-arrow {
  color: #c0c6d0;
}

.pagination-wrap {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 22px 20px;
  color: #8d95a6;
  font-size: 12px;
}

@media (max-width: 760px) {
  .project-reports-page {
    padding: 24px 16px 36px;
  }

  .repo-search {
    width: 100%;
  }

  .date-range {
    width: 100%;
  }

  .pagination-wrap {
    align-items: flex-start;
    flex-direction: column;
    gap: 12px;
  }
}

@media (max-width: 1040px) {
  .ocr-observability {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .ocr-observability {
    grid-template-columns: 1fr;
  }
}
</style>
