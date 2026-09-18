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

// ---------- 治理面（观测代理 / 报告 / 问答） ----------

/** argus 任务类型（argus 侧列表返回字符串；发起侧同词表） */
export type ArgusTaskKind = 'code_review' | 'requirement_compliance' | 'visual_check'

/** argus 侧任务概要（GET /api/v1/argus/tasks，对账用） */
export interface ArgusTaskSummary {
  task_id: string
  status: 'pending' | 'running' | 'done' | 'failed' | 'stale'
  kind: string
  platform: string
  owner: string
  repo: string
  number: string
  head_sha?: string
  verdict?: string
  /** Unix 秒 */
  created_at: number
}

/** argus 本月各仓库 token 用量（GET /api/v1/argus/usage） */
export interface ArgusUsageRepo {
  platform: string
  owner: string
  repo: string
  tokens: number
  tasks: number
}

export interface ArgusUsage {
  month: string
  total: number
  /** argus 侧仓库月度预算；0 = 未设置 */
  budget: number
  repos: ArgusUsageRepo[]
}

/** argus 审查健康报告（GET /api/v1/argus/health，payload 原始 JSON 解析后） */
export interface ArgusHealthReport {
  generated_at?: string
  empty?: boolean
  repos?: Array<Record<string, unknown>>
  [key: string]: unknown
}

/** argus 侧报告全文（GET /api/v1/argus/reviews/:id/report，懒拉取代理） */
export interface ArgusReport {
  argus_task_id: string
  status: string
  verdict?: string
  report?: string
  findings_count?: number
}
