import request from './request'
import type { ReviewDetail, ListResult, ListOptions, CreateReviewOptions } from '@/types/review'

export async function getReviews(params: ListOptions): Promise<ListResult> {
  return request.get('/reviews', { params }) as any
}

export async function getReview(id: string): Promise<ReviewDetail> {
  const data: any = await request.get(`/reviews/${id}`)
  return data?.review ?? data
}

export async function createReview(data: CreateReviewOptions): Promise<ReviewDetail> {
  const headers: Record<string, string> = {}
  if (data.idempotency_key) {
    headers['Idempotency-Key'] = data.idempotency_key
  }
  const { idempotency_key, ...body } = data
  const res: any = await request.post('/reviews', body, {
    headers,
    timeout: data.wait ? 600000 : 30000,
  })
  return res?.review ?? res
}

export async function getSessionMap(reviewId: string, sessionId: string): Promise<any> {
  return request.get(`/reviews/${reviewId}/sessions/${sessionId}/map`) as any
}

export async function getReviewMaps(reviewId: string): Promise<any> {
  return request.get(`/reviews/${reviewId}/maps`) as any
}

export async function getReviewMap(reviewId: string, mapId: string): Promise<any> {
  return request.get(`/reviews/${reviewId}/maps/${mapId}`) as any
}
