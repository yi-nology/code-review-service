struct PhaseInfo {
    1: string name (api.body="name")
    2: string title (api.body="title")
    3: string description (api.body="description")
    4: string status (api.body="status")
    5: double progress (api.body="progress")
    6: optional string start_time (api.body="start_time")
    7: optional string end_time (api.body="end_time")
    8: optional i64 duration_ms (api.body="duration_ms")
}

struct AgentStatus {
    1: string name (api.body="name")
    2: string status (api.body="status")
    3: double progress (api.body="progress")
    4: string message (api.body="message")
    5: optional i64 duration_ms (api.body="duration_ms")
    6: optional string error (api.body="error")
}

struct ReviewProgress {
    1: string review_id (api.body="review_id")
    2: string session_id (api.body="session_id")
    3: i64 round (api.body="round")
    4: string phase (api.body="phase")
    5: string status (api.body="status")
    6: double progress (api.body="progress")
    7: list<PhaseInfo> phases (api.body="phases")
    8: optional map<string, AgentStatus> agent_status (api.body="agent_status")
    9: string start_time (api.body="start_time")
    10: string updated_at (api.body="updated_at")
    11: optional string error (api.body="error")
}

struct ActiveReviewItem {
    1: string review_id (api.body="review_id")
    2: string session_id (api.body="session_id")
    3: i64 round (api.body="round")
    4: string phase (api.body="phase")
    5: string status (api.body="status")
    6: double progress (api.body="progress")
    7: string start_time (api.body="start_time")
    8: string updated_at (api.body="updated_at")
}

struct HandleProgressWSReq {
    1: string id (api.path="id")
}

struct HandleProgressWSResp {}

struct GetProgressReq {
    1: string id (api.path="id")
}

struct GetProgressResp {
    1: ReviewProgress progress (api.body="progress")
}

struct GetActiveReviewsReq {}

struct GetActiveReviewsResp {
    1: list<ActiveReviewItem> reviews (api.body="reviews")
    2: i64 total (api.body="total")
}
