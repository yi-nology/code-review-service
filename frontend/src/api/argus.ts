import request from './request'
import type {
  ArgusReview,
  ArgusReviewListOptions,
  ArgusReviewListResult,
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
