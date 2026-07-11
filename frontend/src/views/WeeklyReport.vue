<template>
  <AppLayout>
    <div class="weekly-report-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">WEEKLY REPORT</div>
          <h1>项目周报</h1>
          <p>按项目维度生成最近 7 天的 AI 代码审查周报（Markdown 格式）。</p>
        </div>
      </header>

      <!-- 选择器 -->
      <el-card class="selector-card" shadow="never">
        <div class="selector-row">
          <el-select
            v-model="provider"
            placeholder="选择平台"
            class="provider-select"
            @change="onProviderChange"
          >
            <el-option v-for="p in PROVIDER_OPTIONS" :key="p.value" :label="p.label" :value="p.value" />
          </el-select>
          <el-input
            v-model="repo"
            placeholder="仓库名，例如 owner/repo"
            class="repo-input"
            clearable
            @keyup.enter="handleGenerate"
          />
          <el-button
            type="primary"
            :loading="loading"
            :disabled="!provider || !repo.trim()"
            @click="handleGenerate"
          >
            生成周报
          </el-button>
          <el-button v-if="report" plain @click="handleCopy">复制 Markdown</el-button>
          <el-button v-if="report" plain @click="handleDownload">下载 .md</el-button>
        </div>
        <p v-if="report" class="period-label">统计周期：{{ period }}</p>
      </el-card>

      <!-- 概览指标 -->
      <section v-if="report" class="metric-strip" aria-label="周报概览">
        <div class="metric-item">
          <span class="metric-icon running"><el-icon><Loading /></el-icon></span>
          <span class="metric-label">运行中</span>
          <strong>{{ stats.running }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon failed"><el-icon><WarningFilled /></el-icon></span>
          <span class="metric-label">门禁失败</span>
          <strong>{{ stats.gate_failed }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon findings"><el-icon><Aim /></el-icon></span>
          <span class="metric-label">发现问题</span>
          <strong>{{ stats.findings }}</strong>
        </div>
        <div class="metric-item">
          <span class="metric-icon tokens"><el-icon><Coin /></el-icon></span>
          <span class="metric-label">累计 Tokens</span>
          <strong>{{ stats.total_tokens ? stats.total_tokens.toLocaleString() : 0 }}</strong>
        </div>
      </section>

      <!-- Markdown 渲染 -->
      <el-card v-if="report" class="report-card" shadow="never">
        <div class="report-toolbar">
          <h2>周报正文</h2>
        </div>
        <div class="markdown-body" v-html="renderedHtml" />
      </el-card>

      <!-- 空状态 -->
      <el-empty v-else-if="!loading" description="选择平台和仓库后生成周报" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { Aim, Coin, Loading, WarningFilled } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { getWeeklyReport } from '@/api/report'
import { PROVIDER_OPTIONS } from '@/utils/url-parser'
import type { WeeklyReport as WeeklyReportType } from '@/types/report'
import { ElMessage } from 'element-plus/es/components/message/index'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const provider = ref(typeof route.query.provider === 'string' ? route.query.provider : '')
const repo = ref(typeof route.query.repo === 'string' ? route.query.repo : '')
const loading = ref(false)
const report = ref<WeeklyReportType | null>(null)

const stats = computed(() => report.value?.stats ?? {
  running: 0, gate_failed: 0, findings: 0, total_tokens: 0,
  input_tokens: 0, output_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0,
})
const period = computed(() => report.value?.period ?? '')
const renderedHtml = computed(() => renderMarkdown(report.value?.report ?? ''))

async function handleGenerate() {
  if (!provider.value || !repo.value.trim()) return
  loading.value = true
  try {
    report.value = await getWeeklyReport({ provider: provider.value, repo: repo.value.trim() })
  } catch (e: any) {
    ElMessage.error(e?.message || '生成周报失败')
    report.value = null
  } finally {
    loading.value = false
  }
}

function onProviderChange() {
  // 切换平台时清空已有报告，避免显示上一个项目的内容
  report.value = null
}

async function handleCopy() {
  if (!report.value) return
  try {
    await navigator.clipboard.writeText(report.value.report)
    ElMessage.success('已复制到剪贴板')
  } catch {
    ElMessage.error('复制失败，请手动选择文本复制')
  }
}

function handleDownload() {
  if (!report.value) return
  const blob = new Blob([report.value.report], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  const safeRepo = repo.value.trim().replace(/[^\w.-]/g, '_') || 'weekly-report'
  a.download = `weekly-${safeRepo}.md`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

onMounted(() => {
  // 从项目报告页跳转时自动带 provider+repo，直接生成
  if (provider.value && repo.value.trim()) {
    handleGenerate()
  }
})
</script>

<style scoped>
.weekly-report-page {
  width: min(1440px, 100%);
  margin: 0 auto;
  padding: 28px 32px 44px;
}

.page-header {
  margin-bottom: 24px;
}
.page-header .eyebrow {
  font-size: 11px;
  letter-spacing: 0.12em;
  color: var(--app-muted);
  font-weight: 600;
  text-transform: uppercase;
  margin-bottom: 6px;
}
.page-header h1 {
  font-size: 26px;
  font-weight: 700;
  color: var(--app-text);
  margin: 0 0 4px;
}
.page-header p {
  font-size: 14px;
  color: var(--app-muted);
  margin: 0;
}

.selector-card { border-radius: 14px; margin-bottom: 24px; }
.selector-row {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
.provider-select { width: 160px; }
.repo-input { width: 320px; max-width: 100%; }
.period-label { font-size: 13px; color: var(--app-muted); margin: 12px 0 0; }

.metric-strip {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
  margin-bottom: 24px;
}
.metric-item {
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 14px;
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.metric-icon {
  width: 36px;
  height: 36px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  margin-bottom: 6px;
}
.metric-icon.running { background: rgba(217, 134, 22, 0.1); color: var(--app-warning); }
.metric-icon.failed { background: rgba(216, 75, 91, 0.1); color: var(--app-danger); }
.metric-icon.findings { background: rgba(37, 99, 235, 0.1); color: var(--app-primary); }
.metric-icon.tokens { background: rgba(21, 149, 112, 0.1); color: var(--app-success); }
.metric-label { font-size: 13px; color: var(--app-muted); }
.metric-item strong { font-size: 26px; font-weight: 700; color: var(--app-text); }

.report-card { border-radius: 14px; }
.report-toolbar { margin-bottom: 16px; }
.report-toolbar h2 { font-size: 18px; font-weight: 600; margin: 0; color: var(--app-text); }

.markdown-body {
  font-size: 14px;
  line-height: 1.7;
  color: var(--app-text);
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 20px 0 10px;
  font-weight: 600;
  color: var(--app-text);
}
.markdown-body :deep(h1) { font-size: 22px; }
.markdown-body :deep(h2) { font-size: 18px; border-bottom: 1px solid var(--app-border); padding-bottom: 6px; }
.markdown-body :deep(h3) { font-size: 15px; }
.markdown-body :deep(p) { margin: 8px 0; }
.markdown-body :deep(ul),
.markdown-body :deep(ol) { margin: 8px 0; padding-left: 24px; }
.markdown-body :deep(li) { margin: 4px 0; }
.markdown-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 12px 0;
  font-size: 13px;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--app-border);
  padding: 8px 12px;
  text-align: left;
}
.markdown-body :deep(th) { background: var(--app-bg); font-weight: 600; }
.markdown-body :deep(code) {
  background: var(--app-bg);
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 13px;
  font-family: ui-monospace, 'SF Mono', monospace;
}
.markdown-body :deep(pre) {
  background: var(--app-bg);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  padding: 12px 16px;
  overflow-x: auto;
  margin: 12px 0;
}
.markdown-body :deep(pre code) { background: none; padding: 0; }
.markdown-body :deep(strong) { font-weight: 600; }
.markdown-body :deep(blockquote) {
  border-left: 3px solid var(--app-primary);
  margin: 12px 0;
  padding: 4px 16px;
  color: var(--app-muted);
}

@media (max-width: 720px) {
  .weekly-report-page { padding: 20px 16px 32px; }
  .metric-strip { grid-template-columns: repeat(2, 1fr); }
  .provider-select, .repo-input { width: 100%; }
}
</style>
