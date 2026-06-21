export function getStatusLabel(status: string): string {
  const map: Record<string, string> = {
    queued: '排队中',
    running: '审查中',
    success: '已完成',
    failed: '执行失败',
  }
  return map[status] || status
}

export function getStatusType(status: string): string {
  const map: Record<string, string> = {
    queued: 'info',
    running: 'warning',
    success: 'success',
    failed: 'danger',
  }
  return map[status] || 'info'
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
  return PHASE_LABELS[phase] || phase
}

export function getSessionStatusLabel(status: string): string {
  const map: Record<string, string> = {
    active: '进行中',
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消',
  }
  return map[status] || status
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
