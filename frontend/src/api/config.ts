import request from './request'
import type {
  OCRSettings, ProjectPolicy, ProjectPolicyList, GitPlatform, GitPlatformList,
  GitTestResult, GlobalRules, TestLLMResult, PlatformRepo, PlatformBranch,
  PlatformPullRequest, ReviewerAgent, LLMProvider, CustomProvider,
  RulesCheckResult, OCRVersion, ViewerStatus, OCRAssetVersion, OCRRulePackage, OCRToolPackage,
  OCRUpdateInfo, OCRUpgradeStatus, ViewerSessionLocation
} from '@/types/config'

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

function parseTags(raw: any): string[] {
  if (!raw) return []
  if (Array.isArray(raw)) return raw
  if (typeof raw === 'string') {
    try {
      const parsed = JSON.parse(raw)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return raw.split(',').map((s) => s.trim()).filter(Boolean)
    }
  }
  return []
}

function parsePolicy(raw: any): ProjectPolicy {
  if (!raw) return raw
  return { ...raw, ignore_paths: parseIgnorePaths(raw.ignore_paths), rules: parseRules(raw.rules) }
}

function parseReviewerAgent(raw: any): ReviewerAgent {
  return { ...raw, tags: parseTags(raw?.tags) }
}

// ==================== OCR Settings ====================

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

// ==================== LLM Providers ====================

export async function listLLMProviders(): Promise<LLMProvider[]> {
  const resp: any = await request.get('/settings/ocr/providers')
  return resp?.built_in ?? []
}

// ==================== Custom Providers ====================

export async function listCustomProviders(): Promise<CustomProvider[]> {
  const resp: any = await request.get('/settings/ocr/custom-providers')
  return resp?.items ?? []
}

export async function createCustomProvider(data: {
  name: string
  protocol: string
  url?: string
  api_key?: string
  model?: string
  auth_header?: string
  extra_body?: string
  enabled?: boolean
}): Promise<CustomProvider> {
  const resp: any = await request.post('/settings/ocr/custom-providers', data)
  return resp?.provider ?? resp
}

export async function updateCustomProvider(id: number, data: {
  name?: string
  protocol?: string
  url?: string
  api_key?: string
  model?: string
  auth_header?: string
  extra_body?: string
  enabled?: boolean
}): Promise<CustomProvider> {
  const resp: any = await request.put(`/settings/ocr/custom-providers/${id}`, data)
  return resp?.provider ?? resp
}

export async function deleteCustomProvider(id: number): Promise<void> {
  return request.delete(`/settings/ocr/custom-providers/${id}`) as any
}

// ==================== Rules Check ====================

export async function rulesCheck(path: string): Promise<RulesCheckResult> {
  const resp: any = await request.post('/settings/ocr/rules-check', { path })
  return resp?.result ?? resp
}

// ==================== OCR Version ====================

export async function getOCRVersion(): Promise<OCRVersion> {
  return request.get('/settings/ocr/version') as any
}

// ==================== OCR Upgrade ====================

export async function checkOCRUpdate(): Promise<OCRUpdateInfo> {
  return request.get('/settings/ocr/upgrade/info') as any
}

export async function startOCRUpgrade(version?: string): Promise<OCRUpgradeStatus> {
  return request.post('/settings/ocr/upgrade', { version }) as any
}

export async function getOCRUpgradeStatus(): Promise<OCRUpgradeStatus> {
  return request.get('/settings/ocr/upgrade/status') as any
}

// ==================== Viewer ====================

export async function getViewerStatus(): Promise<ViewerStatus> {
  return request.get('/settings/ocr/viewer') as any
}

export async function startViewer(addr?: string): Promise<ViewerStatus> {
  return request.post('/settings/ocr/viewer/start', { addr }) as any
}

export async function stopViewer(): Promise<ViewerStatus> {
  return request.post('/settings/ocr/viewer/stop') as any
}

export async function locateViewerSession(data: {
  workspace_dir: string
  started_at?: string
}): Promise<ViewerSessionLocation> {
  return request.post('/settings/ocr/viewer/locate', data) as any
}

// ==================== Project Policies ====================

export async function getProjectPolicies(): Promise<ProjectPolicyList> {
  const data: any = await request.get('/settings/projects')
  const items = (data?.items || []).map(parsePolicy)
  return { items }
}

export async function putProjectPolicy(data: ProjectPolicy): Promise<ProjectPolicy> {
  const payload: Record<string, any> = {
    provider: data.provider,
    repo: data.repo,
    fail_on: data.fail_on,
    ignore_paths: JSON.stringify(data.ignore_paths || []),
    rules: JSON.stringify(data.rules || []),
    ocr_concurrency: data.ocr_concurrency || 0,
    ocr_timeout_minutes: data.ocr_timeout_minutes || 0,
    ocr_max_tools: data.ocr_max_tools || 0,
    ocr_default_severity: data.ocr_default_severity || '',
    ocr_background: data.ocr_background || '',
    ocr_audience: data.ocr_audience || '',
    ocr_provider: data.ocr_provider || '',
    ocr_url: data.ocr_url || '',
    ocr_model: data.ocr_model || '',
    ocr_max_git_procs: data.ocr_max_git_procs || 0,
    ocr_extra_body: data.ocr_extra_body || '',
    ocr_language: data.ocr_language || '',
    ocr_binary: data.ocr_binary || '',
    ocr_tools_path: data.ocr_tools_path || '',
    ocr_telemetry_exporter: data.ocr_telemetry_exporter || '',
    ocr_telemetry_otlp_endpoint: data.ocr_telemetry_otlp_endpoint || '',
  }

  if (data.ocr_enabled !== undefined && data.ocr_enabled !== null) {
    payload.ocr_enabled = data.ocr_enabled
  }
  if (data.ocr_use_anthropic !== undefined && data.ocr_use_anthropic !== null) {
    payload.ocr_use_anthropic = data.ocr_use_anthropic
  }
  if (data.ocr_telemetry_enabled !== undefined && data.ocr_telemetry_enabled !== null) {
    payload.ocr_telemetry_enabled = data.ocr_telemetry_enabled
  }
  if (data.ocr_telemetry_content_logging !== undefined && data.ocr_telemetry_content_logging !== null) {
    payload.ocr_telemetry_content_logging = data.ocr_telemetry_content_logging
  }

  const resp: any = await request.put('/settings/projects', { policy: payload })
  return parsePolicy(resp?.policy ?? resp)
}

export async function deleteProjectPolicy(provider: string, repo: string): Promise<void> {
  return request.delete('/settings/projects', { params: { provider, repo } }) as any
}

// ==================== Global Rules ====================

export async function getGlobalRules(): Promise<GlobalRules> {
  const data: any = await request.get('/settings/rules/global')
  return data?.rules ?? data ?? { rules: [] }
}

export async function updateGlobalRules(data: GlobalRules): Promise<GlobalRules> {
  const resp: any = await request.put('/settings/rules/global', { rules: data })
  return resp?.rules ?? resp
}

// ==================== Rule Packages ====================

export async function listRulePackages(): Promise<OCRRulePackage[]> {
  const resp: any = await request.get('/settings/rules/packages')
  return resp?.items ?? []
}

export async function saveRulePackage(data: OCRRulePackage): Promise<OCRRulePackage> {
  const resp: any = data.id
    ? await request.put(`/settings/rules/packages/${data.id}`, data)
    : await request.post('/settings/rules/packages', data)
  return resp?.item ?? resp
}

export async function deleteRulePackage(id: number): Promise<void> {
  return request.delete(`/settings/rules/packages/${id}`) as any
}

export async function listRulePackageVersions(id: number): Promise<OCRAssetVersion<OCRRulePackage>[]> {
  const resp: any = await request.get(`/settings/rules/packages/${id}/history`)
  return resp?.items ?? []
}

export async function restoreRulePackageVersion(id: number, version: number): Promise<OCRRulePackage> {
  const resp: any = await request.post(`/settings/rules/packages/${id}/history/${version}/restore`)
  return resp?.item ?? resp
}

// ==================== Tool Packages ====================

export async function listToolPackages(): Promise<OCRToolPackage[]> {
  const resp: any = await request.get('/settings/ocr/tool-packages')
  return resp?.items ?? []
}

export async function saveToolPackage(data: OCRToolPackage): Promise<OCRToolPackage> {
  const payload = { ...data, config: typeof data.config === 'string' ? JSON.parse(data.config) : data.config }
  const resp: any = data.id
    ? await request.put(`/settings/ocr/tool-packages/${data.id}`, payload)
    : await request.post('/settings/ocr/tool-packages', payload)
  return resp?.item ?? resp
}

export async function deleteToolPackage(id: number): Promise<void> {
  return request.delete(`/settings/ocr/tool-packages/${id}`) as any
}

export async function listToolPackageVersions(id: number): Promise<OCRAssetVersion<OCRToolPackage>[]> {
  const resp: any = await request.get(`/settings/ocr/tool-packages/${id}/history`)
  return resp?.items ?? []
}

export async function restoreToolPackageVersion(id: number, version: number): Promise<OCRToolPackage> {
  const resp: any = await request.post(`/settings/ocr/tool-packages/${id}/history/${version}/restore`)
  return resp?.item ?? resp
}

export async function validateToolPackage(config: any, options?: { dry_run?: boolean }): Promise<{ valid: boolean; message?: string; dry_run?: boolean; output?: string }> {
  const payload = {
    config: typeof config === 'string' ? JSON.parse(config) : config,
    dry_run: Boolean(options?.dry_run),
  }
  return request.post('/settings/ocr/tool-packages/validate', payload) as any
}

// ==================== Git Platforms ====================

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
  skip_tls?: boolean
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

// ==================== Reviewer Agents ====================

export async function listReviewerAgents(): Promise<ReviewerAgent[]> {
  const resp: any = await request.get('/settings/agents')
  return (resp?.agents ?? []).map(parseReviewerAgent)
}

export async function createReviewerAgent(data: {
  name: string
  display_name: string
  description?: string
  background: string
  enabled?: boolean
  is_default?: boolean
  priority?: number
  tags?: string[]
}): Promise<ReviewerAgent> {
  const resp: any = await request.post('/settings/agents', data)
  return parseReviewerAgent(resp?.agent)
}

export async function updateReviewerAgent(id: number, data: {
  display_name?: string
  description?: string
  background?: string
  enabled?: boolean
  is_default?: boolean
  priority?: number
  tags?: string[]
}): Promise<ReviewerAgent> {
  const resp: any = await request.put(`/settings/agents/${id}`, data)
  return parseReviewerAgent(resp?.agent)
}

export async function deleteReviewerAgent(id: number): Promise<void> {
  return request.delete(`/settings/agents/${id}`) as any
}

// ==================== Project Agents ====================

export async function getProjectAgents(provider: string, repo: string): Promise<{ provider: string; repo: string; agents: any[] }> {
  return request.get('/settings/project-agents', { params: { provider, repo } }) as any
}

export async function updateProjectAgent(provider: string, repo: string, data: {
  agent_id: number
  enabled: boolean
  priority: number
}): Promise<void> {
  return request.put('/settings/project-agents', { ...data, provider, repo }) as any
}

export async function batchUpdateProjectAgents(provider: string, repo: string, agents: {
  agent_id: number
  enabled: boolean
  priority: number
}[]): Promise<void> {
  return request.post('/settings/project-agents/batch', { provider, repo, agents }) as any
}
