/**
 * 项目策略表单的纯业务规则（从 ProjectSettings.vue 收敛）：
 * 归一化、payload 组装、校验。全部为纯函数，便于单元测试与复用。
 */

/** 去空、去首尾空白、去重的行列表（忽略路径输入框）。 */
export function uniqueLines(text: string): string[] {
  const values = text
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean)
  return Array.from(new Set(values))
}

/** 规则适用路径归一化：空路径视为通配。 */
export function normalizePath(path: string): string {
  const value = path.trim()
  return value || '**/*'
}

/** 项目策略唯一键 provider:repo。 */
export function policyKey(policy: { provider: string; repo: string }): string {
  return `${policy.provider}:${policy.repo}`
}

export interface PolicyRuleDraft {
  path: string
  rule: string
}

/** 规则草稿归一化：补路径默认值、去空白、丢弃全空行。 */
export function normalizeRules(rules: PolicyRuleDraft[]): PolicyRuleDraft[] {
  return rules
    .map((rule) => ({
      path: normalizePath(rule.path),
      rule: rule.rule.trim(),
    }))
    .filter((rule) => rule.path || rule.rule)
}

/** 校验策略表单草稿，返回人可读的问题列表（空数组 = 通过）。 */
export function validatePolicyDraft(input: {
  provider: string
  repo: string
  fail_on: string
  ocr_default_severity?: string
  failOnOptions: { value: string }[]
  ignorePaths: string[]
  rules: PolicyRuleDraft[]
}): string[] {
  const issues: string[] = []
  if (!input.provider.trim()) issues.push('请选择代码平台')
  if (!input.repo.trim()) issues.push('请选择或输入仓库')
  if (!input.failOnOptions.some((option) => option.value === input.fail_on)) issues.push('门禁级别无效')
  if (input.ocr_default_severity && !input.failOnOptions.some((option) => option.value === input.ocr_default_severity)) {
    issues.push('OCR 默认严重级别无效')
  }

  const seenIgnorePaths = new Set<string>()
  input.ignorePaths.forEach((path) => {
    if (seenIgnorePaths.has(path)) issues.push(`忽略路径重复：${path}`)
    seenIgnorePaths.add(path)
  })

  const seenRules = new Set<string>()
  input.rules.forEach((rule, index) => {
    const path = normalizePath(rule.path)
    const text = rule.rule.trim()
    if (!path && !text) return
    if (!path) issues.push(`第 ${index + 1} 条项目规则缺少适用文件`)
    if (!text) issues.push(`第 ${index + 1} 条项目规则缺少审查要求`)
    const key = `${path}::${text}`
    if (path && text) {
      if (seenRules.has(key)) issues.push(`第 ${index + 1} 条项目规则重复`)
      seenRules.add(key)
    }
  })
  return issues
}
