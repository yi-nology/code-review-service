/**
 * 周报相关类型。
 * 后端 AggregateStats 已加 snake_case JSON tag，与其余 review API 契约一致。
 */
import type { ContributorSummary } from './stats'

export interface AggregateStats {
  running: number
  gate_failed: number
  findings: number
  total_tokens: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
}

export interface WeeklyReport {
  format: string
  report: string
  period: string
  stats: AggregateStats
  /** 本周提交人排行（按审查次数降序，Top 10），可直接渲染交互式卡片。 */
  contributors?: ContributorSummary[]
}
