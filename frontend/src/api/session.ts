import request from './request'

// Session types
export interface ReviewSession {
  id: number
  session_id: string
  review_id: string
  round: number
  phase: string
  status: string
  progress: number
  agent_results: AgentResult[] | null
  discussion: Discussion | null
  final_report: FinalReport | null
  requirements: Requirements | null
  metadata: SessionMetadata | null
  created_at: string
  updated_at: string
  completed_at: string | null
}

export interface AgentResult {
  agent_name: string
  findings: Finding[]
  summary: string
  confidence: number
  duration_ms: number
  tokens_used: number
  warnings: Warning[]
}

export interface Finding {
  id: string
  severity: string
  category: string
  path: string
  line: number
  end_line: number
  title: string
  body: string
  suggestion: string
  confidence: number
  agents: string[]
}

export interface Warning {
  type: string
  message: string
  path: string
}

export interface Discussion {
  id: string
  round: number
  entries: DiscussionEntry[]
  summary: string
  created_at: string
}

export interface DiscussionEntry {
  id: string
  agent_name: string
  finding_id: string
  response: 'AGREE' | 'CHALLENGE' | 'CONNECT' | 'SURFACE'
  content: string
  evidence: string
  confidence: number
  references: string[]
  created_at: string
}

export interface FinalReport {
  verdict: string
  summary: string
  blockers: Finding[]
  should_fix: Finding[]
  suggestions: Finding[]
  requirements: RequirementsCoverage | null
  clarifying_questions: string[]
  score: number
  gate_passed: boolean
  gate_reason: string
}

export interface Requirements {
  inline: string[]
  files: string[]
  urls: string[]
  context: Record<string, string>
}

export interface RequirementsCoverage {
  requirements: RequirementCoverage[]
  summary: string
  score: number
}

export interface RequirementCoverage {
  requirement: string
  status: 'met' | 'partial' | 'missing' | 'unclear'
  confidence: number
  evidence: string
  findings: string[]
}

export interface SessionMetadata {
  branch: string
  base_sha: string
  head_sha: string
  files_changed: number
  diff_size: number
  started_at: string
  completed_at: string | null
  duration_ms: number
}

// Progress types
export interface ProgressEvent {
  type: 'phase_start' | 'phase_progress' | 'phase_complete' | 'agent_status' | 'error' | 'complete'
  phase: string
  status: string
  progress: number
  message: string
  agent_status: Record<string, AgentStatus> | null
  error: string | null
  timestamp: string
}

export interface AgentStatus {
  name: string
  status: string
  progress: number
  message: string
  duration_ms: number | null
  error: string | null
}

export interface ReviewProgress {
  review_id: string
  session_id: string
  round: number
  phase: string
  status: string
  progress: number
  phases: PhaseInfo[]
  agent_status: Record<string, AgentStatus> | null
  start_time: string
  updated_at: string
  error: string | null
}

export interface PhaseInfo {
  name: string
  title: string
  description: string
  status: string
  progress: number
  start_time: string | null
  end_time: string | null
  duration_ms: number | null
}

// Review Map types
export interface ReviewMap {
  id: string
  session_id: string
  title: string
  description: string
  sections: Section[]
  dependencies: Dependency[]
  summary: string
  file_count: number
  section_count: number
  created_at: string
}

export interface Section {
  id: string
  title: string
  description: string
  files: File[]
  findings: string[]
  progress: number
  priority: string
}

export interface File {
  path: string
  language: string
  lines_added: number
  lines_removed: number
  findings: string[]
  reviewed: boolean
}

export interface Dependency {
  from: string
  to: string
  type: string
  strength: string
}

// Persona types
export interface ReviewerPersona {
  id: string
  name: string
  display_name: string
  description: string
  focus_areas: string[]
  prompt_template: string
  category: string
  tags: string[]
  is_active: boolean
  created_at: string
  updated_at: string
}

// Session API
export const sessionApi = {
  // Get sessions for a review
  getSessions(reviewId: string): Promise<{ sessions: ReviewSession[] }> {
    return request.get(`/reviews/${reviewId}/sessions`)
  },

  // Get active session for a review
  getActiveSession(reviewId: string): Promise<ReviewSession> {
    return request.get(`/reviews/${reviewId}/sessions/active`)
  },

  // Get a specific session
  getSession(reviewId: string, sessionId: string): Promise<ReviewSession> {
    return request.get(`/reviews/${reviewId}/sessions/${sessionId}`)
  },

  // Create a new session
  createSession(reviewId: string): Promise<ReviewSession> {
    return request.post(`/reviews/${reviewId}/sessions`)
  },

  // Update session phase
  updatePhase(reviewId: string, sessionId: string, phase: string, progress: number): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/phase`, { phase, progress })
  },

  // Update session status
  updateStatus(reviewId: string, sessionId: string, status: string): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/status`, { status })
  },

  // Update agent results
  updateAgentResults(reviewId: string, sessionId: string, agentResults: AgentResult[]): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/agent-results`, agentResults)
  },

  // Update discussion
  updateDiscussion(reviewId: string, sessionId: string, discussion: Discussion): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/discussion`, discussion)
  },

  // Update final report
  updateFinalReport(reviewId: string, sessionId: string, finalReport: FinalReport): Promise<void> {
    return request.put(`/reviews/${reviewId}/sessions/${sessionId}/final-report`, finalReport)
  },

  // Complete session
  completeSession(reviewId: string, sessionId: string, finalReport: FinalReport): Promise<void> {
    return request.post(`/reviews/${reviewId}/sessions/${sessionId}/complete`, finalReport)
  },

  // Fail session
  failSession(reviewId: string, sessionId: string, reason: string): Promise<void> {
    return request.post(`/reviews/${reviewId}/sessions/${sessionId}/fail`, { reason })
  },

  // Cancel session
  cancelSession(reviewId: string, sessionId: string): Promise<void> {
    return request.post(`/reviews/${reviewId}/sessions/${sessionId}/cancel`)
  },

  // List all sessions
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
  // Get progress for a review
  getProgress(reviewId: string): Promise<ReviewProgress> {
    return request.get(`/reviews/${reviewId}/progress/status`).then((response: any) => response.progress || response)
  },

  // Get active reviews
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

  // Connect to progress WebSocket
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

// Review Map API
export const reviewMapApi = {
  // Get maps for a review
  getMaps(reviewId: string): Promise<{ maps: ReviewMap[] }> {
    return request.get(`/reviews/${reviewId}/maps`)
  },

  // Get a specific map
  getMap(reviewId: string, mapId: string): Promise<ReviewMap> {
    return request.get(`/reviews/${reviewId}/maps/${mapId}`)
  },

  // Generate a map
  generateMap(reviewId: string): Promise<ReviewMap> {
    return request.post(`/reviews/${reviewId}/maps`)
  },

  // Update file reviewed status
  updateFileReviewed(reviewId: string, mapId: string, filePath: string, reviewed: boolean): Promise<void> {
    return request.put(`/reviews/${reviewId}/maps/${mapId}/files/${encodeURIComponent(filePath)}/reviewed`, { reviewed })
  },

  // Update all files in a section
  updateSectionProgress(reviewId: string, mapId: string, sectionId: string, reviewed: boolean): Promise<void> {
    return request.put(`/reviews/${reviewId}/maps/${mapId}/sections/${sectionId}/progress`, { reviewed })
  },
}

// Persona API
export const personaApi = {
  // list personas
  listPersonas(): Promise<{ personas: ReviewerPersona[] }> {
    return request.get('/personas')
  },

  // Get a specific persona
  getPersona(id: string): Promise<ReviewerPersona> {
    return request.get(`/personas/${id}`)
  },

  // Create a persona
  createPersona(persona: Partial<ReviewerPersona>): Promise<ReviewerPersona> {
    return request.post('/personas', persona)
  },

  // Update a persona
  updatePersona(id: string, persona: Partial<ReviewerPersona>): Promise<ReviewerPersona> {
    return request.put(`/personas/${id}`, persona)
  },

  // Delete a persona
  deletePersona(id: string): Promise<void> {
    return request.delete(`/personas/${id}`)
  },

  // Search personas
  searchPersonas(query: string): Promise<{ personas: ReviewerPersona[] }> {
    return request.get('/personas/search', { params: { q: query } })
  },
}
