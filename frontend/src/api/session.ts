import request from './request'
import type {
  ReviewSession,
  AgentResult,
  Discussion,
  FinalReport,
  ReviewProgress,
  ProgressEvent,
  SessionListResult,
  SessionListOptions,
} from '@/types/session'

export type {
  ReviewSession,
  AgentResult,
  Finding,
  Warning,
  Discussion,
  DiscussionEntry,
  FinalReport,
  Requirements,
  RequirementsCoverage,
  RequirementCoverage,
  SessionMetadata,
  ProgressEvent,
  AgentStatus,
  ReviewProgress,
  PhaseInfo,
  SessionListResult,
  SessionListOptions,
} from '@/types/session'

/**
 * 全局会话列表（跨 review 的所有审查会话）。
 * 独立命名导出，供 Sessions.vue 列表页使用；sessionApi.listSessions 仍保留向后兼容。
 */
export async function listAllSessions(params: SessionListOptions = {}): Promise<SessionListResult> {
  return request.get('/sessions', { params }) as any
}

// Session API
export const sessionApi = {
  getSessions(reviewId: string): Promise<{ sessions: ReviewSession[] }> {
    return request.get(`/reviews/${reviewId}/sessions`)
  },

  getActiveSession(reviewId: string): Promise<ReviewSession> {
    return request.get(`/reviews/${reviewId}/sessions/active`)
  },

  getSession(reviewId: string, sessionId: string): Promise<ReviewSession> {
    return request.get(`/reviews/${reviewId}/sessions/${sessionId}`)
  },

  createSession(reviewId: string): Promise<ReviewSession> {
    return request.post(`/reviews/${reviewId}/sessions`)
  },

  updatePhase(reviewId: string, sessionId: string, phase: string, progress: number): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/phase`, { phase, progress })
  },

  updateStatus(reviewId: string, sessionId: string, status: string): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/status`, { status })
  },

  updateAgentResults(reviewId: string, sessionId: string, agentResults: AgentResult[]): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/agent-results`, agentResults)
  },

  updateDiscussion(reviewId: string, sessionId: string, discussion: Discussion): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/discussion`, discussion)
  },

  updateFinalReport(reviewId: string, sessionId: string, finalReport: FinalReport): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/final-report`, finalReport)
  },

  completeSession(reviewId: string, sessionId: string, finalReport: FinalReport): Promise<void> {
    return request.post(`/reviews/${reviewId}/sessions/${sessionId}/complete`, finalReport)
  },

  failSession(reviewId: string, sessionId: string, reason: string): Promise<void> {
    return request.post(`/reviews/${reviewId}/sessions/${sessionId}/fail`, { reason })
  },

  cancelSession(reviewId: string, sessionId: string): Promise<void> {
    return request.post(`/reviews/${reviewId}/sessions/${sessionId}/cancel`)
  },

  listSessions(page: number = 1, pageSize: number = 20): Promise<{
    sessions: ReviewSession[]
    total: number
    page: number
    page_size: number
    total_pages: number
  }> {
    return request.get('/sessions', { params: { page, page_size: pageSize } })
  },
}

// Progress API
export const progressApi = {
  async getProgress(reviewId: string): Promise<ReviewProgress | null> {
    try {
      const response: any = await request.get(`/reviews/${reviewId}/progress/status`)
      return response?.progress || response || null
    } catch (error: any) {
      // 404 表示没有活跃的 review，返回 null 而不是抛出错误
      if (error?.response?.status === 404 || error?.message?.includes('no active review')) {
        return null
      }
      throw error
    }
  },

  getActiveReviews(): Promise<{
    reviews: Array<{
      review_id: string
      session_id: string
      round: number
      phase: string
      status: string
      progress: number
      start_time: string
      updated_at: string
    }>
    total: number
  }> {
    return request.get('/progress/active')
  },

  /**
   * 连接 review 实时进度 WebSocket。
   *
   * 返回一个控制对象，调用方可通过 close() 主动断开（不会触发 onClose 回调，
   * 避免断开后误重连）。onMessage 收到的是已 JSON.parse 的事件对象。
   */
  connectProgress(
    reviewId: string,
    onMessage: (event: ProgressEvent) => void,
    handlers?: { onClose?: () => void; onError?: () => void },
  ): { close: () => void } {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/v1/reviews/${reviewId}/progress`

    let closed = false
    const ws = new WebSocket(wsUrl)

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        onMessage(data)
      } catch {
        // 忽略无法解析的消息
      }
    }

    ws.onerror = () => {
      handlers?.onError?.()
    }

    ws.onclose = () => {
      if (!closed) handlers?.onClose?.()
    }

    return {
      close() {
        closed = true
        ws.onclose = null
        ws.onmessage = null
        ws.onerror = null
        if (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING) {
          ws.close()
        }
      },
    }
  },
}
