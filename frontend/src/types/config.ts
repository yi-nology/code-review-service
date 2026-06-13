export interface OCRSettings {
  enabled: boolean
  provider?: string
  plan?: string
  url: string
  auth_token?: string
  auth_header?: string
  model: string
  use_anthropic: boolean
  extra_body?: any
  language: string
  concurrency: number
  max_git_procs: number
  timeout_minutes: number
  max_tools: number
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
}

export interface ProjectPolicyList {
  items: ProjectPolicy[]
}

export interface GlobalRules {
  rules: ReviewRule[]
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
