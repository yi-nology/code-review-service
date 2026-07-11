/**
 * 周报相关类型。
 *
 * 注意：AggregateStats 的键名是 PascalCase（后端 Go 结构体无 JSON 标签，
 * 走默认 marshal），不要写成 snake_case。
 */
export interface AggregateStats {
  Running: number
  GateFailed: number
  Findings: number
  TotalTokens: number
  InputTokens: number
  OutputTokens: number
  CacheReadTokens: number
  CacheWriteTokens: number
}

export interface WeeklyReport {
  format: string
  report: string
  period: string
  stats: AggregateStats
}
