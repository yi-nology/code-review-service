<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="ARGUS"
        :title="headerTitle"
        :description="headerDesc"
      >
        <template #actions>
          <el-button @click="router.back()">返回</el-button>
        </template>
      </PageHeader>

      <el-empty v-if="!loading && !review" description="未找到这条 Argus 审查任务">
        <el-button @click="router.back()">返回</el-button>
      </el-empty>

      <el-card v-else v-loading="loading" class="argus-detail-card">
        <el-alert
          v-if="review?.verdict === 'error'"
          class="argus-error-alert"
          type="error"
          :closable="false"
          show-icon
          :title="review?.error_message || 'Argus 侧审查失败，未给出原因'"
        />
        <el-descriptions :column="3" border>
          <el-descriptions-item label="平台">{{ review?.platform || '—' }}</el-descriptions-item>
          <el-descriptions-item label="项目">{{ review?.owner }}/{{ review?.repo }}</el-descriptions-item>
          <el-descriptions-item label="PR">#{{ review?.number }} · {{ review?.title || '（无标题）' }}</el-descriptions-item>
          <el-descriptions-item label="裁决">
            <el-tag :type="verdictTagType(review?.verdict)">{{ verdictLabel(review?.verdict) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="head SHA">
            <span class="argus-mono">{{ review?.head_sha?.slice(0, 10) }}</span>
          </el-descriptions-item>
          <el-descriptions-item label="基准分支">{{ review?.base_branch || '—' }}</el-descriptions-item>
          <el-descriptions-item label="审查时间">
            {{ review ? formatDateShort(review.occurred_at) + ' ' + formatClock(review.occurred_at) : '' }}
          </el-descriptions-item>
          <el-descriptions-item label="审查专家">{{ (review?.plugins || []).join(' / ') || '—' }}</el-descriptions-item>
          <el-descriptions-item label="耗时">{{ durationLabel }}</el-descriptions-item>
          <el-descriptions-item label="Argus 任务">
            <span class="argus-mono">{{ review?.argus_task_id || '—' }}</span>
          </el-descriptions-item>
        </el-descriptions>

        <div class="argus-findings-head">
          <h3>审查意见（{{ review?.findings?.length ?? 0 }}）</h3>
        </div>

        <el-empty v-if="review && !review.findings?.length" description="本次审查零意见" />
        <el-table v-else :data="review?.findings ?? []" class="argus-finding-table">
          <el-table-column label="位置" min-width="200">
            <template #default="{ row }">
              <span class="argus-mono">{{ row.file }}{{ row.line ? ':' + row.line : '' }}</span>
            </template>
          </el-table-column>
          <el-table-column label="风险" width="90">
            <template #default="{ row }">
              <el-tag :type="sevTagType(row.severity)" size="small">{{ sevLabel(row.severity) }}</el-tag>
            </template>
          </el-table-column>
          <el-table-column label="意见" min-width="380">
            <template #default="{ row }">
              <div class="argus-comment">{{ row.comment }}</div>
              <pre v-if="row.suggested_fix" class="argus-fix"><code>{{ row.suggested_fix }}</code></pre>
            </template>
          </el-table-column>
          <el-table-column label="来源" width="120">
            <template #default="{ row }">{{ row.by || '—' }}</template>
          </el-table-column>
          <el-table-column label="反馈" width="140">
            <template #default="{ row }">
              <el-tag v-if="row.suppress_state === 'suppressed'" type="success" size="small">已抑制</el-tag>
              <el-button
                v-else-if="row.fp"
                link
                type="danger"
                size="small"
                :loading="suppressing === row.fp"
                @click="onSuppress(row)"
              >
                误报抑制
              </el-button>
              <span v-else class="argus-mono">—</span>
            </template>
          </el-table-column>
        </el-table>
      </el-card>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import { getArgusReview, suppressArgusFinding } from '@/api/argus'
import { formatDateShort, formatClock } from '@/utils/format'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import type { ArgusFinding, ArgusReview } from '@/types/argus'

const route = useRoute()
const router = useRouter()
const review = ref<ArgusReview | null>(null)
const loading = ref(false)
const suppressing = ref('')

const headerTitle = computed(() =>
  review.value ? `${review.value.owner}/${review.value.repo} #${review.value.number}` : 'Argus 审查详情',
)
const headerDesc = computed(() => review.value?.title || '审查任务意见明细。')

const durationLabel = computed(() => {
  const ms = review.value?.duration_ms ?? 0
  if (!ms) return '—'
  return ms >= 1000 ? `${(ms / 1000).toFixed(1)}s` : `${ms}ms`
})

function verdictLabel(v?: string): string {
  switch (v) {
    case 'approve': return '通过'
    case 'comment': return '评论'
    case 'request_changes': return '要求修改'
    case 'error': return '审查失败'
    default: return v || '—'
  }
}

function verdictTagType(v?: string): 'success' | 'warning' | 'danger' | 'info' {
  switch (v) {
    case 'approve': return 'success'
    case 'comment': return 'warning'
    case 'request_changes': return 'danger'
    default: return 'info'
  }
}

function sevLabel(s: string): string {
  switch (s) {
    case 'high': return 'High'
    case 'medium': return 'Medium'
    case 'low': return 'Low'
    default: return s
  }
}

function sevTagType(s: string): 'danger' | 'warning' | 'info' {
  switch (s) {
    case 'high': return 'danger'
    case 'medium': return 'warning'
    default: return 'info'
  }
}

onMounted(async () => {
  loading.value = true
  try {
    review.value = await getArgusReview(route.params.id as string)
  } catch {
    // 404/加载失败：走空态兜底（错误提示由 request 拦截器按需弹出）
    review.value = null
  } finally {
    loading.value = false
  }
})

async function onSuppress(row: ArgusFinding) {
  if (!review.value || !row.fp) return
  const fpBrief = row.fp.length > 16 ? row.fp.slice(0, 16) + '…' : row.fp
  try {
    await ElMessageBox.confirm(
      `确认将该意见标记为误报？argus 下一轮审查起将自动剔除指纹 ${fpBrief} 命中的意见。`,
      '误报抑制',
      { confirmButtonText: '确认抑制', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return
  }
  suppressing.value = row.fp
  try {
    const res = await suppressArgusFinding(review.value.id, row.fp)
    if (res?.matched) {
      ElMessage.success('已抑制：argus 下一轮审查起自动剔除该意见')
      review.value = await getArgusReview(review.value.id)
    } else {
      ElMessage.warning(res?.message || 'argus 侧未命中该指纹')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '抑制失败，请重试')
  } finally {
    suppressing.value = ''
  }
}
</script>

<style scoped>
.argus-detail-card {
  width: 100%;
}
.argus-error-alert {
  margin-bottom: 16px;
}
.argus-mono {
  font-family: var(--el-font-family-mono, monospace);
  font-size: 12px;
}
.argus-findings-head {
  margin: 20px 0 12px;
}
.argus-finding-table {
  width: 100%;
}
.argus-comment {
  white-space: pre-wrap;
  word-break: break-word;
  max-height: 260px;
  overflow-y: auto;
}
.argus-fix {
  margin: 8px 0 0;
  padding: 10px 12px;
  background: var(--el-fill-color-light);
  border-radius: 6px;
  overflow: auto;
  font-size: 12px;
  max-height: 240px;
}
</style>
