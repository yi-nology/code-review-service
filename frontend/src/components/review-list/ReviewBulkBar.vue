<template>
  <!-- 批量操作条：选中行时滑出，与筛选条视觉延续 -->
  <transition name="bulk-slide">
    <div v-if="selectedCount" class="bulk-action-bar">
      <span>已选 <strong>{{ selectedCount }}</strong> 条</span>
      <span class="bulk-hint">
        （可取消 {{ cancellableCount }} · 可重审 {{ rerunnableCount }}）
      </span>
      <div class="bulk-spacer" />
      <el-button size="small" type="danger" plain :loading="busy" :disabled="!cancellableCount" @click="emit('cancel')">
        批量取消
      </el-button>
      <el-button size="small" type="primary" plain :loading="busy" :disabled="!rerunnableCount" @click="emit('rerun')">
        批量重新审查
      </el-button>
    </div>
  </transition>
</template>

<script setup lang="ts">
defineProps<{
  selectedCount: number
  cancellableCount: number
  rerunnableCount: number
  busy: boolean
}>()

const emit = defineEmits<{
  (e: 'cancel'): void
  (e: 'rerun'): void
}>()
</script>

<style scoped>
.bulk-action-bar {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 18px;
  border-bottom: 1px solid var(--app-border);
  background: #eff6ff;
  color: #1e3a8a;
  font-size: 13px;
}

.bulk-hint {
  color: #6366f1;
  font-size: 12px;
}

.bulk-spacer {
  flex: 1;
}

.bulk-slide-enter-active,
.bulk-slide-leave-active {
  transition: all 180ms ease;
  overflow: hidden;
}

.bulk-slide-enter-from,
.bulk-slide-leave-to {
  opacity: 0;
  max-height: 0;
  padding-top: 0;
  padding-bottom: 0;
}
</style>
