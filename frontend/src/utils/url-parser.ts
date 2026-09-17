export interface ParsedMR {
  provider: string
  repo: string
  number: number
}

export interface ParsedCompare {
  provider: string
  repo: string
  base_branch: string
  head_branch: string
}

/** PlatformEndpoint 已配置代码平台的 provider + 服务地址（用于自定义域名归属判定）。 */
export interface PlatformEndpoint {
  provider: string
  baseUrl?: string
}

/** 内置公共平台的域名归属。 */
const BUILTIN_HOSTS: Record<string, string> = {
  'github.com': 'github',
  'gitlab.com': 'gitlab',
  'gitee.com': 'gitee',
  'gitcode.com': 'gitcode',
}

function normalizeHost(host: string): string {
  return host.toLowerCase().replace(/^www\./, '')
}

/**
 * providerForHost 判定 URL host 属于哪个平台：优先匹配已配置平台的
 * base_url 域名（自建 Gitea/Forgejo/GitLab 等），再回退内置公共平台。
 * 未匹配返回 null（由路径形状兜底推断）。
 */
export function providerForHost(host: string, platforms?: PlatformEndpoint[]): string | null {
  const normalized = normalizeHost(host)
  if (platforms) {
    for (const p of platforms) {
      if (!p.baseUrl) continue
      try {
        if (normalizeHost(new URL(p.baseUrl).host) === normalized) return p.provider
      } catch {
        // base_url 非法时跳过该平台
      }
    }
  }
  return BUILTIN_HOSTS[normalized] ?? null
}

function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url.replace(/^https?:\/\/([^/]+).*/, '$1')
  }
}

/**
 * 路径形状 → 默认 provider（host 未能判定时用）。
 * 自定义域名 + /pulls/ 多为自建 Gitea/Forgejo；/pull/ 保留 github 兜底
 *（与历史行为一致）。merge_requests 形状只属于 GitLab。
 */
const SHAPE_PATTERNS: Array<{ provider: string | null; regex: RegExp }> = [
  { provider: 'gitlab', regex: /^https?:\/\/[^/]+\/(.+?)\/([^/]+)\/-\/merge_requests\/(\d+)/ },
  { provider: null, regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/pull\/(\d+)/ },
  { provider: null, regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/pulls\/(\d+)/ },
]

function defaultProviderForShape(shapeProvider: string | null, url: string, platforms?: PlatformEndpoint[]): string {
  if (shapeProvider) return shapeProvider
  const fallback = providerForHost(hostOf(url), platforms)
  if (fallback) return fallback
  return /\/pulls\/\d+/.test(url) ? 'gitea' : 'github'
}

export function parseMRUrl(url: string, platforms?: PlatformEndpoint[]): ParsedMR | null {
  const trimmed = url.trim()
  if (!trimmed) return null

  for (const { provider, regex } of SHAPE_PATTERNS) {
    const match = trimmed.match(regex)
    if (match) {
      return {
        provider: defaultProviderForShape(provider, trimmed, platforms),
        repo: `${match[1]}/${match[2]}`,
        number: parseInt(match[3], 10),
      }
    }
  }

  return null
}

const COMPARE_SHAPE_PATTERNS: Array<{ provider: string | null; regex: RegExp }> = [
  { provider: 'gitlab', regex: /^https?:\/\/[^/]+\/(.+?)\/([^/]+)\/-\/compare\/([^?]+?)\.\.\.([^?#]+)/ },
  { provider: null, regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/compare\/([^?]+?)\.\.\.([^?#]+)/ },
]

export function parseCompareUrl(url: string, platforms?: PlatformEndpoint[]): ParsedCompare | null {
  const trimmed = url.trim()
  if (!trimmed) return null

  for (const { provider, regex } of COMPARE_SHAPE_PATTERNS) {
    const match = trimmed.match(regex)
    if (match) {
      return {
        provider: defaultProviderForShape(provider, trimmed, platforms),
        repo: `${match[1]}/${match[2]}`,
        base_branch: normalizeBranch(match[3]),
        head_branch: normalizeBranch(match[4]),
      }
    }
  }

  const queryMatch = trimmed.match(/^https?:\/\/[^/]+\/(.+?)\/([^/]+)\/-\/compare\?(.+)$/)
  if (queryMatch) {
    const params = new URLSearchParams(queryMatch[3])
    const from = params.get('from')
    const to = params.get('to')
    if (from && to) {
      return {
        provider: providerForHost(hostOf(trimmed), platforms) ?? 'gitlab',
        repo: `${queryMatch[1]}/${queryMatch[2]}`,
        base_branch: normalizeBranch(from),
        head_branch: normalizeBranch(to),
      }
    }
  }

  return null
}

export function isMRUrl(url: string): boolean {
  return parseMRUrl(url) !== null
}

function normalizeBranch(branch: string): string {
  return decodeURIComponent(branch).replace(/^[^:]+:/, '')
}

export const PROVIDER_OPTIONS = [
  { label: 'GitHub', value: 'github' },
  { label: 'GitLab', value: 'gitlab' },
  { label: 'Gitea', value: 'gitea' },
  { label: 'Forgejo', value: 'forgejo' },
  { label: 'GitCode', value: 'gitcode' },
  { label: 'Gitee', value: 'gitee' },
  { label: 'Tencent Code', value: 'tencent_code' },
] as const

export function getProviderLabel(value: string): string {
  return PROVIDER_OPTIONS.find((p) => p.value === value)?.label ?? value
}
