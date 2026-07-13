export interface OCRSettings {
  enabled: boolean
  binary?: string
  audience?: 'human' | 'agent'
  default_severity?: 'critical' | 'major' | 'minor' | 'info'
  provider?: string
  url: string
  auth_token?: string
  auth_header?: string
  extra_headers?: string
  model: string
  use_anthropic: boolean
  extra_body?: any
  language: string
  concurrency: number
  max_git_procs: number
  timeout_minutes: number
  max_tools: number
  background?: string
  tools_path?: string
  tools_config?: string
  mcp_servers?: string
  telemetry_enabled?: boolean
  telemetry_exporter?: string
  telemetry_otlp_endpoint?: string
  telemetry_content_logging?: boolean
  reasoning_mode?: 'disabled' | 'light' | 'medium' | 'full' | 'custom'
  reasoning_budget?: number
  context_window?: number
  max_output_tokens?: number
}

// McpServer 描述单个 MCP server（与 OCR CLI 的 mcp_servers 配置对齐）。
export interface McpServer {
  command: string
  args?: string[]
  env?: string[]
  tools?: string[]
  setup?: string
}

export interface ReviewRule {
  path: string
  rule: string
}

export interface ProjectPolicy {
  provider: string
  repo: string
  fail_on: string
  ignore_paths: string[]
  rules?: ReviewRule[]
  rule_package_ids?: number[]
  ocr_enabled?: boolean | null
  ocr_concurrency?: number
  ocr_timeout_minutes?: number
  ocr_max_tools?: number
  ocr_default_severity?: 'critical' | 'major' | 'minor' | 'info' | ''
  ocr_background?: string
  ocr_audience?: 'human' | 'agent' | ''
  ocr_provider?: string
  ocr_url?: string
  ocr_model?: string
  ocr_max_git_procs?: number
  ocr_use_anthropic?: boolean
  ocr_extra_body?: string
  ocr_language?: string
  ocr_binary?: string
  ocr_tools_path?: string
  ocr_telemetry_enabled?: boolean
  ocr_telemetry_exporter?: string
  ocr_telemetry_otlp_endpoint?: string
  ocr_telemetry_content_logging?: boolean
}

export interface ProjectPolicyList {
  items: ProjectPolicy[]
}

export interface GlobalRules {
  rules: ReviewRule[]
}

export interface OCRRulePackage {
  id?: number
  name: string
  description?: string
  languages?: string[]
  tags?: string[]
  includes?: string[]
  excludes?: string[]
  rules: ReviewRule[]
  enabled: boolean
  project_refs?: string[]
  built_in?: boolean
  scope?: 'global' | 'project'
}

export interface OCRToolPackage {
  id?: number
  name: string
  description?: string
  scan_type?: 'all' | 'diff' | 'scan'
  config: any
  enabled: boolean
  project_refs?: string[]
  built_in?: boolean
}

export interface OCRAssetVersion<T = any> {
  version: number
  created_at: string
  snapshot: T
}

export interface TestLLMResult {
  success: boolean
  message?: string
}

export interface GitPlatform {
  provider: string
  base_url: string
  token: string
  enabled: boolean
  skip_tls?: boolean
  installation_id?: string
  webhook_secret?: string
}

export interface GitPlatformList {
  items: GitPlatform[]
}

export interface GitTestResult {
  connected: boolean
  platform: string
  user_name: string
  message: string
}

export interface PlatformRepo {
  id: number
  full_name: string
  name: string
  owner: string
  description: string
  default_branch: string
  private: boolean
}

export interface PlatformBranch {
  name: string
}

export interface PlatformPullRequest {
  number: number
  title: string
  state: string
  source_branch: string
  target_branch: string
  head_sha: string
  base_sha: string
  web_url: string
  author?: {
    username: string
    name: string
    avatar_url: string
  }
  created_at: string
  updated_at: string
}

export interface ReviewerAgent {
  id: number
  name: string
  display_name: string
  description: string
  background: string
  enabled: boolean
  priority: number
  is_default: boolean
  tags: string[] | null
  rule_file?: string
  rule_package_ids?: number[]
  redundancy?: number
  created_at?: string
  updated_at?: string
}

export interface LLMProvider {
  name: string
  protocol: string
  base_url?: string
  default_model?: string[]
}

export interface CustomProvider {
  id: number
  name: string
  protocol: string
  url?: string
  api_key?: string
  model?: string
  auth_header?: string
  extra_body?: string
  enabled: boolean
}

export interface RulesCheckResult {
  applicable: boolean
  rule?: string
  message?: string
}

export interface OCRVersion {
  version: string
  commit?: string
  platform?: string
  build_time?: string
}

export interface OCRUpdateInfo {
  current_version: string
  latest_version: string
  update_available: boolean
  release_notes?: string
  release_url?: string
}

export interface OCRUpgradeStatus {
  state: 'idle' | 'downloading' | 'verifying' | 'installing' | 'done' | 'failed'
  message?: string
  version?: string
  progress?: number
}

export interface ViewerStatus {
  running: boolean
  addr?: string
  message?: string
}

export interface ViewerSessionLocation {
  repo_slug: string
  path: string
  session_id?: string
  matched: boolean
}

export interface ModelOption {
  id: string
  display_name: string
  max_tokens: number
  supports_thinking: boolean
}

export interface ProviderPreset {
  id: string
  display_name: string
  category: string
  type: string
  base_url: string
  protocol: string
  supports_thinking: boolean
  models: ModelOption[]
  default_model: string
  icon: string
  region: string
  requires_key: boolean
}
