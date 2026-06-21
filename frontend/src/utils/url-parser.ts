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

const PATTERNS: Array<{
  provider: string
  regex: RegExp
}> = [
  {
    provider: 'github',
    regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/pull\/(\d+)/,
  },
  {
    provider: 'gitlab',
    regex: /^https?:\/\/[^/]+\/(.+?)\/([^/]+)\/-\/merge_requests\/(\d+)/,
  },
  {
    provider: 'gitea',
    regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/pulls\/(\d+)/,
  },
  {
    provider: 'gitcode',
    regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/pulls?\/(\d+)/,
  },
]

const COMPARE_PATTERNS: Array<{
  provider: string
  regex: RegExp
}> = [
  {
    provider: 'github',
    regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/compare\/([^?]+?)\.\.\.([^?#]+)/,
  },
  {
    provider: 'gitlab',
    regex: /^https?:\/\/[^/]+\/(.+?)\/([^/]+)\/-\/compare\/([^?]+?)\.\.\.([^?#]+)/,
  },
  {
    provider: 'gitea',
    regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/compare\/([^?]+?)\.\.\.([^?#]+)/,
  },
  {
    provider: 'gitcode',
    regex: /^https?:\/\/[^/]+\/([^/]+)\/([^/]+)\/compare\/([^?]+?)\.\.\.([^?#]+)/,
  },
]

export function parseMRUrl(url: string): ParsedMR | null {
  const trimmed = url.trim()
  if (!trimmed) return null

  for (const { provider, regex } of PATTERNS) {
    const match = trimmed.match(regex)
    if (match) {
      return {
        provider,
        repo: `${match[1]}/${match[2]}`,
        number: parseInt(match[3], 10),
      }
    }
  }

  return null
}

export function parseCompareUrl(url: string): ParsedCompare | null {
  const trimmed = url.trim()
  if (!trimmed) return null

  for (const { provider, regex } of COMPARE_PATTERNS) {
    const match = trimmed.match(regex)
    if (match) {
      return {
        provider,
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
        provider: 'gitlab',
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
