import request from './request'
import type {
  ReviewDetail, ListResult, ListOptions, CreateReviewOptions,
  ScanRepositoryOptions, ScanRepositoryResult, SuggestionPatchCheck, SuggestionPatchPreview,
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

export async function scanRepository(data: ScanRepositoryOptions): Promise<ScanRepositoryResult> {
  return request.post('/reviews/scan', data, { timeout: data.preview ? 120000 : 900000 }) as any
}

export async function cancelReview(id: string): Promise<ReviewDetail> {
  const res: any = await request.post(`/reviews/${id}/cancel`)
  return res?.review ?? res
}

export async function rerunReview(id: string): Promise<ReviewDetail> {
  const res: any = await request.post(`/reviews/${id}/rerun`)
  return res?.review ?? res
}

export async function previewComment(reviewId: string, mode?: 'team'): Promise<{
  review_id: string
  summary: string
  findings: string
  total: number
}> {
  const res: any = await request.get(`/reviews/${reviewId}/preview`, { params: { mode } })
  return res
}

export async function publishReview(reviewId: string, mode?: 'team', summaryOnly = false): Promise<{
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

function findingIndexesParam(indexes?: number[]) {
  return indexes && indexes.length ? indexes.join(',') : undefined
}

export async function previewSuggestionPatch(reviewId: string, indexes?: number[]): Promise<SuggestionPatchPreview> {
  return request.get(`/reviews/${reviewId}/suggestion-patch`, {
    params: { finding_indexes: findingIndexesParam(indexes) },
  }) as any
}

export async function checkSuggestionPatch(reviewId: string, indexes?: number[]): Promise<SuggestionPatchCheck> {
  return request.get(`/reviews/${reviewId}/suggestion-patch/check`, {
    params: { finding_indexes: findingIndexesParam(indexes) },
    timeout: 180000,
  }) as any
}

export async function downloadSuggestionPatch(reviewId: string, indexes?: number[]): Promise<Blob> {
  return request.get(`/reviews/${reviewId}/suggestion-patch.diff`, {
    params: { finding_indexes: findingIndexesParam(indexes) },
    responseType: 'blob',
  }) as any
}
