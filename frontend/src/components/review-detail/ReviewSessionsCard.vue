<template>
  <el-card class="sessions-card" v-if="sessions.length > 0">
    <template #header>
      <div class="card-header">
        <span>审查会话</span>
        <el-tag size="small">{{ sessions.length }} 轮</el-tag>
      </div>
    </template>
    <div class="session-list">
      <div
        v-for="sess in sessions"
        :key="sess.session_id"
        class="session-item"
        @click="$emit('go-to-session', sess)"
      >
        <div class="session-info">
          <div class="session-header">
            <span class="session-round">第 {{ sess.round }} 轮</span>
            <el-tag :type="getSessionStatusType(sess.status)" size="small">
              {{ getSessionStatusLabel(sess.status) }}
            </el-tag>
          </div>
          <div class="session-phase">
            {{ getPhaseLabel(sess.phase) }}
          </div>
          <el-progress :percentage="sess.progress" :stroke-width="4" :show-text="false" />
        </div>
        <el-icon><ArrowRight /></el-icon>
      </div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { ArrowRight } from '@element-plus/icons-vue'
import { getPhaseLabel, getSessionStatusType, getSessionStatusLabel } from '@/utils/status'
import type { ReviewSession } from '@/types/session'

defineProps<{
  sessions: ReviewSession[]
}>()

defineEmits<{
  'go-to-session': [session: ReviewSession]
}>()
</script>

<style scoped>
.session-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.session-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px;
  background: var(--app-surface);
  border: 1px solid var(--app-border);
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.session-item:hover {
  border-color: var(--app-primary);
  background: #f8f9ff;
}

.session-info {
  flex: 1;
  min-width: 0;
}

.session-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.session-round {
  font-weight: 600;
  font-size: 13px;
}

.session-phase {
  margin-bottom: 6px;
  color: #8892a4;
  font-size: 12px;
}
</style>
