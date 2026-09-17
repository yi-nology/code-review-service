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
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Refresh, Search } from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { getArgusReviews } from '@/api/argus'
import { formatDateShort, formatClock } from '@/utils/format'
import type { ArgusReview } from '@/types/argus'

const router = useRouter()
const items = ref<ArgusReview[]>([])
const total = ref(0)
const loading = ref(false)
const page = ref(1)
const pageSize = ref(20)
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

// Argus 为异步推送，列表页定时刷新（页面隐藏或正在加载时跳过）。
const pollIntervalMs = 15000
let pollTimer: number | undefined

onMounted(() => {
  load()
  pollTimer = window.setInterval(() => {
    if (!document.hidden && !loading.value) load()
  }, pollIntervalMs)
})

onUnmounted(() => {
  if (pollTimer !== undefined) window.clearInterval(pollTimer)
})
</script>

<style scoped>
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
