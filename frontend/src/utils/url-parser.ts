export interface ParsedMR {
  provider: string
  repo: string
  number: number
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

export function isMRUrl(url: string): boolean {
  return parseMRUrl(url) !== null
}

export const PROVIDER_OPTIONS = [
  { label: 'GitHub', value: 'github' },
  { label: 'GitLab', value: 'gitlab' },
  { label: 'Gitea', value: 'gitea' },
  { label: 'GitCode', value: 'gitcode' },
] as const

export function getProviderLabel(value: string): string {
  return PROVIDER_OPTIONS.find((p) => p.value === value)?.label ?? value
}
