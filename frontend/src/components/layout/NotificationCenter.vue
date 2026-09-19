<script setup lang="ts">
import { formatRelativeTime } from '@/utils/format'
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { Bell } from '@element-plus/icons-vue'
import { getNotifications, getUnreadCount, markNotificationRead, markAllNotificationsRead } from '@/api/notification'
import type { NotificationItem } from '@/api/notification'
import { ElMessage } from 'element-plus/es/components/message/index'

const router = useRouter()
const unreadCount = ref(0)
const items = ref<NotificationItem[]>([])
const loading = ref(false)
const showPanel = ref(false)

let pollTimer: ReturnType<typeof setInterval> | null = null

async function refreshUnread() {
  try {
    unreadCount.value = await getUnreadCount()
  } catch {
    // 静默失败（全局拦截器会提示）
  }
}

async function loadList() {
  loading.value = true
  try {
    const res = await getNotifications({ limit: 20 })
    items.value = res.items
  } catch {
    // 静默
  } finally {
    loading.value = false
  }
}

async function togglePanel(visible: boolean) {
  showPanel.value = visible
  if (visible) {
    await loadList()
  }
}

async function handleRead(item: NotificationItem) {
  if (item.read) {
    goToReview(item)
    return
  }
  try {
    await markNotificationRead(item.id)
    item.read = true
    if (unreadCount.value > 0) unreadCount.value--
  } catch {
    // 忽略，仍跳转
  }
  goToReview(item)
}

function goToReview(item: NotificationItem) {
  if (item.review_id) {
    showPanel.value = false
    router.push(`/reviews/${item.review_id}`)
  }
}

async function handleReadAll() {
  try {
    await markAllNotificationsRead()
    items.value.forEach((i) => (i.read = true))
    unreadCount.value = 0
    ElMessage.success('已全部标记为已读')
  } catch {
    // 全局拦截器会提示
  }
}

function kindLabel(kind: string): string {
  const map: Record<string, string> = {
    completed: '评审完成',
    failed: '评审失败',
    gate_failed: '门禁未通过',
  }
  return map[kind] || kind
}

function kindColor(kind: string): string {
  if (kind === 'failed') return '#f56c6c'
  if (kind === 'gate_failed') return '#e6a23c'
  return '#67c23a'
}


onMounted(() => {
  refreshUnread()
  // 每 60s 刷新未读数（比健康检查 30s 低频，通知非实时强需求）
  pollTimer = setInterval(refreshUnread, 60000)
})

onBeforeUnmount(() => {
  if (pollTimer) clearInterval(pollTimer)
})
</script>

<template>
  <el-popover
    placement="bottom-end"
    :width="380"
    trigger="click"
    :visible="showPanel"
    @update:visible="togglePanel"
  >
    <template #reference>
      <el-badge :value="unreadCount" :hidden="unreadCount === 0" :max="99" class="notify-badge">
        <button class="notify-button" :class="{ active: showPanel }" title="通知">
          <el-icon :size="18"><Bell /></el-icon>
        </button>
      </el-badge>
    </template>

    <div class="notify-panel">
      <div class="notify-header">
        <span class="notify-title">通知</span>
        <button
          v-if="unreadCount > 0"
          class="notify-read-all"
          @click="handleReadAll"
        >
          全部已读
        </button>
      </div>

      <div v-loading="loading" class="notify-list">
        <div v-if="items.length === 0 && !loading" class="notify-empty">
          暂无通知
        </div>
        <div
          v-for="item in items"
          :key="item.id"
          class="notify-item"
          :class="{ unread: !item.read }"
          @click="handleRead(item)"
        >
          <span class="notify-dot" :style="{ background: kindColor(item.kind) }" />
          <div class="notify-content">
            <div class="notify-item-title">{{ item.title }}</div>
            <div class="notify-item-body">{{ item.body }}</div>
            <div class="notify-item-meta">
              <span class="notify-kind" :style="{ color: kindColor(item.kind) }">{{ kindLabel(item.kind) }}</span>
              <span class="notify-time">{{ formatRelativeTime(item.created_at) }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </el-popover>
</template>

<style scoped>
.notify-badge {
  display: inline-flex;
}
.notify-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border: 1px solid #e5e7eb;
  border-radius: 10px;
  background: #fff;
  color: #4b5563;
  cursor: pointer;
  transition: background 0.15s, color 0.15s, border-color 0.15s;
}
.notify-button:hover {
  background: #f3f4f6;
  color: #1f2937;
  border-color: #d1d5db;
}
.notify-button.active {
  background: #eef2ff;
  color: #4f46e5;
  border-color: #c7d2fe;
}
.notify-panel {
  max-height: 480px;
  display: flex;
  flex-direction: column;
}
.notify-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 4px 12px;
  border-bottom: 1px solid #f0f0f0;
}
.notify-title {
  font-weight: 600;
  font-size: 14px;
  color: #1f2937;
}
.notify-read-all {
  border: none;
  background: none;
  color: #4f46e5;
  font-size: 12px;
  cursor: pointer;
  padding: 2px 4px;
}
.notify-read-all:hover {
  text-decoration: underline;
}
.notify-list {
  overflow-y: auto;
  max-height: 400px;
}
.notify-empty {
  padding: 40px 0;
  text-align: center;
  color: #9ca3af;
  font-size: 13px;
}
.notify-item {
  display: flex;
  gap: 10px;
  padding: 12px 4px;
  border-bottom: 1px solid #f5f5f5;
  cursor: pointer;
  border-radius: 6px;
  transition: background 0.12s;
}
.notify-item:hover {
  background: #f9fafb;
}
.notify-item.unread {
  background: #f0f5ff;
}
.notify-item.unread:hover {
  background: #e6efff;
}
.notify-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 6px;
}
.notify-item.unread .notify-dot {
  box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.15);
}
.notify-content {
  flex: 1;
  min-width: 0;
}
.notify-item-title {
  font-size: 13px;
  font-weight: 500;
  color: #1f2937;
  margin-bottom: 2px;
}
.notify-item-body {
  font-size: 12px;
  color: #6b7280;
  line-height: 1.5;
  overflow: hidden;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}
.notify-item-meta {
  display: flex;
  gap: 8px;
  margin-top: 4px;
  font-size: 11px;
}
.notify-kind {
  font-weight: 500;
}
.notify-time {
  color: #9ca3af;
}
</style>
