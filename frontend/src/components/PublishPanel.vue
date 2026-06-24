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
      <span class="mode-label">评论语气</span>
      <el-radio-group
        v-model="mode"
        size="small"
      >
        <el-radio-button value="team">
          团队评审
        </el-radio-button>
        <el-radio-button value="human">
          拟人化改写
        </el-radio-button>
      </el-radio-group>
      <span class="mode-hint">{{ mode === 'human' ? 'LLM 改写为第一人称自然语言' : '保持多 Agent 原始结构' }}</span>
    </div>

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
          更新已发布评论
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
import { ref, watch } from 'vue'
import { Promotion, Refresh } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { previewComment, publishReview, updatePublishedComment } from '@/api/review'

const props = defineProps<{
  reviewId: string
  published?: boolean
}>()

const emit = defineEmits<{
  (e: 'published'): void
}>()

const mode = ref<'team' | 'human'>('team')
const commentScope = ref<'inline' | 'summary'>('inline')
const previewLoading = ref(false)
const publishing = ref(false)
const published = ref(props.published ?? false)
const previewData = ref<{ review_id: string; summary: string; findings: string; total: number } | null>(null)

watch(() => props.published, (v) => {
  published.value = Boolean(v)
})

async function loadPreview() {
  previewLoading.value = true
  try {
    previewData.value = await previewComment(props.reviewId, mode.value)
  } catch {
    previewData.value = null
  } finally {
    previewLoading.value = false
  }
}

watch(mode, () => {
  if (previewData.value) loadPreview()
})
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
    await publishReview(props.reviewId, mode.value, commentScope.value === 'summary')
    published.value = true
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
    ElMessage.success('已更新已发布评论')
    emit('published')
  } catch {
    ElMessage.error('更新失败，请稍后重试')
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
