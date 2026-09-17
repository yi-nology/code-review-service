/**
 * reviewMode — 审查记录的三种审查模式定义。
 *
 * 同一条 /reviews/:id 记录可能来自三种审查模式，页面按模式切换视觉身份与信息结构：
 * - ocr   : OCR 多智能体审查（本服务多 agent 工作流，PR/commit 维度）
 * - argus : Argus 外部引擎审查（派发外部 Bot，异步回报裁决，无本地评分）
 * - scan  : 仓库全量扫描（callback_mode=scan，无 PR diff 范围概念）
 *
 * 模式判定依据后端落库字段（见 internal/service/review：resolveEngine / scanAsync）：
 * engine='argus' → argus；callback_mode='scan' → scan；其余 → ocr。
 */
import type { ReviewDetail, Event } from '@/types/review'
import type { PhaseDefinition } from '@/constants/phases'

export type ReviewMode = 'ocr' | 'argus' | 'scan'

export interface ReviewModeMeta {
  key: ReviewMode
  /** 模式名（徽章/侧栏标题用） */
  label: string
  /** 模式一句话说明（Hero 副标题用） */
  tagline: string
  /** 模式主题色 */
  color: string
  /** 主题色的浅背景（卡片底色） */
  soft: string
  /** 主题色边框 */
  border: string
}

export const REVIEW_MODE_META: Record<ReviewMode, ReviewModeMeta> = {
  ocr: {
    key: 'ocr',
    label: 'OCR 多智能体审查',
    tagline: '多智能体工作流 · 上下文发现 → 并行审查 → 聚合讨论 → 综合裁决',
    color: '#6366f1',
    soft: '#eef2ff',
    border: '#c7d2fe',
  },
  argus: {
    key: 'argus',
    label: 'Argus 引擎审查',
    tagline: '外部审查 Bot · 创建任务 → Argus 审查 → 异步回报裁决',
    color: '#0d9488',
    soft: '#f0fdfa',
    border: '#99f6e4',
  },
  scan: {
    key: 'scan',
    label: '仓库全量扫描',
    tagline: '全库扫描模式 · 无 PR diff 范围，按路径全量审查并生成会话',
    color: '#d97706',
    soft: '#fffbeb',
    border: '#fde68a',
  },
}

export function resolveReviewMode(detail: Pick<ReviewDetail, 'engine' | 'callback_mode'> | null | undefined): ReviewMode {
  if (!detail) return 'ocr'
  if (detail.engine === 'argus') return 'argus'
  if (detail.callback_mode === 'scan') return 'scan'
  return 'ocr'
}

/** Argus 三阶段进度（ProgressTracker 的 phaseDefinitions，name 对齐 history.stage） */
export const ARGUS_PHASES: PhaseDefinition[] = [
  { name: 'queued', title: '任务创建', description: '审查任务已入队' },
  { name: 'dispatch', title: '派发 Argus', description: '创建 Argus 审查任务，等待外部 Bot 接手' },
  { name: 'argus_report', title: 'Argus 审查与回报', description: 'Argus 完成审查后回报裁决与发现' },
  { name: 'complete', title: '完成', description: '裁决已落库，门禁生效' },
]

/** 仓库扫描进度（scan 走 OCR 引擎，但阶段语义是全库扫描） */
export const SCAN_PHASES: PhaseDefinition[] = [
  { name: 'context', title: '克隆与配置', description: '克隆仓库并加载扫描配置' },
  { name: 'change_context', title: '范围圈定', description: '按路径规则圈定全库扫描范围' },
  { name: 'reviews', title: '全量审查', description: '分批审查圈定文件（可在会话页查看分批进度）' },
  { name: 'complete', title: '完成', description: '扫描完成，报告与发现已生成' },
]

export type ArgusVerdict = 'approve' | 'comment' | 'request_changes' | 'error' | 'pending'

export interface ArgusOutcome {
  verdict: ArgusVerdict
  findingsTotal?: number
  findingsHigh?: number
  findingsMedium?: number
  findingsLow?: number
  /** Argus 侧承接记录 id（可跳转 /argus/:id 查看原始报告） */
  argusReviewId?: string
  argusTaskId?: string
  durationMs?: number
}

/** Event.input/output 可能是 JSON 字符串（thrift any 落库），安全展开 */
function looseParse(value: unknown): Record<string, any> {
  if (value && typeof value === 'object') return value as Record<string, any>
  if (typeof value === 'string' && value.trim()) {
    try { return JSON.parse(value) } catch { return {} }
  }
  return {}
}

/**
 * 从 history / gate 提取 Argus 裁决结果。
 * 优先取最近一条 argus_report 事件；等待回报时 verdict=pending；
 * 无回报事件但已终态时，从 gate.reason（"argus: request_changes"）兜底解析。
 */
export function extractArgusOutcome(detail: ReviewDetail): ArgusOutcome {
  const events: Event[] = [...(detail.history || [])].sort(
    (a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime(),
  )
  const report = [...events].reverse().find((e) => e.stage === 'argus_report')
  const dispatch = [...events].reverse().find((e) => e.stage === 'dispatch')

  const outcome: ArgusOutcome = { verdict: 'pending' }
  const dispatchOut = looseParse(dispatch?.output)
  outcome.argusTaskId = String(dispatchOut.taskId || '')

  if (report) {
    const input = looseParse(report.input)
    const output = looseParse(report.output)
    const v = String(input.verdict || output.verdict || '').toLowerCase()
    if (['approve', 'comment', 'request_changes', 'error'].includes(v)) {
      outcome.verdict = v as ArgusVerdict
    }
    outcome.findingsTotal = Number(input.findings_total ?? output.findings_total ?? 0) || undefined
    outcome.findingsHigh = Number(input.findings_high ?? output.findings_high ?? 0) || 0
    outcome.findingsMedium = Number(input.findings_medium ?? output.findings_medium ?? 0) || 0
    outcome.findingsLow = Number(input.findings_low ?? output.findings_low ?? 0) || 0
    if (input.argus_review_id != null) outcome.argusReviewId = String(input.argus_review_id)
    if (input.argus_task_id) outcome.argusTaskId = String(input.argus_task_id)
    if (report.duration_ms) outcome.durationMs = report.duration_ms
    return outcome
  }

  if (['queued', 'running'].includes(detail.status)) return outcome

  const gateMatch = (detail.gate?.reason || '').match(/argus:\s*(\w+)/)
  if (gateMatch && ['approve', 'comment', 'request_changes', 'error'].includes(gateMatch[1])) {
    outcome.verdict = gateMatch[1] as ArgusVerdict
  } else if (detail.status === 'failed') {
    outcome.verdict = 'error'
  }
  return outcome
}

export const ARGUS_VERDICT_META: Record<ArgusVerdict, { label: string; color: string; soft: string; border: string }> = {
  approve: { label: '通过', color: '#16a34a', soft: '#f0fdf4', border: '#bbf7d0' },
  comment: { label: '评论', color: '#2563eb', soft: '#eff6ff', border: '#bfdbfe' },
  request_changes: { label: '要求修改', color: '#dc2626', soft: '#fef2f2', border: '#fecaca' },
  error: { label: '审查失败', color: '#dc2626', soft: '#fef2f2', border: '#fecaca' },
  pending: { label: '等待 Argus 回报', color: '#6b7280', soft: '#f9fafb', border: '#e5e7eb' },
}
