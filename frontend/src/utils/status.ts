/**
 * 状态 / 阶段文案与配色的单一映射表。
 *
 * 约定：**未知取值一律不把原始枚举直出给用户**。后端一旦新增状态而前端
 * 尚未同步，直出 `success_with_publish_failed` 这类原始串会让界面出现
 * 英文枚举混排（曾经真实发生过），兜底文案必须是人能读懂的。
 */

/** 评审状态 → 文案。键覆盖后端 domain/review 的全部落库状态。 */
const STATUS_LABELS: Record<string, string> = {
  queued: '排队中',
  running: '审查中',
  success: '已完成',
  // 评审本身成功、但评论回写失败（写回 PR 失败）：必须与 success 区分开，
  // 否则用户会以为评论已经发出去了。
  success_with_publish_failed: '已完成，回写失败',
  completed: '已完成',
  failed: '执行失败',
  cancelled: '已取消',
}

/** 评审状态 → Element Plus 标签类型。 */
const STATUS_TYPES: Record<string, string> = {
  queued: 'info',
  running: 'warning',
  success: 'success',
  success_with_publish_failed: 'warning',
  completed: 'success',
  failed: 'danger',
  cancelled: 'info',
}

export function getStatusLabel(status: string): string {
  return STATUS_LABELS[status] || '未知状态'
}

export function getStatusType(status: string): string {
  return STATUS_TYPES[status] || 'info'
}

export function getSeverityType(severity: string): string {
  const map: Record<string, string> = {
    critical: 'danger',
    major: 'warning',
    minor: 'info',
    info: '',
  }
  return map[severity] || ''
}

/**
 * 评审流程阶段 → 文案。
 *
 * 键与 internal/domain/session/phase.go 的阶段常量一一对应（context /
 * change_context / analysis / reviews / aggregation / discourse /
 * synthesis / complete），改动时两侧要同步。
 */
const PHASE_LABELS: Record<string, string> = {
  context: '上下文发现',
  change_context: '变更分析',
  analysis: '分析',
  reviews: '审查',
  aggregation: '聚合',
  discourse: '讨论',
  synthesis: '综合',
  complete: '完成',
}

export function getPhaseLabel(phase: string): string {
  return PHASE_LABELS[phase] || '未知阶段'
}

const SESSION_STATUS_LABELS: Record<string, string> = {
  active: '进行中',
  completed: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

export function getSessionStatusLabel(status: string): string {
  return SESSION_STATUS_LABELS[status] || '未知状态'
}

export function getSessionStatusType(status: string): string {
  const map: Record<string, string> = {
    active: 'primary',
    completed: 'success',
    failed: 'danger',
    cancelled: 'warning',
  }
  return map[status] || 'info'
}

/**
 * 审查步骤（history event）状态 → 文案。
 *
 * 后端写入的取值：completed / failed / running（internal/service/review/
 * audit.go、internal/service/progress/tracker.go）。
 */
export function getEventStatusLabel(status: string): string {
  const map: Record<string, string> = {
    completed: '完成',
    success: '完成',
    failed: '失败',
    error: '失败',
    running: '执行中',
    started: '进行中',
    pending: '等待中',
    skipped: '已跳过',
  }
  return map[status] || '未知'
}
