// 将 "owner/repo" 全名拆分为 owner 与 repo 两部分。
// 支持 repo 含子路径（如 "group/sub/repo"），标准 "owner/repo" 之外（少于两段）返回 null。
export function splitRepo(fullName: string): { owner: string; repo: string } | null {
  if (!fullName) return null
  const parts = fullName.split('/').filter(Boolean)
  if (parts.length < 2) return null
  return { owner: parts[0], repo: parts.slice(1).join('/') }
}
