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
