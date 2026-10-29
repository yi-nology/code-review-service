const DISPLAY_TIME_ZONE = 'Asia/Shanghai'

function parseDate(time: string | Date): Date | null {
  if (!time) return null
  const date = time instanceof Date ? time : new Date(time)
  if (Number.isNaN(date.getTime())) return null
  return date
}

function dateTimeParts(time: string | Date): Record<string, string> | null {
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

/**
 * 标识符缩略：超长时保留**尾部**并加前导省略号。
 *
 * 取尾而不是取头：审查 ID 是 ULID，前 8 位是时间戳（同一时间创建的记录
 * 头部几乎一样），尾部才是区分度所在；取头会让一列记录看起来完全相同。
 */
export function shortId(id: string): string {
  if (!id) return ''
  return id.length > 12 ? `…${id.slice(-6)}` : id
}

// ===== 以下为从各视图/组件收敛来的公共格式化（单一真源，避免本地复制） =====

/** 'HH:mm:ss'（显示时区），供时间线/讨论流等高频时间戳使用。 */
export function formatClockSeconds(time: string | Date): string {
  const parts = dateTimeParts(time)
  if (!parts) return '--:--:--'
  return `${parts.hour}:${parts.minute}:${parts.second}`
}

/** 相对时间：刚刚 / N 分钟前 / N 小时前 / 本地日期（通知中心）。 */
export function formatRelativeTime(iso: string): string {
  const d = parseDate(iso)
  if (!d) return '-'
  const diff = Date.now() - d.getTime()
  if (diff < 60000) return '刚刚'
  if (diff < 3600000) return `${Math.floor(diff / 60000)} 分钟前`
  if (diff < 86400000) return `${Math.floor(diff / 3600000)} 小时前`
  return d.toLocaleDateString('zh-CN')
}

/** token 数缩写：1.2M / 3.4k / 原值；空值出 '—'。 */
export function formatTokens(n?: number): string {
  if (n === undefined || n === null) return '—'
  if (n >= 1_000_000) return trimZero(n / 1_000_000) + 'M'
  if (n >= 1000) return trimZero(n / 1000) + 'k'
  return String(n)
}

/** 去掉小数尾部的 .0：776.0k → 776k。 */
function trimZero(v: number): string {
  const s = v.toFixed(1)
  return s.endsWith('.0') ? s.slice(0, -2) : s
}

/** 千分位数字；空值出 fallback。 */
export function formatNumber(value?: number, fallback = '0'): string {
  return value ? value.toLocaleString() : fallback
}

/** 时长：ms / s / min 自适应。 */
export function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`
  return `${(ms / 60000).toFixed(1)}min`
}
