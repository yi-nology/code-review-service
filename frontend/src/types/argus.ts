// Argus 审查承接（push 模型）的前端类型，对齐 GET /api/v1/argus/reviews 返回。

export interface ArgusFinding {
  fp?: string
  file: string
  line: number
  severity: 'high' | 'medium' | 'low'
  comment: string
  suggested_fix?: string
  by?: string
  /** 误报抑制回流状态：'suppressed' = argus 已确认抑制（下一轮审查起剔除） */
  suppress_state?: string
}

export interface ArgusReview {
  id: number
  argus_task_id: string
  platform: string
  owner: string
  repo: string
  number: string
  title: string
  head_sha: string
  base_branch?: string
  verdict: 'approve' | 'comment' | 'request_changes' | 'error'
  findings_total: number
  findings_high: number
  findings_medium: number
  findings_low: number
  plugins?: string[]
  duration_ms?: number
  token_usage?: number
  /** 仅 verdict=error 时有意义：argus 侧审查失败原因 */
  error_message?: string
  occurred_at: string
  findings?: ArgusFinding[]
}

export interface ArgusReviewListResult {
  items: ArgusReview[]
  total: number
}

export interface ArgusReviewListOptions {
  repo?: string
  /** 平台精确匹配（多平台同名 repo 区分用，如 gitea / github） */
  platform?: string
  severity?: 'high' | 'medium' | 'low' | ''
  since?: string
  page?: number
  page_size?: number
}
