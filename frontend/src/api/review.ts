import request from './request'
import type {
  ReviewDetail, ListResult, ListOptions, CreateReviewOptions,
} from '@/types/review'

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

export async function previewReview(opts: {
  provider: string
  repo: string
  base_sha?: string
  head_sha?: string
  project_id?: string
  pull_request_id?: string
  merge_request_iid?: string
}): Promise<string[]> {
  const res: any = await request.post('/reviews/preview', opts, { timeout: 60000 })
  return res?.files ?? []
}

export async function cancelReview(id: string): Promise<ReviewDetail> {
  const res: any = await request.post(`/reviews/${id}/cancel`)
  return res?.review ?? res
}

export async function rerunReview(id: string): Promise<ReviewDetail> {
  const res: any = await request.post(`/reviews/${id}/rerun`)
  return res?.review ?? res
}

function parseJSON(raw: any): any {
  if (raw == null) return null
  if (typeof raw === 'string') {
    try { return JSON.parse(raw) } catch { return raw }
  }
  return raw
}

function parseReviewMap(raw: any): any {
  if (!raw) return raw
  return {
    ...raw,
    sections: parseJSON(raw.sections) ?? [],
    dependencies: parseJSON(raw.dependencies) ?? [],
  }
}

export async function getSessionMap(reviewId: string, sessionId: string): Promise<any> {
  const data: any = await request.get(`/reviews/${reviewId}/sessions/${sessionId}/map`)
  return parseReviewMap(data?.map ?? data)
}

export async function getReviewMaps(reviewId: string): Promise<any> {
  return request.get(`/reviews/${reviewId}/maps`) as any
}

export async function getReviewMap(reviewId: string, mapId: string): Promise<any> {
  const data: any = await request.get(`/reviews/${reviewId}/maps/${mapId}`)
  return parseReviewMap(data?.map ?? data)
}

export async function generateReviewMap(reviewId: string, sessionId: string, diff?: string): Promise<any> {
  const body: Record<string, string> = { session_id: sessionId }
  if (diff) body.diff = diff
  const res: any = await request.post(`/reviews/${reviewId}/maps`, body, { timeout: 300000 })
  return res?.map ?? res
}

export async function updateFileReviewed(reviewId: string, mapId: string, filePath: string, reviewed: boolean): Promise<void> {
  return request.put(`/reviews/${reviewId}/maps/${mapId}/files/${encodeURIComponent(filePath)}/reviewed`, { reviewed }) as any
}

export async function updateSectionProgress(reviewId: string, mapId: string, sectionId: string, reviewed: boolean): Promise<void> {
  return request.put(`/reviews/${reviewId}/maps/${mapId}/sections/${sectionId}/progress`, { reviewed }) as any
}

export async function previewComment(reviewId: string, mode: 'team' | 'human' = 'team'): Promise<{
  review_id: string
  summary: string
  findings: string
  total: number
}> {
  const res: any = await request.get(`/reviews/${reviewId}/preview`, { params: { mode } })
  return res
}

export async function publishReview(reviewId: string, mode: 'team' | 'human' = 'team', summaryOnly = false): Promise<{
  message: string
  review_id: string
}> {
  return request.post(`/reviews/${reviewId}/publish`, { mode, summary_only: summaryOnly }) as any
}

export async function updatePublishedComment(reviewId: string): Promise<{
  message: string
  review_id: string
}> {
  return request.put(`/reviews/${reviewId}/publish`) as any
}
