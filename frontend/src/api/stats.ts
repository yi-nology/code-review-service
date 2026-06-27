import request from './request'
import type {
  ContributorListResult,
  ContributorDetail,
  TrendPoint,
  CategoryStat,
  ContributorListOptions,
  ContributorSummary,
  FileRiskStat,
  ProjectDetail,
  ProjectListOptions,
  ProjectListResult,
  ProjectOCRMetrics,
  ProjectReportParams,
  ProjectTrendPoint,
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

export async function getProjects(params: ProjectListOptions): Promise<ProjectListResult> {
  return request.get('/stats/projects', { params }) as any
}

export async function getProject(params: ProjectReportParams): Promise<ProjectDetail> {
  return request.get('/stats/projects/detail', { params }) as any
}

export async function getProjectTrend(params: ProjectReportParams): Promise<ProjectTrendPoint[]> {
  const data: any = await request.get('/stats/projects/trend', { params })
  return data?.trend ?? data ?? []
}

export async function getProjectCategories(params: ProjectReportParams): Promise<CategoryStat[]> {
  const data: any = await request.get('/stats/projects/categories', { params })
  return data?.categories ?? data ?? []
}

export async function getProjectFiles(params: ProjectReportParams): Promise<FileRiskStat[]> {
  const data: any = await request.get('/stats/projects/files', { params })
  return data?.files ?? data ?? []
}

export async function getProjectContributors(params: ProjectReportParams & { limit?: number }): Promise<ContributorSummary[]> {
  const data: any = await request.get('/stats/projects/contributors', { params })
  return data?.contributors ?? data ?? []
}

export async function getProjectOCRMetrics(params: Partial<ProjectReportParams>): Promise<ProjectOCRMetrics> {
  const data: any = await request.get('/stats/projects/ocr', { params })
  return data?.metrics ?? data
}
