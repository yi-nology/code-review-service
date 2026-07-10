<script setup lang="ts">
import { ref, computed } from 'vue'

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
      <h2>文件预览</h2>
      <div class="section-actions">
        <el-input v-model="previewFilter" size="small" placeholder="搜索文件…" style="width: 200px" clearable />
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
        <span class="file-badge">审查</span>
        <code>{{ file }}</code>
      </div>
      <div v-if="filteredReviewable.length === 0" class="file-list-empty">没有匹配的可审查文件</div>
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
  margin-bottom: 16px;
}
.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}
.section-head h2 {
  font-size: 16px;
  font-weight: 600;
  color: #1f2937;
  margin: 0;
}
.section-actions {
  display: flex;
  gap: 8px;
}
.preview-summary {
  display: flex;
  gap: 16px;
  margin-bottom: 12px;
  padding: 12px 16px;
  background: #f9fafb;
  border-radius: 8px;
}
.preview-stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.preview-stat span {
  font-size: 11px;
  color: #6b7280;
}
.preview-stat strong {
  font-size: 18px;
  color: #1f2937;
}
.preview-stat.primary strong {
  color: #4f46e5;
}
.file-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
  max-height: 280px;
  overflow-y: auto;
  padding: 4px 0;
}
.file-list.empty {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 60px;
}
.file-list-empty {
  color: #9ca3af;
  font-size: 13px;
}
.file-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
}
.file-row:hover {
  background: #f3f4f6;
}
.file-row code {
  font-size: 12px;
  color: #374151;
}
.file-badge {
  font-size: 10px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
  background: #dcfce7;
  color: #16a34a;
}
.file-badge.muted {
  background: #f3f4f6;
  color: #9ca3af;
}
.excluded-collapse {
  margin-top: 8px;
  border: 1px solid var(--app-border);
  border-radius: 8px;
}
.excluded-list {
  max-height: 200px;
}
</style>
