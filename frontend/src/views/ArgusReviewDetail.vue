<template>
  <AppLayout>
    <PageShell>
      <PageHeader
        eyebrow="ARGUS"
        :title="headerTitle"
        :description="headerDesc"
      >
        <template #actions>
          <el-button :disabled="!review?.argus_task_id" @click="openReport">完整报告</el-button>
          <el-button type="primary" plain :disabled="!review?.argus_task_id" @click="openAsk">问 Argus</el-button>
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
                v-else
                link
                type="danger"
                size="small"
                :loading="suppressing === (row.fp || row.file + ':' + row.line)"
                @click="onSuppress(row)"
              >
                误报抑制
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-dialog v-model="reportVisible" title="Argus 审查报告" width="72%" top="6vh" destroy-on-close>
        <div v-loading="reportLoading" class="argus-report-body">
          <el-alert
            v-if="reportError"
            type="error"
            :closable="false"
            show-icon
            :title="reportError"
          />
          <div
            v-else-if="reportHtml"
            class="markdown-body"
            v-html="reportHtml"
          />
          <el-empty v-else description="argus 侧暂无该任务的报告全文" />
        </div>
      </el-dialog>

      <el-dialog v-model="askVisible" title="问 Argus" width="60%" top="8vh" destroy-on-close>
        <div class="argus-ask-context">
          基于 argus 对 <strong>{{ review?.owner }}/{{ review?.repo }} #{{ review?.number }}</strong> 的最近一次审查报告回答追问。
        </div>
        <el-input
          v-model="askQuestion"
          type="textarea"
          :rows="3"
          maxlength="2000"
          show-word-limit
          placeholder="例如：为什么这条是 high？这个结论考虑了哪些文件？"
        />
        <div class="argus-ask-quick">
          <el-button
            v-for="q in quickQuestions"
            :key="q"
            size="small"
            plain
            @click="askQuestion = q"
          >
            {{ q }}
          </el-button>
        </div>
        <div v-loading="askLoading" class="argus-ask-answer">
          <div v-if="askAnswerHtml" class="markdown-body" v-html="askAnswerHtml" />
          <el-empty v-else-if="askAsked" :description="askError || '本次没有返回内容'" />
        </div>
        <template #footer>
          <el-button @click="askVisible = false">关闭</el-button>
          <el-button type="primary" :loading="askLoading" :disabled="!askQuestion.trim()" @click="onAsk">
            提问
          </el-button>
        </template>
      </el-dialog>
    </PageShell>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageShell from '@/components/common/PageShell.vue'
import PageHeader from '@/components/common/PageHeader.vue'
import {
  askArgus,
  getArgusReview,
  getArgusReviewReport,
  suppressArgusFinding,
  suppressArgusFindingByComment,
} from '@/api/argus'
import { formatDateShort, formatClock } from '@/utils/format'
import { renderMarkdown } from '@/utils/markdown'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import type { ArgusFinding, ArgusReview } from '@/types/argus'

const route = useRoute()
const router = useRouter()
const review = ref<ArgusReview | null>(null)
const loading = ref(false)
const suppressing = ref('')

// 报告弹窗（懒拉取：argus 侧为报告单一真源，本站不落库）
const reportVisible = ref(false)
const reportLoading = ref(false)
const reportError = ref('')
const reportHtml = ref('')

// 问答弹窗（服务端代理 argus MCP ask_argus，90s 语义）
const askVisible = ref(false)
const askLoading = ref(false)
const askQuestion = ref('')
const askAnswerHtml = ref('')
const askError = ref('')
const askAsked = ref(false)
const quickQuestions = [
  '总结这次审查的结论',
  '为什么存在 high 级意见？',
  '还有哪些没审到的地方？',
]

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
  if (!review.value) return
  // fp 缺失（旧数据/轮询恢复）时降级 file+comment 兜底模式（服务端按原文匹配）。
  const key = row.fp || row.file + ':' + row.line
  const fpBrief = row.fp ? (row.fp.length > 16 ? row.fp.slice(0, 16) + '…' : row.fp) : `${row.file}:${row.line}`
  try {
    await ElMessageBox.confirm(
      `确认将该意见标记为误报？argus 下一轮审查起将自动剔除该意见（${fpBrief}）。`,
      '误报抑制',
      { confirmButtonText: '确认抑制', cancelButtonText: '取消', type: 'warning' },
    )
  } catch {
    return
  }
  suppressing.value = key
  try {
    const res = row.fp
      ? await suppressArgusFinding(review.value.id, row.fp)
      : await suppressArgusFindingByComment(review.value.id, row.file, row.comment)
    if (res?.matched) {
      ElMessage.success('已抑制：argus 下一轮审查起自动剔除该意见')
      review.value = await getArgusReview(review.value.id)
    } else {
      ElMessage.warning(res?.message || 'argus 侧未命中该意见')
    }
  } catch (e: any) {
    ElMessage.error(e?.message || '抑制失败，请重试')
  } finally {
    suppressing.value = ''
  }
}

async function openReport() {
  if (!review.value) return
  reportVisible.value = true
  reportLoading.value = true
  reportError.value = ''
  reportHtml.value = ''
  try {
    const rep = await getArgusReviewReport(review.value.id)
    reportHtml.value = renderMarkdown(rep?.report || '')
    if (!rep?.report) reportError.value = ''
  } catch (e: any) {
    reportError.value = e?.message || '报告拉取失败'
  } finally {
    reportLoading.value = false
  }
}

function openAsk() {
  askVisible.value = true
  askQuestion.value = ''
  askAnswerHtml.value = ''
  askError.value = ''
  askAsked.value = false
}

async function onAsk() {
  if (!review.value || !askQuestion.value.trim()) return
  askLoading.value = true
  askAsked.value = true
  askError.value = ''
  askAnswerHtml.value = ''
  try {
    const res = await askArgus({
      platform: review.value.platform,
      owner: review.value.owner,
      repo: review.value.repo,
      number: review.value.number,
      question: askQuestion.value.trim(),
    })
    askAnswerHtml.value = renderMarkdown(res?.answer || '')
  } catch (e: any) {
    askError.value = e?.message || 'argus 问答失败，请稍后重试'
  } finally {
    askLoading.value = false
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
.argus-report-body {
  min-height: 200px;
  max-height: 68vh;
  overflow-y: auto;
}
.argus-ask-context {
  color: var(--el-text-color-secondary);
  font-size: 13px;
  margin-bottom: 10px;
}
.argus-ask-quick {
  display: flex;
  gap: 8px;
  margin: 10px 0 14px;
  flex-wrap: wrap;
}
.argus-ask-answer {
  min-height: 120px;
  max-height: 46vh;
  overflow-y: auto;
}
.markdown-body :deep(h1),
.markdown-body :deep(h2),
.markdown-body :deep(h3) {
  margin: 14px 0 8px;
  font-size: 15px;
}
.markdown-body :deep(h1) {
  font-size: 17px;
}
.markdown-body :deep(p) {
  margin: 6px 0;
  line-height: 1.7;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  margin: 6px 0;
  padding-left: 20px;
}
.markdown-body :deep(li) {
  margin: 3px 0;
  line-height: 1.6;
}
.markdown-body :deep(code) {
  font-family: var(--el-font-family-mono, monospace);
  font-size: 12px;
  background: var(--el-fill-color-light);
  padding: 1px 5px;
  border-radius: 4px;
}
.markdown-body :deep(pre) {
  background: var(--el-fill-color-light);
  padding: 10px 12px;
  border-radius: 6px;
  overflow: auto;
  font-size: 12px;
}
.markdown-body :deep(table) {
  border-collapse: collapse;
  margin: 8px 0;
  width: 100%;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--el-border-color-lighter);
  padding: 5px 9px;
  font-size: 12px;
  text-align: left;
}
</style>
