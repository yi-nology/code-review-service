import request from './request'
import type {
  ContributorListResult,
  ContributorDetail,
  TrendPoint,
  CategoryStat,
  ContributorListOptions,
} from '@/types/stats'

export async function getContributors(params: ContributorListOptions): Promise<ContributorListResult> {
  return request.get('/stats/contributors', { params }) as any
}

export async function getContributor(author: string, params?: { provider?: string; repo?: string; from?: string; to?: string }): Promise<ContributorDetail> {
  return request.get(`/stats/contributors/${author}`, { params }) as any
}

export async function getContributorTrend(author: string, params?: { provider?: string; repo?: string; from?: string; to?: string }): Promise<TrendPoint[]> {
  const data: any = await request.get(`/stats/contributors/${author}/trend`, { params })
  return data?.trend ?? data ?? []
}

export async function getContributorCategories(author: string, params?: { provider?: string; repo?: string; from?: string; to?: string }): Promise<CategoryStat[]> {
  const data: any = await request.get(`/stats/contributors/${author}/categories`, { params })
  return data?.categories ?? data ?? []
}
