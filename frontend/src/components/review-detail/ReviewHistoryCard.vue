<template>
  <el-card class="history-card" v-if="history?.length">
    <template #header>执行历史</template>
    <el-timeline>
      <el-timeline-item
        v-for="(event, index) in history"
        :key="index"
        :timestamp="formatTime(event.created_at)"
        :type="getEventType(event.status)"
      >
        <h4>{{ event.message }}</h4>
        <p v-if="event.details">{{ formatDetails(event.details) }}</p>
      </el-timeline-item>
    </el-timeline>
  </el-card>
</template>

<script setup lang="ts">
import { formatDateTime } from '@/utils/format'
import type { Event } from '@/types/review'

defineProps<{
  history: Event[]
}>()

function getEventType(status: string) {
  const map: Record<string, string> = {
    completed: 'success',
    running: 'warning',
    failed: 'danger',
  }
  return map[status] || 'info'
}

function formatTime(time: string) {
  return formatDateTime(time)
}

function formatDetails(details: Record<string, unknown>) {
  return Object.entries(details)
    .map(([key, value]) => `${key}: ${value}`)
    .join(', ')
}
</script>

<style scoped>
.history-card h4 {
  margin: 0 0 4px;
  font-size: 13px;
}

.history-card p {
  margin: 0;
  color: #8892a4;
  font-size: 12px;
}
</style>
