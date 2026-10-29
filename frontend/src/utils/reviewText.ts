/**
 * 后端写入的**机器占位摘要**与**机器占位门禁原因** → 控制台中文文案。
 *
 * 为什么在展示层翻译而不是改后端：summary 同时是 API 字段（MCP 工具输出
 * 给 agent 用），保持机器可读；界面文案是展示层的事。门禁原因（gate reason）
 * 只面向人，后端已直接写中文，这里的英文键是**升级前遗留行**的兼容。
 *
 * 键的集合必须与后端的产生点对齐，漏一个就有一条路径把英文原样漏到界面：
 *   - summary：internal/eino/persistence.go 的 `"review " + status`、
 *     internal/service/review/{create,requeue,scan_orchestrator}.go 的固定串；
 *   - gate：internal/service/review/{failure,requeue,scan_orchestrator,
 *     argus_executor}.go 的门禁原因。
 * 补后端产生点时，**两处都要改**（后端写新串 + 这里加旧串的键）。
 */
const MACHINE_SUMMARIES: Record<string, string> = {
  'review queued': '评审已排队，等待审查引擎处理。',
  'review queued (webhook)': '评审已排队（由 Webhook 触发），等待审查引擎处理。',
  'review running': '评审进行中，审查引擎正在分析本次变更。',
  'review success': '评审已完成。',
  'review completed': '评审已完成。',
  'review failed': '评审执行失败，请查看失败提示与执行步骤。',
  'review cancelled': '评审已取消。',
  'review re-queued': '评审已重新入队，等待审查引擎处理。',
  'review cancelled by user': '评审已由用户取消。',
  'repository scan queued': '代码库扫描已排队，等待扫描引擎处理。',
  'repository scan running': '代码库扫描进行中，正在分析整个仓库。',
  'repository scan failed': '代码库扫描失败，请查看失败提示与执行步骤。',
}

/** 门禁失败原因里的机器占位串（含升级前遗留的英文键）。 */
const MACHINE_GATE_REASONS: Record<string, string> = {
  'review execution failed': '评审流程未完整完成，请查看失败提示与执行步骤。',
  'review failed': '评审流程未完整完成，请查看失败提示与执行步骤。',
  'review cancelled': '评审已取消。',
  'scan execution failed': '代码库扫描执行失败，请查看失败提示与执行步骤。',
  'argus: approve': 'argus 裁决：通过',
  'argus: comment': 'argus 裁决：评论',
  'argus: request_changes': 'argus 裁决：要求修改',
  'argus: error': 'argus 裁决：审查失败',
}

/** 把机器占位摘要换成中文人话；其余摘要（OCR 产出的真实摘要）原样返回。 */
export function formatReviewSummary(summary?: string): string {
  if (!summary) return ''
  return MACHINE_SUMMARIES[summary.trim()] || summary
}

/** 把机器占位的门禁原因换成中文人话；其余原因（真实错误信息）原样返回。 */
export function formatGateReason(reason?: string): string {
  if (!reason) return ''
  return MACHINE_GATE_REASONS[reason.trim()] || reason
}
