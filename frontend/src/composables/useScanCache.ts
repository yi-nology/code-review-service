/**
 * 扫描页的 localStorage 持久化（表单草稿 + 进行中扫描会话）。
 * 从 RepositoryScan.vue 收敛：序列化/容错集中在这里，视图只做状态绑定。
 */

export interface ScanFormState {
  provider: string
  repo: string
  branch: string
  head_sha: string
  pathsText: string
  excludeText: string
  background: string
  max_tokens_budget: number
  batch: string
  no_plan: boolean
  no_dedup: boolean
  no_summary: boolean
  timeout_minutes: number
}

const STORAGE_KEY = 'scan-form-state'
const SCAN_SESSION_KEY = 'scan-session'

export function saveScanFormState(s: ScanFormState): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(s))
  } catch { /* ignore */ }
}

/** 读取表单草稿；无草稿返回 null（调用方据此决定是否提示「已恢复」）。 */
export function clearScanFormState(): void {
  try { localStorage.removeItem(STORAGE_KEY) } catch { /* ignore */ }
}

export function loadScanFormState(): ScanFormState | null {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return null
    return JSON.parse(raw) as ScanFormState
  } catch { return null }
}

export interface ScanSessionCache {
  review_id: string
  start_time: number | null
}

export function saveScanSessionCache(c: ScanSessionCache): void {
  try {
    localStorage.setItem(SCAN_SESSION_KEY, JSON.stringify(c))
  } catch { /* ignore */ }
}

export function readScanSessionCache(): ScanSessionCache | null {
  try {
    const raw = localStorage.getItem(SCAN_SESSION_KEY)
    if (!raw) return null
    return JSON.parse(raw) as ScanSessionCache
  } catch { return null }
}

export function clearScanSessionCache(): void {
  try { localStorage.removeItem(SCAN_SESSION_KEY) } catch { /* ignore */ }
}
