<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="ARGUS"
        title="Argus 审查"
        description="Argus（PR 自动审查 Bot）上报的审查任务清单：项目、裁决与意见分布。"
      >
        <template #actions>
          <el-button circle plain aria-label="刷新列表" @click="refresh">
            <el-icon><Refresh /></el-icon>
          </el-button>
        </template>
      </PageHeader>

      <el-card class="argus-gov-card">
        <div class="argus-gov" v-loading="govLoading">
          <div class="argus-gov-item">
            <span class="argus-gov-label">本月 token 用量</span>
            <span class="argus-gov-value">{{ usage ? formatTokens(usage.total) : '—' }}</span>
            <span v-if="usage?.budget" class="argus-gov-sub">预算 {{ formatTokens(usage.budget) }}</span>
          </div>
          <div class="argus-gov-item">
            <span class="argus-gov-label">本月活跃仓库</span>
            <span class="argus-gov-value">{{ usage ? String(usage.repos?.length ?? 0) : '—' }}</span>
            <span class="argus-gov-sub">{{ usage?.month || '' }}</span>
          </div>
          <div class="argus-gov-item">
            <span class="argus-gov-label">审查健康（学习环）</span>
            <span class="argus-gov-value">{{ healthLabel }}</span>
          </div>
          <div class="argus-gov-actions">
            <el-button plain size="small" @click="openTasks">任务对账</el-button>
          </div>
        </div>
      </el-card>

      <el-card>
        <div class="argus-filter-bar">
          <el-input
            v-model="filters.repo"
            class="argus-repo-input"
            placeholder="仓库名或 owner/repo"
            clearable
            @keyup.enter="applyFilters"
            @clear="applyFilters"
          >
            <template #prefix><el-icon><Search /></el-icon></template>
          </el-input>
          <el-input
            v-model="filters.platform"
            class="argus-platform-input"
            placeholder="平台（如 gitea）"
            clearable
            @keyup.enter="applyFilters"
            @clear="applyFilters"
          />
          <el-select v-model="filters.severity" placeholder="全部风险档" clearable @change="applyFilters">
            <el-option label="含 high 意见" value="high" />
            <el-option label="含 medium 意见" value="medium" />
            <el-option label="含 low 意见" value="low" />
          </el-select>
          <el-date-picker
            v-model="filters.since"
            type="datetime"
            placeholder="起始时间"
            format="YYYY-MM-DD HH:mm"
            clearable
            @change="applyFilters"
          />
          <el-button type="primary" plain @click="applyFilters">筛选</el-button>
        </div>

        <el-table :data="items" v-loading="loading" @row-click="openDetail" class="argus-table">
          <el-table-column label="项目" min-width="220">
            <template #default="{ row }">
              <div class="argus-repo-cell">
                <span class="argus-repo-name">{{ row.owner }}/{{ row.repo }}</span>
                <span class="argus-pr-line">#{{ row.number }} · {{ row.platform }} · {{ row.title || '（无标题）' }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="裁决" width="150">
            <template #default="{ row }">
              <el-tag :type="verdictTagType(row.verdict)" effect="light">
                {{ verdictLabel(row.verdict) }}
              </el-tag>
            </template>
          </el-table-column>
          <el-table-column label="意见分布" min-width="180">
            <template #default="{ row }">
              <div class="argus-sev-line">
                <el-tag v-if="row.findings_high" type="danger" size="small">H {{ row.findings_high }}</el-tag>
                <el-tag v-if="row.findings_medium" type="warning" size="small">M {{ row.findings_medium }}</el-tag>
                <el-tag v-if="row.findings_low" type="info" size="small">L {{ row.findings_low }}</el-tag>
                <span v-if="!row.findings_total" class="argus-clean">零意见</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column label="审查专家" min-width="140">
            <template #default="{ row }">
              <span class="argus-plugins">{{ (row.plugins || []).join(' / ') || '—' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="审查时间" width="160">
            <template #default="{ row }">{{ formatDateShort(row.occurred_at) }} {{ formatClock(row.occurred_at) }}</template>
          </el-table-column>
        </el-table>

        <div class="argus-pager">
          <el-pagination
            v-model:current-page="page"
            v-model:page-size="pageSize"
            :total="total"
            :page-sizes="[10, 20, 50]"
            layout="total, sizes, prev, pager, next"
            @current-change="load"
            @size-change="applyFilters"
          />
        </div>
      </el-card>

      <el-dialog v-model="tasksVisible" title="Argus 侧任务对账" width="72%" top="6vh">
        <div class="argus-tasks-toolbar">
          <el-select v-model="tasksStatus" placeholder="全部状态" clearable size="small" @change="loadTasks">
            <el-option label="pending" value="pending" />
            <el-option label="running" value="running" />
            <el-option label="done" value="done" />
            <el-option label="failed" value="failed" />
            <el-option label="stale" value="stale" />
          </el-select>
          <span class="argus-gov-sub">argus 侧真实任务队列，用于与承接表对账</span>
        </div>
        <el-table :data="tasks" v-loading="tasksLoading" size="small" max-height="56vh">
          <el-table-column label="任务" min-width="180">
            <template #default="{ row }"><span class="argus-mono">{{ row.task_id }}</span></template>
          </el-table-column>
          <el-table-column label="类型" width="190">
            <template #default="{ row }">{{ taskKindLabel(row.kind) }}</template>
          </el-table-column>
          <el-table-column label="状态" width="100">
            <template #default="{ row }">
              <el-tag :type="taskStatusTagType(row.status)" size="small">{{ row.status }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="项目" min-width="200">
            <template #default="{ row }">{{ row.owner }}/{{ row.repo }} #{{ row.number }}（{{ row.platform }}）</template>
          </el-table-column>
          <el-table-column label="裁决" width="110">
            <template #default="{ row }">{{ row.verdict || '—' }}</template>
          </el-table-column>
          <el-table-column label="创建时间" width="160">
            <template #default="{ row }">{{ formatTs(row.created_at) }}</template>
          </el-table-column>
        </el-table>
      </el-dialog>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh, Search } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { getArgusHealth, getArgusReviews, getArgusTasks, getArgusUsage } from '@/api/argus'
import { formatDateShort, formatClock } from '@/utils/format'
import type { ArgusHealthReport, ArgusReview, ArgusTaskSummary, ArgusUsage } from '@/types/argus'

const router = useRouter()
const items = ref<ArgusReview[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)

// 治理卡：用量 / 健康 / argus 侧任务对账（代理端点，argus 未配置时静默降级为 —）
const govLoading = ref(false)
const usage = ref<ArgusUsage | null>(null)
const health = ref<ArgusHealthReport | null>(null)
const tasksVisible = ref(false)
const tasksLoading = ref(false)
const tasks = ref<ArgusTaskSummary[]>([])
const tasksStatus = ref('')
const filters = ref<{
  repo: string
  platform: string
  severity: 'high' | 'medium' | 'low' | ''
  since: Date | null
}>({ repo: '', platform: '', severity: '', since: null })

function verdictLabel(v: string): string {
  switch (v) {
    case 'approve': return '通过'
    case 'comment': return '评论'
    case 'request_changes': return '要求修改'
    case 'error': return '审查失败'
    default: return v || '—'
  }
}

function verdictTagType(v: string): 'success' | 'warning' | 'danger' | 'info' {
  switch (v) {
    case 'approve': return 'success'
    case 'comment': return 'warning'
    case 'request_changes': return 'danger'
    default: return 'info'
  }
}

async function load() {
  loading.value = true
  try {
    const res = await getArgusReviews({
      repo: filters.value.repo || undefined,
      platform: filters.value.platform || undefined,
      severity: filters.value.severity || undefined,
      since: filters.value.since ? filters.value.since.toISOString() : undefined,
      page: page.value,
      page_size: pageSize.value,
    })
    items.value = res?.items ?? []
    total.value = res?.total ?? 0
  } catch {
    // 失败提示由 request 拦截器统一弹出，这里只保证 loading 复位
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  page.value = 1
  load()
}

function refresh() {
  load()
}

function openDetail(row: ArgusReview) {
  router.push(`/argus/${row.id}`)
}

function formatTokens(n?: number): string {
  if (!n && n !== 0) return '—'
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M'
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k'
  return String(n)
}

const healthLabel = computed(() => {
  if (!health.value || health.value.empty) return '暂无数据'
  const n = health.value.repos?.length ?? 0
  return n ? `${n} 个仓库有学习环数据` : '暂无数据'
})

async function loadGov() {
  govLoading.value = true
  try {
    const [u, h] = await Promise.allSettled([getArgusUsage(), getArgusHealth()])
    if (u.status === 'fulfilled') usage.value = u.value
    if (h.status === 'fulfilled') health.value = h.value
  } finally {
    govLoading.value = false
  }
}

function openTasks() {
  tasksVisible.value = true
  loadTasks()
}

async function loadTasks() {
  tasksLoading.value = true
  try {
    tasks.value = await getArgusTasks(tasksStatus.value || undefined, 100)
  } catch {
    tasks.value = []
  } finally {
    tasksLoading.value = false
  }
}

function taskKindLabel(k: string): string {
  switch (k) {
    case 'code_review': return '代码评审'
    case 'requirement_compliance': return '需求复核'
    case 'visual_check': return '视觉检查'
    default: return k || '—'
  }
}

function taskStatusTagType(s: string): 'success' | 'warning' | 'danger' | 'info' {
  switch (s) {
    case 'done': return 'success'
    case 'pending':
    case 'running': return 'warning'
    case 'failed':
    case 'stale': return 'danger'
    default: return 'info'
  }
}

function formatTs(sec?: number): string {
  if (!sec) return '—'
  const d = new Date(sec * 1000)
  return `${formatDateShort(d.toISOString())} ${formatClock(d.toISOString())}`
}

// Argus 为异步推送，列表页定时刷新（页面隐藏或正在加载时跳过）。
const pollIntervalMs = 15000
let pollTimer: number | undefined

onMounted(() => {
  load()
  loadGov()
  pollTimer = window.setInterval(() => {
    if (!document.hidden && !loading.value) load()
  }, pollIntervalMs)
})

onUnmounted(() => {
  if (pollTimer !== undefined) window.clearInterval(pollTimer)
})
</script>

<style scoped>
.argus-gov-card {
  margin-bottom: 16px;
}
.argus-gov {
  display: flex;
  gap: 40px;
  align-items: center;
  flex-wrap: wrap;
}
.argus-gov-item {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 140px;
}
.argus-gov-label {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.argus-gov-value {
  font-size: 18px;
  font-weight: 600;
}
.argus-gov-sub {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.argus-gov-actions {
  margin-left: auto;
}
.argus-tasks-toolbar {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-bottom: 12px;
}
.argus-mono {
  font-family: var(--el-font-family-mono, monospace);
  font-size: 12px;
}
.argus-filter-bar {
  display: flex;
  gap: 12px;
  margin-bottom: 16px;
}
.argus-repo-input {
  width: 260px;
}
.argus-platform-input {
  width: 170px;
}
.argus-table {
  width: 100%;
  cursor: pointer;
}
.argus-repo-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.argus-repo-name {
  font-weight: 600;
}
.argus-pr-line {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.argus-sev-line {
  display: flex;
  gap: 6px;
  align-items: center;
}
.argus-clean {
  color: var(--el-text-color-secondary);
  font-size: 12px;
}
.argus-plugins {
  color: var(--el-text-color-regular);
  font-size: 12px;
}
.argus-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}
</style>
