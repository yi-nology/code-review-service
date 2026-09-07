export interface Review {
  review_id: string
  provider: string
  repo: string
  status: string
  summary: string
  gate: Gate
  findings: number
  score: number
  grade: string
  report_generated: boolean
  retry_count?: number
  author?: string
  author_name?: string
  author_avatar?: string
  created_at: string
  updated_at: string
}

export interface ReviewDetail {
  review_id: string
  provider: string
  repo: string
  project_id?: string
  pull_request_id?: string
  merge_request_iid?: string
  base_sha: string
  head_sha: string
  callback_mode?: string
  status: string
  summary: string
  gate: Gate
  analysis: Analysis
  history: Event[]
  findings: Finding[]
  report: Report
  judgment?: Judgment
  raw_ocr?: any
  raw_ocr_input?: any
  error?: string
  author?: string
  author_name?: string
  author_avatar?: string
  retry_count?: number
  created_at: string
  updated_at: string
}

// Judgment 是 OCR 结果二次综合判断（agentkit LLM）。
export interface Judgment {
  overall_assessment: string
  confidence: 'high' | 'medium' | 'low'
  recommended_gate: 'pass' | 'fail' | 'conditional_pass'
  gate_reason: string
  prioritized_findings?: FindingJudgment[]
  summary_for_human?: string
}

// FindingJudgment 是单个 finding 的判断结果。
export interface FindingJudgment {
  path: string
  line?: number
  title: string
  severity: string
  is_likely_true: boolean
  priority: number
  reason: string
}

export interface Gate {
  status: string
  passed?: boolean
  reason?: string
}

export interface Analysis {
  overview?: string
  root_cause?: string[]
  risks?: string[]
  recommendations?: string[]
}

export interface Event {
  stage: string
  status: string
  message: string
  details?: Record<string, any>
  created_at: string
  how?: string
  input?: any
  output?: any
  duration_ms?: number
}

export interface Finding {
  severity: string
  category?: string
  path?: string
  line?: number
  start_line?: number
  end_line?: number
  title: string
  body: string
  existing_code?: string
  suggestion_code?: string
  fingerprint?: string
}

export interface FindingFeedback {
  review_id: string
  finding_index: number
  fingerprint?: string
  status: string
  note?: string
  created_at: string
  updated_at: string
}

export interface SuggestionPatchItem {
  index: number
  path: string
  start_line: number
  end_line: number
  title?: string
}

export interface SuggestionPatchSkipped {
  index: number
  path?: string
  title?: string
  reason: string
}

export interface SuggestionPatchPreview {
  review_id: string
  patch: string
  items: SuggestionPatchItem[]
  skipped: SuggestionPatchSkipped[]
  generated_count: number
  skipped_count: number
}

export interface SuggestionPatchCheck extends SuggestionPatchPreview {
  applicable: boolean
  error?: string
  output?: string
}

export interface SuggestionPatchApplyResult extends SuggestionPatchPreview {
  applied: boolean
  pushed: boolean
  branch: string
  commit_sha?: string
  workspace?: string
  cr_number?: number
  cr_url?: string
  output?: string
  error?: string
}

export interface Report {
  ocr: OCRMetadata
  scope: ScopeMetadata
  score: Score
  counts: SeverityCounts
  warnings?: OCRWarning[]
  complete: boolean
  generated: boolean
}

export interface OCRMetadata {
  engine?: string
  status?: string
  files_reviewed?: number
  comments?: number
  total_tokens?: number
  input_tokens?: number
  output_tokens?: number
  cache_read_tokens?: number
  cache_write_tokens?: number
  elapsed?: string
  tool_calls?: {
    total: number
    by_tool?: Record<string, number>
  }
  project_summary?: string
  budget_exceeded?: boolean
}

export interface ScopeMetadata {
  changed_files: number
  reviewed_files: number
  excluded_files: number
  changed_lines?: number
  ignore_paths?: string[]
}

export interface Score {
  value: number
  grade: string
  version: string
  deductions: ScoreDeduction[]
  changed_lines?: number
  findings_density?: number
}

export interface ScoreDeduction {
  reason: string
  points: number
  count?: number
}

export interface SeverityCounts {
  critical: number
  major: number
  minor: number
  info: number
}

export interface OCRWarning {
  type?: string
  message: string
  path?: string
}

export interface ListResult {
  items: Review[]
  total: number
  page: number
  page_size: number
  total_pages: number
  stats: ListStats
}

export interface ListStats {
  total: number
  running: number
  gate_failed: number
  findings: number
  total_tokens?: number
}

export interface ListOptions {
  status?: string
  gate_status?: string
  provider?: string
  repo?: string
  page?: number
  page_size?: number
  sort_by?: string
  sort_order?: string
}

export interface CreateReviewOptions {
  provider: string
  repo: string
  base_sha?: string
  head_sha?: string
  commit_sha?: string
  project_id?: string
  pull_request_id?: string
  merge_request_iid?: string
  idempotency_key?: string
  callback_mode?: string
  wait?: boolean
  requirements?: string[]
  ephemeral_reviewers?: EphemeralReviewer[]
}

export interface ScanRepositoryOptions {
  provider: string
  repo: string
  project_id?: string
  head_sha: string
  paths?: string[]
  exclude?: string[]
  background?: string
  max_tokens_budget?: number
  batch?: 'none' | 'by-language' | 'by-directory' | ''
  no_plan?: boolean
  no_dedup?: boolean
  no_summary?: boolean
  preview?: boolean
  timeout_minutes?: number
  resume_session_id?: string
}

export interface ScanRepositoryResult {
  review_id?: string
  preview_files?: string[]
  result?: {
    summary: string
    ocr: OCRMetadata
    warnings?: OCRWarning[]
    findings: Finding[]
  }
  report?: Report
  analysis?: Analysis
  warnings?: OCRWarning[]
}

export interface EphemeralReviewer {
  name?: string
  background: string
}

export interface PublishPreview {
  review_id: string
  summary: string
  findings: string
  total: number
}

export interface PublishResult {
  message: string
  review_id: string
}

export interface PublishStatusItem {
  finding_key: string
  kind: string
  status: string
  attempts: number
  external_id?: string
  url?: string
  last_error?: string
}

export interface PublishStatus {
  review_id: string
  total: number
  published: number
  failed: number
  downgraded: number
  pending: number
  items: PublishStatusItem[]
}
