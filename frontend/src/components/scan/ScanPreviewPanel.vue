<script setup lang="ts">
import { ref, computed } from 'vue'
import { Document, FolderOpened } from '@element-plus/icons-vue'

const props = defineProps<{
  files: string[]
}>()

const emit = defineEmits<{
  (e: 'clear'): void
}>()

const previewFilter = ref('')

interface PreviewInfo {
  totalChanged: number
  reviewable: string[]
  excluded: string[]
}

// normalizePreviewOutput 解析预览文件列表：区分可审查 / 已排除。
// 格式约定：「Excluded from review:」前缀的行归入 excluded，其余为 reviewable。
function parsePreviewLine(line: string): string {
  let value = line.trim()
  if (!value) return ''
  if (value.startsWith('[')) {
    const end = value.indexOf(']')
    if (end >= 0) value = value.slice(end + 1).trim()
  }
  return value
}

function stripAnsi(value: string): string {
  const escapeChar = String.fromCharCode(27)
  return value.replace(new RegExp(`${escapeChar}\\[[0-9;?]*[ -/]*[@-~]`, 'g'), '')
}

const previewInfo = computed<PreviewInfo>(() => {
  const reviewable: string[] = []
  const excluded: string[] = []
  for (const raw of props.files) {
    const line = parsePreviewLine(stripAnsi(raw))
    if (!line) continue
    if (/^Excluded from review/i.test(line)) {
      excluded.push(line.replace(/^Excluded from review:?\s*/i, ''))
    } else {
      reviewable.push(line)
    }
  }
  return {
    totalChanged: reviewable.length + excluded.length,
    reviewable: Array.from(new Set(reviewable)),
    excluded: Array.from(new Set(excluded)),
  }
})

const filteredReviewable = computed(() => {
  if (!previewFilter.value) return previewInfo.value.reviewable
  const q = previewFilter.value.toLowerCase()
  return previewInfo.value.reviewable.filter((f) => f.toLowerCase().includes(q))
})

const filteredExcluded = computed(() => {
  if (!previewFilter.value) return previewInfo.value.excluded
  const q = previewFilter.value.toLowerCase()
  return previewInfo.value.excluded.filter((f) => f.toLowerCase().includes(q))
})
</script>

<template>
  <div class="preview-section">
    <div class="section-head">
      <div class="section-title">
        <el-icon class="section-icon"><FolderOpened /></el-icon>
        <h2>文件预览</h2>
        <span class="section-count">{{ previewInfo.reviewable.length }} 个文件待审查</span>
      </div>
      <div class="section-actions">
        <el-input
          v-model="previewFilter"
          size="small"
          placeholder="搜索文件…"
          style="width: 200px"
          clearable
        />
        <el-button text type="primary" @click="emit('clear')">清空</el-button>
      </div>
    </div>

    <div class="preview-summary" :class="{ 'has-excluded': previewInfo.excluded.length > 0 }">
      <div class="preview-stat">
        <span>总变更</span>
        <strong>{{ previewInfo.totalChanged }}</strong>
      </div>
      <div class="preview-stat primary">
        <span>将审查</span>
        <strong>{{ filteredReviewable.length }}</strong>
      </div>
      <div v-if="previewInfo.excluded.length" class="preview-stat">
        <span>已排除</span>
        <strong>{{ previewInfo.excluded.length }}</strong>
      </div>
    </div>

    <div class="file-list" :class="{ empty: filteredReviewable.length === 0 }">
      <div v-for="file in filteredReviewable" :key="file" class="file-row">
        <span class="file-badge"><el-icon><Document /></el-icon></span>
        <code>{{ file }}</code>
      </div>
      <div v-if="filteredReviewable.length === 0" class="file-list-empty">
        没有匹配的可审查文件
      </div>
    </div>

    <el-collapse v-if="filteredExcluded.length" class="excluded-collapse">
      <el-collapse-item :title="`已排除文件 (${filteredExcluded.length})`" name="excluded">
        <div class="file-list excluded-list">
          <div v-for="file in filteredExcluded" :key="file" class="file-row">
            <span class="file-badge muted">排除</span>
            <code>{{ file }}</code>
          </div>
        </div>
      </el-collapse-item>
    </el-collapse>
  </div>
</template>

<style scoped>
.preview-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.section-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.section-icon {
  color: var(--app-primary);
}

.section-title h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: var(--app-text);
}

.section-count {
  color: var(--app-muted);
  font-size: 12px;
}

.section-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.preview-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1px;
  overflow: hidden;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-border);
}

.preview-summary.has-excluded {
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.preview-stat {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 12px 16px;
  background: var(--app-surface);
}

.preview-stat span {
  display: block;
  color: var(--app-muted);
  font-size: 12px;
  font-weight: 700;
}

.preview-stat strong {
  display: block;
  color: var(--app-text);
  font-size: 22px;
  line-height: 1;
}

.preview-stat.primary strong {
  color: var(--app-primary);
}

.file-list {
  display: flex;
  flex-direction: column;
  max-height: 320px;
  overflow-y: auto;
  border: 1px solid var(--app-border);
  border-radius: 10px;
  background: var(--app-bg);
}

.file-list.empty {
  align-items: center;
  justify-content: center;
  min-height: 80px;
}

.file-list-empty {
  color: var(--app-muted);
  font-size: 13px;
}

.file-row {
  display: grid;
  grid-template-columns: 40px minmax(0, 1fr);
  gap: 10px;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--app-border);
  transition: background 140ms ease;
}

.file-row:last-child {
  border-bottom: 0;
}

.file-row:hover {
  background: var(--app-surface);
}

.file-row code {
  overflow: hidden;
  color: var(--app-text);
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 24px;
  border-radius: 6px;
  background: #dff7f1;
  color: #0f766e;
  font-size: 11px;
  font-weight: 800;
}

.file-badge.muted {
  background: var(--app-bg);
  color: var(--app-muted);
}

.excluded-collapse {
  border: 1px solid var(--app-border);
  border-radius: 10px;
}

.excluded-collapse :deep(.el-collapse-item__header) {
  padding: 0 12px;
  color: var(--app-muted);
  font-size: 13px;
  font-weight: 700;
}

.excluded-collapse :deep(.el-collapse-item__wrap) {
  border-bottom: 0;
}

.excluded-list {
  max-height: 240px;
  border: 0;
  border-top: 1px solid var(--app-border);
  border-radius: 0;
}

@media (max-width: 720px) {
  .preview-summary,
  .preview-summary.has-excluded {
    grid-template-columns: 1fr;
  }
}
</style>
