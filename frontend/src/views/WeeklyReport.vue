<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="WEEKLY REPORT"
        title="项目周报"
        description="按项目维度查看最近 7 天的 AI 代码审查汇总。"
      >
        <template v-if="report" #actions>
          <el-button plain @click="handleCopy">
            <el-icon><CopyDocument /></el-icon>复制 Markdown
          </el-button>
          <el-button plain @click="handleDownload">
            <el-icon><Download /></el-icon>下载 .md
          </el-button>
          <el-button plain @click="handlePrint">
            <el-icon><Printer /></el-icon>打印
          </el-button>
        </template>
      </PageHeader>

      <!-- 项目选择区 -->
      <el-card class="selector-card" shadow="never">
        <div class="selector-row">
          <el-select
            v-model="provider"
            placeholder="选择平台"
            class="provider-select"
            clearable
            @change="onProviderChange"
          >
            <el-option v-for="p in providerOptions" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <RepoSelect
            v-model="repo"
            :provider="provider"
            :immediate="true"
            secondary-field="none"
            placeholder="搜索仓库（owner/repo）"
            class="repo-select"
            @change="() => handleGenerate()"
          />
          <el-button type="primary" :loading="loading" :disabled="!provider || !repo" @click="handleGenerate">
            <el-icon><Document /></el-icon>生成周报
          </el-button>
        </div>

        <!-- 最近看过 / 有审查记录的项目：一键直达 -->
        <div v-if="quickProjects.length" class="quick-picks">
          <span class="quick-label">
            <el-icon><Clock /></el-icon>
            {{ recentProjects.length ? '最近看过' : '审查过的项目' }}
          </span>
          <button
            v-for="p in quickProjects"
            :key="`${p.provider}/${p.repo}`"
            type="button"
            class="quick-chip"
            :class="{ active: p.provider === provider && p.repo === repo }"
            @click="pickProject(p.provider, p.repo)"
          >
            <span class="chip-provider">{{ getProviderLabel(p.provider) }}</span>
            <span class="chip-repo">{{ p.repo }}</span>
            <span v-if="'review_count' in p && p.review_count" class="chip-count">{{ (p as ProjectSummary).review_count }} 次</span>
          </button>
        </div>
      </el-card>

      <!-- 加载中 -->
      <div v-if="loading" class="state-block">
        <el-icon class="is-loading big-spin"><Loading /></el-icon>
        <p>正在生成周报…</p>
      </div>

      <!-- 错误态 -->
      <el-result
        v-else-if="errorMsg"
        icon="error"
        title="生成失败"
        :sub-title="errorMsg"
      >
        <template #extra>
          <el-button type="primary" @click="handleGenerate">重试</el-button>
        </template>
      </el-result>

      <!-- 报告内容 -->
      <template v-else-if="report">
        <!-- 周期 + 关键指标 -->
        <section class="overview-section">
          <div class="period-bar">
            <el-icon><Calendar /></el-icon>
            <span>统计周期：<strong>{{ period }}</strong></span>
            <span class="period-meta">{{ providerLabel }} · {{ repo }}</span>
          </div>
          <div class="metric-strip">
            <div class="metric-card primary">
              <small>活跃审查</small>
              <strong>{{ reviewCount }}</strong>
              <span class="metric-sub">本周进行中</span>
            </div>
            <div class="metric-card">
              <small>门禁失败</small>
              <strong :class="{ danger: stats.gate_failed > 0 }">{{ stats.gate_failed }}</strong>
              <span class="metric-sub">{{ passRateText }}</span>
            </div>
            <div class="metric-card">
              <small>发现问题</small>
              <strong :class="{ warn: stats.findings > 0 }">{{ stats.findings }}</strong>
              <span class="metric-sub">需关注</span>
            </div>
            <div class="metric-card">
              <small>Token 消耗</small>
              <strong>{{ formatTokens(stats.total_tokens) }}</strong>
              <span class="metric-sub">{{ formatTokens(stats.input_tokens + stats.output_tokens) }} 输入+输出</span>
            </div>
          </div>
        </section>

        <!-- 提交人排行（交互式卡片，数据同 Markdown 正文里的表格） -->
        <el-card v-if="contributors.length" class="contributors-card" shadow="never">
          <div class="report-toolbar">
            <h2>提交人排行</h2>
            <span class="report-meta">本周按审查次数排序，共 {{ contributors.length }} 位</span>
          </div>
          <div class="contributor-grid">
            <div v-for="(ct, idx) in contributors" :key="ct.author" class="contributor-item">
              <span class="rank">{{ idx + 1 }}</span>
              <el-avatar v-if="ct.author_avatar" :src="ct.author_avatar" :size="34" />
              <span v-else class="avatar-fallback">{{ initialOf(ct) }}</span>
              <div class="contributor-main">
                <div class="contributor-head">
                  <strong>{{ ct.author_name || ct.author }}</strong>
                  <span class="contributor-reviews">{{ ct.review_count }} 次审查</span>
                </div>
                <div class="contributor-stats">
                  <span class="stat-chip danger" v-if="ct.critical_count">严重 {{ ct.critical_count }}</span>
                  <span class="stat-chip warn" v-if="ct.major_count">高 {{ ct.major_count }}</span>
                  <span class="stat-chip" v-if="ct.minor_count">中 {{ ct.minor_count }}</span>
                  <span class="stat-chip muted" v-if="ct.info_count">低 {{ ct.info_count }}</span>
                  <span class="stat-total">共 {{ ct.total_findings }} 个问题</span>
                  <span class="stat-pass" :class="{ ok: ct.pass_rate >= 0.8, bad: ct.pass_rate < 0.6 }">
                    通过率 {{ Math.round(ct.pass_rate * 100) }}%
                  </span>
                </div>
              </div>
            </div>
          </div>
        </el-card>

        <!-- 周报正文（Markdown） -->
        <el-card class="report-card" shadow="never">
          <div class="report-toolbar">
            <h2>周报正文</h2>
            <span class="report-meta">Markdown 格式，可复制或下载</span>
          </div>
          <div class="markdown-body" v-html="renderedHtml" />
        </el-card>
      </template>

      <!-- 空状态（首次进入，未选项目） -->
      <div v-else class="state-block empty-state">
        <el-icon class="empty-icon"><Document /></el-icon>
        <h2>选择一个项目查看周报</h2>
        <p v-if="quickProjects.length">从上方「审查过的项目」中点选，或选择平台 + 搜索仓库。</p>
        <p v-else>先选择平台，再搜索仓库名，然后生成周报。</p>
      </div>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import {
  Calendar, Clock, CopyDocument, Document, Download, Loading, Printer,
} from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { getWeeklyReport } from '@/api/report'
import { getProjects } from '@/api/stats'
import { getGitPlatforms } from '@/api/config'
import { getProviderLabel } from '@/utils/url-parser'
import { renderMarkdown } from '@/utils/markdown'
import RepoSelect from '@/components/common/RepoSelect.vue'
import type { WeeklyReport, AggregateStats } from '@/types/report'
import type { ProjectSummary } from '@/types/stats'

interface RecentProject { provider: string; repo: string; ts: number }

const route = useRoute()
const provider = ref('')
const repo = ref('')
const loading = ref(false)
const report = ref<WeeklyReport | null>(null)
const errorMsg = ref('')

// 平台下拉只展示 DB 中已配置（有 token）的平台，避免选到未配置平台时 400 报错。
const providerOptions = ref<{ label: string; value: string }[]>([])

async function loadPlatforms() {
  try {
    const res = await getGitPlatforms()
    const items = (res as any)?.items || []
    providerOptions.value = items
      .filter((p: any) => p.enabled)
      .map((p: any) => ({ label: getProviderLabel(p.provider), value: p.provider }))
  } catch { /* 静默：不影响主流程 */ }
}

// 有审查记录的项目（用于一键快选）+ 最近看过的项目（localStorage 记忆）。
const reviewedProjects = ref<ProjectSummary[]>([])
const recentProjects = ref<RecentProject[]>([])
const RECENT_KEY = 'weekly-recent-projects'
const RECENT_MAX = 6

// 快选列表：优先展示最近看过的，其次展示有审查记录的，去重合并。
const quickProjects = computed(() => {
  const seen = new Set<string>()
  const out: Array<(RecentProject & { review_count?: number }) | ProjectSummary> = []
  // 最近看过优先
  for (const r of recentProjects.value) {
    const k = `${r.provider}/${r.repo}`
    if (seen.has(k)) continue
    seen.add(k)
    out.push({ provider: r.provider, repo: r.repo, ts: r.ts, review_count: undefined })
  }
  // 补充有审查记录的项目
  for (const p of reviewedProjects.value) {
    const k = `${p.provider}/${p.repo}`
    if (seen.has(k)) continue
    seen.add(k)
    out.push(p)
  }
  return out.slice(0, 8)
})

const stats = computed<AggregateStats>(() => report.value?.stats ?? {
  running: 0, gate_failed: 0, findings: 0, total_tokens: 0,
  input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0,
})
const period = computed(() => report.value?.period ?? '')
const renderedHtml = computed(() => renderMarkdown(report.value?.report ?? ''))
const contributors = computed(() => report.value?.contributors ?? [])
const providerLabel = computed(() => getProviderLabel(provider.value))
// 后端 stats 只有 running（进行中）/gate_failed/findings/tokens，无明确的"审查总数"。
// 用 running 作为"本周活动审查数"展示，比编造一个不存在的总数更诚实。
const reviewCount = computed(() => stats.value.running)
const passRateText = computed(() => {
  const total = stats.value.running + stats.value.gate_failed
  if (total <= 0) return '本周无审查'
  const pct = Math.round(((total - stats.value.gate_failed) / total) * 100)
  return `通过率 ${pct}%`
})

function formatTokens(n: number) {
  if (!n) return '0'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K'
  return String(n)
}

// 提交人无头像时取首字母占位。
function initialOf(ct: { author_name?: string; author: string }): string {
  const name = ct.author_name || ct.author || '?'
  return name.charAt(0).toUpperCase()
}

function onProviderChange() {
  repo.value = ''
}

function pickProject(p: string, r: string) {
  provider.value = p
  repo.value = r
  handleGenerate()
}

async function handleGenerate() {
  if (!provider.value || !repo.value) return
  loading.value = true
  errorMsg.value = ''
  try {
    report.value = await getWeeklyReport({ provider: provider.value, repo: repo.value })
    recordRecent(provider.value, repo.value)
  } catch (e: any) {
    report.value = null
    errorMsg.value = e.message || '生成周报失败，请检查平台和仓库名是否正确'
  } finally {
    loading.value = false
  }
}

function recordRecent(p: string, r: string) {
  const list = recentProjects.value.filter((x) => !(x.provider === p && x.repo === r))
  list.unshift({ provider: p, repo: r, ts: Date.now() })
  recentProjects.value = list.slice(0, RECENT_MAX)
  try { localStorage.setItem(RECENT_KEY, JSON.stringify(recentProjects.value)) } catch { /* ignore */ }
}

function loadRecent() {
  try {
    const raw = localStorage.getItem(RECENT_KEY)
    if (raw) recentProjects.value = JSON.parse(raw)
  } catch { /* ignore */ }
}

async function loadReviewedProjects() {
  try {
    const res = await getProjects({ page: 1, page_size: 20 })
    reviewedProjects.value = res.projects || []
  } catch { /* 静默：不影响主流程 */ }
}

async function handleCopy() {
  if (!report.value?.report) return
  try {
    await navigator.clipboard.writeText(report.value.report)
    ElMessage.success('Markdown 已复制')
  } catch {
    ElMessage.error('复制失败，请手动选择文本复制')
  }
}

function handleDownload() {
  if (!report.value?.report) return
  const blob = new Blob([report.value.report], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `weekly-report-${provider.value}-${repo.value.replace('/', '-')}.md`
  a.click()
  URL.revokeObjectURL(url)
}

function handlePrint() {
  window.print()
}

onMounted(async () => {
  loadRecent()
  await Promise.all([loadPlatforms(), loadReviewedProjects()])
  // 支持 query 参数预选项目（从 ProjectReports 跳转过来）
  const qProvider = route.query.provider
  const qRepo = route.query.repo
  if (typeof qProvider === 'string' && typeof qRepo === 'string') {
    provider.value = qProvider
    repo.value = qRepo
    handleGenerate()
  }
})
</script>

<style scoped>
/* 选择器 */
.selector-card {
  margin-bottom: 18px;
}

.selector-row {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

.provider-select {
  width: 180px;
}

.repo-select {
  flex: 1;
  min-width: 260px;
}

/* 快选 chips */
.quick-picks {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 14px;
  padding-top: 14px;
  border-top: 1px dashed var(--app-border);
}

.quick-label {
  display: flex;
  align-items: center;
  gap: 5px;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
  margin-right: 4px;
}

.quick-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 5px 12px;
  border: 1px solid var(--app-border);
  border-radius: 20px;
  background: #fff;
  color: var(--app-text);
  font-size: 13px;
  cursor: pointer;
  transition: all 150ms ease;
}

.quick-chip:hover {
  border-color: #93c5fd;
  color: #1d4ed8;
  background: #eff6ff;
}

.quick-chip.active {
  border-color: #2563eb;
  background: #2563eb;
  color: #fff;
}

.quick-chip.active .chip-provider,
.quick-chip.active .chip-count {
  color: rgba(255, 255, 255, 0.85);
}

.chip-provider {
  color: #6b7280;
  font-size: 11px;
  font-weight: 600;
}

.chip-count {
  padding: 1px 6px;
  border-radius: 8px;
  background: #f2f4f7;
  color: #6b7280;
  font-size: 11px;
}

.quick-chip.active .chip-count {
  background: rgba(255, 255, 255, 0.2);
}

/* 概览 */
.overview-section {
  margin-bottom: 18px;
}

.period-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 10px 16px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
  background: #fff;
  color: var(--app-text);
  font-size: 14px;
}

.period-bar .el-icon {
  color: var(--app-primary);
}

.period-meta {
  margin-left: auto;
  color: var(--app-muted);
  font-size: 13px;
}

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.metric-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 16px 18px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: #fff;
}

.metric-card.primary {
  border-color: #bfdbfe;
  background: linear-gradient(135deg, #eff6ff 0%, #fff 100%);
}

.metric-card small {
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 600;
}

.metric-card strong {
  font-size: 28px;
  line-height: 1.2;
  color: var(--app-text);
}

.metric-card strong.danger {
  color: var(--app-danger, #dc2626);
}

.metric-card strong.warn {
  color: #d97706;
}

.metric-sub {
  color: var(--app-muted);
  font-size: 11px;
}

/* 报告正文 */
.report-card {
  overflow: hidden;
}

/* 提交人排行 */
.contributors-card {
  margin-bottom: 18px;
}

.contributor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 12px;
}

.contributor-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 14px;
  border: 1px solid var(--app-border);
  border-radius: var(--card-radius);
  background: #fff;
}

.rank {
  flex-shrink: 0;
  width: 24px;
  text-align: center;
  color: var(--app-muted);
  font-size: 14px;
  font-weight: 800;
}

.avatar-fallback {
  display: grid;
  width: 34px;
  height: 34px;
  flex-shrink: 0;
  place-items: center;
  border-radius: 50%;
  background: #eef2ff;
  color: #4f46e5;
  font-size: 14px;
  font-weight: 700;
}

.contributor-main {
  flex: 1;
  min-width: 0;
}

.contributor-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}

.contributor-head strong {
  font-size: 14px;
  color: var(--app-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.contributor-reviews {
  flex-shrink: 0;
  color: var(--app-muted);
  font-size: 12px;
}

.contributor-stats {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 8px;
}

.stat-chip {
  padding: 1px 7px;
  border-radius: 6px;
  background: #f2f4f7;
  color: #6b7280;
  font-size: 11px;
}

.stat-chip.danger {
  background: #fef2f2;
  color: #dc2626;
}

.stat-chip.warn {
  background: #fffbeb;
  color: #d97706;
}

.stat-chip.muted {
  color: #9ca3af;
}

.stat-total {
  color: var(--app-muted);
  font-size: 11px;
}

.stat-pass {
  margin-left: auto;
  font-size: 11px;
  font-weight: 600;
}

.stat-pass.ok {
  color: #059669;
}

.stat-pass.bad {
  color: #dc2626;
}

.report-toolbar {
  display: flex;
  align-items: baseline;
  gap: 10px;
  margin-bottom: 14px;
}

.report-toolbar h2 {
  margin: 0;
  font-size: 17px;
  color: var(--app-text);
}

.report-meta {
  color: var(--app-muted);
  font-size: 12px;
}

/* Markdown 渲染样式 */
.markdown-body {
  color: var(--app-text);
  font-size: 14px;
  line-height: 1.8;
}

.markdown-body :deep(h1) {
  margin: 0 0 12px;
  font-size: 22px;
  padding-bottom: 8px;
  border-bottom: 2px solid var(--app-border);
}

.markdown-body :deep(h2) {
  margin: 24px 0 10px;
  font-size: 17px;
  color: var(--app-text);
}

.markdown-body :deep(h3) {
  margin: 18px 0 8px;
  font-size: 15px;
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 8px 0;
  padding-left: 22px;
}

.markdown-body :deep(li) {
  margin: 4px 0;
}

.markdown-body :deep(strong) {
  color: var(--app-text);
}

.markdown-body :deep(code) {
  padding: 2px 6px;
  border-radius: 4px;
  background: #f2f4f7;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 13px;
}

.markdown-body :deep(pre) {
  margin: 10px 0;
  padding: 14px;
  border-radius: 8px;
  background: #1e1e2e;
  color: #cdd6f4;
  overflow-x: auto;
  font-size: 13px;
  line-height: 1.6;
}

.markdown-body :deep(pre code) {
  padding: 0;
  background: none;
  color: inherit;
}

.markdown-body :deep(table) {
  width: 100%;
  margin: 12px 0;
  border-collapse: collapse;
  font-size: 13px;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  padding: 8px 12px;
  border: 1px solid var(--app-border);
  text-align: left;
}

.markdown-body :deep(th) {
  background: #fafbfc;
  font-weight: 600;
}

.markdown-body :deep(hr) {
  margin: 18px 0;
  border: 0;
  border-top: 1px solid var(--app-border);
}

.markdown-body :deep(blockquote) {
  margin: 10px 0;
  padding: 8px 14px;
  border-left: 3px solid var(--app-primary);
  background: #fafbfc;
  color: var(--app-muted);
}

/* 状态块（加载/空） */
.state-block {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  min-height: 320px;
  color: var(--app-muted);
}

.big-spin {
  font-size: 32px;
  color: var(--app-primary);
}

.empty-state h2 {
  margin: 0;
  font-size: 18px;
  color: var(--app-text);
}

.empty-state p {
  margin: 0;
  font-size: 14px;
}

.empty-icon {
  font-size: 48px;
  color: #c4cad6;
}

@media (max-width: 980px) {
  .selector-row {
    flex-direction: column;
  }

  .provider-select,
  .repo-select {
    width: 100%;
  }
}

@media print {
  .selector-card {
    display: none;
  }
}
</style>
