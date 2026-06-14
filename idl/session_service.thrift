include "session.thrift"

service sessionService {
    session.ListSessionsResp ListSessions(1: session.ListSessionsReq req) (api.get="/api/v1/sessions")
    session.GetSessionsResp GetSessions(1: session.GetSessionsReq req) (api.get="/api/v1/reviews/:id/sessions")
    session.CreateSessionResp CreateSession(1: session.CreateSessionReq req) (api.post="/api/v1/reviews/:id/sessions")
    session.GetActiveSessionResp GetActiveSession(1: session.GetActiveSessionReq req) (api.get="/api/v1/reviews/:id/sessions/active")
    session.GetSessionResp GetSession(1: session.GetSessionReq req) (api.get="/api/v1/reviews/:id/sessions/:sessionId")
    session.UpdateSessionPhaseResp UpdateSessionPhase(1: session.UpdateSessionPhaseReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/phase")
    session.UpdateSessionStatusResp UpdateSessionStatus(1: session.UpdateSessionStatusReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/status")
    session.UpdateSessionAgentResultsResp UpdateSessionAgentResults(1: session.UpdateSessionAgentResultsReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/agent-results")
    session.UpdateSessionDiscussionResp UpdateSessionDiscussion(1: session.UpdateSessionDiscussionReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/discussion")
    session.UpdateSessionFinalReportResp UpdateSessionFinalReport(1: session.UpdateSessionFinalReportReq req) (api.put="/api/v1/reviews/:id/sessions/:sessionId/final-report")
    session.CompleteSessionResp CompleteSession(1: session.CompleteSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/complete")
    session.FailSessionResp FailSession(1: session.FailSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/fail")
    session.CancelSessionResp CancelSession(1: session.CancelSessionReq req) (api.post="/api/v1/reviews/:id/sessions/:sessionId/cancel")
}
