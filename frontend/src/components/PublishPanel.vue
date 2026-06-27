<template>
  <el-card class="publish-panel">
    <template #header>
      <div class="card-header">
        <span>发布到代码平台</span>
        <div class="header-meta">
          <el-tag
            v-if="published"
            type="success"
            size="small"
          >
            已发布
          </el-tag>
          <el-tag
            v-else
            type="info"
            size="small"
          >
            未发布
          </el-tag>
        </div>
      </div>
    </template>

    <div class="mode-row">
      <span class="mode-label">评论范围</span>
      <el-radio-group
        v-model="commentScope"
        size="small"
      >
        <el-radio-button value="inline">
          行内评论 + 总结
        </el-radio-button>
        <el-radio-button value="summary">
          仅总结
        </el-radio-button>
      </el-radio-group>
      <span class="mode-hint">{{ commentScope === 'inline' ? '每条问题评论到代码对应行上（GitHub 风格）' : '只发布一条总结评论' }}</span>
    </div>

    <div v-if="publishStatus" class="status-strip">
      <span>记录 {{ publishStatus.total }}</span>
      <el-tag size="small" type="success">成功 {{ publishStatus.published }}</el-tag>
      <el-tag v-if="publishStatus.downgraded" size="small" type="warning">降级 {{ publishStatus.downgraded }}</el-tag>
      <el-tag v-if="publishStatus.failed" size="small" type="danger">失败 {{ publishStatus.failed }}</el-tag>
      <el-tag v-if="publishStatus.pending" size="small" type="info">等待 {{ publishStatus.pending }}</el-tag>
      <el-button size="small" text :loading="statusLoading" @click="loadStatus">刷新状态</el-button>
    </div>

    <div v-if="failedItems.length" class="failed-list">
      <div v-for="item in failedItems.slice(0, 3)" :key="item.finding_key" class="failed-item">
        <span>{{ item.finding_key }}</span>
        <small>{{ item.last_error || '发布失败' }}</small>
      </div>
    </div>

    <div class="preview-area">
      <div class="preview-toolbar">
        <span class="preview-title">评论预览</span>
        <el-button
          size="small"
          plain
          :loading="previewLoading"
          :disabled="!reviewId"
          @click="loadPreview"
        >
          <el-icon><Refresh /></el-icon>
          {{ previewData ? '刷新预览' : '生成预览' }}
        </el-button>
      </div>

      <el-empty
        v-if="!previewData && !previewLoading"
        description="点击「生成预览」查看将发布的评论内容"
        :image-size="60"
      />
      <div
        v-else
        v-loading="previewLoading"
        class="preview-body"
      >
        <div
          v-if="previewData"
          class="preview-stats"
        >
          <span>共 <strong>{{ previewData.total }}</strong> 条问题</span>
        </div>
        <div
          v-if="previewData?.summary"
          class="preview-block"
        >
          <small>总结</small>
          <pre>{{ previewData.summary }}</pre>
        </div>
        <div
          v-if="previewData?.findings"
          class="preview-block"
        >
          <small>问题清单</small>
          <pre>{{ previewData.findings }}</pre>
        </div>
      </div>
    </div>

    <div class="publish-actions">
      <el-button
        v-if="!published"
        type="primary"
        :loading="publishing"
        :disabled="!reviewId"
        @click="handlePublish"
      >
        <el-icon><Promotion /></el-icon>
        发布评论
      </el-button>
      <template v-else>
        <el-button
          :loading="publishing"
          :disabled="!reviewId"
          @click="handleUpdate"
        >
          <el-icon><Refresh /></el-icon>
          重新发布更新
        </el-button>
        <el-button
          v-if="publishStatus?.failed || publishStatus?.pending"
          type="warning"
          plain
          :loading="publishing"
          :disabled="!reviewId"
          @click="handleRetry"
        >
          重试失败项
        </el-button>
        <el-button
          type="danger"
          plain
          :loading="publishing"
          :disabled="!reviewId"
          @click="handlePublish"
        >
          重新发布
        </el-button>
      </template>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Promotion, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { getPublishStatus, previewComment, publishReview, retryPublish, updatePublishedComment } from '@/api/review'
import type { PublishStatus } from '@/types/review'

const props = defineProps<{
  reviewId: string
  published?: boolean
}>()

const emit = defineEmits<{
  (e: 'published'): void
}>()

const commentScope = ref<'inline' | 'summary'>('inline')
const previewLoading = ref(false)
const statusLoading = ref(false)
const publishing = ref(false)
const published = ref(props.published ?? false)
const previewData = ref<{ review_id: string; summary: string; findings: string; total: number } | null>(null)
const publishStatus = ref<PublishStatus | null>(null)
const failedItems = computed(() => (publishStatus.value?.items || []).filter((item) => item.status === 'failed' || item.status === 'pending'))

watch(() => props.published, (v) => {
  published.value = Boolean(v)
})

onMounted(() => {
  loadStatus()
})

async function loadPreview() {
  previewLoading.value = true
  try {
    previewData.value = await previewComment(props.reviewId)
  } catch {
    previewData.value = null
  } finally {
    previewLoading.value = false
  }
}

async function loadStatus() {
  if (!props.reviewId) return
  statusLoading.value = true
  try {
    publishStatus.value = await getPublishStatus(props.reviewId)
    if (publishStatus.value.total > 0) {
      published.value = true
    }
  } finally {
    statusLoading.value = false
  }
}

watch(commentScope, () => {
  if (previewData.value) loadPreview()
})

async function handlePublish() {
  try {
    await ElMessageBox.confirm(
      '发布后会将评审评论写到代码平台的 PR/MR 上，此操作会暴露在仓库中。确认发布？',
      '发布评论',
      { type: 'warning', confirmButtonText: '确认发布', cancelButtonText: '取消' }
    )
  } catch {
    return
  }
  publishing.value = true
  try {
    await publishReview(props.reviewId, undefined, commentScope.value === 'summary')
    published.value = true
    await loadStatus()
    ElMessage.success('评论已发布到代码平台')
    emit('published')
  } catch {
    ElMessage.error('发布失败，请稍后重试')
  } finally {
    publishing.value = false
  }
}

async function handleUpdate() {
  publishing.value = true
  try {
    await updatePublishedComment(props.reviewId)
    await loadStatus()
    ElMessage.success('已重新发布并刷新状态')
    emit('published')
  } catch {
    ElMessage.error('更新失败，请稍后重试')
  } finally {
    publishing.value = false
  }
}

async function handleRetry() {
  publishing.value = true
  try {
    await retryPublish(props.reviewId)
    await loadStatus()
    published.value = true
    ElMessage.success('已重试失败发布项')
    emit('published')
  } catch {
    ElMessage.error('重试失败，请稍后重试')
  } finally {
    publishing.value = false
  }
}
</script>

<style scoped>
.publish-panel {
  margin-bottom: 16px;
  border: 1px solid var(--app-border, #e4e7ed);
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.card-header > span:first-child {
  color: var(--app-text, #303133);
  font-weight: 700;
}

.mode-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.mode-label {
  color: var(--app-text, #303133);
  font-size: 13px;
  font-weight: 600;
}

.mode-hint {
  color: #909399;
  font-size: 12px;
}

.status-strip {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  color: #606266;
  font-size: 12px;
}

.failed-list {
  display: grid;
  gap: 8px;
  margin-bottom: 14px;
}

.failed-item {
  display: grid;
  gap: 4px;
  padding: 8px 10px;
  border: 1px solid #fed7aa;
  border-radius: 6px;
  background: #fff7ed;
}

.failed-item span {
  overflow-wrap: anywhere;
  color: #9a3412;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
}

.failed-item small {
  overflow-wrap: anywhere;
  color: #c2410c;
}

.preview-area {
  background: #fafbfc;
  border: 1px solid var(--app-border, #edf0f4);
  border-radius: 8px;
  padding: 14px;
  margin-bottom: 16px;
}

.preview-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 10px;
}

.preview-title {
  color: var(--app-text, #303133);
  font-size: 13px;
  font-weight: 600;
}

.preview-stats {
  margin-bottom: 10px;
  color: #606266;
  font-size: 12px;
}

.preview-block {
  margin-bottom: 12px;
}

.preview-block:last-child {
  margin-bottom: 0;
}

.preview-block small {
  display: block;
  margin-bottom: 4px;
  color: #909399;
  font-size: 11px;
}

.preview-block pre {
  margin: 0;
  padding: 10px;
  background: #fff;
  border: 1px solid var(--app-border, #edf0f4);
  border-radius: 6px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
  color: var(--app-text, #303133);
  max-height: 240px;
  overflow-y: auto;
}

.publish-actions {
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
}

@media (max-width: 720px) {
  .mode-row {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
