import request from './request'
import type {
  ArgusHealthReport,
  ArgusReport,
  ArgusReview,
  ArgusReviewListOptions,
  ArgusReviewListResult,
  ArgusTaskSummary,
  ArgusUsage,
} from '@/types/argus'

export async function getArgusReviews(params: ArgusReviewListOptions): Promise<ArgusReviewListResult> {
  return request.get('/argus/reviews', { params }) as any
}

export async function getArgusReview(id: number | string): Promise<ArgusReview> {
  const data: any = await request.get(`/argus/reviews/${id}`)
  return data?.review ?? data
}

/** 误报抑制回流：服务端代理调 argus /findings/suppress（按 fp），命中后下一轮审查起剔除 */
export async function suppressArgusFinding(
  id: number | string,
  fp: string,
): Promise<{ matched: boolean; state?: string; message?: string }> {
  return request.post(`/argus/reviews/${id}/findings/suppress`, { fp }) as any
}

/** 误报抑制兜底模式：无 fp（旧数据）或指纹未命中（措辞漂移）时按 file+意见原文匹配 */
export async function suppressArgusFindingByComment(
  id: number | string,
  file: string,
  comment: string,
): Promise<{ matched: boolean; state?: string; message?: string }> {
  return request.post(`/argus/reviews/${id}/findings/suppress`, { file, comment }) as any
}

/** argus 侧任务列表（对账）：代理 GET /tasks */
export async function getArgusTasks(status?: string, limit = 50): Promise<ArgusTaskSummary[]> {
  const data: any = await request.get('/argus/tasks', { params: { status, limit } })
  return data?.tasks ?? data ?? []
}

/** argus 本月 token 用量 */
export async function getArgusUsage(): Promise<ArgusUsage> {
  return request.get('/argus/usage') as any
}

/** argus 审查健康报告（原始 JSON 透传，这里解析） */
export async function getArgusHealth(): Promise<ArgusHealthReport> {
  const data: any = await request.get('/argus/health')
  const payload = data?.payload ?? data
  if (typeof payload === 'string') {
    try {
      return JSON.parse(payload)
    } catch {
      return {}
    }
  }
  return payload ?? {}
}

/** argus 报告全文（懒拉取代理） */
export async function getArgusReviewReport(id: number | string): Promise<ArgusReport> {
  return request.get(`/argus/reviews/${id}/report`) as any
}

/** 问 Argus：基于该 PR 最近审查报告的追问代理；question 空返回结论摘要 */
export async function askArgus(
  params: { platform: string; owner: string; repo: string; number: string; question: string },
): Promise<{ answer: string }> {
  return request.post('/argus/ask', params, { timeout: 100000 }) as any
}
