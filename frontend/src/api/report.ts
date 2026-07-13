import request from './request'
import type { WeeklyReport } from '@/types/report'

/**
 * 获取指定项目的周报（Markdown 格式）。
 * provider 和 repo 均为必填查询参数。
 */
export async function getWeeklyReport(params: { provider: string; repo: string }): Promise<WeeklyReport> {
  return request.get('/reports/weekly', { params }) as any
}
