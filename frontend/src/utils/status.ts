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
