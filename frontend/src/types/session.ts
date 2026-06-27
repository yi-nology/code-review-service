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

export interface ReviewHistoryEvent {
  stage: string
  status: string
  message: string
  created_at: string
}
