<template>
  <AppLayout>
    <div class="dashboard-page">
      <header class="dashboard-hero">
        <div>
          <div class="eyebrow">OPERATIONS OVERVIEW</div>
          <h1>运营总览</h1>
          <p>审查队列、质量风险、项目表现与配置入口集中在一个工作台。</p>
        </div>
        <div class="hero-actions">
          <el-button plain size="large" :loading="loading" @click="refreshAll">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
          <el-button type="primary" size="large" @click="showCreateDialog = true">
            <el-icon><Plus /></el-icon>
            发起审查
          </el-button>
        </div>
      </header>

      <section class="health-band">
        <div class="health-card service" :class="healthStatus">
          <span class="health-indicator" />
          <div>
            <span>服务状态</span>
            <strong>{{ healthLabel }}</strong>
          </div>
        </div>
        <router-link class="health-card" to="/reviews">
          <span class="card-icon running"><el-icon><Loading /></el-icon></span>
          <div>
            <span>运行中</span>
            <strong>{{ listStats.running }}</strong>
          </div>
        </router-link>
        <router-link class="health-card" to="/reviews?status=failed">
          <span class="card-icon failed"><el-icon><WarningFilled /></el-icon></span>
          <div>
            <span>失败任务</span>
            <strong>{{ failedCount }}</strong>
          </div>
        </router-link>
        <router-link class="health-card" to="/settings/model">
          <span class="card-icon config"><el-icon><Cpu /></el-icon></span>
          <div>
            <span>模型配置</span>
            <strong>就绪</strong>
          </div>
        </router-link>
      </section>

      <section class="kpi-grid" aria-label="核心指标">
        <div class="kpi-card">
          <span class="kpi-icon total"><el-icon><Files /></el-icon></span>
          <span>审查总数</span>
          <strong>{{ listStats.total }}</strong>
          <small>当前记录池</small>
        </div>
        <div class="kpi-card">
          <span class="kpi-icon score"><el-icon><TrendCharts /></el-icon></span>
          <span>平均分</span>
          <strong>{{ averageScore }}</strong>
          <small>最近 {{ recentReviews.length }} 次</small>
        </div>
        <div class="kpi-card">
          <span class="kpi-icon gate"><el-icon><CircleCheckFilled /></el-icon></span>
          <span>门禁通过率</span>
          <strong>{{ gatePassRate }}%</strong>
          <small>{{ passedCount }} / {{ completedCount }}</small>
        </div>
        <div class="kpi-card">
          <span class="kpi-icon findings"><el-icon><Aim /></el-icon></span>
          <span>问题总量</span>
          <strong>{{ listStats.findings }}</strong>
          <small>{{ listStats.gate_failed }} 个门禁未通过</small>
        </div>
      </section>

      <section class="dashboard-grid">
        <el-card class="panel-card queue-card" shadow="never">
          <template #header>
            <div class="panel-head">
              <div>
                <h2>审查队列</h2>
                <span>最近更新</span>
              </div>
              <router-link to="/reviews">全部记录 <el-icon><ArrowRight /></el-icon></router-link>
            </div>
          </template>

          <div v-loading="loading" class="review-feed">
            <router-link
              v-for="item in recentReviews"
              :key="item.review_id"
              class="review-feed-item"
              :to="`/reviews/${item.review_id}`"
            >
              <span class="provider-mark">{{ getProviderInitial(item.provider) }}</span>
              <span class="feed-main">
                <strong>{{ item.repo }}</strong>
                <small>{{ getStatusLabel(item.status) }} · {{ formatDateRelative(item.updated_at) }}</small>
              </span>
              <span class="feed-side">
                <strong v-if="item.report_generated">{{ item.score }}</strong>
                <strong v-else>--</strong>
                <small>{{ item.findings }} 问题</small>
              </span>
            </router-link>
            <el-empty v-if="!loading && recentReviews.length === 0" description="暂无审查记录" :image-size="64" />
          </div>
        </el-card>

        <el-card class="panel-card action-card" shadow="never">
          <template #header>
            <div class="panel-head">
              <div>
                <h2>快捷操作</h2>
                <span>高频入口</span>
              </div>
            </div>
          </template>

          <div class="action-list">
            <button type="button" class="action-item primary" @click="showCreateDialog = true">
              <span><el-icon><Plus /></el-icon></span>
              <strong>发起审查</strong>
              <small>PR、MR 或提交范围</small>
            </button>
            <router-link class="action-item" to="/scan">
              <span><el-icon><Search /></el-icon></span>
              <strong>代码库扫描</strong>
              <small>全仓风险摸底</small>
            </router-link>
            <router-link class="action-item" to="/settings/projects">
              <span><el-icon><FolderChecked /></el-icon></span>
              <strong>项目策略</strong>
              <small>规则覆盖与门禁</small>
            </router-link>
            <router-link class="action-item" to="/settings/platforms">
              <span><el-icon><Connection /></el-icon></span>
              <strong>Git 平台</strong>
              <small>凭据、Webhook、发布</small>
            </router-link>
          </div>
        </el-card>

        <el-card class="panel-card risk-card" shadow="never">
          <template #header>
            <div class="panel-head">
              <div>
                <h2>风险项目</h2>
                <span>按严重问题与平均分排序</span>
              </div>
              <router-link to="/projects/reports">项目报告 <el-icon><ArrowRight /></el-icon></router-link>
            </div>
          </template>

          <div v-loading="projectLoading" class="project-risk-list">
            <router-link
              v-for="project in riskProjects"
              :key="`${project.provider}:${project.repo}`"
              class="project-risk-item"
              :to="{ path: '/projects/reports/detail', query: { provider: project.provider, repo: project.repo } }"
            >
              <span class="provider-mark">{{ getProviderInitial(project.provider) }}</span>
              <span class="risk-main">
                <strong>{{ project.repo }}</strong>
                <small>{{ project.review_count }} 次审查 · {{ project.total_findings }} 个问题</small>
              </span>
              <span class="risk-score" :class="scoreClass(project.avg_score)">
                {{ project.avg_score }}
              </span>
            </router-link>
            <el-empty v-if="!projectLoading && riskProjects.length === 0" description="暂无项目报告" :image-size="64" />
          </div>
        </el-card>

        <el-card class="panel-card readiness-card" shadow="never">
          <template #header>
            <div class="panel-head">
              <div>
                <h2>配置体检</h2>
                <span>上线前关键项</span>
              </div>
            </div>
          </template>

          <div class="readiness-list">
            <router-link v-for="item in readinessItems" :key="item.to" class="readiness-item" :to="item.to">
              <span :class="['readiness-dot', item.state]" />
              <span>
                <strong>{{ item.title }}</strong>
                <small>{{ item.desc }}</small>
              </span>
              <el-icon><ArrowRight /></el-icon>
            </router-link>
          </div>
        </el-card>
      </section>

      <ReviewCreateDialog
        v-model="showCreateDialog"
        @created="handleCreated"
        @goto="(id) => router.push(`/reviews/${id}`)"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  Aim,
  ArrowRight,
  CircleCheckFilled,
  Connection,
  Cpu,
  Files,
  FolderChecked,
  Loading,
  Plus,
  Refresh,
  Search,
  TrendCharts,
  WarningFilled,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ReviewCreateDialog from '@/components/ReviewCreateDialog.vue'
import { getReviews } from '@/api/review'
import { getProjects } from '@/api/stats'
import type { ListStats, Review } from '@/types/review'
import type { ProjectSummary } from '@/types/stats'
import { formatDateRelative, getProviderInitial } from '@/utils/format'
import { getStatusLabel } from '@/utils/status'
import { ElMessage } from 'element-plus'

const router = useRouter()
const loading = ref(false)
const projectLoading = ref(false)
const showCreateDialog = ref(false)
const recentReviews = ref<Review[]>([])
const projects = ref<ProjectSummary[]>([])
const healthStatus = ref<'healthy' | 'down' | 'unknown'>('unknown')
const failedCount = ref(0)
const listStats = ref<ListStats>({
  total: 0,
  running: 0,
  gate_failed: 0,
  findings: 0,
  total_tokens: 0,
})

const healthLabel = computed(() => {
  if (healthStatus.value === 'healthy') return '运行中'
  if (healthStatus.value === 'down') return '异常'
  return '检测中'
})

const completedReviews = computed(() => recentReviews.value.filter((item) => item.status === 'success' && item.report_generated))
const completedCount = computed(() => completedReviews.value.length)
const passedCount = computed(() => completedReviews.value.filter((item) => item.gate?.passed).length)
const gatePassRate = computed(() => completedCount.value ? Math.round((passedCount.value / completedCount.value) * 100) : 0)
const averageScore = computed(() => {
  if (!completedReviews.value.length) return '--'
  const total = completedReviews.value.reduce((sum, item) => sum + (Number(item.score) || 0), 0)
  return Math.round(total / completedReviews.value.length)
})

const riskProjects = computed(() => {
  return [...projects.value]
    .sort((a, b) => {
      const criticalDiff = b.critical_count - a.critical_count
      if (criticalDiff) return criticalDiff
      return a.avg_score - b.avg_score
    })
    .slice(0, 6)
})

const readinessItems = computed(() => [
  {
    title: '模型配置',
    desc: '默认模型、推理参数与审查关注点',
    to: '/settings/model',
    state: healthStatus.value === 'healthy' ? 'ok' : 'warn',
  },
  {
    title: 'OCR 设置',
    desc: '扫描引擎、工具包与规则包',
    to: '/settings/ocr',
    state: 'ok',
  },
  {
    title: '审查规则',
    desc: '全局规则、严重级别与质量门禁',
    to: '/settings/rules',
    state: listStats.value.gate_failed > 0 ? 'warn' : 'ok',
  },
  {
    title: '审查代理',
    desc: '专业代理、临时评审人与执行策略',
    to: '/settings/agents',
    state: 'ok',
  },
])

function scoreClass(score: number) {
  if (score >= 85) return 'good'
  if (score >= 70) return 'warn'
  return 'bad'
}

async function checkHealth() {
  try {
    const resp = await window.fetch('/ping')
    healthStatus.value = resp.ok ? 'healthy' : 'down'
  } catch {
    healthStatus.value = 'down'
  }
}

async function fetchReviews() {
  loading.value = true
  try {
    const [latest, failed] = await Promise.all([
      getReviews({ page: 1, page_size: 8 }),
      getReviews({ page: 1, page_size: 1, status: 'failed' }),
    ])
    recentReviews.value = latest.items || []
    listStats.value = latest.stats || listStats.value
    failedCount.value = failed.total || 0
  } catch (err: any) {
    ElMessage.error(err?.message || '加载审查概览失败')
  } finally {
    loading.value = false
  }
}

async function fetchProjects() {
  projectLoading.value = true
  try {
    const res = await getProjects({ page: 1, page_size: 12, sort: 'critical_count', order: 'desc' })
    projects.value = res.projects || []
  } catch (err: any) {
    ElMessage.error(err?.message || '加载项目概览失败')
  } finally {
    projectLoading.value = false
  }
}

async function refreshAll() {
  await Promise.all([checkHealth(), fetchReviews(), fetchProjects()])
}

function handleCreated() {
  showCreateDialog.value = false
  refreshAll()
}

onMounted(refreshAll)
</script>

<style scoped>
.dashboard-page {
  min-height: 100vh;
  padding: 30px;
}

.dashboard-hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
  margin-bottom: 20px;
}

.eyebrow {
  margin-bottom: 8px;
  color: #2563eb;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0;
}

.dashboard-hero h1 {
  margin: 0;
  color: var(--app-text);
  font-size: 32px;
  letter-spacing: 0;
}

.dashboard-hero p {
  max-width: 620px;
  margin: 10px 0 0;
  color: var(--app-muted);
  line-height: 1.7;
}

.hero-actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 10px;
}

.health-band,
.kpi-grid,
.dashboard-grid {
  display: grid;
  gap: 16px;
}

.health-band {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 16px;
}

.health-card,
.kpi-card {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 14px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  text-decoration: none;
  box-shadow: var(--app-shadow);
}

.health-card {
  min-height: 78px;
  padding: 16px;
  color: var(--app-text);
}

.health-card span,
.kpi-card span {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
}

.health-card strong,
.kpi-card strong {
  display: block;
  margin-top: 3px;
  color: var(--app-text);
  font-size: 24px;
  line-height: 1.1;
}

.health-indicator {
  width: 12px;
  height: 12px;
  flex: 0 0 12px;
  border-radius: 50%;
  background: #909399;
  box-shadow: 0 0 0 5px rgba(144, 147, 153, 0.14);
}

.service.healthy .health-indicator {
  background: #159570;
  box-shadow: 0 0 0 5px rgba(21, 149, 112, 0.14);
}

.service.down .health-indicator {
  background: #d84b5b;
  box-shadow: 0 0 0 5px rgba(216, 75, 91, 0.16);
}

.card-icon,
.kpi-icon {
  display: grid;
  place-items: center;
  border-radius: 8px;
}

.card-icon {
  width: 42px;
  height: 42px;
  flex: 0 0 42px;
  font-size: 19px;
}

.card-icon.running,
.kpi-icon.score {
  color: #0f766e;
  background: #dff7f1;
}

.card-icon.failed,
.kpi-icon.findings {
  color: #b42339;
  background: #fee4e8;
}

.card-icon.config,
.kpi-icon.total {
  color: #1d4ed8;
  background: #dbeafe;
}

.kpi-icon.gate {
  color: #b7791f;
  background: #fff3cf;
}

.kpi-grid {
  grid-template-columns: repeat(4, minmax(0, 1fr));
  margin-bottom: 16px;
}

.kpi-card {
  display: block;
  min-height: 154px;
  padding: 18px;
}

.kpi-icon {
  width: 38px;
  height: 38px;
  margin-bottom: 18px;
  font-size: 18px;
}

.kpi-card small {
  display: block;
  margin-top: 8px;
  color: #8a94a6;
  font-size: 12px;
}

.dashboard-grid {
  grid-template-columns: minmax(0, 1.35fr) minmax(320px, 0.8fr);
  align-items: start;
}

.panel-card {
  border-radius: 8px;
}

.panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.panel-head h2 {
  margin: 0;
  color: var(--app-text);
  font-size: 18px;
  letter-spacing: 0;
}

.panel-head span {
  display: block;
  margin-top: 3px;
  color: var(--app-muted);
  font-size: 12px;
}

.panel-head a {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--app-primary);
  font-size: 13px;
  font-weight: 700;
  text-decoration: none;
}

.review-feed,
.project-risk-list,
.readiness-list,
.action-list {
  display: grid;
  gap: 10px;
}

.review-feed-item,
.project-risk-item,
.readiness-item,
.action-item {
  display: flex;
  min-width: 0;
  align-items: center;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  color: var(--app-text);
  background: #fff;
  text-decoration: none;
  transition: border-color 160ms ease, background 160ms ease;
}

.review-feed-item:hover,
.project-risk-item:hover,
.readiness-item:hover,
.action-item:hover {
  border-color: #b8cdfa;
  background: #f8fbff;
}

.review-feed-item,
.project-risk-item {
  gap: 12px;
  min-height: 72px;
  padding: 12px;
}

.provider-mark {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border-radius: 8px;
  color: #1d4ed8;
  background: #e8f0ff;
  font-size: 12px;
  font-weight: 800;
}

.feed-main,
.risk-main {
  min-width: 0;
  flex: 1;
}

.feed-main strong,
.risk-main strong,
.action-item strong,
.readiness-item strong {
  display: block;
  overflow: hidden;
  color: var(--app-text);
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feed-main small,
.risk-main small,
.action-item small,
.readiness-item small {
  display: block;
  margin-top: 4px;
  overflow: hidden;
  color: var(--app-muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.feed-side {
  min-width: 64px;
  text-align: right;
}

.feed-side strong {
  display: block;
  color: var(--app-text);
  font-size: 20px;
}

.feed-side small {
  color: var(--app-muted);
  font-size: 12px;
}

.action-card,
.readiness-card {
  grid-column: 2;
}

.risk-card {
  grid-column: 1;
}

.action-item {
  gap: 12px;
  min-height: 68px;
  padding: 12px;
}

.action-item span:first-child {
  display: grid;
  width: 38px;
  height: 38px;
  flex: 0 0 38px;
  place-items: center;
  border-radius: 8px;
  color: #1d4ed8;
  background: #e8f0ff;
  font-size: 18px;
}

.action-item.primary {
  border-color: #bdd0fa;
  background: #f5f8ff;
  cursor: pointer;
}

button.action-item {
  width: 100%;
  text-align: left;
}

.risk-score {
  display: grid;
  width: 48px;
  height: 34px;
  place-items: center;
  border-radius: 8px;
  font-weight: 800;
}

.risk-score.good {
  color: #0f766e;
  background: #dff7f1;
}

.risk-score.warn {
  color: #b7791f;
  background: #fff3cf;
}

.risk-score.bad {
  color: #b42339;
  background: #fee4e8;
}

.readiness-item {
  gap: 12px;
  min-height: 62px;
  padding: 11px 12px;
}

.readiness-dot {
  width: 10px;
  height: 10px;
  flex: 0 0 10px;
  border-radius: 50%;
  background: #159570;
}

.readiness-dot.warn {
  background: #d98616;
}

.readiness-item span:nth-child(2) {
  min-width: 0;
  flex: 1;
}

.readiness-item .el-icon {
  color: #9aa4b4;
}

@media (max-width: 1120px) {
  .health-band,
  .kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-grid {
    grid-template-columns: 1fr;
  }

  .action-card,
  .risk-card,
  .readiness-card {
    grid-column: auto;
  }
}

@media (max-width: 720px) {
  .dashboard-page {
    padding: 18px;
  }

  .dashboard-hero {
    display: block;
  }

  .dashboard-hero h1 {
    font-size: 27px;
  }

  .hero-actions {
    justify-content: flex-start;
    margin-top: 18px;
  }

  .health-band,
  .kpi-grid {
    grid-template-columns: 1fr;
  }
}
</style>
