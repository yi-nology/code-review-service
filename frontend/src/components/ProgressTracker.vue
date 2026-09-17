<template>
  <div class="progress-tracker" :class="mode ? `mode-${mode}` : ''">
    <div class="progress-header">
      <h3>{{ mode === 'argus' ? 'Argus 审查进度' : mode === 'scan' ? '扫描进度' : '审查进度' }}</h3>
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
import { ARGUS_PHASES, SCAN_PHASES, type ReviewMode } from '@/constants/reviewMode'

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
  /** 审查模式：决定阶段定义与进度推断逻辑（ocr 多智能体 / argus 外部引擎 / scan 全库扫描） */
  mode?: ReviewMode
}>()

// 阶段定义优先级：显式 prop > 模式默认（argus/scan 有各自的阶段语义）
const modePhases = computed<PhaseDefinition[]>(() => {
  if (props.mode === 'argus') return ARGUS_PHASES
  if (props.mode === 'scan') return SCAN_PHASES
  return REVIEW_PHASES
})

const phaseDefinitions = computed(() => props.phaseDefinitions?.length ? props.phaseDefinitions : modePhases.value)

const phases = ref<Phase[]>(
  REVIEW_PHASES.map(p => ({ ...p, status: 'pending' as const, progress: 0 }))
)

const error = ref<Error | null>(null)
let wsConnection: { close: () => void } | null = null
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

  // Argus 模式：阶段即 history.stage（queued → dispatch → argus_report），按事件推导。
  if (props.mode === 'argus') {
    if (status === 'queued') {
      applyPhaseState('queued', 'active', 0)
      return
    }
    if (status === 'running') {
      const done = new Set(history.filter(e => e.status === 'completed').map(e => e.stage))
      const current = done.has('dispatch') ? 'argus_report' : done.has('queued') ? 'dispatch' : 'queued'
      applyPhaseState(current, 'active', current === 'argus_report' ? 40 : 15)
      return
    }
    if (status === 'failed') {
      applyPhaseState('argus_report', 'active', 40)
      error.value = { title: 'Argus 审查异常', message: '审查任务失败，请查看执行历史与失败提示。' }
    }
    return
  }

  if (status === 'failed') {
    const failedStage = inferPhaseFromHistory(history) || 'analysis'
    applyPhaseState(failedStage, 'active', inferProgressFromHistory(history, status))
    error.value = {
      title: props.mode === 'scan' ? '扫描执行失败' : '审查执行失败',
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
  if (stages.has('ocr')) {
    const ocrDone = history.some(item => item.stage === 'ocr' && item.status === 'completed')
    // scan 无 synthesis 阶段，OCR 完成即进入收尾；OCR 模式还有综合裁决阶段
    return ocrDone ? (props.mode === 'scan' ? 'complete' : 'synthesis') : 'reviews'
  }
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
  wsConnection = progressApi.connectProgress(
    props.reviewId,
    handleProgressUpdate,
    {
      onError: () => applySessionFallback(),
      onClose: () => {
        wsConnection = null
        if (!shouldReconnect || !['queued', 'running'].includes(props.reviewStatus || '')) return
        reconnectTimer = setTimeout(connectWebSocket, 3000)
      },
    },
  )
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
  if (wsConnection) {
    wsConnection.close()
    wsConnection = null
  }
}

onMounted(() => {
  resetPhases()
  applySessionFallback()
  // Argus 为外部异步回报，没有 OCR tracker 进度流，仅按 history 推导；
  // ocr/scan 在 queued/running 时拉取实时进度（scan 走同一 OCR 管线）。
  if (props.mode !== 'argus' && ['queued', 'running'].includes(props.reviewStatus || '')) {
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
    props.mode,
    props.sessions?.map(s => `${s.session_id}:${s.phase}:${s.status}:${s.progress}`).join('|'),
    props.history?.map(item => `${item.stage}:${item.status}`).join('|'),
  ],
  (_, prev) => {
    // 模式切换意味着阶段定义整体更换，重置后再按新阶段推导
    if (prev && prev[1] !== props.mode) {
      resetPhases()
    }
    applySessionFallback()
    if (props.mode !== 'argus' && ['queued', 'running'].includes(props.reviewStatus || '') && !wsConnection && shouldReconnect) {
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

/* 模式主题色（argus=teal / scan=amber），覆盖进度与激活态蓝色 */
.progress-tracker.mode-argus { --pt-color: #0d9488; }
.progress-tracker.mode-scan { --pt-color: #d97706; }
.progress-tracker.mode-argus .progress-percentage,
.progress-tracker.mode-scan .progress-percentage { color: var(--pt-color); }
.progress-tracker.mode-argus .phase.active,
.progress-tracker.mode-scan .phase.active {
  background: color-mix(in srgb, var(--pt-color) 8%, #fff);
  border-color: var(--pt-color);
}
.progress-tracker.mode-argus .phase.active .phase-icon,
.progress-tracker.mode-scan .phase.active .phase-icon {
  background: var(--pt-color);
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
