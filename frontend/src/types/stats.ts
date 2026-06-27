export interface ContributorSummary {
  author: string
  author_name: string
  author_avatar: string
  review_count: number
  avg_score: number
  pass_rate: number
  total_findings: number
  critical_count: number
  major_count: number
  minor_count: number
  info_count: number
  last_review_at: string
}

export interface ContributorDetail extends ContributorSummary {
  median_score: number
  max_score: number
  min_score: number
  grade_distribution: Record<string, number>
  avg_findings: number
  avg_changed_lines: number
}

export interface TrendPoint {
  period: string
  avg_score: number
  review_count: number
}

export interface CategoryStat {
  category: string
  count: number
}

export interface ContributorListResult {
  contributors: ContributorSummary[]
  total: number
  page: number
  page_size: number
}

export interface ContributorListOptions {
  provider?: string
  repo?: string
  from?: string
  to?: string
  sort?: string
  order?: string
  page?: number
  page_size?: number
}

export interface ProjectSummary {
  provider: string
  repo: string
  review_count: number
  avg_score: number
  pass_rate: number
  total_findings: number
  critical_count: number
  major_count: number
  minor_count: number
  info_count: number
  last_review_at: string
}

export interface ProjectDetail extends ProjectSummary {
  median_score: number
  max_score: number
  min_score: number
  grade_distribution: Record<string, number>
  avg_findings: number
  avg_changed_lines: number
}

export interface ProjectTrendPoint {
  period: string
  avg_score: number
  review_count: number
  total_findings: number
}

export interface FileRiskStat {
  path: string
  count: number
  critical_count: number
  major_count: number
}

export interface ProjectListResult {
  projects: ProjectSummary[]
  total: number
  page: number
  page_size: number
}

export interface ProjectOCRMetrics {
  review_count: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  total_reviewed_files: number
  avg_tokens: number
  avg_score: number
  avg_findings_density: number
  grade_distribution: Record<string, number>
  tool_calls: Array<{ tool: string; count: number }>
}

export interface ProjectListOptions {
  provider?: string
  repo?: string
  from?: string
  to?: string
  sort?: string
  order?: string
  page?: number
  page_size?: number
}

export interface ProjectReportParams {
  provider: string
  repo: string
  from?: string
  to?: string
}
