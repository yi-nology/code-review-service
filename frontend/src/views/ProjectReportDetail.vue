<template>
  <AppLayout>
    <div class="project-report-detail">
      <header class="page-header">
        <div>
          <button class="back-link" type="button" @click="router.push('/projects/reports')">
            <el-icon><ArrowLeft /></el-icon>
            返回项目报告
          </button>
          <div class="eyebrow">PROJECT QUALITY REPORT</div>
          <h1>{{ repo || '项目报告' }}</h1>
          <p>{{ providerLabel(provider) }} 项目维度代码审查报告</p>
        </div>
        <div class="header-actions">
          <el-button :disabled="!detail" @click="exportJSON">
            <el-icon><Download /></el-icon>
            导出 JSON
          </el-button>
          <el-button type="primary" :disabled="!detail" @click="exportMarkdown">
            <el-icon><Download /></el-icon>
            导出 Markdown
          </el-button>
        </div>
      </header>

      <div v-if="missingParams" class="empty-state">
        缺少项目参数，请从项目报告列表进入。
      </div>

      <div v-else v-loading="loading" class="report-shell">
        <div class="report-controls">
          <el-date-picker
            v-model="dateRange"
            class="date-range"
            type="daterange"
            start-placeholder="开始日期"
            end-placeholder="结束日期"
            unlink-panels
          />
          <el-button type="primary" plain @click="applyDateRange">更新报告</el-button>
          <el-button v-if="hasDateRange" link @click="clearDateRange">清除时间范围</el-button>
        </div>

        <el-row v-if="detail" :gutter="16" class="stats-row">
          <el-col :xs="12" :sm="12" :md="6">
            <el-card shadow="never" class="stat-card score">
              <span>平均分</span>
              <strong :class="scoreClass(detail.avg_score)">{{ detail.avg_score }}</strong>
              <small>中位数 {{ detail.median_score }}</small>
            </el-card>
          </el-col>
          <el-col :xs="12" :sm="12" :md="6">
            <el-card shadow="never" class="stat-card pass">
              <span>门禁通过率</span>
              <strong>{{ detail.pass_rate.toFixed(0) }}%</strong>
              <small>{{ detail.review_count }} 次审查</small>
            </el-card>
          </el-col>
          <el-col :xs="12" :sm="12" :md="6">
            <el-card shadow="never" class="stat-card findings">
              <span>问题总数</span>
              <strong>{{ detail.total_findings }}</strong>
              <small>平均 {{ detail.avg_findings }} 个 / 次</small>
            </el-card>
          </el-col>
          <el-col :xs="12" :sm="12" :md="6">
            <el-card shadow="never" class="stat-card critical">
              <span>严重问题</span>
              <strong>{{ detail.critical_count }}</strong>
              <small>Major {{ detail.major_count }} 个</small>
            </el-card>
          </el-col>
        </el-row>

        <div v-if="detail" class="report-grid">
          <section class="panel trend-panel">
            <div class="panel-head">
              <div>
                <h2>质量趋势</h2>
                <p>按周聚合平均分、审查量与风险项</p>
              </div>
            </div>
            <TrendChart :data="trend" :height="220" />
            <div class="trend-meta">
              <span>最高分 {{ detail.max_score }}</span>
              <span>最低分 {{ detail.min_score }}</span>
              <span>平均变更行 {{ detail.avg_changed_lines.toFixed(0) }}</span>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>风险分布</h2>
                <p>按严重程度统计当前项目风险结构</p>
              </div>
            </div>
            <div class="severity-grid">
              <div class="severity-card critical">
                <span>Critical</span>
                <strong>{{ detail.critical_count }}</strong>
              </div>
              <div class="severity-card major">
                <span>Major</span>
                <strong>{{ detail.major_count }}</strong>
              </div>
              <div class="severity-card minor">
                <span>Minor</span>
                <strong>{{ detail.minor_count }}</strong>
              </div>
              <div class="severity-card info">
                <span>Info</span>
                <strong>{{ detail.info_count }}</strong>
              </div>
            </div>
            <div class="grade-row">
              <span v-for="grade in gradeItems" :key="grade.label">
                {{ grade.label }} <strong>{{ grade.count }}</strong>
              </span>
            </div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>问题分类</h2>
                <p>出现频率最高的审查问题类型</p>
              </div>
            </div>
            <div v-if="categories.length" class="bar-list">
              <div v-for="item in categories" :key="item.category" class="bar-item">
                <div>
                  <span>{{ item.category }}</span>
                  <strong>{{ item.count }}</strong>
                </div>
                <i :style="{ width: `${categoryPercent(item.count)}%` }" />
              </div>
            </div>
            <div v-else class="panel-empty">暂无问题分类数据</div>
          </section>

          <section class="panel">
            <div class="panel-head">
              <div>
                <h2>高风险文件</h2>
                <p>按 Critical、Major 和问题总量排序</p>
              </div>
            </div>
            <el-table :data="files" size="small" empty-text="暂无文件风险数据">
              <el-table-column label="文件" min-width="220">
                <template #default="{ row }">
                  <span class="file-path">{{ row.path }}</span>
                </template>
              </el-table-column>
              <el-table-column label="问题" width="70" align="center" prop="count" />
              <el-table-column label="C" width="58" align="center" prop="critical_count" />
              <el-table-column label="M" width="58" align="center" prop="major_count" />
            </el-table>
          </section>

          <section class="panel wide-panel">
            <div class="panel-head">
              <div>
                <h2>项目开发者</h2>
                <p>该项目下参与审查记录的开发者表现</p>
              </div>
            </div>
            <el-table :data="contributors" empty-text="暂无开发者数据">
              <el-table-column label="开发者" min-width="180">
                <template #default="{ row }">
                  <router-link :to="`/contributors/${encodeURIComponent(row.author)}`" class="author-link">
                    {{ row.author_name || row.author }}
                  </router-link>
                </template>
              </el-table-column>
              <el-table-column label="审查数" width="90" align="center" prop="review_count" />
              <el-table-column label="平均分" width="120">
                <template #default="{ row }">
                  <strong :class="scoreClass(row.avg_score)">{{ row.avg_score }}</strong>
                </template>
              </el-table-column>
              <el-table-column label="通过率" width="110">
                <template #default="{ row }">{{ row.pass_rate.toFixed(0) }}%</template>
              </el-table-column>
              <el-table-column label="问题" width="90" align="center" prop="total_findings" />
            </el-table>
          </section>
        </div>

        <div v-if="!loading && !detail" class="empty-state">
          暂未找到该项目的审查报告数据。
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ElMessage } from 'element-plus/es/components/message/index'
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Download } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import TrendChart from '@/components/common/TrendChart.vue'
import {
  getProject,
  getProjectCategories,
  getProjectContributors,
  getProjectFiles,
  getProjectTrend,
} from '@/api/stats'
import type {
  CategoryStat,
  ContributorSummary,
  FileRiskStat,
  ProjectDetail,
  ProjectTrendPoint,
} from '@/types/stats'
import { formatDateFull } from '@/utils/format'

const route = useRoute()
const router = useRouter()
const loading = ref(false)
const detail = ref<ProjectDetail | null>(null)
const trend = ref<ProjectTrendPoint[]>([])
const categories = ref<CategoryStat[]>([])
const files = ref<FileRiskStat[]>([])
const contributors = ref<ContributorSummary[]>([])

const provider = computed(() => String(route.query.provider || ''))
const repo = computed(() => String(route.query.repo || ''))
const from = computed(() => String(route.query.from || ''))
const to = computed(() => String(route.query.to || ''))
const missingParams = computed(() => !provider.value || !repo.value)
const dateRange = ref<[Date, Date] | []>([])
const hasDateRange = computed(() => dateRange.value.length === 2)

const gradeItems = computed(() => {
  const dist = detail.value?.grade_distribution || {}
  return ['A', 'B', 'C', 'D', 'F'].map((label) => ({ label, count: dist[label] || 0 }))
})

async function fetchReport() {
  if (missingParams.value) return
  loading.value = true
  try {
    syncDateRangeFromQuery()
    const params = reportParams()
    const [project, trendData, categoryData, fileData, contributorData] = await Promise.all([
      getProject(params),
      getProjectTrend(params),
      getProjectCategories(params),
      getProjectFiles(params),
      getProjectContributors({ ...params, limit: 10 }),
    ])
    detail.value = project
    trend.value = trendData
    categories.value = categoryData
    files.value = fileData
    contributors.value = contributorData
  } finally {
    loading.value = false
  }
}

function reportParams() {
  return {
    provider: provider.value,
    repo: repo.value,
    ...(from.value ? { from: from.value } : {}),
    ...(to.value ? { to: to.value } : {}),
  }
}

function syncDateRangeFromQuery() {
  if (!from.value || !to.value) {
    dateRange.value = []
    return
  }
  dateRange.value = [new Date(from.value), new Date(to.value)]
}

function dateParamsFromPicker() {
  if (dateRange.value.length !== 2) return {}
  const [startDate, endDate] = dateRange.value
  const start = new Date(startDate)
  const end = new Date(endDate)
  start.setHours(0, 0, 0, 0)
  end.setHours(23, 59, 59, 999)
  return {
    from: start.toISOString(),
    to: end.toISOString(),
  }
}

function applyDateRange() {
  router.push({
    path: '/projects/reports/detail',
    query: {
      provider: provider.value,
      repo: repo.value,
      ...dateParamsFromPicker(),
    },
  })
}

function clearDateRange() {
  dateRange.value = []
  applyDateRange()
}

function categoryPercent(count: number) {
  const max = Math.max(...categories.value.map((item) => item.count), 1)
  return Math.max(8, Math.round((count / max) * 100))
}

function scoreClass(score: number) {
  if (score >= 80) return 'score-high'
  if (score >= 60) return 'score-mid'
  return 'score-low'
}

function providerLabel(value: string) {
  const labels: Record<string, string> = { github: 'GitHub', gitlab: 'GitLab', gitea: 'Gitea', gitcode: 'GitCode' }
  return labels[value] || value
}

function exportMarkdown() {
  if (!detail.value) return
  const report = detail.value
  const categoryLines = categories.value.map((item) => `- ${item.category}: ${item.count}`).join('\n') || '- 暂无'
  const fileLines = files.value.map((item) => `- ${item.path}: ${item.count} 个问题，Critical ${item.critical_count}，Major ${item.major_count}`).join('\n') || '- 暂无'
  const contributorLines = contributors.value.map((item) => `- ${item.author_name || item.author}: ${item.review_count} 次审查，平均分 ${item.avg_score}，通过率 ${item.pass_rate.toFixed(0)}%`).join('\n') || '- 暂无'
  const content = `# ${report.repo} 代码审查项目报告

- 平台: ${providerLabel(report.provider)}
- 时间范围: ${formatReportRange()}
- 生成时间: ${new Date().toLocaleString('zh-CN')}
- 最近审查: ${formatDateFull(report.last_review_at)}

## 质量概览

- 审查次数: ${report.review_count}
- 平均分: ${report.avg_score}
- 中位数: ${report.median_score}
- 门禁通过率: ${report.pass_rate.toFixed(0)}%
- 问题总数: ${report.total_findings}
- 严重问题: ${report.critical_count}
- 主要问题: ${report.major_count}

## 风险分布

- Critical: ${report.critical_count}
- Major: ${report.major_count}
- Minor: ${report.minor_count}
- Info: ${report.info_count}

## 问题分类

${categoryLines}

## 高风险文件

${fileLines}

## 项目开发者

${contributorLines}
`
  downloadReport(content, 'text/markdown;charset=utf-8', 'md')
  ElMessage.success('项目报告已导出')
}

function exportJSON() {
  if (!detail.value) return
  const payload = {
    generated_at: new Date().toISOString(),
    range: { from: from.value || null, to: to.value || null },
    detail: detail.value,
    trend: trend.value,
    categories: categories.value,
    files: files.value,
    contributors: contributors.value,
  }
  downloadReport(JSON.stringify(payload, null, 2), 'application/json;charset=utf-8', 'json')
  ElMessage.success('项目报告 JSON 已导出')
}

function downloadReport(content: string, type: string, extension: string) {
  if (!detail.value) return
  const blob = new Blob([content], { type })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${detail.value.repo.replace(/[\\/]/g, '-')}-review-report.${extension}`
  a.click()
  URL.revokeObjectURL(url)
}

function formatReportRange() {
  if (!from.value || !to.value) return '全部时间'
  return `${formatDateFull(from.value)} 至 ${formatDateFull(to.value)}`
}

watch(() => route.query, fetchReport)
onMounted(() => {
  syncDateRangeFromQuery()
  fetchReport()
})
</script>

<style scoped>
.project-report-detail {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 34px 36px 48px;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 24px;
}

.back-link {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  margin: 0 0 12px;
  padding: 0;
  border: 0;
  color: var(--app-primary);
  background: transparent;
  font-size: 13px;
  cursor: pointer;
}

.eyebrow {
  color: var(--app-primary);
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 1.4px;
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

.header-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.report-controls {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 16px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 12px;
  background: #fff;
}

.date-range {
  width: 300px;
}

.stats-row {
  margin-bottom: 16px;
}

.stats-row .el-col {
  margin-bottom: 16px;
}

.stat-card {
  height: 108px;
}

.stat-card :deep(.el-card__body) {
  display: flex;
  height: 100%;
  flex-direction: column;
  justify-content: center;
  padding: 18px 20px;
}

.stat-card span {
  color: var(--app-muted);
  font-size: 13px;
  font-weight: 700;
}

.stat-card strong {
  margin-top: 4px;
  color: var(--app-text);
  font-size: 28px;
  line-height: 1.1;
}

.stat-card small {
  margin-top: 5px;
  color: #9aa2b1;
  font-size: 11px;
}

.report-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.3fr) minmax(360px, 0.7fr);
  gap: 16px;
}

.panel {
  min-width: 0;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  box-shadow: var(--app-shadow);
}

.trend-panel,
.wide-panel {
  grid-column: 1 / -1;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 20px 10px;
}

.panel-head h2 {
  margin: 0 0 4px;
  color: var(--app-text);
  font-size: 17px;
}

.panel-head p {
  margin: 0;
  color: #929aab;
  font-size: 12px;
}

.trend-panel :deep(.trend-chart),
.trend-panel :deep(.empty-chart) {
  padding: 0 14px 14px;
}

.trend-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  padding: 0 20px 18px;
}

.trend-meta span,
.grade-row span {
  border-radius: 8px;
  padding: 7px 10px;
  color: #647084;
  background: #f5f7fa;
  font-size: 12px;
  font-weight: 700;
}

.severity-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
  padding: 10px 20px 14px;
}

.severity-card {
  border-radius: 8px;
  padding: 14px;
}

.severity-card span {
  display: block;
  font-size: 12px;
  font-weight: 800;
}

.severity-card strong {
  display: block;
  margin-top: 8px;
  font-size: 24px;
}

.severity-card.critical { color: #d84b5b; background: #fff0f2; }
.severity-card.major { color: #d98616; background: #fff6e8; }
.severity-card.minor { color: #2878c7; background: #eaf4ff; }
.severity-card.info { color: #687385; background: #f2f4f7; }

.grade-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 0 20px 20px;
}

.bar-list {
  display: flex;
  flex-direction: column;
  gap: 13px;
  padding: 10px 20px 20px;
}

.bar-item div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
  color: #596275;
  font-size: 13px;
  font-weight: 700;
}

.bar-item i {
  display: block;
  height: 8px;
  border-radius: 8px;
  background: #5b5bd6;
}

.panel-empty {
  padding: 28px 20px;
  color: #929aab;
  font-size: 13px;
}

.file-path {
  color: #394150;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.author-link {
  color: var(--app-primary);
  font-weight: 700;
  text-decoration: none;
}

.score-high { color: #159570; }
.score-mid { color: #d98616; }
.score-low { color: #d84b5b; }

.empty-state {
  display: grid;
  min-height: 220px;
  place-items: center;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  color: #929aab;
  background: #fff;
}

@media (max-width: 960px) {
  .project-report-detail {
    padding: 24px 16px 36px;
  }

  .page-header {
    flex-direction: column;
  }

  .header-actions {
    justify-content: flex-start;
  }

  .date-range {
    width: 100%;
  }

  .report-grid {
    grid-template-columns: 1fr;
  }
}
</style>
