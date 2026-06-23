struct ReviewSession {
    1: i64 id (api.body="id")
    2: string session_id (api.body="session_id")
    3: string review_id (api.body="review_id")
    4: i64 round (api.body="round")
    5: string phase (api.body="phase")
    6: string status (api.body="status")
    7: double progress (api.body="progress")
    8: optional string agent_results (api.body="agent_results")
    9: optional string discussion (api.body="discussion")
    10: optional string final_report (api.body="final_report")
    11: optional string requirements (api.body="requirements")
    12: optional string metadata (api.body="metadata")
    13: optional string completed_at (api.body="completed_at")
    14: string created_at (api.body="created_at")
    15: string updated_at (api.body="updated_at")
}

struct GetSessionsReq {
    1: string id (api.path="id")
}

struct GetSessionsResp {
    1: list<ReviewSession> sessions (api.body="sessions")
}

struct CreateSessionReq {
    1: string id (api.path="id")
}

struct CreateSessionResp {
    1: ReviewSession session (api.body="session")
}

struct GetActiveSessionReq {
    1: string id (api.path="id")
}

struct GetActiveSessionResp {
    1: ReviewSession session (api.body="session")
}

struct GetSessionReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
}

struct GetSessionResp {
    1: ReviewSession session (api.body="session")
}

struct UpdateSessionPhaseReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string phase (api.body="phase")
    4: optional double progress (api.body="progress")
}

struct UpdateSessionPhaseResp {
    1: string message (api.body="message")
}

struct UpdateSessionStatusReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string status (api.body="status")
}

struct UpdateSessionStatusResp {
    1: string message (api.body="message")
}

struct UpdateSessionAgentResultsReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string agent_results (api.body="agent_results")
}

struct UpdateSessionAgentResultsResp {
    1: string message (api.body="message")
}

struct UpdateSessionDiscussionReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string discussion (api.body="discussion")
}

struct UpdateSessionDiscussionResp {
    1: string message (api.body="message")
}

struct UpdateSessionFinalReportReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string final_report (api.body="final_report")
}

struct UpdateSessionFinalReportResp {
    1: string message (api.body="message")
}

struct CompleteSessionReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string final_report (api.body="final_report")
}

struct CompleteSessionResp {
    1: string message (api.body="message")
}

struct FailSessionReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
    3: string reason (api.body="reason")
}

struct FailSessionResp {
    1: string message (api.body="message")
}

struct CancelSessionReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
}

struct CancelSessionResp {
    1: string message (api.body="message")
}

struct ListSessionsReq {
    1: optional i64 page (api.query="page")
    2: optional i64 page_size (api.query="page_size")
}

struct ListSessionsResp {
    1: list<ReviewSession> sessions (api.body="sessions")
    2: i64 total (api.body="total")
    3: i64 page (api.body="page")
    4: i64 page_size (api.body="page_size")
    5: i64 total_pages (api.body="total_pages")
}

service sessionService {
    ListSessionsResp ListSessions(1: ListSessionsReq req) (api.get="/api/v1/sessions")
    GetSessionsResp GetSessions(1: GetSessionsReq req) (api.get="/api/v1/reviews/:id/sessions")
    CreateSessionResp CreateSession(1: CreateSessionReq req) (api.post="/api/v1/reviews/:id/sessions")
    GetActiveSessionResp GetActiveSession(1: GetActiveSessionReq req) (api.get="/api/v1/reviews/:id/sessions/active")
    GetSessionResp GetSession(1: GetSessionReq req) (api.get="/api/v1/reviews/:id/sessions/:sessionId")
    UpdateSessionPhaseResp UpdateSessionPhase(1: UpdateSessionPhaseReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/phase")
    UpdateSessionStatusResp UpdateSessionStatus(1: UpdateSessionStatusReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/status")
    UpdateSessionAgentResultsResp UpdateSessionAgentResults(1: UpdateSessionAgentResultsReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/agent-results")
    UpdateSessionDiscussionResp UpdateSessionDiscussion(1: UpdateSessionDiscussionReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/discussion")
    UpdateSessionFinalReportResp UpdateSessionFinalReport(1: UpdateSessionFinalReportReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/final-report")
    CompleteSessionResp CompleteSession(1: CompleteSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/complete")
    FailSessionResp FailSession(1: FailSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/fail")
    CancelSessionResp CancelSession(1: CancelSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/cancel")
}
