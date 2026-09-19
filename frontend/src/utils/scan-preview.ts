/** 仓库扫描 preview 输出解析（从 RepositoryScan.vue 收敛的纯函数，单一真源）。 */

/** 去掉 OCR CLI 输出中的 ANSI 转义序列。 */
export function stripAnsi(value: string): string {
  const escapeChar = String.fromCharCode(27)
  return value.replace(new RegExp(`${escapeChar}\\[[0-9;?]*[ -/]*[@-~]`, 'g'), '')
}

/** 按行切分并去空（扫描路径输入框等）。 */
export function splitLines(value: string): string[] {
  return value.split('\n').map((l) => l.trim()).filter(Boolean)
}

/** 解析 preview 输出的单行文件条目：去 ANSI、去段落标记、去排除原因与行数后缀。 */
export function parsePreviewLine(line: string): string {
  let value = stripAnsi(line).trim()
  if (!value) return ''
  if (value.startsWith('[')) {
    const end = value.indexOf(']')
    if (end >= 0) value = value.slice(end + 1).trim()
  }
  value = value.replace(/\s+\((unsupported_ext|excluded|ignored|binary|too_large|generated)[^)]+\)\s*$/i, '').trim()
  value = value.replace(/\s+[+-]\d+.*$/, '').trim()
  return value
}

export interface PreviewSummary {
  totalChanged: number
  reviewable: string[]
  excluded: string[]
}

/** 把 `ocr scan --preview` 的完整输出解析为 可审文件/排除文件/变更总数。 */
export function normalizePreviewOutput(values: string[]): PreviewSummary {
  const reviewable: string[] = []
  const excluded: string[] = []
  let totalChanged = 0
  let section: 'reviewable' | 'excluded' | '' = ''

  for (const raw of values) {
    const line = stripAnsi(raw).trim()
    if (!line) continue
    const summary = line.match(/^Preview:\s*(\d+)\s+file/i)
    if (summary) {
      totalChanged = Number(summary[1]) || totalChanged
      continue
    }
    if (/^Will review/i.test(line)) {
      section = 'reviewable'
      continue
    }
    if (/^Excluded from review/i.test(line)) {
      section = 'excluded'
      continue
    }
    const file = parsePreviewLine(line)
    if (!file) continue
    if (section === 'excluded' || /\((unsupported_ext|excluded|ignored|binary|too_large|generated)/i.test(line)) {
      excluded.push(file)
    } else if (section === 'reviewable' || !/^Preview:/i.test(file)) {
      reviewable.push(file)
    }
  }

  return {
    totalChanged,
    reviewable: Array.from(new Set(reviewable)),
    excluded: Array.from(new Set(excluded)),
  }
}
