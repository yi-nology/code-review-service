export function formatDateShort(time: string): string {
  const date = new Date(time)
  const today = new Date()
  if (date.toDateString() === today.toDateString()) return '今天'
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

export function formatDateRelative(time: string): string {
  const date = new Date(time)
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - date.getTime()) / 86400000)
  if (diffDays === 0) return '今天'
  if (diffDays === 1) return '昨天'
  if (diffDays < 7) return `${diffDays} 天前`
  return date.toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

export function formatDateFull(time: string): string {
  if (!time) return '无记录'
  return new Date(time).toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}

export function formatClock(time: string): string {
  return new Date(time).toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
}

export function getInitials(name: string): string {
  if (!name) return '?'
  return name.slice(0, 2).toUpperCase()
}

export function getProviderInitial(provider: string): string {
  const map: Record<string, string> = { github: 'GH', gitlab: 'GL', gitea: 'GT', gitcode: 'GC' }
  return map[provider] || provider.slice(0, 2).toUpperCase()
}

export function shortId(id: string): string {
  return id.length > 12 ? `${id.slice(0, 8)}…` : id
}
