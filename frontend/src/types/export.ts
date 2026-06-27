export interface ExportOptions {
  format: 'markdown' | 'json' | 'html'
  includeDetails: boolean
  includeFindings: boolean
  includeSummary: boolean
}

export interface ExportReport {
  reviewId: string
  provider: string
  repo: string
  status: string
  summary: string
  score: number
  grade: string
  findings: ExportFinding[]
  createdAt: string
  updatedAt: string
}

export interface ExportFinding {
  severity: string
  category: string
  path: string
  line: number
  title: string
  body: string
  suggestion?: string
}
