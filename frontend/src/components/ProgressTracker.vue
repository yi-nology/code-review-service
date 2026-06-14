<template>
  <div class="progress-tracker">
    <div class="progress-header">
      <h3>审查进度</h3>
      <span class="progress-percentage">{{ overallProgress }}%</span>
    </div>

    <div class="phases">
      <div
        v-for="phase in phases"
        :key="phase.name"
        :class="['phase', phase.status]"
      >
        <div class="phase-icon">
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

        <div class="phase-info">
          <h4>{{ phase.title }}</h4>
          <p>{{ phase.description }}</p>

          <div v-if="phase.status === 'active' && phase.agents" class="agents">
            <div
              v-for="agent in phase.agents"
              :key="agent.name"
              class="agent"
            >
              <span class="agent-name">{{ agent.name }}</span>
              <el-progress
                :percentage="agent.progress"
                :status="getAgentStatus(agent.status)"
                :stroke-width="4"
              />
            </div>
          </div>
        </div>

        <div v-if="phase.status === 'active'" class="phase-progress">
          <el-progress
            type="circle"
            :percentage="phase.progress"
            :width="48"
            :stroke-width="4"
          />
        </div>
      </div>
    </div>

    <div v-if="error" class="error">
      <el-alert
        :title="error.title"
        :description="error.message"
        type="error"
        show-icon
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { CircleCheckFilled, Loading, Clock } from '@element-plus/icons-vue'

interface Agent {
  name: string
  status: string
  progress: number
  message: string
}

interface Phase {
  name: string
  title: string
  description: string
  status: 'pending' | 'active' | 'completed'
  progress: number
  agents?: Agent[]
}

interface Error {
  title: string
  message: string
}

const props = defineProps<{
  reviewId: string
}>()

const phases = ref<Phase[]>([
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
])

const error = ref<Error | null>(null)
let ws: WebSocket | null = null

const overallProgress = computed(() => {
  const total = phases.value.length
  const completed = phases.value.filter(p => p.status === 'completed').length
  const active = phases.value.find(p => p.status === 'active')
  const activeProgress = active ? active.progress / 100 : 0
  return Math.round(((completed + activeProgress) / total) * 100)
})

function getAgentStatus(status: string): '' | 'success' | 'exception' | 'warning' {
  switch (status) {
    case 'completed':
      return 'success'
    case 'failed':
      return 'exception'
    case 'running':
      return ''
    default:
      return ''
  }
}

function connectWebSocket() {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/v1/reviews/${props.reviewId}/progress`

  ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    handleProgressUpdate(data)
  }

  ws.onerror = () => {
    error.value = {
      title: '连接错误',
      message: '无法连接到进度流',
    }
  }

  ws.onclose = () => {
    // 3 秒后重连
    setTimeout(connectWebSocket, 3000)
  }
}

function handleProgressUpdate(event: any) {
  const phaseIndex = phases.value.findIndex(p => p.name === event.phase)
  if (phaseIndex === -1) return

  phases.value[phaseIndex].status = event.status === 'completed' ? 'completed' : 'active'
  phases.value[phaseIndex].progress = event.progress

  if (event.agent_status) {
    phases.value[phaseIndex].agents = Object.values(event.agent_status)
  }

  // 将之前的阶段标记为已完成
  for (let i = 0; i < phaseIndex; i++) {
    if (phases.value[i].status !== 'completed') {
      phases.value[i].status = 'completed'
      phases.value[i].progress = 100
    }
  }

  // 将之后的阶段标记为待处理
  for (let i = phaseIndex + 1; i < phases.value.length; i++) {
    if (phases.value[i].status !== 'pending') {
      phases.value[i].status = 'pending'
      phases.value[i].progress = 0
    }
  }

  if (event.type === 'error') {
    error.value = {
      title: '审查错误',
      message: event.error,
    }
  }
}

onMounted(() => {
  connectWebSocket()
})

onUnmounted(() => {
  if (ws) {
    ws.close()
  }
})
</script>

<style scoped>
.progress-tracker {
  padding: 20px;
  background: #fff;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.progress-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}

.progress-header h3 {
  margin: 0;
  font-size: 18px;
  color: #303133;
}

.progress-percentage {
  font-size: 24px;
  font-weight: bold;
  color: #409eff;
}

.phases {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.phase {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  border-radius: 8px;
  background: #f5f7fa;
  transition: all 0.3s ease;
}

.phase.active {
  background: #ecf5ff;
  border: 1px solid #409eff;
}

.phase.completed {
  background: #f0f9eb;
}

.phase-icon {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: #dcdfe6;
  color: #909399;
}

.phase.active .phase-icon {
  background: #409eff;
  color: #fff;
}

.phase.completed .phase-icon {
  background: #67c23a;
  color: #fff;
}

.phase-info {
  flex: 1;
}

.phase-info h4 {
  margin: 0 0 4px;
  font-size: 14px;
  color: #303133;
}

.phase-info p {
  margin: 0;
  font-size: 12px;
  color: #909399;
}

.agents {
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.agent {
  display: flex;
  align-items: center;
  gap: 8px;
}

.agent-name {
  width: 100px;
  font-size: 12px;
  color: #606266;
}

.phase-progress {
  flex-shrink: 0;
}

.error {
  margin-top: 16px;
}
</style>
