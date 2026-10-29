<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="OPERATIONS DIAGNOSTICS"
        title="诊断中心"
        description="集中查看 Webhook 投递、写回发布和近期故障线索。"
      >
        <template #actions>
          <el-select v-model="hours" class="hours-select" @change="fetchSnapshot">
            <el-option label="最近 24 小时" :value="24" />
            <el-option label="最近 7 天" :value="168" />
            <el-option label="最近 30 天" :value="720" />
          </el-select>
          <el-button plain :loading="loading" @click="fetchSnapshot">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </template>
      </PageHeader>

      <StatStrip :items="diagnosticStats" />

      <section class="recommendations">
        <div
          v-for="item in snapshot?.recommendations || []"
          :key="`${item.level}:${item.title}`"
          class="recommendation"
          :class="item.level"
        >
          <span class="recommendation-icon">
            <el-icon v-if="item.level === 'success'"><CircleCheckFilled /></el-icon>
            <el-icon v-else-if="item.level === 'danger'"><CircleCloseFilled /></el-icon>
            <el-icon v-else><InfoFilled /></el-icon>
          </span>
          <span>
            <strong>{{ item.title }}</strong>
            <small>{{ item.message }}</small>
            <small v-if="item.action" class="action-text">{{ item.action }}</small>
          </span>
        </div>
      </section>

      <section class="table-grid">
        <el-card class="panel-card" shadow="never">
          <template #header>
            <div class="panel-head">
              <div>
                <h2>最近 Webhook</h2>
                <span>投递、解析和入队状态</span>
              </div>
            </div>
          </template>
          <el-table v-loading="loading" :data="snapshot?.recent_webhooks || []" empty-text="暂无 Webhook 记录">
            <el-table-column label="平台" min-width="86">
              <template #default="{ row }">
                <span class="provider-mark">{{ getProviderInitial(row.provider) }}</span>
              </template>
            </el-table-column>
            <el-table-column prop="event_type" label="事件" min-width="130" />
            <el-table-column prop="action" label="动作" min-width="100" />
            <el-table-column prop="delivery_id" label="投递 ID" min-width="160" show-overflow-tooltip />
            <el-table-column label="状态" min-width="110">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="收到时间" min-width="160">
              <template #default="{ row }">{{ formatDateTime(row.received_at) }}</template>
            </el-table-column>
            <el-table-column label="处理时间" min-width="160">
              <template #default="{ row }">{{ row.processed_at ? formatDateTime(row.processed_at) : '-' }}</template>
            </el-table-column>
            <el-table-column prop="error" label="错误" min-width="180" show-overflow-tooltip />
          </el-table>
        </el-card>

        <el-card class="panel-card" shadow="never">
          <template #header>
            <div class="panel-head">
              <div>
                <h2>最近写回</h2>
                <span>评论发布与重试状态</span>
              </div>
            </div>
          </template>
          <el-table v-loading="loading" :data="snapshot?.recent_writebacks || []" empty-text="暂无写回记录">
            <el-table-column prop="review_id" label="审查 ID" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">
                <router-link class="review-link" :to="`/reviews/${row.review_id}`">{{ row.review_id }}</router-link>
              </template>
            </el-table-column>
            <el-table-column prop="kind" label="类型" min-width="90">
              <template #default="{ row }">{{ kindLabel(row.kind) }}</template>
            </el-table-column>
            <el-table-column label="状态" min-width="110">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small">{{ statusLabel(row.status) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="评论链接" min-width="110">
              <template #default="{ row }">
                <a v-if="row.provider_url" :href="row.provider_url" target="_blank" rel="noopener" class="provider-link">
                  查看 <el-icon><TopRight /></el-icon>
                </a>
                <span v-else class="muted-value">-</span>
              </template>
            </el-table-column>
            <el-table-column prop="attempts" label="尝试" min-width="80" />
            <el-table-column label="发布时间" min-width="160">
              <template #default="{ row }">{{ row.published_at ? formatDateTime(row.published_at) : '-' }}</template>
            </el-table-column>
            <el-table-column prop="last_error" label="错误" min-width="200" show-overflow-tooltip />
          </el-table>
        </el-card>
      </section>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  CircleCheckFilled,
  CircleCloseFilled,
  Connection,
  InfoFilled,
  Promotion,
  Refresh,
  TopRight,
  WarningFilled,
} from '@element-plus/icons-vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import StatStrip from '@/components/common/StatStrip.vue'
import { getDiagnostics } from '@/api/diagnostics'
import type { DiagnosticsSnapshot } from '@/types/diagnostics'
import { formatDateTime, getProviderInitial } from '@/utils/format'

const loading = ref(false)
const hours = ref(24)
const snapshot = ref<DiagnosticsSnapshot | null>(null)

const webhookAccepted = computed(() => snapshot.value?.webhooks.accepted || 0)
const webhookErrors = computed(() => snapshot.value?.webhooks.error || 0)
const webhookIgnored = computed(() => snapshot.value?.webhooks.ignored || 0)
const webhookProblems = computed(() => webhookErrors.value + webhookIgnored.value)
const writebackPublished = computed(() => snapshot.value?.writebacks.published || 0)
const writebackProblems = computed(() => (snapshot.value?.writebacks.failed || 0) + (snapshot.value?.writebacks.pending || 0))

const diagnosticStats = computed(() => [
  { icon: Connection, label: 'Webhook 成功', value: webhookAccepted.value, sublabel: `${snapshot.value?.since_hours || hours.value} 小时窗口`, tone: 'primary' as const },
  { icon: WarningFilled, label: 'Webhook 异常 / 忽略', value: webhookProblems.value, sublabel: (webhookErrors.value || webhookIgnored.value) ? `${webhookErrors.value} 错误 · ${webhookIgnored.value} 忽略` : '错误 / 忽略', tone: 'warning' as const },
  { icon: Promotion, label: '写回成功', value: writebackPublished.value, sublabel: '已发布评论或总结', tone: 'success' as const },
  { icon: CircleCloseFilled, label: '写回待处理', value: writebackProblems.value, sublabel: '失败 / 待处理', tone: 'danger' as const },
])

function statusTag(status: string) {
  if (['accepted', 'published', 'success'].includes(status)) return 'success'
  if (['failed', 'error'].includes(status)) return 'danger'
  if (['pending', 'received'].includes(status)) return 'warning'
  return 'info'
}

/**
 * 状态列文案。取值域来自后端诊断快照的两组统计键：
 * - Webhook 投递：received / accepted / ignored / error
 * - 写回条目：pending / published / failed / downgraded
 * 未知取值不直出原始串。
 */
function statusLabel(status: string): string {
  const map: Record<string, string> = {
    received: '已接收',
    accepted: '已受理',
    ignored: '已忽略',
    error: '异常',
    pending: '待处理',
    published: '已发布',
    failed: '失败',
    downgraded: '已降级',
    success: '成功',
  }
  return map[status] || '未知'
}

/** 写回条目类型文案。 */
function kindLabel(kind: string): string {
  const map: Record<string, string> = {
    inline: '行内评论',
    summary: '总结评论',
  }
  return map[kind] || kind || '-'
}

async function fetchSnapshot() {
  loading.value = true
  try {
    snapshot.value = await getDiagnostics({ hours: hours.value, limit: 30 })
  } finally {
    loading.value = false
  }
}

onMounted(fetchSnapshot)
</script>

<style scoped>
.hours-select {
  width: 150px;
}

.recommendations {
  display: grid;
  gap: 10px;
  margin-bottom: 18px;
}

.recommendation {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 14px 16px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #eff6ff;
}

.recommendation.danger {
  border-color: #fecaca;
  background: #fef2f2;
}

.recommendation.warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.recommendation.success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.recommendation-icon {
  margin-top: 2px;
  font-size: 20px;
}

.recommendation strong,
.recommendation small {
  display: block;
}

.recommendation small {
  margin-top: 4px;
  color: #4b5563;
}

.action-text {
  color: #1d4ed8 !important;
  font-weight: 600;
}

.table-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr);
  gap: 16px;
}

.panel-card {
  border-radius: 8px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  gap: 16px;
}

.panel-head h2 {
  font-size: 18px;
}

.panel-head span {
  display: block;
  margin-top: 4px;
  color: #6b7280;
  font-size: 13px;
}

.provider-mark {
  display: inline-grid;
  min-width: 34px;
  height: 26px;
  place-items: center;
  border-radius: 6px;
  color: #fff;
  background: #111827;
  font-size: 12px;
  font-weight: 700;
}

.review-link {
  color: #2563eb;
  font-weight: 600;
  text-decoration: none;
}

.provider-link {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  color: #2563eb;
  font-size: 13px;
  text-decoration: none;
}
.provider-link:hover { text-decoration: underline; }

.muted-value {
  color: #9ca3af;
}

@media (max-width: 720px) {
  .hours-select {
    width: 100%;
  }
}
</style>
