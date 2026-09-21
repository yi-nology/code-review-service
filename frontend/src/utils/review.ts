import type { Review } from '@/types/review'
import { DEAD_LETTER_THRESHOLD } from '@/constants/review'

// 审查列表的行级谓词与文案：桌面表格、移动卡片、批量操作共用单一实现，
// 避免"哪些行能取消/重审"在不同入口各写一遍导致行为分叉。

/** 可取消：排队中/审查中。 */
export function isCancellable(row: Pick<Review, 'status'>): boolean {
  return row.status === 'queued' || row.status === 'running'
}

/** 可重审：失败/成功。 */
export function isRerunnable(row: Pick<Review, 'status'>): boolean {
  return row.status === 'failed' || row.status === 'success'
}

/**
 * 死信：重试次数耗尽的 failed 评审。后端校验条件，非死信重排队返回 404；
 * 前端按 retry_count >= 阈值精确判断是否显示重新入队入口。
 */
export function isDeadLetter(row: Pick<Review, 'status' | 'retry_count'>): boolean {
  return row.status === 'failed' && (row.retry_count ?? 0) >= DEAD_LETTER_THRESHOLD
}

export function gateLabel(row: Review): string {
  if (row.gate?.status === 'pending') return '待评估'
  return row.gate?.passed ? '已通过' : '未通过'
}

export function gateClass(row: Review): string {
  if (row.gate?.status === 'pending') return 'pending'
  return row.gate?.passed ? 'passed' : 'failed'
}
