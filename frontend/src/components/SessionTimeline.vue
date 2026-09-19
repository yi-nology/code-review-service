<template>
  <div class="session-timeline">
    <div class="timeline">
      <div
        v-for="phase in phases"
        :key="phase.name"
        :class="['timeline-item', phase.status]"
      >
        <div class="timeline-marker">
          <el-icon v-if="phase.status === 'completed'">
            <CircleCheckFilled />
          </el-icon>
          <el-icon v-else-if="phase.status === 'active'">
            <Loading />
          </el-icon>
          <el-icon v-else>
            <Clock />
          </el-icon>
        </div>
        <div class="timeline-content">
          <div class="timeline-header">
            <h4>{{ phase.title }}</h4>
            <span
              v-if="phase.startTime"
              class="timeline-time"
            >
              {{ formatClockSeconds(phase.startTime) }}
            </span>
          </div>
          <p>{{ phase.description }}</p>
          <div
            v-if="phase.status === 'active'"
            class="timeline-progress"
          >
            <el-progress
              :percentage="phase.progress"
              :stroke-width="4"
            />
          </div>
          <div
            v-if="phase.duration"
            class="timeline-duration"
          >
            耗时: {{ formatDuration(phase.duration) }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { formatClockSeconds, formatDuration } from '@/utils/format'
import { computed } from 'vue'
import { CircleCheckFilled, Loading, Clock } from '@element-plus/icons-vue'
import { REVIEW_PHASES } from '@/constants/phases'

interface Phase {
  name: string
  title: string
  description: string
  status: 'pending' | 'active' | 'completed'
  progress: number
  startTime?: Date
  endTime?: Date
  duration?: number
}

const props = defineProps<{
  session: {
    phase: string
    progress: number
    status?: string
    created_at: string
    completed_at?: string
  }
}>()

const phases = computed(() => {
  const allPhases: Phase[] = REVIEW_PHASES.map(p => ({ ...p, status: 'pending' as const, progress: 0 }))

  // 会话已完成时，所有阶段标记为 completed（包括最终的 complete 阶段）。
  if (props.session.status === 'completed') {
    return allPhases.map(p => ({ ...p, status: 'completed' as const, progress: 100 }))
  }

  const currentIndex = allPhases.findIndex(p => p.name === props.session.phase)
  if (currentIndex !== -1) {
    for (let i = 0; i <= currentIndex; i++) {
      if (i === currentIndex) {
        allPhases[i].status = 'active'
        allPhases[i].progress = props.session.progress
      } else {
        allPhases[i].status = 'completed'
        allPhases[i].progress = 100
      }
    }
  }

  return allPhases
})

</script>

<style scoped>
.session-timeline {
  width: 100%;
}

.timeline {
  position: relative;
  padding-left: 30px;
}

.timeline::before {
  content: '';
  position: absolute;
  left: 15px;
  top: 0;
  bottom: 0;
  width: 2px;
  background: var(--app-border, #e4e7ed);
}

.timeline-item {
  position: relative;
  margin-bottom: 16px;
  padding-left: 20px;
}

.timeline-item:last-child {
  margin-bottom: 0;
}

.timeline-marker {
  position: absolute;
  left: -30px;
  top: 0;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #e4e7ed;
  color: #9aa2b1;
  z-index: 1;
  font-size: 16px;
}

.timeline-item.active .timeline-marker {
  background: var(--app-primary, #409eff);
  color: #fff;
}

.timeline-item.completed .timeline-marker {
  background: var(--app-success, #67c23a);
  color: #fff;
}

.timeline-content {
  padding: 12px;
  background: #f7f8fc;
  border: 1px solid var(--app-border, #e4e7ed);
  border-radius: 10px;
}

.timeline-item.active .timeline-content {
  background: #eef0ff;
  border-color: var(--app-primary, #409eff);
}

.timeline-item.completed .timeline-content {
  background: #f0f9eb;
  border-color: #d8efcd;
}

.timeline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 6px;
}

.timeline-header h4 {
  margin: 0;
  font-size: 14px;
  color: var(--app-text, #303133);
}

.timeline-time {
  font-size: 12px;
  color: #9aa2b1;
}

.timeline-content p {
  margin: 0;
  font-size: 12px;
  color: #707782;
  line-height: 1.5;
}

.timeline-progress {
  margin-top: 8px;
}

.timeline-duration {
  margin-top: 8px;
  font-size: 12px;
  color: #9aa2b1;
}
</style>
