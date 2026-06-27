import request from './request'
import type {
  ReviewSession,
  AgentResult,
  Discussion,
  FinalReport,
  ReviewProgress,
  ProgressEvent,
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
} from '@/types/session'

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
  getProgress(reviewId: string): Promise<ReviewProgress> {
    return request.get(`/reviews/${reviewId}/progress/status`).then((response: any) => response.progress || response)
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

  connectProgress(reviewId: string, onMessage: (event: ProgressEvent) => void): WebSocket {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/api/v1/reviews/${reviewId}/progress`
    
    const ws = new WebSocket(wsUrl)
    
    ws.onmessage = (event) => {
      const data = JSON.parse(event.data)
      onMessage(data)
    }
    
    ws.onerror = (error) => {
      console.error('WebSocket error:', error)
    }
    
    return ws
  },
}
