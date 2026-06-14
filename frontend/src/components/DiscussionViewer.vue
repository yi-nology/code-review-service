<template>
  <div class="discussion-viewer">
    <h3>审查员讨论</h3>
    <div v-if="entries.length === 0" class="empty">
      暂无讨论记录
    </div>
    <div v-else class="entries">
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
            {{ formatTime(entry.created_at) }}
          </span>
        </div>
        <div class="entry-content">
          <p>{{ entry.content }}</p>
          <div v-if="entry.evidence" class="evidence">
            <strong>证据:</strong> {{ entry.evidence }}
          </div>
        </div>
        <div class="entry-meta">
          <span class="confidence">
            置信度: {{ (entry.confidence * 100).toFixed(0) }}%
          </span>
          <span v-if="entry.finding_id" class="finding-ref">
            关联发现: {{ entry.finding_id }}
          </span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
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

function formatTime(dateStr: string): string {
  const date = new Date(dateStr)
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}
</script>

<style scoped>
.discussion-viewer {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.discussion-viewer h3 {
  margin: 0 0 20px;
  font-size: 18px;
  color: #303133;
}

.empty {
  padding: 40px;
  text-align: center;
  color: #909399;
  font-size: 14px;
}

.entries {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.entry {
  padding: 16px;
  background: #f5f7fa;
  border-radius: 8px;
  border-left: 4px solid #dcdfe6;
}

.entry.agree {
  border-left-color: #67c23a;
  background: #f0f9eb;
}

.entry.challenge {
  border-left-color: #e6a23c;
  background: #fdf6ec;
}

.entry.connect {
  border-left-color: #409eff;
  background: #ecf5ff;
}

.entry.surface {
  border-left-color: #909399;
  background: #f5f7fa;
}

.entry-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.agent-info {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-name {
  font-weight: 600;
  color: #303133;
  font-size: 14px;
}

.response-type {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
}

.response-type.agree {
  background: #e1f3d8;
  color: #67c23a;
}

.response-type.challenge {
  background: #faecd8;
  color: #e6a23c;
}

.response-type.connect {
  background: #d9ecff;
  color: #409eff;
}

.response-type.surface {
  background: #e4e7ed;
  color: #909399;
}

.entry-time {
  font-size: 12px;
  color: #909399;
}

.entry-content {
  margin-bottom: 12px;
}

.entry-content p {
  margin: 0 0 8px;
  font-size: 14px;
  color: #606266;
  line-height: 1.6;
}

.evidence {
  padding: 8px 12px;
  background: rgba(0, 0, 0, 0.05);
  border-radius: 4px;
  font-size: 13px;
  color: #606266;
}

.evidence strong {
  color: #303133;
}

.entry-meta {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #909399;
}

.confidence {
  font-weight: 500;
}

.finding-ref {
  font-family: monospace;
}
</style>
