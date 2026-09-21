import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { ElMessage } from 'element-plus/es/components/message/index'
import { ElMessageBox } from 'element-plus/es/components/message-box/index'
import { cancelReview, rerunReview, requeueReview } from '@/api/review'
import type { Review } from '@/types/review'
import { isCancellable, isRerunnable } from '@/utils/review'
import { shortId } from '@/utils/format'

// 审查列表的行级与批量操作（取消/重审/死信重排队）。
// 单一实现供桌面表格与移动卡片共用；列表刷新以回调注入（操作后后台刷新，
// 不打断滚动位置与轮询）。
export interface ReviewActions {
  actingId: Ref<string>
  selectedRows: Ref<Review[]>
  bulkAction: Ref<boolean>
  cancellableCount: ComputedRef<number>
  rerunnableCount: ComputedRef<number>
  handleCancel: (row: Review) => Promise<void>
  handleRerun: (row: Review) => Promise<void>
  handleRequeue: (row: Review) => Promise<void>
  handleSelectionChange: (rows: Review[]) => void
  handleBulkCancel: () => Promise<void>
  handleBulkRerun: () => Promise<void>
}

export function useReviewActions(refresh: () => void): ReviewActions {
  const actingId = ref('')
  // 批量选择：对运行中/失败/成功的记录支持批量取消 / 批量重新审查。
  const selectedRows = ref<Review[]>([])
  const bulkAction = ref(false)

  const cancellableCount = computed(() => selectedRows.value.filter(isCancellable).length)
  const rerunnableCount = computed(() => selectedRows.value.filter(isRerunnable).length)

  async function confirmBox(message: string, title: string, confirmButtonText: string): Promise<boolean> {
    try {
      await ElMessageBox.confirm(message, title, {
        type: 'warning',
        confirmButtonText,
        cancelButtonText: '再想想',
      })
      return true
    } catch {
      return false
    }
  }

  async function handleCancel(row: Review) {
    if (!(await confirmBox(`确定取消这条审查任务？(${shortId(row.review_id)})`, '取消审查', '取消任务'))) return
    actingId.value = row.review_id
    try {
      await cancelReview(row.review_id)
      ElMessage.success('审查任务已取消')
      refresh()
    } catch {
      ElMessage.error('取消失败，任务可能已完成或状态不允许')
    } finally {
      actingId.value = ''
    }
  }

  async function handleRerun(row: Review) {
    actingId.value = row.review_id
    try {
      await rerunReview(row.review_id)
      ElMessage.success('已重新加入审查队列')
      refresh()
    } catch {
      ElMessage.error('重新审查失败，请稍后重试')
    } finally {
      actingId.value = ''
    }
  }

  async function handleRequeue(row: Review) {
    const ok = await confirmBox(
      `这条审查因重试次数耗尽被标记为死信（重试 ${row.retry_count} 次）。确定重新入队？`,
      '重新入队',
      '重新入队',
    )
    if (!ok) return
    actingId.value = row.review_id
    try {
      await requeueReview(row.review_id)
      ElMessage.success('已从死信队列重新入队')
      refresh()
    } catch {
      ElMessage.error('重新入队失败，该评审可能不满足死信条件')
    } finally {
      actingId.value = ''
    }
  }

  function handleSelectionChange(rows: Review[]) {
    selectedRows.value = rows
  }

  // 批量操作骨架：空选提示 → 确认 → 并发执行 → 汇总结果 + 后台刷新。
  async function bulkRun(
    targets: Review[],
    emptyHint: string,
    title: string,
    confirmText: string,
    confirmMessage: string,
    doneMessage: (ok: number) => string,
    run: (row: Review) => Promise<unknown>,
  ) {
    if (!targets.length) {
      ElMessage.warning(emptyHint)
      return
    }
    if (!(await confirmBox(confirmMessage, title, confirmText))) return
    bulkAction.value = true
    let ok = 0
    await Promise.allSettled(targets.map((r) => run(r).then(() => ok++)))
    bulkAction.value = false
    ElMessage.success(doneMessage(ok))
    refresh()
  }

  async function handleBulkRerun() {
    const targets = selectedRows.value.filter(isRerunnable)
    await bulkRun(
      targets,
      '选中记录中没有可重新审查的（仅失败/成功可重审）',
      '批量重新审查',
      '批量重审',
      `将对选中的 ${targets.length} 条记录重新发起审查，确定继续？`,
      (ok) => `已重新加入审查队列：${ok}/${targets.length}`,
      (r) => rerunReview(r.review_id),
    )
  }

  async function handleBulkCancel() {
    const targets = selectedRows.value.filter(isCancellable)
    await bulkRun(
      targets,
      '选中记录中没有可取消的（仅运行中/排队中可取消）',
      '批量取消',
      '批量取消',
      `将取消选中的 ${targets.length} 条审查任务，确定继续？`,
      (ok) => `已取消审查任务：${ok}/${targets.length}`,
      (r) => cancelReview(r.review_id),
    )
  }

  return {
    actingId,
    selectedRows,
    bulkAction,
    cancellableCount,
    rerunnableCount,
    handleCancel,
    handleRerun,
    handleRequeue,
    handleSelectionChange,
    handleBulkCancel,
    handleBulkRerun,
  }
}
