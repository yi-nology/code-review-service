/**
 * 仓库扫描的执行生命周期：启动/预览、轮询保护、耗时计时、取消、
 * 会话缓存恢复。从 RepositoryScan.vue 收敛——视图只保留表单与展示编排，
 * 轮询的超时/连续失败/404/页面可见性等保护策略集中在这一处。
 */
import { computed, ref, type Ref } from 'vue'
import { ElMessage } from 'element-plus'
import { cancelReview, getReview, scanRepository } from '@/api/review'
import { sessionApi, type ReviewSession } from '@/api/session'
import type { ReviewDetail } from '@/types/review'
import { normalizePreviewOutput, splitLines } from '@/utils/scan-preview'
import { clearScanSessionCache, readScanSessionCache, saveScanSessionCache } from '@/composables/useScanCache'

export type ScanState = 'idle' | 'queued' | 'running' | 'success' | 'failed'

// 轮询保护：超过 MAX_POLL_DURATION_MS 仍 running 则放弃（防止卡死的 review 导致页面无限轮询）
const MAX_POLL_DURATION_MS = 5 * 60 * 1000 // 5 分钟
// 连续轮询失败（非 404）超过此次数也放弃，避免服务异常时无限重试。
const MAX_POLL_FAILURES = 10

export interface ScanFormLike {
  provider: string
  repo: string
  head_sha: string
  background: string
  max_tokens_budget: number
  batch: '' | 'none' | 'by-language' | 'by-directory'
  no_plan: boolean
  no_dedup: boolean
  no_summary: boolean
  timeout_minutes: number
  [key: string]: unknown
}

export interface UseScanRunDeps {
  /** 表单是否具备发起条件（provider+repo+head_sha）。 */
  canRun: Ref<boolean> | import('vue').ComputedRef<boolean>
  form: ScanFormLike
  pathsText: Ref<string>
  excludeText: Ref<string>
  /** 预览文件列表（预览结果写入；reset 时清空）。 */
  previewFiles: Ref<string[]>
  /** 发起扫描前持久化表单草稿。 */
  saveFormState: () => void
}

export function useScanRun(deps: UseScanRunDeps) {
  const { canRun, form, pathsText, excludeText, previewFiles, saveFormState } = deps

  const scanState = ref<ScanState>('idle')
  const activeReviewId = ref('')
  const reviewDetail = ref<ReviewDetail | null>(null)
  const sessions = ref<ReviewSession[]>([])
  const scanError = ref('')
  const scanWarning = ref('')
  const scanStartTime = ref<number | null>(null)
  const elapsedText = ref('')
  const confirmVisible = ref(false)
  const starting = ref(false)
  const cancelling = ref(false)
  const previewing = ref(false)

  const scanRunning = computed(() => scanState.value === 'queued' || scanState.value === 'running')

  let pollTimer: ReturnType<typeof setTimeout> | null = null
  let elapsedTimer: ReturnType<typeof setInterval> | null = null
  let pollStartTime = 0
  let pollFailureCount = 0

  function saveScanSession() {
    if (activeReviewId.value && scanRunning.value) {
      saveScanSessionCache({ review_id: activeReviewId.value, start_time: scanStartTime.value })
    } else {
      clearScanSessionCache()
    }
  }

  function clearScanSession() {
    clearScanSessionCache()
  }

  async function runScan(preview: boolean) {
    if (!canRun.value) return
    if (!preview) confirmVisible.value = false

    if (preview) {
      previewing.value = true
      try {
        const data = await scanRepository({
          ...form,
          paths: splitLines(pathsText.value),
          exclude: splitLines(excludeText.value),
          preview: true,
        })
        previewFiles.value = data.preview_files || []
        const info = normalizePreviewOutput(previewFiles.value)
        ElMessage.success(`预览完成：将审查 ${info.reviewable.length} 个文件`)
      } catch (e: any) {
        ElMessage.error(e.message || '预览失败')
      } finally {
        previewing.value = false
      }
      return
    }

    starting.value = true
    saveFormState()
    try {
      const data = await scanRepository({
        ...form,
        paths: splitLines(pathsText.value),
        exclude: splitLines(excludeText.value),
        preview: false,
      })
      if (data.review_id) {
        activeReviewId.value = data.review_id
        scanState.value = 'queued'
        scanWarning.value = ''
        pollFailureCount = 0
        scanStartTime.value = Date.now()
        saveScanSession()
        startPolling()
        startElapsedTimer()
      }
    } catch (e: any) {
      ElMessage.error(e.message || '启动扫描失败')
      scanState.value = 'failed'
      scanError.value = e.message || '启动扫描失败'
    } finally {
      starting.value = false
    }
  }

  function startPolling() {
    if (pollTimer) clearTimeout(pollTimer)
    if (!pollStartTime) pollStartTime = Date.now()
    pollTimer = setTimeout(async () => {
      await pollReview()
      if (scanState.value === 'queued' || scanState.value === 'running') {
        if (document.visibilityState === 'visible') startPolling()
        else pollTimer = null
      }
    }, 3000)
  }

  async function fetchSessions() {
    if (!activeReviewId.value) return
    try {
      const response = await sessionApi.getSessions(activeReviewId.value)
      sessions.value = response.sessions || []
    } catch {
      sessions.value = []
    }
  }

  async function pollReview() {
    if (!activeReviewId.value) return
    try {
      const detail = await getReview(activeReviewId.value)
      pollFailureCount = 0
      scanWarning.value = ''
      reviewDetail.value = detail
      if (detail.status === 'success' || detail.status === 'completed') {
        scanState.value = 'success'
        stopPolling()
        stopElapsedTimer()
        clearScanSession()
        ElMessage.success('扫描完成')
        try {
          const final = await getReview(activeReviewId.value)
          reviewDetail.value = final
        } catch {
          // keep the previous detail
        }
        await fetchSessions()
      } else if (detail.status === 'failed') {
        scanState.value = 'failed'
        scanError.value = detail.error || '扫描失败'
        stopPolling()
        stopElapsedTimer()
        clearScanSession()
      } else if (detail.status === 'running') {
        // 轮询超时保护：超过 MAX_POLL_DURATION_MS 仍 running 则放弃，避免页面卡死。
        if (pollStartTime && Date.now() - pollStartTime > MAX_POLL_DURATION_MS) {
          scanState.value = 'failed'
          scanError.value = '扫描超时：任务运行时间过长，可能已停止响应。请重新发起扫描。'
          stopPolling()
          stopElapsedTimer()
          clearScanSession()
        } else {
          scanState.value = 'running'
        }
      }
      await fetchSessions()
    } catch (e: any) {
      // 区分"review 不存在"（404）与"网络抖动"：
      // - 404 说明 review 已被删除或 session 数据损坏，立即放弃，避免无限重试导致页面卡死在"扫描中"。
      // - 其它错误（超时、5xx、网络断开）按可重试处理，连续失败超过阈值后也放弃。
      const status = e?.response?.status || e?.statusCode
      const isNotFound = status === 404 || /not found|404/i.test(e?.message || '')
      if (isNotFound) {
        scanState.value = 'failed'
        scanError.value = '扫描任务不存在或已被删除。可能是之前的扫描记录已清理，请重新发起扫描。'
        stopPolling()
        stopElapsedTimer()
        clearScanSession()
        return
      }
      pollFailureCount += 1
      if (pollFailureCount >= MAX_POLL_FAILURES) {
        scanState.value = 'failed'
        scanError.value = '连续多次无法获取扫描进度，可能服务异常。请稍后重试。'
        stopPolling()
        stopElapsedTimer()
        clearScanSession()
        return
      }
      if (pollFailureCount >= 2) {
        scanWarning.value = e?.message
          ? `服务连接异常，正在重试：${e.message}`
          : '服务连接异常，正在重试。扫描任务可能仍在后端执行。'
      }
    }
  }

  function stopPolling() {
    if (pollTimer) { clearTimeout(pollTimer); pollTimer = null }
    pollStartTime = 0
  }

  function startElapsedTimer() {
    if (elapsedTimer) clearInterval(elapsedTimer)
    elapsedTimer = setInterval(() => {
      if (!scanStartTime.value) return
      const sec = Math.floor((Date.now() - scanStartTime.value) / 1000)
      if (sec < 60) elapsedText.value = `${sec}s`
      else elapsedText.value = `${Math.floor(sec / 60)}m ${sec % 60}s`
    }, 1000)
  }

  function stopElapsedTimer() {
    if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null }
  }

  async function cancelScan() {
    if (!activeReviewId.value) return
    cancelling.value = true
    try {
      await cancelReview(activeReviewId.value)
      scanState.value = 'failed'
      scanError.value = '扫描已取消'
      scanWarning.value = ''
      pollFailureCount = 0
      stopPolling()
      stopElapsedTimer()
      clearScanSession()
    } catch (e: any) {
      ElMessage.error(e.message || '取消失败')
    } finally {
      cancelling.value = false
    }
  }

  function resetScan() {
    scanState.value = 'idle'
    activeReviewId.value = ''
    reviewDetail.value = null
    sessions.value = []
    scanError.value = ''
    scanWarning.value = ''
    pollFailureCount = 0
    scanStartTime.value = null
    elapsedText.value = ''
    previewFiles.value = []
    clearScanSession()
  }

  async function resumeScanSession() {
    try {
      const session = readScanSessionCache()
      if (!session || !session.review_id) return
      // 恢复前先校验 review 确实存在且仍在运行/排队，避免恢复一个已失效的 session
      // 后陷入 404 无限重试。校验失败直接清掉残留 session。
      const detail = await getReview(session.review_id)
      if (!['queued', 'running'].includes(detail.status)) {
        clearScanSession()
        return
      }
      activeReviewId.value = session.review_id
      scanStartTime.value = session.start_time ?? null
      scanState.value = detail.status as ScanState
      reviewDetail.value = detail
      pollStartTime = Date.now()
      pollFailureCount = 0
      startPolling()
      startElapsedTimer()
      await fetchSessions()
    } catch {
      // review 不存在或查询失败：清掉残留 session，不进入卡死的轮询。
      clearScanSession()
    }
  }

  // 页面重新可见时恢复轮询（后台标签页期间轮询已暂停）。
  function onVisibilityChange() {
    if (document.visibilityState === 'visible' && scanRunning.value && !pollTimer) {
      startPolling()
    }
  }

  // 卸载时停掉全部定时器（视图 onBeforeUnmount 调用）。
  function dispose() {
    stopPolling()
    stopElapsedTimer()
  }

  return {
    scanState, activeReviewId, reviewDetail, sessions,
    scanError, scanWarning, scanStartTime, elapsedText,
    confirmVisible, starting, cancelling, previewing, scanRunning,
    runScan, cancelScan, resetScan, resumeScanSession, fetchSessions,
    onVisibilityChange, dispose,
  }
}
