struct GetSessionsReq {
    1: string id (api.path="id")
}

struct GetSessionsResp {
    1: string sessions (api.body="sessions")
}

struct CreateSessionReq {
    1: string id (api.path="id")
}

struct CreateSessionResp {
    1: string session (api.body="session")
}

struct GetActiveSessionReq {
    1: string id (api.path="id")
}

struct GetActiveSessionResp {
    1: string session (api.body="session")
}

struct GetSessionReq {
    1: string id (api.path="id")
    2: string session_id (api.path="sessionId")
}

struct GetSessionResp {
    1: string session (api.body="session")
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
    1: string sessions (api.body="sessions")
    2: i64 total (api.body="total")
    3: i64 page (api.body="page")
    4: i64 page_size (api.body="page_size")
    5: i64 total_pages (api.body="total_pages")
}
