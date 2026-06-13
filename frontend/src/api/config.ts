import request from './request'
import type { OCRSettings, ProjectPolicy, ProjectPolicyList, GitPlatform, GitPlatformList, GitTestResult, GlobalRules, TestLLMResult, PlatformRepo, PlatformBranch, PlatformPullRequest } from '@/types/config'

function parseIgnorePaths(raw: any): string[] {
  if (!raw) return []
  if (Array.isArray(raw)) return raw
  if (typeof raw === 'string') {
    try { return JSON.parse(raw) } catch { return [] }
  }
  return []
}

function parseRules(raw: any): any[] {
  if (!raw) return []
  if (Array.isArray(raw)) return raw
  if (typeof raw === 'string') {
    try { return JSON.parse(raw) } catch { return [] }
  }
  return []
}

function parsePolicy(raw: any): ProjectPolicy {
  if (!raw) return raw
  return { ...raw, ignore_paths: parseIgnorePaths(raw.ignore_paths), rules: parseRules(raw.rules) }
}

export async function getOCRSettings(): Promise<OCRSettings> {
  const data: any = await request.get('/settings/ocr')
  return data?.settings ?? data
}

export async function updateOCRSettings(data: OCRSettings): Promise<OCRSettings> {
  const resp: any = await request.put('/settings/ocr', { settings: data })
  return resp?.settings ?? resp
}

export async function testLLM(): Promise<TestLLMResult> {
  const resp: any = await request.post('/settings/ocr/test-llm')
  return resp?.result ?? resp
}

export async function getProjectPolicies(): Promise<ProjectPolicyList> {
  const data: any = await request.get('/settings/projects')
  const items = (data?.items || []).map(parsePolicy)
  return { items }
}

export async function putProjectPolicy(data: ProjectPolicy): Promise<ProjectPolicy> {
  const payload = {
    provider: data.provider,
    repo: data.repo,
    fail_on: data.fail_on,
    ignore_paths: JSON.stringify(data.ignore_paths || []),
    rules: JSON.stringify(data.rules || []),
  }
  const resp: any = await request.put('/settings/projects', { policy: payload })
  return parsePolicy(resp?.policy ?? resp)
}

export async function deleteProjectPolicy(provider: string, repo: string): Promise<void> {
  return request.delete('/settings/projects', { params: { provider, repo } }) as any
}

export async function getGlobalRules(): Promise<GlobalRules> {
  const data: any = await request.get('/settings/rules/global')
  return data?.rules ?? data ?? { rules: [] }
}

export async function updateGlobalRules(data: GlobalRules): Promise<GlobalRules> {
  const resp: any = await request.put('/settings/rules/global', { rules: data })
  return resp?.rules ?? resp
}

export async function getGitPlatforms(): Promise<GitPlatformList> {
  return request.get('/settings/platforms') as any
}

export async function saveGitPlatform(data: GitPlatform): Promise<GitPlatform> {
  return request.put('/settings/platforms', data) as any
}

export async function deleteGitPlatform(provider: string): Promise<void> {
  return request.delete('/settings/platforms', { params: { provider } }) as any
}

export async function testGitPlatform(data: {
  provider: string
  base_url: string
  token: string
}): Promise<GitTestResult> {
  const resp: any = await request.post('/settings/platforms/test', data)
  return resp?.result ?? resp
}

export async function getProviderRepos(provider: string, page = 1, perPage = 30): Promise<PlatformRepo[]> {
  const resp: any = await request.get(`/platforms/${provider}/repos`, { params: { page, per_page: perPage } })
  return resp?.items ?? []
}

export async function getProviderBranches(provider: string, owner: string, repo: string): Promise<PlatformBranch[]> {
  const resp: any = await request.get(`/platforms/${provider}/repos/${owner}/${repo}/branches`)
  return resp?.items ?? []
}

export async function getProviderPulls(provider: string, owner: string, repo: string, state = 'opened'): Promise<PlatformPullRequest[]> {
  const resp: any = await request.get(`/platforms/${provider}/repos/${owner}/${repo}/pulls`, { params: { state } })
  return resp?.items ?? []
}

export async function getBranchSHA(provider: string, owner: string, repo: string, branch: string): Promise<string> {
  const resp: any = await request.get(`/platforms/${provider}/repos/${owner}/${repo}/branch-sha`, { params: { branch } })
  return resp?.sha ?? ''
}
