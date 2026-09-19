<template>
  <div class="discussion-viewer">
    <div
      v-if="entries.length === 0"
      class="empty"
    >
      暂无讨论记录
    </div>
    <div
      v-else
      class="entries"
    >
      <div
        v-for="entry in entries"
        :key="entry.id"
        :class="['entry', entry.response.toLowerCase()]"
      >
        <div class="entry-header">
          <div class="agent-info">
            <span class="agent-name">{{ entry.agent_name }}</span>
            <span :class="['response-type', entry.response.toLowerCase()]">
              {{ getResponseTypeLabel(entry.response) }}
            </span>
          </div>
          <span class="entry-time">
            {{ formatClockSeconds(entry.created_at) }}
          </span>
        </div>
        <div class="entry-content">
          <p>{{ entry.content }}</p>
          <div
            v-if="entry.evidence"
            class="evidence"
          >
            <strong>证据:</strong> {{ entry.evidence }}
          </div>
        </div>
        <div class="entry-meta">
          <span class="confidence">
            置信度: {{ (entry.confidence * 100).toFixed(0) }}%
          </span>
          <span
            v-if="entry.finding_id"
            class="finding-ref"
          >
            关联发现: {{ entry.finding_id }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatClockSeconds } from '@/utils/format'
import { computed } from 'vue'

interface DiscussionEntry {
  id: string
  agent_name: string
  finding_id: string
  response: 'AGREE' | 'CHALLENGE' | 'CONNECT' | 'SURFACE'
  content: string
  evidence: string
  confidence: number
  references: string[]
  created_at: string
}

const props = defineProps<{
  entries: DiscussionEntry[]
}>()

function getResponseTypeLabel(response: string): string {
  switch (response) {
    case 'AGREE':
      return '同意'
    case 'CHALLENGE':
      return '质疑'
    case 'CONNECT':
      return '关联'
    case 'SURFACE':
      return '发现'
    default:
      return response
  }
}

</script>

<style scoped>
.discussion-viewer {
  width: 100%;
}

.empty {
  padding: 28px 12px;
  text-align: center;
  color: #909399;
  font-size: 13px;
}

.entries {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.entry {
  padding: 14px;
  background: #f7f8fc;
  border: 1px solid var(--app-border, #e4e7ed);
  border-left: 4px solid #dcdfe6;
  border-radius: 10px;
}

.entry.agree {
  border-left-color: var(--app-success, #67c23a);
  background: #f0f9eb;
}

.entry.challenge {
  border-left-color: #e6a23c;
  background: #fdf6ec;
}

.entry.connect {
  border-left-color: var(--app-primary, #409eff);
  background: #eef0ff;
}

.entry.surface {
  border-left-color: #909399;
  background: #f7f8fc;
}

.entry-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}

.agent-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-name {
  font-weight: 600;
  color: var(--app-text, #303133);
  font-size: 14px;
}

.response-type {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
}

.response-type.agree {
  background: #e1f3d8;
  color: #67c23a;
}

.response-type.challenge {
  background: #faecd8;
  color: #c08c1e;
}

.response-type.connect {
  background: #d9ecff;
  color: #3984cf;
}

.response-type.surface {
  background: #e4e7ed;
  color: #909399;
}

.entry-time {
  font-size: 12px;
  color: #9aa2b1;
}

.entry-content {
  margin-bottom: 10px;
}

.entry-content p {
  margin: 0 0 8px;
  font-size: 13px;
  color: #606266;
  line-height: 1.6;
}

.evidence {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.04);
  border-radius: 6px;
  font-size: 12px;
  color: #606266;
  line-height: 1.5;
}

.evidence strong {
  color: var(--app-text, #303133);
}

.entry-meta {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 12px;
  color: #9aa2b1;
}

.confidence {
  font-weight: 500;
}

.finding-ref {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
</style>
