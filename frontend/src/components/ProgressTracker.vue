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
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { CircleCheckFilled, Loading, Clock } from '@element-plus/icons-vue'
import { progressApi } from '@/api/session'
import type { ReviewSession, ReviewProgress, ProgressEvent, ReviewHistoryEvent } from '@/types/session'
import { REVIEW_PHASES, type PhaseDefinition } from '@/constants/phases'

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
  reviewStatus?: string
  sessions?: ReviewSession[]
  history?: ReviewHistoryEvent[]
  phaseDefinitions?: PhaseDefinition[]
}>()

const phaseDefinitions = computed(() => props.phaseDefinitions?.length ? props.phaseDefinitions : REVIEW_PHASES)

const phases = ref<Phase[]>(
  REVIEW_PHASES.map(p => ({ ...p, status: 'pending' as const, progress: 0 }))
)

const error = ref<Error | null>(null)
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null
let shouldReconnect = true

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

function normalizeProgress(value: unknown) {
  const n = Number(value)
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(100, Math.round(n)))
}

function resetPhases() {
  phases.value = phaseDefinitions.value.map(phase => ({
    ...phase,
    status: 'pending',
    progress: 0,
    agents: undefined,
  }))
}

function applyPhaseState(phaseName: string, status: string, progress: number, agentStatus?: Record<string, Agent> | null) {
  const phaseIndex = phases.value.findIndex(p => p.name === phaseName)
  if (phaseIndex === -1) return

  phases.value[phaseIndex].status = status === 'completed' ? 'completed' : status === 'pending' ? 'pending' : 'active'
  phases.value[phaseIndex].progress = normalizeProgress(progress)

  if (agentStatus && Object.keys(agentStatus).length > 0) {
    phases.value[phaseIndex].agents = Object.values(agentStatus).map(agent => ({
      ...agent,
      progress: normalizeProgress(agent.progress),
    }))
  }

  for (let i = 0; i < phaseIndex; i++) {
    phases.value[i].status = 'completed'
    phases.value[i].progress = 100
  }

  for (let i = phaseIndex + 1; i < phases.value.length; i++) {
    if (phases.value[i].status !== 'pending') {
      phases.value[i].status = 'pending'
      phases.value[i].progress = 0
      phases.value[i].agents = undefined
    }
  }
}

function applyProgressSnapshot(snapshot: ReviewProgress) {
  if (!snapshot) return

  if (Array.isArray(snapshot.phases) && snapshot.phases.length > 0) {
    const phaseMap = new Map(snapshot.phases.map(phase => [phase.name, phase]))
    phases.value = phases.value.map(phase => {
      const incoming = phaseMap.get(phase.name)
      if (!incoming) return phase
      return {
        ...phase,
        description: incoming.description || phase.description,
        status: incoming.status === 'completed' ? 'completed' : incoming.status === 'pending' ? 'pending' : 'active',
        progress: normalizeProgress(incoming.progress),
        agents: phase.name === snapshot.phase && snapshot.agent_status
          ? Object.values(snapshot.agent_status).map(agent => ({
              ...agent,
              progress: normalizeProgress(agent.progress),
            }))
          : phase.agents,
      }
    })
    return
  }

  if (snapshot.phase) {
    applyPhaseState(snapshot.phase, snapshot.status, snapshot.progress, snapshot.agent_status as Record<string, Agent> | null)
  }
}

function applySessionFallback() {
  const sessions = props.sessions || []
  if (!sessions.length) {
    applyReviewFallback()
    return
  }

  const latest = [...sessions].sort((a, b) => {
    const aTime = new Date(a.updated_at || a.created_at).getTime()
    const bTime = new Date(b.updated_at || b.created_at).getTime()
    return bTime - aTime
  })[0]

  if (!latest) {
    applyReviewFallback()
    return
  }
  if (latest.status === 'completed' || isSuccessfulReviewStatus(props.reviewStatus)) {
    phases.value = phases.value.map(phase => ({ ...phase, status: 'completed', progress: 100 }))
    return
  }
  if (latest.status === 'failed') {
    applyPhaseState(latest.phase || 'analysis', 'active', latest.progress || 0)
    error.value = {
      title: '审查中断',
      message: '审查任务已失败，当前展示的是最后一次会话进度。',
    }
    return
  }

  applyPhaseState(latest.phase || 'context', latest.status === 'active' ? 'active' : latest.status, latest.progress || 0)
}

function isSuccessfulReviewStatus(status?: string) {
  return ['success', 'completed'].includes(status || '')
}

function applyReviewFallback() {
  const status = props.reviewStatus || ''

  if (isSuccessfulReviewStatus(status)) {
    phases.value = phases.value.map(phase => ({ ...phase, status: 'completed', progress: 100 }))
    error.value = null
    return
  }

  const history = props.history || []
  if (status === 'failed') {
    const failedStage = inferPhaseFromHistory(history) || 'analysis'
    applyPhaseState(failedStage, 'active', inferProgressFromHistory(history, status))
    error.value = {
      title: '审查执行失败',
      message: '实时进度已不可用，当前展示的是审查记录推导出的最后进度。',
    }
    return
  }

  if (status === 'queued') {
    applyPhaseState('context', 'active', 0)
    return
  }

  if (status === 'running') {
    applyPhaseState(inferPhaseFromHistory(history) || 'context', 'active', inferProgressFromHistory(history, status))
  }
}

function inferPhaseFromHistory(history: ReviewHistoryEvent[]) {
  const stages = new Set(history.map(item => item.stage))
  if (stages.has('success')) return 'complete'
  if (stages.has('ocr')) return history.some(item => item.stage === 'ocr' && item.status === 'completed') ? 'synthesis' : 'reviews'
  if (stages.has('scope')) return 'change_context'
  if (stages.has('repository')) return 'analysis'
  if (stages.has('running')) return 'context'
  return ''
}

function inferProgressFromHistory(history: ReviewHistoryEvent[], status: string) {
  if (isSuccessfulReviewStatus(status)) return 100
  const stages = new Set(history.map(item => `${item.stage}:${item.status}`))
  if (stages.has('ocr:completed')) return 75
  if ([...stages].some(stage => stage.startsWith('ocr:'))) return 50
  if ([...stages].some(stage => stage.startsWith('scope:'))) return 35
  if ([...stages].some(stage => stage.startsWith('repository:'))) return 25
  if (status === 'running') return 10
  return 0
}

async function loadInitialProgress() {
  try {
    const progress = await progressApi.getProgress(props.reviewId)
    if (progress) {
      applyProgressSnapshot(progress)
      error.value = null
    } else {
      applySessionFallback()
    }
  } catch {
    applySessionFallback()
  }
}

function connectWebSocket() {
  closeWebSocket()
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsUrl = `${protocol}//${window.location.host}/api/v1/reviews/${props.reviewId}/progress`

  ws = new WebSocket(wsUrl)

  ws.onmessage = (event) => {
    const data = JSON.parse(event.data)
    handleProgressUpdate(data)
  }

  ws.onerror = () => {
    applySessionFallback()
  }

  ws.onclose = () => {
    ws = null
    if (!shouldReconnect || !['queued', 'running'].includes(props.reviewStatus || '')) return
    reconnectTimer = setTimeout(connectWebSocket, 3000)
  }
}

function handleProgressUpdate(event: ProgressEvent | ReviewProgress | any) {
  if (Array.isArray(event.phases)) {
    applyProgressSnapshot(event)
    return
  }

  if (event.type === 'error') {
    error.value = {
      title: '审查进度不可用',
      message: event.message || event.error || '当前没有活跃的实时进度，已尝试展示最近一次会话进度。',
    }
    applySessionFallback()
    if (['queued', 'running'].includes(props.reviewStatus || '')) {
      closeWebSocket()
      reconnectTimer = setTimeout(connectWebSocket, 3000)
    } else {
      shouldReconnect = false
      closeWebSocket()
    }
    return
  }

  if (event.type === 'complete') {
    phases.value = phases.value.map(phase => ({ ...phase, status: 'completed', progress: 100 }))
    shouldReconnect = false
    closeWebSocket()
    return
  }

  applyPhaseState(event.phase, event.status, event.progress, event.agent_status as Record<string, Agent> | null)

  if (event.type === 'error') {
    error.value = {
      title: '审查错误',
      message: event.error,
    }
  }
}

function closeWebSocket() {
  if (reconnectTimer) {
    clearTimeout(reconnectTimer)
    reconnectTimer = null
  }
  if (ws) {
    const current = ws
    ws = null
    current.onclose = null
    current.close()
  }
}

onMounted(() => {
  resetPhases()
  applySessionFallback()
  // 实时进度（tracker）仅在 queued/running 时存在；终态评审直接走会话/历史回退，避免 404 噪音。
  if (['queued', 'running'].includes(props.reviewStatus || '')) {
    loadInitialProgress()
    connectWebSocket()
  }
})

onUnmounted(() => {
  shouldReconnect = false
  closeWebSocket()
})

watch(
  () => [
    props.reviewStatus,
    props.sessions?.map(s => `${s.session_id}:${s.phase}:${s.status}:${s.progress}`).join('|'),
    props.history?.map(item => `${item.stage}:${item.status}`).join('|'),
  ],
  () => {
    applySessionFallback()
    if (['queued', 'running'].includes(props.reviewStatus || '') && !ws && shouldReconnect) {
      connectWebSocket()
    }
  }
)
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
