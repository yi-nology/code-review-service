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
  raw_ocr?: any
  raw_ocr_input?: any
  error?: string
  author?: string
  author_name?: string
  author_avatar?: string
  created_at: string
  updated_at: string
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
}

export interface ListOptions {
  status?: string
  provider?: string
  repo?: string
  page?: number
  page_size?: number
}

export interface CreateReviewOptions {
  provider: string
  repo: string
  base_sha?: string
  head_sha?: string
  project_id?: string
  pull_request_id?: string
  merge_request_iid?: string
  idempotency_key?: string
  callback_mode?: string
  wait?: boolean
}
