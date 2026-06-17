const DISPLAY_TIME_ZONE = 'Asia/Shanghai'

function parseDate(time: string): Date | null {
  if (!time) return null
  const date = new Date(time)
  if (Number.isNaN(date.getTime())) return null
  return date
}

function dateTimeParts(time: string): Record<string, string> | null {
  const date = parseDate(time)
  if (!date) return null
  const parts = new Intl.DateTimeFormat('zh-CN', {
    timeZone: DISPLAY_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).formatToParts(date)
  return Object.fromEntries(parts.map((part) => [part.type, part.value]))
}

export function formatDateShort(time: string): string {
  const date = parseDate(time)
  if (!date) return '-'
  const parts = dateTimeParts(time)
  const todayParts = dateTimeParts(new Date().toISOString())
  if (parts?.year === todayParts?.year && parts?.month === todayParts?.month && parts?.day === todayParts?.day) return '今天'
  return parts ? `${parts.month}/${parts.day}` : '-'
}

export function formatDateRelative(time: string): string {
  const date = parseDate(time)
  if (!date) return '无记录'
  const now = new Date()
  const diffDays = Math.floor((now.getTime() - date.getTime()) / 86400000)
  if (diffDays === 0) return '今天'
  if (diffDays === 1) return '昨天'
  if (diffDays < 7) return `${diffDays} 天前`
  const parts = dateTimeParts(time)
  return parts ? `${parts.month}/${parts.day}` : '无记录'
}

export function formatDateFull(time: string): string {
  const parts = dateTimeParts(time)
  if (!parts) return '无记录'
  return `${parts.year}/${parts.month}/${parts.day}`
}

export function formatClock(time: string): string {
  const parts = dateTimeParts(time)
  if (!parts) return '--:--'
  return `${parts.hour}:${parts.minute}`
}

export function formatDateTime(time: string): string {
  const parts = dateTimeParts(time)
  if (!parts) return '无记录'
  return `${parts.year}/${parts.month}/${parts.day} ${parts.hour}:${parts.minute}:${parts.second} UTC+8`
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
