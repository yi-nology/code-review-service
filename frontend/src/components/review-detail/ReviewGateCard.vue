<template>
  <el-card class="gate-card">
    <template #header>质量门禁</template>
    <div class="gate-status" :class="gateStatus">
      <div class="gate-icon">
        <el-icon v-if="gatePassed" :size="48"><SuccessFilled /></el-icon>
        <el-icon v-else-if="isFailed" :size="48"><CircleCloseFilled /></el-icon>
        <el-icon v-else :size="48"><InfoFilled /></el-icon>
      </div>
      <div class="gate-text">
        {{ gatePassed ? '门禁通过' : isFailed ? '执行失败' : '待定' }}
      </div>
      <div class="gate-reason" v-if="reason">{{ reason }}</div>
    </div>
  </el-card>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { SuccessFilled, CircleCloseFilled, InfoFilled } from '@element-plus/icons-vue'

const props = defineProps<{
  gateStatus?: string
  gatePassed?: boolean
  reviewStatus: string
  reason?: string
}>()

const isFailed = computed(() =>
  props.reviewStatus === 'failed' || Boolean(props.gateStatus && ['failed', 'error'].includes(props.gateStatus)),
)
</script>

<style scoped>
.gate-status {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  padding: 24px;
  text-align: center;
}

.gate-status.passed .gate-icon { color: var(--app-success); }
.gate-status.failed .gate-icon { color: var(--app-danger); }
.gate-status.pending .gate-icon { color: #8e96a5; }

.gate-text {
  font-size: 16px;
  font-weight: 600;
}

.gate-reason {
  color: #8892a4;
  font-size: 13px;
}
</style>
