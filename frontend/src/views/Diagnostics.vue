<template>
  <AppLayout>
    <div class="diagnostics-page">
      <header class="page-header">
        <div>
          <div class="eyebrow">OPERATIONS DIAGNOSTICS</div>
          <h1>诊断中心</h1>
          <p>集中查看 Webhook 投递、写回发布和近期故障线索。</p>
        </div>
        <div class="header-actions">
          <el-select v-model="hours" class="hours-select" @change="fetchSnapshot">
            <el-option label="最近 24 小时" :value="24" />
            <el-option label="最近 7 天" :value="168" />
            <el-option label="最近 30 天" :value="720" />
          </el-select>
          <el-button plain :loading="loading" @click="fetchSnapshot">
            <el-icon><Refresh /></el-icon>
            刷新
          </el-button>
        </div>
      </header>

      <section class="metric-grid">
        <div class="metric-card">
          <span class="metric-icon webhooks"><el-icon><Connection /></el-icon></span>
          <span>Webhook 成功</span>
          <strong>{{ webhookAccepted }}</strong>
          <small>{{ snapshot?.since_hours || hours }} 小时窗口</small>
        </div>
        <div class="metric-card">
          <span class="metric-icon warning"><el-icon><WarningFilled /></el-icon></span>
          <span>Webhook 异常 / 忽略</span>
          <strong>{{ webhookProblems }}</strong>
          <small v-if="webhookErrors || webhookIgnored">{{ webhookErrors }} 错误 · {{ webhookIgnored }} 忽略</small>
          <small v-else>error / ignored</small>
        </div>
        <div class="metric-card">
          <span class="metric-icon published"><el-icon><Promotion /></el-icon></span>
          <span>写回成功</span>
          <strong>{{ writebackPublished }}</strong>
          <small>已发布评论或总结</small>
        </div>
        <div class="metric-card">
          <span class="metric-icon failed"><el-icon><CircleCloseFilled /></el-icon></span>
          <span>写回待处理</span>
          <strong>{{ writebackProblems }}</strong>
          <small>failed / pending</small>
        </div>
      </section>

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
            <el-table-column prop="delivery_id" label="Delivery" min-width="160" show-overflow-tooltip />
            <el-table-column label="状态" min-width="110">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small">{{ row.status }}</el-tag>
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
            <el-table-column prop="review_id" label="Review" min-width="160" show-overflow-tooltip>
              <template #default="{ row }">
                <router-link class="review-link" :to="`/reviews/${row.review_id}`">{{ row.review_id }}</router-link>
              </template>
            </el-table-column>
            <el-table-column prop="kind" label="类型" min-width="90" />
            <el-table-column label="状态" min-width="110">
              <template #default="{ row }">
                <el-tag :type="statusTag(row.status)" size="small">{{ row.status }}</el-tag>
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
    </div>
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

function statusTag(status: string) {
  if (['accepted', 'published', 'success'].includes(status)) return 'success'
  if (['failed', 'error'].includes(status)) return 'danger'
  if (['pending', 'received'].includes(status)) return 'warning'
  return 'info'
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
.diagnostics-page {
  min-height: 100vh;
  padding: 28px;
  color: #111827;
  background: #f6f7f9;
}

.page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20px;
  margin-bottom: 22px;
}

.eyebrow {
  color: #6b7280;
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0;
}

h1,
h2,
p {
  margin: 0;
}

h1 {
  margin-top: 6px;
  font-size: 30px;
}

.page-header p {
  margin-top: 8px;
  color: #6b7280;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.hours-select {
  width: 150px;
}

.metric-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
  margin-bottom: 18px;
}

.metric-card {
  display: grid;
  gap: 7px;
  padding: 18px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.metric-icon {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  border-radius: 8px;
  color: #fff;
  font-size: 19px;
}

.metric-icon.webhooks {
  background: #2563eb;
}

.metric-icon.warning {
  background: #d97706;
}

.metric-icon.published {
  background: #059669;
}

.metric-icon.failed {
  background: #dc2626;
}

.metric-card span:not(.metric-icon) {
  color: #6b7280;
  font-size: 13px;
}

.metric-card strong {
  font-size: 28px;
}

.metric-card small {
  color: #9ca3af;
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

@media (max-width: 1080px) {
  .metric-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 720px) {
  .diagnostics-page {
    padding: 18px;
  }

  .page-header,
  .header-actions {
    flex-direction: column;
    align-items: stretch;
  }

  .metric-grid {
    grid-template-columns: 1fr;
  }

  .hours-select {
    width: 100%;
  }
}
</style>
