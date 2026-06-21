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
              {{ formatTime(phase.startTime) }}
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
import { ref, computed } from 'vue'
import { CircleCheckFilled, Loading, Clock } from '@element-plus/icons-vue'

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
    created_at: string
    completed_at?: string
  }
}>()

const phases = computed(() => {
  const allPhases: Phase[] = [
    {
      name: 'context',
      title: '上下文发现',
      description: '加载配置并发现项目标准',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'change_context',
      title: '变更分析',
      description: '分析 git diff 并理解变更',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'analysis',
      title: '技术负责人分析',
      description: '总结变更并识别风险',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'reviews',
      title: '并行审查',
      description: '运行多个审查代理并行工作',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'aggregation',
      title: '聚合',
      description: '合并多个代理的发现',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'discourse',
      title: '讨论',
      description: '代理辩论并验证发现',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'synthesis',
      title: '综合',
      description: '生成最终优先级排序的审查',
      status: 'pending',
      progress: 0,
    },
    {
      name: 'complete',
      title: '完成',
      description: '审查完成',
      status: 'pending',
      progress: 0,
    },
  ]

  // Update phases based on session
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

function formatTime(date: Date): string {
  return date.toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}min`
}
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
